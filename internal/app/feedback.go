package app

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

// Feedback is the inbox of what people send from the form on hel.ink. HOP ID keeps the reports and their pictures;
// Imperium reads and changes them through the signed API (/api/hop/feedback) and stores nothing of them. Hub-wide
// roles see every app, admin:<app> only that app. The sender's IP address is for owners and admins alone: it is
// not put into the page for anybody else.

var (
	fbKinds    = []string{"problem", "idea", "other"}
	fbStatuses = []string{"new", "read", "done"}
	fbApps     = []string{"hub", "linkhop", "biohop", "pixelhop", "notehop", "filehop", "payhop"}
	fbAppName  = map[string]string{"hub": "HOP ID", "linkhop": "LinkHop", "biohop": "BioHop", "pixelhop": "PixelHop", "notehop": "NoteHop", "filehop": "FileHop", "payhop": "PayHop"}
	fbCursorRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

const (
	fbPageSize     = 25
	fbShotMax      = 10 << 20
	fbBadgeTTL     = 60 * time.Second
	fbBadgeWait    = 1500 * time.Millisecond
	fbBadgeRefresh = 5 * time.Second
	fbRetryAfter   = 15 * time.Second
)

// fbRow is one line of the list; fbReport is one report with everything the hub sends.
type fbRow struct {
	ID            string  `json:"id"`
	Kind          string  `json:"kind"`
	Status        string  `json:"status"`
	App           string  `json:"app"`
	Page          string  `json:"page"`
	Excerpt       string  `json:"excerpt"`
	HasScreenshot bool    `json:"has_screenshot"`
	Username      *string `json:"username"`
	CreatedAt     string  `json:"created_at"`
}

type fbReport struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	App        string `json:"app"`
	Page       string `json:"page"`
	AppVersion string `json:"app_version"`
	Message    string `json:"message"`
	User       *struct {
		Username string `json:"username"`
	} `json:"user"`
	IP            *string `json:"ip"`
	UserAgent     string  `json:"user_agent"`
	Viewport      string  `json:"viewport"`
	Locale        string  `json:"locale"`
	HasScreenshot bool    `json:"has_screenshot"`
	CreatedAt     string  `json:"created_at"`
	ReadAt        string  `json:"read_at"`
	DoneAt        string  `json:"done_at"`
}

// ---- who may do what ----

// fbScopeApps are the apps an admin:<app> role names (only apps that can send feedback).
func fbScopeApps(s *db.Session) []string {
	var out []string
	if s == nil {
		return out
	}
	for _, app := range fbApps {
		if contains(s.Roles, "admin:"+app) {
			out = append(out, app)
		}
	}
	return out
}

// fbAllowed: a hub-wide role, or the admin of one app.
func fbAllowed(s *db.Session) bool { return globalRole(s) || len(fbScopeApps(s)) > 0 }

// fbSees says whether a report of this app may be shown to the session.
func fbSees(s *db.Session, app string) bool {
	return globalRole(s) || (s != nil && contains(s.Roles, "admin:"+app))
}

// fbMayAct: owner, admin, moderator and any admin:<app> change reports; support only reads.
func fbMayAct(s *db.Session) bool {
	return s != nil && (isAdmin(s) || contains(s.Roles, "moderator") || len(fbScopeApps(s)) > 0)
}

// fbCanAct is fbMayAct for one app.
func fbCanAct(s *db.Session, app string) bool { return s != nil && canActIn(s, app) }

// fbCanDelete and fbShowIP: owners and admins only.
func fbCanDelete(s *db.Session) bool { return s != nil && isAdmin(s) }
func fbShowIP(s *db.Session) bool    { return s != nil && isAdmin(s) }

// ---- the count on the menu ----

// fbBadges keeps the number of new reports for a minute. A dead or slow hub must not slow the other pages: the
// first count waits a short while, every later one is taken from here and refreshed in the background.
type fbBadges struct {
	mu sync.Mutex
	m  map[string]*fbBadge
}

type fbBadge struct {
	n    int
	at   time.Time
	busy bool
}

func fbScopeKey(s *db.Session) string {
	if globalRole(s) {
		return "*"
	}
	return strings.Join(fbScopeApps(s), ",")
}

func (a *App) fbClear() {
	a.fb.mu.Lock()
	a.fb.m = nil
	a.fb.mu.Unlock()
}

