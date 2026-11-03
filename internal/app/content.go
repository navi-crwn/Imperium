package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

// Generic modules driven by each app's /hop/manifest: content lists with
// actions, the report queue, settings and extensions. Imperium renders
// whatever an app declares, so a new app needs no code here.

// Manifest is what an app says about itself.
type Manifest struct {
	App          string              `json:"app"`
	Name         string              `json:"name"`
	Icon         string              `json:"icon"`
	BaseURL      string              `json:"base_url"`
	Capabilities []string            `json:"capabilities"`
	ContentTypes []ContentType       `json:"content_types"`
	Extensions   []string            `json:"extensions"`
	Settings     []SettingDef        `json:"settings_schema"`
	Actions      map[string][]string `json:"actions"` // user/report/content action names, as PixelHop and NoteHop list them
	FetchedAt    time.Time           `json:"-"`
}

type ContentType struct {
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Actions []string `json:"actions"`
}

type SettingDef struct {
	Key             string `json:"key"`
	Type            string `json:"type"`
	Description     string `json:"description"`
	RestartRequired bool   `json:"restart_required"`
	Readonly        bool   `json:"readonly"`
	Value           any    `json:"value"`
	Default         any    `json:"default"`
}

func (m *Manifest) Has(c string) bool { return contains(m.Capabilities, c) }

// HasReports: the app declares the reports capability or report actions.
func (m *Manifest) HasReports() bool { return m.Has("reports") || len(m.Actions["report"]) > 0 }

// HasSettings: the app declares the settings capability or a settings schema.
func (m *Manifest) HasSettings() bool { return m.Has("settings") || len(m.Settings) > 0 }

// ReportActions are the buttons a report row gets, by kind: LinkHop's abuse
// flags (confirm/restore) and visitor reports (investigate/close), else what the
// manifest lists, else dismiss/resolve.
func (m *Manifest) ReportActions(kind string) []string {
	switch kind {
	case "flag":
		return []string{"confirm", "restore"}
	case "report":
		return []string{"investigate", "close"}
	}
	if acts := m.Actions["report"]; len(acts) > 0 {
		return acts
	}
	return []string{"dismiss", "resolve"}
}

// extActions are the actions an extension understands, when Imperium knows
// them; the form offers a free action field otherwise.
var extActions = map[string][]string{
	"ip":      {"ban", "unban", "unwatch"},
	"domains": {"add", "remove"},
	"names":   {"release"},
}

func (m *Manifest) ContentType(t string) *ContentType {
	for i := range m.ContentTypes {
		if m.ContentTypes[i].Type == t {
			return &m.ContentTypes[i]
		}
	}
	return nil
}

// manifestCache holds the manifests fetched so far, per App instance.
type manifestCache struct {
	sync.Mutex
	m map[string]*Manifest
}

// source is an app Imperium can read through the protocol: HOP ID itself
// ("hub", prefix /api/hop) or a registry app with the admin_api capability.
func (a *App) source(ctx context.Context, slug string) (*db.App, bool) {
	if slug == "hub" {
		return &db.App{Slug: "hub", Name: "HOP ID", URL: a.cfg.HopURL, Prefix: "/api/hop", Secret: a.cfg.HopSecret, Capabilities: []string{"admin_api"}, Status: "active"}, true
	}
	ap, err := a.db.App(ctx, slug)
	if err != nil || !ap.Can("admin_api") || ap.Secret == "" {
		return nil, false
	}
	return ap, true
}

// sources lists every readable app, HOP ID first.
func (a *App) sources(ctx context.Context) []db.App {
	lh, _ := a.source(ctx, "hub")
	out := []db.App{*lh}
	apps, _ := a.db.Apps(ctx)
	for _, ap := range apps {
		if ap.Slug != "hub" && ap.Can("admin_api") && ap.Secret != "" && ap.Status != "off" {
			out = append(out, ap)
		}
	}
	return out
}

