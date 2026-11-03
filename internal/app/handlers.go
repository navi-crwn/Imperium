package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/navi-crwn/imperium/internal/db"
)

// ---- home ----

type stats struct {
	KPI    map[string]int64    `json:"kpi"`
	Totals map[string]int64    `json:"totals"`
	Series map[string][][2]any `json:"series"` // [["2026-10-20", 3], …]
}

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	apps, _ := a.db.Apps(r.Context())
	if len(apps) == 0 {
		_ = a.syncApps(s)
		apps, _ = a.db.Apps(r.Context())
	}
	health := a.probeAll(r.Context(), s, apps)
	var st stats
	var statsErr string
	if _, err := a.callHop(r.Context(), s, "GET", "/stats", nil, &st); err != nil {
		statsErr = err.Error()
	}
	kpiOrder := []string{"users", "users_new_7d", "users_suspended"}
	attention := map[string]int64{}
	for _, k := range []string{"open_reports", "open_flags", "users_suspended"} {
		if v := st.KPI[k]; v > 0 {
			attention[k] = v
		}
	}
	a.render(w, r, "home.html", a.T("nav.home"), map[string]any{
		"Health": health, "Stats": st, "StatsErr": statsErr, "KPIOrder": kpiOrder, "Attention": attention,
		"Series": a.seriesViews(st.Series), "NewErrors": a.newErrors(r),
	})
}

type point struct {
	Label string
	N     int64
	Pct   int // bar height, 0..100
}

func series(raw [][2]any) []point {
	var out []point
	var max int64
	for _, p := range raw {
		var n int64
		switch v := p[1].(type) {
		case float64:
			n = int64(v)
		case string:
			n, _ = strconv.ParseInt(v, 10, 64)
		}
		if n > max {
			max = n
		}
		out = append(out, point{Label: str(p[0]), N: n})
	}
	for i := range out {
		if max > 0 {
			out[i].Pct = int((out[i].N*100/max + 2) / 5 * 5) // bar heights are CSS classes h0…h100 in steps of five
		}
		if len(out[i].Label) >= 10 {
			out[i].Label = out[i].Label[5:10]
		}
	}
	return out
}

// ---- search ----

type hit struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
	Sub   string `json:"sub"`
	URL   string `json:"url"`
}

func (a *App) search(w http.ResponseWriter, r *http.Request) {
	if !a.requireGlobal(w, r) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var hits []hit
	var errMsg string
	if q != "" {
		if _, err := a.callHop(r.Context(), current(r), "GET", "/search?q="+url.QueryEscape(q), nil, &hits); err != nil {
			errMsg = err.Error()
		}
	}
	for i := range hits {
		if hits[i].Kind == "user" {
			hits[i].URL = "/users/" + url.PathEscape(hits[i].ID)
		}
	}
	a.render(w, r, "search.html", "Search", map[string]any{"Q": q, "Hits": hits, "Err": errMsg})
}

// ---- users ----

type userRow struct {
	Sub         string   `json:"sub"`
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Status      string   `json:"status"`
	Roles       []string `json:"roles"`
	Verified    bool     `json:"verified"`
	CreatedAt   string   `json:"created_at"`
	LastLoginAt string   `json:"last_login_at"`
	Links       int64    `json:"links"`
	BioPages    int64    `json:"bio_pages"`
}

func (a *App) users(w http.ResponseWriter, r *http.Request) {
	if !a.requireGlobal(w, r) {
		return
	}
	q := r.URL.Query()
	path := "/users?" + url.Values{"q": {q.Get("q")}, "status": {q.Get("status")}, "role": {q.Get("role")}, "cursor": {q.Get("cursor")}, "limit": {"50"}}.Encode()
	var rows []userRow
	var errMsg, next string
	env, err := a.callHop(r.Context(), current(r), "GET", path, nil, &rows)
	if err != nil {
		errMsg = err.Error()
	} else if env.Page != nil {
		next = env.Page.Next
	}
	for i := range rows {
		if alias := a.cfg.Alias(rows[i].Username, rows[i].Email); alias != "" {
			rows[i].Name, rows[i].Email = alias, ""
		}
	}
	a.render(w, r, "users.html", a.T("users.title"), map[string]any{"Rows": rows, "Err": errMsg, "Next": next, "Q": q.Get("q"), "Status": q.Get("status"), "Role": q.Get("role")})
}