// fbCount asks HOP ID for the number of new reports of the apps the session sees.
func (a *App) fbCount(ctx context.Context, s *db.Session) (int, error) {
	apps := []string{""}
	if !globalRole(s) {
		apps = fbScopeApps(s)
	}
	total := 0
	for _, app := range apps {
		v := url.Values{"status": {"new"}, "limit": {"1"}}
		if app != "" {
			v.Set("app", app)
		}
		env, err := a.callHop(ctx, s, "GET", "/feedback?"+v.Encode(), nil, nil)
		if err != nil {
			return 0, err
		}
		if env.Page != nil {
			total += env.Page.Total
		}
	}
	return total, nil
}

// feedbackBadge is the number shown next to Feedback in the menu; 0 (no badge) when the hub cannot answer.
func (a *App) feedbackBadge(r *http.Request) int {
	s := current(r)
	if !fbAllowed(s) {
		return 0
	}
	key := fbScopeKey(s)
	a.fb.mu.Lock()
	if a.fb.m == nil {
		a.fb.m = map[string]*fbBadge{}
	}
	e := a.fb.m[key]
	switch {
	case e == nil:
		e = &fbBadge{busy: true}
		a.fb.m[key] = e
		a.fb.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), fbBadgeWait)
		defer cancel()
		n, err := a.fbCount(ctx, s)
		a.fb.mu.Lock()
		defer a.fb.mu.Unlock()
		if a.fb.m[key] == e {
			e.n, e.at, e.busy = n, time.Now(), false
			if err != nil {
				e.at = time.Now().Add(fbRetryAfter - fbBadgeTTL) // try again soon
			}
		}
		return n
	case e.busy || time.Since(e.at) < fbBadgeTTL:
		n := e.n
		a.fb.mu.Unlock()
		return n
	}
	e.busy = true
	n := e.n
	a.fb.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), fbBadgeRefresh)
		defer cancel()
		c, err := a.fbCount(ctx, s)
		a.fb.mu.Lock()
		defer a.fb.mu.Unlock()
		if a.fb.m[key] != e {
			return
		}
		e.n, e.at, e.busy = c, time.Now(), false
		if err != nil {
			e.n, e.at = 0, time.Now().Add(fbRetryAfter-fbBadgeTTL)
		}
	}()
	return n
}

// ---- helpers ----

func fbKindChip(l, kind string) string { return hChip("fb-"+kind, tl(l, "fb.kind."+kind)) }
func fbStatusChip(l, st string) string { return hChip("fb-"+st, tl(l, "fb.status."+st)) }

func fbAppLabel(app string) string {
	if n, ok := fbAppName[app]; ok {
		return n
	}
	return app
}

// fbFilters keeps only what a list may be filtered by, so nothing else travels from page to page.
func fbFilters(v url.Values) url.Values {
	out := url.Values{}
	if x := v.Get("app"); contains(fbApps, x) {
		out.Set("app", x)
	}
	if x := v.Get("kind"); contains(fbKinds, x) {
		out.Set("kind", x)
	}
	if x := v.Get("status"); contains(fbStatuses, x) {
		out.Set("status", x)
	}
	if x := v.Get("cursor"); fbCursorRE.MatchString(x) {
		out.Set("cursor", x)
	}
	return out
}

// fbF is the list's filters as the "f" parameter that detail pages and actions carry, so Back returns to the same list.
func fbF(r *http.Request) string {
	raw := r.URL.Query().Get("f")
	if raw == "" {
		raw = r.FormValue("f")
	}
	v, _ := url.ParseQuery(raw)
	return fbFilters(v).Encode()
}

func fbListURL(f string) string {
	if f == "" {
		return "/feedback"
	}
	return "/feedback?" + f
}

func fbDetailURL(id, f string, extra ...string) string {
	v := url.Values{}
	if f != "" {
		v.Set("f", f)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		v.Set(extra[i], extra[i+1])
	}
	u := "/feedback/" + url.PathEscape(id)
	if len(v) > 0 {
		u += "?" + v.Encode()
	}
	return u
}

func hubStatus(err error) (int, bool) {
	var ae *hopproto.APIError
	if errors.As(err, &ae) {
		return ae.Status, true
	}
	if err != nil && err.Error() == "http_404" {
		return http.StatusNotFound, true
	}
	return 0, false
}