// manifest fetches (and caches for ten minutes) an app's manifest.
func (a *App) manifest(ctx context.Context, s *db.Session, ap *db.App, force bool) (*Manifest, error) {
	a.manifests.Lock()
	if m, ok := a.manifests.m[ap.Slug]; ok && !force && time.Since(m.FetchedAt) < 10*time.Minute {
		a.manifests.Unlock()
		return m, nil
	}
	a.manifests.Unlock()
	var m Manifest
	env, err := a.hop.Call(ctx, a.target(*ap), actorOf(s), "GET", "/manifest", nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(env.Data, &m); err != nil {
		return nil, err
	}
	m.FetchedAt = time.Now()
	a.manifests.Lock()
	a.manifests.m[ap.Slug] = &m
	a.manifests.Unlock()
	return &m, nil
}

// call does a signed call to an app and decodes data into out.
func (a *App) call(ctx context.Context, s *db.Session, ap *db.App, method, path string, body, out any) error {
	env, err := a.hop.Call(ctx, a.target(*ap), actorOf(s), method, path, body)
	if err != nil {
		return err
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

// row is one generic content row; unknown fields are kept in Extra.
type row struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Owner   map[string]any `json:"owner"`
	Status  string         `json:"status"`
	Created string         `json:"created_at"`
	URL     string         `json:"url"`
	Preview string         `json:"preview"`
	Flags   []string       `json:"flags"`
	Size    int64          `json:"size"`
	Clicks  int64          `json:"clicks"`
	Views   int64          `json:"views"`
	Extra   map[string]any `json:"-"`
}

func (r *row) UnmarshalJSON(b []byte) error {
	type plain row
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*r = row(p)
	_ = json.Unmarshal(b, &r.Extra)
	for _, k := range []string{"id", "type", "title", "owner", "status", "created_at", "url", "preview", "flags", "size", "clicks", "views"} {
		delete(r.Extra, k)
	}
	if r.Created == "" {
		r.Created = str(r.Extra["created"])
	}
	return nil
}

func (r row) OwnerName() string {
	if r.Owner == nil {
		return ""
	}
	if u := str(r.Owner["username"]); u != "" {
		return "@" + u
	}
	return str(r.Owner["sub"])
}

func (r row) OwnerSub() string {
	if r.Owner == nil {
		return ""
	}
	return str(r.Owner["sub"])
}

// canActIn says whether the session may run a content/report action on one
// app. admin:<app> is an admin of that app only (hub contract, HEL.ink
// User.php roleNames); owner, admin and moderator act everywhere.
func canActIn(s *db.Session, app string) bool {
	return isAdmin(s) || contains(s.Roles, "moderator") || contains(s.Roles, "admin:"+app)
}

func (a *App) withSource(w http.ResponseWriter, r *http.Request) (*db.App, *Manifest, bool) {
	ap, ok := a.source(r.Context(), r.PathValue("app"))
	if !ok {
		a.render(w, r, "error.html", "App", map[string]any{"Err": "unknown app or no admin API", "Back": "/apps"})
		return nil, nil, false
	}
	m, err := a.manifest(r.Context(), current(r), ap, r.URL.Query().Get("refresh") == "1")
	if err != nil {
		a.render(w, r, "error.html", ap.Name, map[string]any{"Err": a.T("err.manifest") + ": " + err.Error(), "Back": "/apps"})
		return nil, nil, false
	}
	return ap, m, true
}

// GET /apps/{app}/content?type=&q=&status=&owner=&cursor=
func (a *App) content(w http.ResponseWriter, r *http.Request) {
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	typ := q.Get("type")
	if typ == "" && len(m.ContentTypes) > 0 {
		typ = m.ContentTypes[0].Type
	}
	ct := m.ContentType(typ)
	params := url.Values{"type": {typ}, "q": {q.Get("q")}, "status": {q.Get("status")}, "owner": {q.Get("owner")}, "cursor": {q.Get("cursor")}, "limit": {"50"}}
	var rows []row
	var errMsg, next string
	var total int
	env, err := a.hop.Call(r.Context(), a.target(*ap), actorOf(current(r)), "GET", "/content?"+params.Encode(), nil)
	if err != nil {
		errMsg = err.Error()
	} else {
		_ = json.Unmarshal(env.Data, &rows)
		if env.Page != nil {
			next, total = env.Page.Next, env.Page.Total
		}
	}
	a.render(w, r, "content.html", ap.Name+" · "+typ, map[string]any{"App": ap, "M": m, "Type": typ, "CT": ct, "Rows": rows, "Err": errMsg, "Next": next, "Total": total,
		"Q": q.Get("q"), "Status": q.Get("status"), "Owner": q.Get("owner"), "CanAct": canActIn(current(r), ap.Slug)})
}

// GET /apps/{app}/content/{type}/{id}
func (a *App) item(w http.ResponseWriter, r *http.Request) {
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	typ, id := r.PathValue("type"), r.PathValue("id")
	env, err := a.hop.Call(r.Context(), a.target(*ap), actorOf(current(r)), "GET", "/content/"+url.PathEscape(typ)+"/"+url.PathEscape(id), nil)
	if err != nil {
		a.render(w, r, "error.html", ap.Name, map[string]any{"Err": err.Error(), "Back": "/apps/" + ap.Slug + "/content?type=" + typ})
		return
	}
	var it row
	_ = json.Unmarshal(env.Data, &it)
	var pretty any
	_ = json.Unmarshal(env.Data, &pretty)
	a.render(w, r, "item.html", ap.Name+" · "+typ+" "+id, map[string]any{"App": ap, "M": m, "Type": typ, "ID": id, "CT": m.ContentType(typ), "It": it, "Raw": jsonPretty(pretty), "CanAct": canActIn(current(r), ap.Slug)})
}

// POST /apps/{app}/content/{type}/{id}/actions
func (a *App) itemAction(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	typ, id := r.PathValue("type"), r.PathValue("id")
	back := "/apps/" + ap.Slug + "/content/" + url.PathEscape(typ) + "/" + url.PathEscape(id)
	_ = r.ParseForm()
	action := r.FormValue("action")
	ct := m.ContentType(typ)
	if !canActIn(s, ap.Slug) || ct == nil || !contains(ct.Actions, action) {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	params := map[string]any{}
	for k, v := range r.Form {
		if strings.HasPrefix(k, "p_") && len(v) > 0 && v[0] != "" {
			params[strings.TrimPrefix(k, "p_")] = v[0]
		}
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	err := a.call(r.Context(), s, ap, "POST", "/content/"+url.PathEscape(typ)+"/"+url.PathEscape(id)+"/actions", map[string]any{"action": action, "reason": reason, "params": params}, nil)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: ap.Slug, Action: typ + "." + action, Target: id,
		Detail: map[string]any{"reason": reason, "params": params, "error": errString(err)}, OK: err == nil})
	if err != nil {
		a.flashError(w, a.T("action.failed")+": "+err.Error())
	} else {
		a.flash(w, a.T("action.done"))
		if action == "delete" || action == "purge" {
			back = "/apps/" + ap.Slug + "/content?type=" + typ
		}
	}
	http.Redirect(w, r, back, http.StatusFound)
}

// GET /apps/{app}/reports?status=
func (a *App) reports(w http.ResponseWriter, r *http.Request) {
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}
	var rows []map[string]any
	var errMsg string
	if err := a.call(r.Context(), current(r), ap, "GET", "/reports?status="+url.QueryEscape(status)+"&limit=100", nil, &rows); err != nil {
		errMsg = err.Error()
	}
	a.render(w, r, "reports.html", ap.Name+" · reports", map[string]any{"App": ap, "M": m, "Rows": rows, "Err": errMsg, "Status": status, "CanAct": canActIn(current(r), ap.Slug)})
}

// POST /apps/{app}/reports/{id}/actions  (form field kind = report|flag|"" picks the upstream path)
func (a *App) reportAction(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	ap, _, ok := a.withSource(w, r)
	if !ok {
		return
	}
	_ = r.ParseForm()
	id, kind, action := r.PathValue("id"), r.FormValue("kind"), r.FormValue("action")
	back := "/apps/" + ap.Slug + "/reports"
	if !canActIn(s, ap.Slug) || action == "" {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	// LinkHop keeps two queues, abuse flags and visitor reports, and routes them by kind; other apps have one.
	path := "/reports/" + url.PathEscape(id) + "/actions"
	if kind == "flag" || kind == "report" {
		path = "/reports/" + url.PathEscape(kind) + "/" + url.PathEscape(id) + "/actions"
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	err := a.call(r.Context(), s, ap, "POST", path, map[string]any{"action": action, "reason": reason}, nil)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: ap.Slug, Action: "report." + action, Target: kind + ":" + id,
		Detail: map[string]any{"reason": reason, "error": errString(err)}, OK: err == nil})
	if err != nil {
		a.flashError(w, a.T("action.failed")+": "+err.Error())
	} else {
		a.flash(w, a.T("action.done"))
	}
	http.Redirect(w, r, back, http.StatusFound)
}

// GET /apps/{app}/settings
func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	var rows []SettingDef
	var errMsg string
	if err := a.call(r.Context(), current(r), ap, "GET", "/settings", nil, &rows); err != nil {
		errMsg = err.Error()
	}
	a.render(w, r, "settings.html", ap.Name+" · settings", map[string]any{"App": ap, "M": m, "Rows": rows, "Err": errMsg, "IsAdmin": isAdmin(current(r))})
}

