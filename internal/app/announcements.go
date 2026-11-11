package app

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

// Announcements are the notices HOP ID shows on hel.ink, in the launcher and on /announcements. HOP ID keeps
// them; Imperium works on them through the admin API (contract: docs/announcements-contract.md). Everyone with a
// hub-wide role may read them, owners and admins may change them, and every change asks for a reason and is
// written to the audit trail like any other action.

const (
	annTitleMax  = 160
	annBodyMax   = 5000
	annReasonMax = 255
)

var annLevels = []string{"info", "warning", "critical"}

// annStates are the states a list filter offers.
var annStates = []string{"draft", "scheduled", "live", "expired"}

// announcement mirrors one row of HOP ID's answer.
type announcement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Level       string `json:"level"`
	AppSlug     string `json:"app_slug"`
	State       string `json:"state"`
	PublishedAt string `json:"published_at"`
	ExpiresAt   string `json:"expires_at"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (n announcement) state() string {
	if n.State != "" {
		return n.State
	}
	switch p, e := parseTime(n.PublishedAt), parseTime(n.ExpiresAt); {
	case n.PublishedAt == "":
		return "draft"
	case p.After(time.Now()):
		return "scheduled"
	case n.ExpiresAt != "" && !e.After(time.Now()):
		return "expired"
	}
	return "live"
}

func annCanWrite(s *db.Session) bool { return s != nil && isAdmin(s) }

// annApps are the apps a notice may be about: every registered app except Imperium itself.
func (a *App) annApps(r *http.Request) []string {
	apps, _ := a.db.Apps(r.Context())
	var out []string
	for _, ap := range apps {
		if ap.Slug != "" && ap.Slug != "imperium" && !ap.Hidden && !contains(out, ap.Slug) {
			out = append(out, ap.Slug)
		}
	}
	return out
}

// annShell is what the pages of the hub section share: the hub's header and tabs.
func (a *App) annShell(r *http.Request, data map[string]any) map[string]any {
	if ap, ok := a.source(r.Context(), "hub"); ok && ap != nil {
		data["App"] = ap
	}
	return data
}

func levelChip(l, level string) string { return hChip("ann-"+level, tl(l, "ann.level."+level)) }
func stateChip(l, state string) string { return hChip("ann-"+state, tl(l, "ann.state."+state)) }

func (a *App) annList(w http.ResponseWriter, r *http.Request) {
	if !a.requireGlobal(w, r) {
		return
	}
	l, s, q := a.langOf(r), current(r), r.URL.Query()
	state, level := q.Get("state"), q.Get("level")
	if !contains(annStates, state) {
		state = ""
	}
	if !contains(annLevels, level) {
		level = ""
	}
	path := "/announcements?" + url.Values{"state": {state}, "level": {level}, "cursor": {q.Get("cursor")}, "limit": {"25"}}.Encode()
	var rows []announcement
	env, err := a.callHop(r.Context(), s, "GET", path, nil, &rows)
	p := listPage{Title: tl(l, "ann.title"), Path: "/announcements", Empty: tl(l, "ann.none"), HasMenu: annCanWrite(s)}
	if annCanWrite(s) {
		p.New, p.NewLabel = "/announcements/new", tl(l, "ann.new")
	}
	if err != nil {
		p.Err = tl(l, "err.hop") + ": " + err.Error()
	} else if env.Page != nil {
		if env.Page.Total > 0 {
			p.Count = humanInt(int64(env.Page.Total))
		}
		if env.Page.Next != "" {
			p.Next = cursorURL("/announcements", q, env.Page.Next)
		}
	}
	if q.Get("cursor") != "" {
		p.First = pageURL("/announcements", q, "cursor")
	}
	p.Filters = []uiFilter{
		{Kind: "select", Name: "state", Label: tl(l, "ann.col.state"), Value: state, Options: annStates},
		{Kind: "select", Name: "level", Label: tl(l, "ann.col.level"), Value: level, Options: annLevels},
	}
	p.Cols = []uiCol{{Label: tl(l, "ann.col.title")}, {Label: tl(l, "ann.col.state")}, {Label: tl(l, "ann.col.level")}, {Label: tl(l, "ann.col.app")}, {Label: tl(l, "ann.col.published")}, {Label: tl(l, "ann.col.expires")}}
	for _, n := range rows {
		st := n.state()
		row := uiRow{Cells: []uiCell{
			{hLink("/announcements/"+url.PathEscape(n.ID), n.Title), "ann-title"},
			{template.HTML(stateChip(l, st)), "nw"}, {template.HTML(levelChip(l, n.Level)), "nw"},
			{txt(annAppLabel(l, n.AppSlug)), "nw"}, {datesz(l, n.PublishedAt), "small muted nw"}, {datesz(l, n.ExpiresAt), "small muted nw"}}}
		if annCanWrite(s) {
			n0 := len(p.Rows)
			row.Actions = a.annActions(l, n, n0)
		}
		p.Rows = append(p.Rows, row)
	}
	data := a.annShell(r, map[string]any{"P": p})
	a.render(w, r, "list.html", p.Title, data)
}

func annAppLabel(l, slug string) string {
	if slug == "" {
		return tl(l, "ann.app_all")
	}
	return slug
}

// annActions is the Actions menu of one announcement: the half of publish/unpublish that fits its state, and
// delete last. The edit page is a link, so it is not in the menu.
func (a *App) annActions(l string, n announcement, i int) []uiAction {
	base := "/announcements/" + url.PathEscape(n.ID) + "/actions"
	act := func(name string) uiAction {
		return uiAction{Label: actionLabel(l, name), Action: name, URL: base, ID: dlgID(i, "ann", n.ID, name), Target: n.Title, Danger: name == "delete"}
	}
	var out []uiAction
	switch n.state() {
	case "draft", "scheduled":
		out = append(out, act("publish"))
		if n.state() == "scheduled" {
			out = append(out, act("unpublish"))
		}
	case "live":
		out = append(out, act("unpublish"))
	case "expired":
		out = append(out, act("unpublish"))
	}
	return append(out, act("delete"))
}

func (a *App) annShow(w http.ResponseWriter, r *http.Request) {
	if !a.requireGlobal(w, r) {
		return
	}
	l, s, id := a.langOf(r), current(r), r.PathValue("id")
	var n announcement
	if _, err := a.callHop(r.Context(), s, "GET", "/announcements/"+url.PathEscape(id), nil, &n); err != nil {
		a.render(w, r, "error.html", tl(l, "ann.title"), map[string]any{"Err": err.Error(), "Back": "/announcements"})
		return
	}
	d := detailPage{Title: n.Title, Back: "/announcements", BackLabel: tl(l, "ann.title")}
	d.Chips = []uiChip{{tl(l, "ann.state."+n.state()), "ann-" + n.state()}, {tl(l, "ann.level."+n.Level), "ann-" + n.Level}}
	if annCanWrite(s) {
		d.Edit, d.EditLabel = "/announcements/"+url.PathEscape(n.ID)+"/edit", tl(l, "ann.edit")
		d.Actions = a.annActions(l, n, 0)
	}
	kv := []uiKV{{tl(l, "ann.col.app"), txt(annAppLabel(l, n.AppSlug))}, {tl(l, "ann.col.published"), datesz(l, n.PublishedAt)}, {tl(l, "ann.col.expires"), datesz(l, n.ExpiresAt)}}
	if n.Author != "" {
		kv = append(kv, uiKV{tl(l, "ann.author"), txt(n.Author)})
	}
	if n.CreatedAt != "" {
		kv = append(kv, uiKV{tl(l, "lbl.created_at"), datesz(l, n.CreatedAt)})
	}
	d.Sections = []uiSection{{KV: kv}, {Title: tl(l, "ann.f.body"), HTML: template.HTML(`<p class="pre-wrap">` + esc(n.Body) + `</p>`)}}
	a.render(w, r, "detail.html", n.Title, a.annShell(r, map[string]any{"P": d}))
}

// annForm is the create and edit form: what the person typed, and what was wrong with it.
type annForm struct {
	ID, Heading, Action, Back    string
	Title, Body, Level, App      string
	AppWas                       string
	Published, Expires, Reason   string
	PublishNow                   bool
	Err                          string
	Errs                         map[string]string
	LevelOpts, AppOpts           []annOpt
	TitleMax, BodyMax, ReasonMax int
}

type annOpt struct {
	Value, Label string
	On           bool
}

const localLayout = "2006-01-02T15:04"

// inputTime turns a stored time into the value of a datetime-local input (UTC).
func inputTime(v string) string {
	if v == "" {
		return ""
	}
	if t := parseTime(v); !t.IsZero() {
		return t.UTC().Format(localLayout)
	}
	return ""
}

func (a *App) annFormPage(r *http.Request, f *annForm) map[string]any {
	l := a.langOf(r)
	f.TitleMax, f.BodyMax, f.ReasonMax = annTitleMax, annBodyMax, annReasonMax
	if f.Level == "" {
		f.Level = "info"
	}
	f.LevelOpts, f.AppOpts = nil, []annOpt{{"", tl(l, "ann.app_all"), f.App == ""}}
	for _, lv := range annLevels {
		f.LevelOpts = append(f.LevelOpts, annOpt{lv, tl(l, "ann.level."+lv), f.Level == lv})
	}
	apps := a.annApps(r)
	if f.App != "" && !contains(apps, f.App) {
		apps = append(apps, f.App) // an app the registry no longer lists keeps its notice editable
	}
	for _, ap := range apps {
		f.AppOpts = append(f.AppOpts, annOpt{ap, ap, f.App == ap})
	}
	return a.annShell(r, map[string]any{"F": f})
}

func (a *App) annNew(w http.ResponseWriter, r *http.Request) {
	if !a.annGate(w, r) {
		return
	}
	l := a.langOf(r)
	f := &annForm{Heading: tl(l, "ann.new"), Action: "/announcements", Back: "/announcements", Level: "info"}
	a.render(w, r, "announce_form.html", f.Heading, a.annFormPage(r, f))
}

// annGate answers the pages and posts only an owner or admin may use.
func (a *App) annGate(w http.ResponseWriter, r *http.Request) bool {
	if annCanWrite(current(r)) {
		return true
	}
	a.forbidden(w, r)
	return false
}

func (a *App) annEdit(w http.ResponseWriter, r *http.Request) {
	if !a.annGate(w, r) {
		return
	}
	l, id := a.langOf(r), r.PathValue("id")
	var n announcement
	if _, err := a.callHop(r.Context(), current(r), "GET", "/announcements/"+url.PathEscape(id), nil, &n); err != nil {
		a.render(w, r, "error.html", tl(l, "ann.title"), map[string]any{"Err": err.Error(), "Back": "/announcements"})
		return
	}
	f := &annForm{ID: n.ID, Heading: tl(l, "ann.edit"), Action: "/announcements/" + url.PathEscape(n.ID), Back: "/announcements/" + url.PathEscape(n.ID),
		Title: n.Title, Body: n.Body, Level: n.Level, App: n.AppSlug, AppWas: n.AppSlug, Published: inputTime(n.PublishedAt), Expires: inputTime(n.ExpiresAt)}
	a.render(w, r, "announce_form.html", f.Heading, a.annFormPage(r, f))
}

// annRead reads the form and checks it with the rules HOP ID applies, so a mistake is shown beside its field
// and nothing is sent. The body for HOP ID is built only when everything is fine.
func (a *App) annRead(r *http.Request, f *annForm) (map[string]any, bool) {
	l := a.langOf(r)
	_ = r.ParseForm()
	f.Title, f.Body = strings.TrimSpace(r.FormValue("title")), strings.TrimSpace(r.FormValue("body"))
	f.Level, f.App, f.AppWas = r.FormValue("level"), r.FormValue("app_slug"), r.FormValue("app_was")
	f.Published, f.Expires = strings.TrimSpace(r.FormValue("published_at")), strings.TrimSpace(r.FormValue("expires_at"))
	f.PublishNow = r.FormValue("publish_now") != ""
	f.Reason = strings.TrimSpace(r.FormValue("reason"))
	f.Errs = map[string]string{}
	switch n := utf8.RuneCountInString(f.Title); {
	case n == 0:
		f.Errs["title"] = tl(l, "ann.err.title")
	case n > annTitleMax:
		f.Errs["title"] = fmt.Sprintf(tl(l, "ann.err.title_long"), annTitleMax)
	}
	switch n := utf8.RuneCountInString(f.Body); {
	case n == 0:
		f.Errs["body"] = tl(l, "ann.err.body")
	case n > annBodyMax:
		f.Errs["body"] = fmt.Sprintf(tl(l, "ann.err.body_long"), annBodyMax)
	}
	if !contains(annLevels, f.Level) {
		f.Errs["level"] = tl(l, "ann.err.level")
	}
	apps := a.annApps(r)
	if f.App != "" && !contains(apps, f.App) && f.App != f.AppWas {
		f.Errs["app_slug"] = tl(l, "ann.err.app")
	}
	pub, pubErr := inputToUTC(f.Published)
	exp, expErr := inputToUTC(f.Expires)
	if pubErr != nil {
		f.Errs["published_at"] = tl(l, "ann.err.date")
	}
	if expErr != nil {
		f.Errs["expires_at"] = tl(l, "ann.err.date")
	}
	if pubErr == nil && expErr == nil && f.Expires != "" && !f.PublishNow && (f.Published == "" || !exp.After(pub)) {
		f.Errs["expires_at"] = tl(l, "ann.err.order")
	}
	if f.Reason == "" {
		f.Errs["reason"] = tl(l, "err.need_reason")
	} else if utf8.RuneCountInString(f.Reason) > annReasonMax {
		f.Errs["reason"] = fmt.Sprintf(tl(l, "ann.err.reason_long"), annReasonMax)
	}
	if len(f.Errs) > 0 {
		return nil, false
	}
	body := map[string]any{"title": f.Title, "body": f.Body, "level": f.Level, "app_slug": nil, "published_at": nil, "expires_at": nil, "publish_now": f.PublishNow, "reason": f.Reason}
	if f.App != "" {
		body["app_slug"] = f.App
	}
	if f.Published != "" {
		body["published_at"] = pub.Format(time.RFC3339)
	}
	if f.Expires != "" {
		body["expires_at"] = exp.Format(time.RFC3339)
	}
	return body, true
}

// inputToUTC reads a datetime-local value (UTC, with or without seconds); empty is no time.
func inputToUTC(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{localLayout, "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, v, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("bad time")
}

func (a *App) annAudit(r *http.Request, action, id string, detail map[string]any, err error) {
	s := current(r)
	detail["error"] = errString(err)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: "hub", Action: "announcement." + action, Target: id, Detail: detail, OK: err == nil})
}

// annSave handles both the create form (no id) and the edit form.
func (a *App) annSave(w http.ResponseWriter, r *http.Request) {
	if !a.annGate(w, r) {
		return
	}
	l, s, id := a.langOf(r), current(r), r.PathValue("id")
	f := &annForm{ID: id, Action: "/announcements", Back: "/announcements", Heading: tl(l, "ann.new")}
	if id != "" {
		f.Action, f.Back, f.Heading = "/announcements/"+url.PathEscape(id), "/announcements/"+url.PathEscape(id), tl(l, "ann.edit")
	}
	body, ok := a.annRead(r, f)
	if !ok {
		a.renderStatus(w, r, http.StatusUnprocessableEntity, "announce_form.html", f.Heading, a.annFormPage(r, f))
		return
	}
	verb, method, path := "create", "POST", "/announcements"
	if id != "" {
		verb, method, path = "update", "PUT", "/announcements/"+url.PathEscape(id)
	}
	var saved announcement
	_, err := a.callHop(r.Context(), s, method, path, body, &saved)
	if err == nil && id == "" {
		id = saved.ID
	}
	detail := map[string]any{"reason": f.Reason, "title": f.Title, "level": f.Level, "app": f.App, "published_at": body["published_at"], "expires_at": body["expires_at"], "publish_now": f.PublishNow}
	a.annAudit(r, verb, id, detail, err)
	if err != nil {
		f.Err = tl(l, "action.failed") + ": " + err.Error()
		a.renderStatus(w, r, hubFormStatus(err), "announce_form.html", f.Heading, a.annFormPage(r, f))
		return
	}
	a.flash(w, a.Tr(r, "ann."+verb+"d"))
	http.Redirect(w, r, "/announcements/"+url.PathEscape(id), http.StatusFound)
}

// hubFormStatus is the status a form is shown with when HOP ID refused it: its own for a refusal of the input, 502 otherwise.
func hubFormStatus(err error) int {
	var ae *hopproto.APIError
	if errors.As(err, &ae) && ae.Status == http.StatusUnprocessableEntity {
		return http.StatusUnprocessableEntity
	}
	return http.StatusBadGateway
}

// annAction is publish, unpublish or delete from a dialog: a reason is required, delete also needs its ack.
func (a *App) annAction(w http.ResponseWriter, r *http.Request) {
	if !a.annGate(w, r) {
		return
	}
	s, id := current(r), r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", 400)
		return
	}
	action, reason := r.FormValue("action"), strings.TrimSpace(r.FormValue("reason"))
	if !contains([]string{"publish", "unpublish", "delete"}, action) {
		http.Error(w, "unknown action", 400)
		return
	}
	back := "/announcements/" + url.PathEscape(id)
	if needAck(action, r.FormValue("ack")) {
		a.flashError(w, a.Tr(r, "err.need_ack"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	if a.refuseNoReason(w, r, back, reason) {
		return
	}
	if utf8.RuneCountInString(reason) > annReasonMax {
		reason = string([]rune(reason)[:annReasonMax])
	}
	_, err := a.callHop(r.Context(), s, "POST", "/announcements/"+url.PathEscape(id)+"/actions", map[string]any{"action": action, "reason": reason}, nil)
	a.annAudit(r, action, id, map[string]any{"reason": reason}, err)
	if err != nil {
		a.flashError(w, a.Tr(r, "action.failed")+": "+err.Error())
	} else {
		a.flash(w, a.Tr(r, "action.done"))
	}
	if action == "delete" && err == nil {
		http.Redirect(w, r, "/announcements", http.StatusFound)
		return
	}
	http.Redirect(w, r, returnTo(r, back), http.StatusFound)
}