// fbErrText is the message for a refusal or failure of HOP ID: a missing report, a hub without the inbox, or the
// usual "could not be reached".
func fbErrText(l string, err error) (int, string) {
	if st, ok := hubStatus(err); ok && st == http.StatusNotFound {
		var ae *hopproto.APIError
		if errors.As(err, &ae) && ae.Code == "not_found" {
			return http.StatusNotFound, tl(l, "fb.err.gone")
		}
		return http.StatusBadGateway, tl(l, "fb.err.old_hub")
	}
	return http.StatusBadGateway, tl(l, "err.hop") + ": " + err.Error()
}

func (a *App) fbFail(w http.ResponseWriter, r *http.Request, err error) {
	st, msg := fbErrText(a.langOf(r), err)
	a.renderStatus(w, r, st, "error.html", a.Tr(r, "fb.title"), map[string]any{"Err": msg, "Back": "/feedback"})
}

func (a *App) fbNotFound(w http.ResponseWriter, r *http.Request) {
	a.renderStatus(w, r, http.StatusNotFound, "error.html", a.Tr(r, "fb.title"), map[string]any{"Err": a.Tr(r, "fb.err.gone"), "Back": "/feedback"})
}

func (a *App) fbGet(ctx context.Context, s *db.Session, id string) (*fbReport, error) {
	var rep fbReport
	if _, err := a.callHop(ctx, s, "GET", "/feedback/"+url.PathEscape(id), nil, &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

// ---- list ----

func (a *App) fbList(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	if !fbAllowed(s) {
		a.forbidden(w, r)
		return
	}
	l, q := a.langOf(r), r.URL.Query()
	f := fbFilters(q)
	apps := fbApps
	if !globalRole(s) { // an admin:<app> sees its own apps only: the list is always filtered
		apps = fbScopeApps(s)
		if !contains(apps, f.Get("app")) {
			f.Set("app", apps[0])
		}
	}
	hub := url.Values{"limit": {"25"}}
	for k, v := range f {
		hub[k] = v
	}
	var rows []fbRow
	env, err := a.callHop(r.Context(), s, "GET", "/feedback?"+hub.Encode(), nil, &rows)
	p := listPage{Title: tl(l, "fb.title"), Path: "/feedback", Empty: tl(l, "fb.none")}
	if err != nil {
		_, p.Err = fbErrText(l, err)
		rows = nil
	} else if env.Page != nil {
		if env.Page.Total > 0 {
			p.Count = humanInt(int64(env.Page.Total))
		}
		if env.Page.Next != "" {
			p.Next = cursorURL("/feedback", f, env.Page.Next)
		}
	}
	if f.Get("cursor") != "" {
		p.First = pageURL("/feedback", f, "cursor")
	}
	appLabels := map[string]string{}
	for _, ap := range apps {
		appLabels[ap] = fbAppLabel(ap)
	}
	p.Filters = []uiFilter{
		{Kind: "select", Name: "app", Label: tl(l, "ann.col.app"), Value: f.Get("app"), Options: apps, Labels: appLabels},
		{Kind: "select", Name: "kind", Label: tl(l, "fb.col.kind"), Value: f.Get("kind"), Options: fbKinds, Labels: annLabels(l, "fb.kind.", fbKinds)},
		{Kind: "select", Name: "status", Label: tl(l, "fb.col.status"), Value: f.Get("status"), Options: fbStatuses, Labels: annLabels(l, "fb.status.", fbStatuses)},
	}
	p.Cols = []uiCol{{Label: tl(l, "fb.col.kind")}, {Label: tl(l, "ann.col.app")}, {Label: tl(l, "fb.col.message")}, {Label: tl(l, "fb.col.from")}, {Label: tl(l, "fb.col.received")}, {Label: tl(l, "fb.col.status")}}
	keep := f.Encode()
	for _, n := range rows {
		msg := string(hLink(fbDetailURL(n.ID, keep), n.Excerpt))
		if n.HasScreenshot {
			msg += ` <span class="small muted nw">· ` + esc(tl(l, "fb.has_image")) + `</span>`
		}
		from := template.HTML(`<span class="muted">` + esc(tl(l, "fb.guest")) + `</span>`)
		if n.Username != nil && *n.Username != "" {
			from = txt(*n.Username)
		}
		p.Rows = append(p.Rows, uiRow{Cells: []uiCell{
			{template.HTML(fbKindChip(l, n.Kind)), "nw"}, {txt(fbAppLabel(n.App)), "nw"}, {template.HTML(msg), "fb-msg"},
			{from, "nw"}, {datesz(l, n.CreatedAt), "small muted nw"}, {template.HTML(fbStatusChip(l, n.Status)), "nw"}}})
	}
	a.render(w, r, "list.html", p.Title, map[string]any{"P": p})
}

// ---- one report ----

func (a *App) fbShow(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	if !fbAllowed(s) {
		a.forbidden(w, r)
		return
	}
	l, id, f := a.langOf(r), r.PathValue("id"), fbF(r)
	rep, err := a.fbGet(r.Context(), s, id)
	if err != nil {
		a.fbFail(w, r, err)
		return
	}
	if !fbSees(s, rep.App) { // another app's report: as if it did not exist
		a.fbNotFound(w, r)
		return
	}
	if rep.Status == "new" && fbCanAct(s, rep.App) { // opening it is reading it; support only looks
		var fresh fbReport
		if _, err := a.callHop(r.Context(), s, "POST", "/feedback/"+url.PathEscape(id)+"/actions", map[string]any{"action": "read"}, &fresh); err == nil && fresh.ID != "" {
			rep = &fresh
			a.fbClear()
		} else if err != nil {
			log.Printf("feedback: mark read: %v", err)
		}
	}
	d := detailPage{Title: tl(l, "fb.kind."+rep.Kind), Back: fbListURL(f), BackLabel: tl(l, "fb.title")}
	d.Chips = []uiChip{{tl(l, "fb.status."+rep.Status), "fb-" + rep.Status}, {fbAppLabel(rep.App), ""}}
	if fbCanAct(s, rep.App) {
		act := "done"
		if rep.Status == "done" {
			act = "reopen"
		}
		d.Posts = []uiPost{{Label: tl(l, "fb.act."+act), URL: "/feedback/" + url.PathEscape(rep.ID) + "/actions", Fields: [][2]string{{"action", act}, {"f", f}}}}
	}
	if fbCanDelete(s) {
		d.Actions = []uiAction{{Label: actionLabel(l, "delete"), Action: "delete", URL: "/feedback/" + url.PathEscape(rep.ID) + "/actions", ID: dlgID(0, "fb", rep.ID, "delete"),
			Target: tl(l, "fb.kind."+rep.Kind) + " · " + fbAppLabel(rep.App), Danger: true, Fields: [][2]string{{"f", f}}}}
	}
	from := template.HTML(`<span class="muted">` + esc(tl(l, "fb.guest")) + `</span>`)
	if rep.User != nil && rep.User.Username != "" {
		from = txt(rep.User.Username)
	}
	kv := []uiKV{
		{tl(l, "fb.col.kind"), template.HTML(fbKindChip(l, rep.Kind))},
		{tl(l, "ann.col.app"), txt(fbAppLabel(rep.App))},
		{tl(l, "fb.d.page"), fbText(rep.Page)},
		{tl(l, "fb.d.version"), fbText(rep.AppVersion)},
		{tl(l, "fb.col.from"), from},
		{tl(l, "fb.col.received"), datesz(l, rep.CreatedAt)},
	}
	if rep.ReadAt != "" {
		kv = append(kv, uiKV{tl(l, "fb.d.read_at"), datesz(l, rep.ReadAt)})
	}
	if rep.DoneAt != "" {
		kv = append(kv, uiKV{tl(l, "fb.d.done_at"), datesz(l, rep.DoneAt)})
	}
	kv = append(kv, uiKV{tl(l, "fb.d.agent"), fbText(rep.UserAgent)}, uiKV{tl(l, "fb.d.viewport"), fbText(rep.Viewport)}, uiKV{tl(l, "fb.d.locale"), fbText(rep.Locale)})
	if fbShowIP(s) { // for nobody else: the address is not in their page at all
		if rep.IP != nil && *rep.IP != "" {
			kv = append(kv, uiKV{tl(l, "fb.d.ip"), template.HTML(`<span class="mono">` + esc(*rep.IP) + `</span>`)})
		} else {
			kv = append(kv, uiKV{tl(l, "fb.d.ip"), template.HTML(`<span class="muted">` + esc(tl(l, "fb.d.ip_gone")) + `</span>`)})
		}
	}
	shot := uiSection{Title: tl(l, "fb.d.image")}
	switch {
	case !rep.HasScreenshot:
		shot.Empty = tl(l, "fb.d.no_image")
	case r.URL.Query().Get("image") == "1":
		shot.HTML = template.HTML(`<p><img class="fb-shot" src="/feedback/` + esc(url.PathEscape(rep.ID)) + `/screenshot" alt="` + esc(tl(l, "fb.d.image_alt")) + `"></p>`)
	default:
		shot.HTML = template.HTML(`<p><a class="btn" href="` + esc(fbDetailURL(rep.ID, f, "image", "1")) + `">` + esc(tl(l, "fb.d.show_image")) + `</a></p>`)
	}
	d.Sections = []uiSection{{KV: kv}, {Title: tl(l, "fb.col.message"), HTML: template.HTML(`<p class="pre-wrap">` + esc(rep.Message) + `</p>`)}, shot}
	a.render(w, r, "detail.html", d.Title, map[string]any{"P": d})
}

// fbText is a value of the report shown as plain text; nothing in it becomes a link or markup.
func fbText(v string) template.HTML {
	if strings.TrimSpace(v) == "" {
		return template.HTML(`<span class="muted">–</span>`)
	}
	return txt(v)
}

// ---- the picture ----

// fbShot hands on the picture of a report. The session and the role are checked as for the report itself, and the
// bytes are asked from HOP ID only now, when the page asked for them.
func (a *App) fbShot(w http.ResponseWriter, r *http.Request) {
	s, id := current(r), r.PathValue("id")
	if !fbAllowed(s) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rep, err := a.fbGet(r.Context(), s, id)
	if err != nil {
		if st, _ := hubStatus(err); st == http.StatusNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	if !fbSees(s, rep.App) || !rep.HasScreenshot {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_, b, err := a.hop.Fetch(r.Context(), a.hopID(), actorOf(s), "/feedback/"+url.PathEscape(id)+"/screenshot", fbShotMax)
	if err != nil {
		if st, _ := hubStatus(err); st == http.StatusNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	if len(b) < 3 || b[0] != 0xFF || b[1] != 0xD8 || b[2] != 0xFF { // HOP ID stores JPEG only; anything else is not passed on
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, no-store")
	_, _ = w.Write(b)
}

// ---- actions ----

func (a *App) fbAudit(r *http.Request, action, id, app string, detail map[string]any, err error) {
	s := current(r)
	detail["error"] = errString(err)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: app, Action: "feedback." + action, Target: id, Detail: detail, OK: err == nil})
}

// fbAction is Mark done, Reopen or Delete. The reason is asked for Delete (it goes to the audit trail only); the
// audit never holds the text of the report or the sender's address.
func (a *App) fbAction(w http.ResponseWriter, r *http.Request) {
	s, id := current(r), r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	action, f := r.FormValue("action"), fbF(r)
	if !contains([]string{"done", "reopen", "delete"}, action) {
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if !fbAllowed(s) || !fbMayAct(s) || (action == "delete" && !fbCanDelete(s)) {
		a.forbidden(w, r)
		return
	}
	back := fbDetailURL(id, f)
	rep, err := a.fbGet(r.Context(), s, id)
	if err != nil {
		_, msg := fbErrText(a.langOf(r), err)
		a.flashError(w, msg)
		if st, _ := hubStatus(err); st == http.StatusNotFound {
			back = fbListURL(f)
		}
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	if !fbSees(s, rep.App) {
		a.fbNotFound(w, r)
		return
	}
	if !fbCanAct(s, rep.App) {
		a.forbidden(w, r)
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	if action == "delete" {
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
	}
	_, err = a.callHop(r.Context(), s, "POST", "/feedback/"+url.PathEscape(id)+"/actions", map[string]any{"action": action}, nil)
	detail := map[string]any{}
	if reason != "" {
		detail["reason"] = reason
	}
	a.fbAudit(r, action, id, rep.App, detail, err)
	if err != nil {
		_, msg := fbErrText(a.langOf(r), err)
		a.flashError(w, a.Tr(r, "action.failed")+": "+msg)
		if st, _ := hubStatus(err); st == http.StatusNotFound {
			back = fbListURL(f)
		}
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	a.fbClear()
	a.flash(w, a.Tr(r, "action.done"))
	if action == "delete" {
		back = fbListURL(f)
	}
	http.Redirect(w, r, back, http.StatusFound)
}