// POST /apps/{app}/settings
func (a *App) saveSettings(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	ap, _, ok := a.withSource(w, r)
	if !ok {
		return
	}
	back := "/apps/" + ap.Slug + "/settings"
	if !isAdmin(s) {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	_ = r.ParseForm()
	body := map[string]any{}
	for k, v := range r.Form {
		if !strings.HasPrefix(k, "s_") || len(v) == 0 {
			continue
		}
		key := strings.TrimPrefix(k, "s_")
		switch r.FormValue("t_" + key) {
		case "bool":
			body[key] = v[0] == "1" || v[0] == "true" || v[0] == "on"
		default:
			body[key] = v[0]
		}
	}
	for k, v := range r.Form { // unchecked checkboxes are absent: send false for declared bools
		if strings.HasPrefix(k, "t_") && len(v) > 0 && v[0] == "bool" {
			if _, ok := body[strings.TrimPrefix(k, "t_")]; !ok {
				body[strings.TrimPrefix(k, "t_")] = false
			}
		}
	}
	err := a.call(r.Context(), s, ap, "PUT", "/settings", body, nil)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: ap.Slug, Action: "settings.update", Detail: map[string]any{"keys": keysOf(body), "error": errString(err)}, OK: err == nil})
	if err != nil {
		a.flashError(w, a.T("action.failed")+": "+err.Error())
	} else {
		a.flash(w, a.T("action.done"))
	}
	http.Redirect(w, r, back, http.StatusFound)
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// GET /apps/{app}/x/{ext}: an extension's data, rendered generically.
func (a *App) extension(w http.ResponseWriter, r *http.Request) {
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	ext := r.PathValue("ext")
	q := r.URL.Query().Get("q")
	var data any
	var errMsg string
	if err := a.call(r.Context(), current(r), ap, "GET", "/x/"+url.PathEscape(ext)+"?q="+url.QueryEscape(q), nil, &data); err != nil {
		errMsg = err.Error()
	}
	a.render(w, r, "extension.html", ap.Name+" · "+a.extLabel(ext), map[string]any{"App": ap, "M": m, "Ext": ext, "Data": data, "Tables": tables(data), "Raw": jsonPretty(data), "Err": errMsg, "Q": q,
		"IsAdmin": isAdmin(current(r)), "Actions": extActions[ext], "KV": kvRows(data)})
}