// user360 mirrors HOP ID's /api/hop/users/{id} answer; unknown fields stay as raw JSON for the debug panel.
type user360 struct {
	Profile     map[string]any   `json:"profile"`
	Counts      map[string]int64 `json:"counts"`
	Sessions    map[string]any   `json:"sessions"`
	Logins      []map[string]any `json:"logins"`
	Activity    []map[string]any `json:"activity"`
	Roles       []string         `json:"roles"`
	Notes       []map[string]any `json:"notes"`
	RecentLinks []map[string]any `json:"recent_links"`
}

func (u user360) protected() bool {
	if owner, _ := u.Profile["owner"].(bool); owner {
		return true
	}
	for _, role := range u.Roles {
		if role == "owner" {
			return true
		}
	}
	if roles, ok := u.Profile["roles"].([]any); ok {
		for _, role := range roles {
			if role == "owner" {
				return true
			}
		}
	}
	return false
}

func (a *App) user(w http.ResponseWriter, r *http.Request) {
	if !a.requireGlobal(w, r) {
		return
	}
	id := r.PathValue("id")
	var u user360
	var raw json.RawMessage
	env, err := a.callHop(r.Context(), current(r), "GET", "/users/"+url.PathEscape(id), nil, &u)
	if err != nil {
		a.render(w, r, "error.html", "User", map[string]any{"Err": err.Error(), "Back": "/users"})
		return
	}
	raw = env.Data
	if alias := a.cfg.Alias(str(u.Profile["username"]), str(u.Profile["email"])); alias != "" {
		u.Profile["name"], u.Profile["email"] = alias, ""
		raw = nil
	}

	// Each app's own view of the person, fetched side by side. An app that has
	// no account for this HOP id answers not_found; any other error is shown as
	// such, so a broken app never reads as "not linked".
	summaries := map[string]*appSummary{"pixelhop": {}, "notehop": {}}
	var wg sync.WaitGroup
	for slug, sum := range summaries {
		ap, err := a.db.App(r.Context(), slug)
		if err != nil || ap == nil || ap.Secret == "" {
			continue
		}
		wg.Add(1)
		go func(ap *db.App, sum *appSummary) {
			defer wg.Done()
			var resp map[string]any
			err := a.call(r.Context(), current(r), ap, "GET", "/users/"+url.PathEscape(id), nil, &resp)
			switch {
			case err == nil && len(resp) > 0:
				sum.User = resp
			case err != nil && err.Error() != "not_found":
				sum.Err = err.Error()
			}
		}(ap, sum)
	}
	wg.Wait()

	a.render(w, r, "user.html", "@"+str(u.Profile["username"]), map[string]any{
		"ID": id, "U": u, "Raw": string(raw), "IsAdmin": isAdmin(current(r)), "CanNote": canNote(current(r)),
		"Protected": u.protected(),
		"PixelUser": summaries["pixelhop"].User, "PixelErr": summaries["pixelhop"].Err,
		"NoteUser": summaries["notehop"].User, "NoteErr": summaries["notehop"].Err,
		"RoleOptions": []string{"owner", "admin", "moderator", "support"},
	})
}

// canNote says whether the session may add a note to a hub user: every
// hub-wide role except the read-only support role.
func canNote(s *db.Session) bool { return isAdmin(s) || contains(s.Roles, "moderator") }

func str(v any) string { s, _ := v.(string); return s }

