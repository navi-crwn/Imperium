package app

import (
	"fmt"
	"html/template"
	"net/http"
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

type uiInput struct {
	Name, Label string
	Required    bool
}

// uiAction is one entry of an Actions menu. Every entry opens a confirmation dialog that asks for a reason (required),
// which also works without JavaScript (the menu links to #id and CSS shows the dialog). A destructive one is red and
// the server refuses it unless the dialog sent ack=yes; one that must be typed or needs a value asks for it there.
type uiAction struct {
	Label, Action, URL, ID, Target string
	Typed                          string
	Inputs                         []uiInput
	Fields                         [][2]string
	Danger                         bool
}

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
	New          string // a primary button next to the title, for example "New announcement"
	NewLabel     string
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
	Edit                   string // a secondary link next to the Actions menu
	EditLabel              string
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

// extDestructive are the extension actions that cannot be taken back (ban an address, remove a domain, release a
// reserved name); they too are confirmed in a dialog and refused without its ack.
func extDestructive(action string) bool {
	switch action {
	case "release", "ban", "remove":
		return true
	}
	return false
}

func needExtAck(action, ack string) bool { return extDestructive(action) && ack != "yes" }

// dlgID is the id of the dialog of one entry. The row index keeps two rows apart even when their own ids
// differ only in characters that idSafe replaces.
func dlgID(n int, parts ...string) string {
	return fmt.Sprintf("dlg-%d-%s", n, idSafe(strings.Join(parts, "-")))
}

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
	activeStates    = map[string]bool{"active": true, "live": true, "published": true, "visible": true, "enabled": true, "open": true, "public": true, "ok": true, "approved": true}
)

// pairedActions keeps one half of every pair: the positive one (Show, Enable, Publish) when the item is in an
// inactive state, the negative one (Hide, Disable, Unpublish) when it is in an active one. A status that is
// neither is not guessed: both halves are offered, so no action is ever out of reach.
func pairedActions(actions []string, status string, flags []string) []string {
	if !inactiveStates[status] && !activeStates[status] {
		return actions
	}
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
func (a *App) itemActions(l, slug, typ, id, target string, n int, actions []string, status string, flags []string, withParams bool) []uiAction {
	var normal, last []uiAction
	for _, act := range pairedActions(actions, status, flags) {
		ua := uiAction{Label: actionLabel(l, act), Action: act, Target: target,
			URL: "/apps/" + slug + "/content/" + url.PathEscape(typ) + "/" + url.PathEscape(id) + "/actions", ID: dlgID(n, typ, id, act), Danger: destructiveAction(act)}
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

// refuseNoReason answers a form that came without a reason (empty after trimming): a flash and a redirect, like
// a missing confirmation, and nothing is sent or logged. It reports whether it refused.
func (a *App) refuseNoReason(w http.ResponseWriter, r *http.Request, back, reason string) bool {
	if strings.TrimSpace(reason) != "" {
		return false
	}
	a.flashError(w, a.Tr(r, "err.need_reason"))
	http.Redirect(w, r, back, http.StatusFound)
	return true
}

// extPlainActions are the actions of an extension that run from the plain form; the irreversible ones
// (ban, remove, release) have a button and a dialog of their own.
func extPlainActions(all []string) []string {
	var out []string
	for _, a := range all {
		if !extDestructive(a) {
			out = append(out, a)
		}
	}
	return out
}

// extDialogs are the dialogs of the irreversible actions of one extension: each asks for the value it acts on
// and for a reason, and sends ack=yes.
func (a *App) extDialogs(l, slug, ext string) []uiAction {
	value := map[string]uiInput{
		"ban":     {Name: "p_ip", Label: tl(l, "lbl.ip"), Required: true},
		"remove":  {Name: "p_domain", Label: tl(l, "lbl.domain"), Required: true},
		"release": {Name: "path", Label: tl(l, "ext.sub"), Required: true},
	}
	var out []uiAction
	for _, act := range extActions[ext] {
		if in, ok := value[act]; ok && extDestructive(act) {
			out = append(out, uiAction{Label: actionLabel(l, act), Action: act, URL: "/apps/" + slug + "/x/" + url.PathEscape(ext) + "/actions",
				ID: dlgID(0, "x", ext, act), Target: extLabel(l, ext), Inputs: []uiInput{in}, Danger: true})
		}
	}
	return out
}
