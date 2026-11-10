package app

import (
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

// The shared page models. One list template, one detail template, one settings template and one
// dashboard template render every app and Users; a handler only fills the model.

type uiCell struct {
	HTML  template.HTML
	Class string
}

type uiCol struct{ Label, Class string }

type uiInput struct{ Name, Label string }

// uiAction is one entry of an Actions menu. A plain action is a one-click form of its own. A destructive
// one, one that must be typed, and one that needs a value open a dialog, which also works without JavaScript
// (the menu links to #id and CSS shows the dialog); the server refuses a destructive action unless the dialog
// sent ack=yes.
type uiAction struct {
	Label, Action, URL, ID, Target string
	Typed                          string
	Inputs                         []uiInput
	Fields                         [][2]string
	Danger                         bool
}

func (u uiAction) Dialog() bool { return u.Danger || u.Typed != "" || len(u.Inputs) > 0 }

type uiRow struct {
	Pick, PickLabel string
	Cells           []uiCell
	Actions         []uiAction
}

type uiFilter struct {
	Kind, Name, Label, Value, Class string
	Options                         []string
}

type uiChip struct{ Label, Class string }

type listPage struct {
	Title, Count string
	Subnav       []navItem
	Path         string
	Hidden       [][2]string
	Filters      []uiFilter
	Err          string
	BulkURL      string
	BulkActions  [][2]string // action, label
	BulkHidden   [][2]string
	Cols         []uiCol
	Rows         []uiRow
	Empty        string
	First, Next  string
	Back         string
	HasMenu      bool
}

type uiKV struct {
	K string
	V template.HTML
}

type uiTable struct {
	Cols []string
	Rows [][]template.HTML
}

type uiSection struct {
	Title string
	KV    []uiKV
	Table *uiTable
	HTML  template.HTML
	Empty string
}

type detailPage struct {
	Title, Back, BackLabel string
	Chips                  []uiChip
	Link                   string
	Primary                *uiAction
	Actions                []uiAction
	Tabs                   []navItem
	Sections               []uiSection
	Err                    string
	Dialogs                []uiAction
}

// destructiveAction is an action that cannot be taken back; it lives at the bottom of a menu and is
// confirmed in a dialog, and the server refuses it without that confirmation.
func destructiveAction(action string) bool {
	switch action {
	case "delete", "purge", "takedown", "ban_ip", "clear_custom_css", "release_slug":
		return true
	}
	return false
}

// needAck refuses a destructive action that did not come through the confirmation dialog.
func needAck(action, ack string) bool { return destructiveAction(action) && ack != "yes" }

func esc(s string) string { return template.HTMLEscapeString(s) }

func hLink(href, text string) template.HTML {
	return template.HTML(`<a href="` + esc(href) + `">` + esc(text) + `</a>`)
}

func hChip(class, text string) string {
	return `<span class="chip ` + esc(class) + `">` + esc(text) + `</span>`
}

func idSafe(s string) string {
	return regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(s, "-")
}

// pairs: the two halves of a switch; a menu offers the half that fits the item's state.
var (
	positiveActions = map[string]string{"unhide": "hide", "enable": "disable", "publish": "unpublish", "activate": "suspend", "restore": "confirm", "unmark_nsfw": "mark_nsfw", "unmark_sensitive": "mark_sensitive"}
	inactiveStates  = map[string]bool{"hidden": true, "disabled": true, "unpublished": true, "draft": true, "suspended": true, "takedown": true, "removed": true}
)

// pairedActions keeps one half of every pair: the positive one (Show, Enable, Publish) when the item is in an
// inactive state, the negative one (Hide, Disable, Unpublish) otherwise.
func pairedActions(actions []string, status string, flags []string) []string {
	have := map[string]bool{}
	for _, a := range actions {
		have[a] = true
	}
	inactive := inactiveStates[status]
	flagged := func(a string) bool {
		for _, f := range flags {
			if strings.Contains(strings.ToLower(f), strings.TrimPrefix(strings.TrimPrefix(a, "mark_"), "unmark_")) {
				return true
			}
		}
		return false
	}
	var out []string
	for _, a := range actions {
		if neg, ok := positiveActions[a]; ok && have[neg] { // a positive half
			if strings.HasPrefix(a, "unmark_") {
				if flagged(a) {
					out = append(out, a)
				}
			} else if inactive {
				out = append(out, a)
			}
			continue
		}
		for pos, neg := range positiveActions { // a negative half
			if neg == a && have[pos] {
				if strings.HasPrefix(a, "mark_") {
					if !flagged(pos) {
						out = append(out, a)
					}
				} else if !inactive {
					out = append(out, a)
				}
				goto next
			}
		}
		out = append(out, a)
	next:
	}
	return out
}

var paramInputs = map[string][]string{"transfer": {"p_to", "item.to"}, "set_expiry": {"p_expires", "item.expires"}}

// itemActions builds the menu of one content item: paired actions collapse to one, the ones that need a
// value ask for it in a dialog, irreversible ones go last.
func (a *App) itemActions(l, slug, typ, id, target string, actions []string, status string, flags []string, withParams bool) []uiAction {
	var normal, last []uiAction
	for _, act := range pairedActions(actions, status, flags) {
		ua := uiAction{Label: actionLabel(l, act), Action: act, Target: target,
			URL: "/apps/" + slug + "/content/" + url.PathEscape(typ) + "/" + url.PathEscape(id) + "/actions", ID: "dlg-" + idSafe(typ+"-"+id+"-"+act), Danger: destructiveAction(act)}
		if p, ok := paramInputs[act]; ok {
			if !withParams {
				continue
			}
			ua.Inputs = []uiInput{{Name: p[0], Label: tl(l, p[1])}}
		}
		if ua.Danger {
			last = append(last, ua)
		} else {
			normal = append(normal, ua)
		}
	}
	return append(normal, last...)
}

func (a *App) bulkChoices(l string, actions []string) [][2]string {
	var out [][2]string
	for _, act := range actions {
		out = append(out, [2]string{act, actionLabel(l, act)})
	}
	return out
}

func fmtCount(n int64) string {
	if n <= 0 {
		return ""
	}
	return humanInt(n)
}

func plural(l, key string, n int) string { return fmt.Sprintf(tl(l, key), n) }