// userAction forwards an action to HOP ID and records it locally too.
func (a *App) userAction(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	if !globalRole(s) {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", 400)
		return
	}
	action := r.FormValue("action")
	reason := strings.TrimSpace(r.FormValue("reason"))
	params := map[string]any{}
	switch action {
	case "suspend", "activate", "revoke_sessions", "revoke_tokens", "delete", "set_roles":
		if !isAdmin(s) {
			a.flashError(w, a.T("err.forbidden"))
			http.Redirect(w, r, "/users/"+url.PathEscape(id), http.StatusFound)
			return
		}
	case "note":
		// support is read-only (docs/01-FITUR.md); a note is a write.
		if !canNote(s) {
			a.flashError(w, a.T("err.forbidden"))
			http.Redirect(w, r, "/users/"+url.PathEscape(id), http.StatusFound)
			return
		}
	default:
		http.Error(w, "unknown action", 400)
		return
	}
	switch action {
	case "note":
		params["text"] = strings.TrimSpace(r.FormValue("text"))
	case "set_roles":
		params["roles"] = r.Form["roles"]
	case "delete":
		params["confirm"] = strings.TrimSpace(r.FormValue("confirm"))
	}
	body := map[string]any{"action": action, "reason": reason, "params": params}
	var target user360
	if _, err := a.callHop(r.Context(), s, "GET", "/users/"+url.PathEscape(id), nil, &target); err != nil {
		a.flashError(w, a.T("action.failed")+": "+err.Error())
		http.Redirect(w, r, "/users/"+url.PathEscape(id), http.StatusFound)
		return
	}
	if target.protected() {
		a.flashError(w, "Superadmin accounts are permanent and read-only, including for the account itself.")
		http.Redirect(w, r, "/users/"+url.PathEscape(id), http.StatusFound)
		return
	}
	_, err := a.callHop(r.Context(), s, "POST", "/users/"+url.PathEscape(id)+"/actions", body, nil)
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: "hub", Action: "user." + action, Target: id,
		Detail: map[string]any{"reason": reason, "params": params, "error": errString(err)}, OK: err == nil})
	if err != nil {
		a.flashError(w, a.T("action.failed")+": "+err.Error())
	} else {
		a.flash(w, a.T("action.done"))
	}
	if action == "delete" && err == nil {
		http.Redirect(w, r, "/users", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/users/"+url.PathEscape(id), http.StatusFound)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ---- apps ----

func (a *App) apps(w http.ResponseWriter, r *http.Request) {
	apps, _ := a.db.Apps(r.Context())
	a.mu.Lock()
	health := map[string]Health{}
	for k, v := range a.health {
		health[k] = v
	}
	a.mu.Unlock()
	a.render(w, r, "apps.html", a.T("apps.title"), map[string]any{"Apps": apps, "Health": health, "HopURL": a.cfg.HopURL})
}

func (a *App) appsSync(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	if err := a.syncApps(s); err != nil {
		a.flashError(w, a.T("err.hop")+": "+err.Error())
	} else {
		a.flash(w, a.T("apps.synced"))
	}
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), Action: "apps.sync"})
	http.Redirect(w, r, "/apps", http.StatusFound)
}

// ---- audit ----

func (a *App) audit(w http.ResponseWriter, r *http.Request) {
	if !a.requireGlobal(w, r) {
		return
	}
	local, _ := a.db.Audits(r.Context(), 0, 100)
	var remote []map[string]any
	var errMsg string
	if _, err := a.callHop(r.Context(), current(r), "GET", "/audit?limit=50", nil, &remote); err != nil {
		errMsg = err.Error()
	}
	a.render(w, r, "audit.html", a.T("audit.title"), map[string]any{"Local": local, "Remote": remote, "Err": errMsg})
}

// appSummary is one app's answer for the User 360 page.
type appSummary struct {
	User map[string]any
	Err  string
}