// POST /apps/{app}/x/{ext}/actions: free-form action with p_* params.
func (a *App) extensionAction(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	ap, _, ok := a.withSource(w, r)
	if !ok {
		return
	}
	ext := r.PathValue("ext")
	back := "/apps/" + ap.Slug + "/x/" + url.PathEscape(ext)
	if !isAdmin(s) {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	_ = r.ParseForm()
	action := r.FormValue("action")
	params := map[string]any{}
	for k, v := range r.Form {
		if strings.HasPrefix(k, "p_") && len(v) > 0 && v[0] != "" {
			params[strings.TrimPrefix(k, "p_")] = v[0]
		}
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	// An optional sub-path picks a nested resource: NoteHop releases a slug hold at /x/names/holds/{slug}/actions.
	sub := strings.Trim(r.FormValue("path"), "/")
	if !safeSubPath(sub) {
		a.flashError(w, a.T("action.failed"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	path := "/x/" + url.PathEscape(ext) + "/actions"
	if sub != "" {
		path = "/x/" + url.PathEscape(ext) + "/" + sub + "/actions"
	}
	err := a.call(r.Context(), s, ap, "POST", path, map[string]any{"action": action, "reason": reason, "params": params}, nil)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: ap.Slug, Action: ext + "." + action, Target: sub + str(params["ip"]) + str(params["domain"]) + str(params["slug"]) + str(params["id"]),
		Detail: map[string]any{"reason": reason, "params": params, "error": errString(err)}, OK: err == nil})
	if err != nil {
		a.flashError(w, a.T("action.failed")+": "+err.Error())
	} else {
		a.flash(w, a.T("action.done"))
	}
	http.Redirect(w, r, back, http.StatusFound)
}

// safeSubPath allows "holds/some-slug" style segments only.
func safeSubPath(p string) bool {
	if p == "" {
		return true
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
		for _, c := range seg {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
				return false
			}
		}
	}
	return true
}

// table is a generic rendering of a list of objects.
type table struct {
	Name string
	Cols []string
	Rows [][]string
}

// tables turns {name: [ {..}, .. ], ...} or [ {..} ] into renderable tables.
func tables(data any) []table {
	var out []table
	add := func(name string, list []any) {
		if len(list) == 0 {
			return
		}
		var cols []string
		seen := map[string]bool{}
		for _, it := range list {
			if m, ok := it.(map[string]any); ok {
				for k := range m {
					if !seen[k] {
						seen[k] = true
						cols = append(cols, k)
					}
				}
			}
		}
		sortStrings(cols)
		t := table{Name: name, Cols: cols}
		for _, it := range list {
			m, _ := it.(map[string]any)
			var r []string
			for _, c := range cols {
				r = append(r, cell(m[c]))
			}
			t.Rows = append(t.Rows, r)
		}
		out = append(out, t)
	}
	switch v := data.(type) {
	case []any:
		add("", v)
	case map[string]any:
		keys := keysOfAny(v)
		sortStrings(keys)
		for _, k := range keys {
			if list, ok := v[k].([]any); ok {
				add(k, list)
			}
		}
	}
	return out
}

func cell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return humanInt(int64(x))
		}
		return strings.TrimRight(strings.TrimRight(jsonPretty(x), "0"), ".")
	case bool:
		if x {
			return "✓"
		}
		return ""
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func keysOfAny(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
