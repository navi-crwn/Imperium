package app

import (
	"encoding/json"
	"fmt"
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
	var health []Health
	if globalRole(s) {
		health = a.probeAll(r.Context(), s, apps)
	} else { // admin:<app>: the cards of the apps it may open
		health = a.probeApps(r.Context(), s, visibleApps(s, apps))
	}
	// HOP ID's own numbers (accounts, sign-ups, reports across the hub) are for the hub-wide roles. admin:<app>
	// gets the cards of its apps, whose dashboards carry that app's numbers; HOP ID is not even asked.
	hubWide := globalRole(s)
	var st stats
	var statsErr string
	if hubWide {
		if _, err := a.callHop(r.Context(), s, "GET", "/stats", nil, &st); err != nil {
			statsErr = err.Error()
		}
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
		"Series": a.seriesViews(st.Series), "NewErrors": a.newErrors(r), "HubWide": hubWide,
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
			if alias := a.cfg.Alias(hitIdentity(hits[i].Label)); alias != "" {
				hits[i].Label, hits[i].Sub = alias, ""
			}
		}
	}
	a.render(w, r, "search.html", "Search", map[string]any{"Q": q, "Hits": hits, "Err": errMsg})
}

// hitIdentity splits the label HOP ID gives a person, "@username name <email>", into username and email.
func hitIdentity(label string) (username, email string) {
	username, _, _ = strings.Cut(strings.TrimPrefix(label, "@"), " ")
	if i, j := strings.LastIndex(label, "<"), strings.LastIndex(label, ">"); i >= 0 && j > i {
		email = label[i+1 : j]
	}
	return username, email
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
			rows[i].Username, rows[i].Name, rows[i].Email = alias, alias, "" // the username is as telling as the name
		}
	}
	apps, _ := a.db.Apps(r.Context())
	a.render(w, r, "users.html", a.T("users.title"), map[string]any{"Rows": rows, "Err": errMsg, "Next": next, "Q": q.Get("q"), "Status": q.Get("status"), "Role": q.Get("role"),
		"Cursor": q.Get("cursor"), "Filters": q, "IsAdmin": isAdmin(current(r)), "Me": current(r).Sub, "BulkActions": userBulkActions, "RoleFilter": roleOptions(apps, nil)})
}

// roleOptions are the roles a form offers: the hub-wide ones, admin:<app> for every registered app (HOP ID
// accepts both, see its set_roles action), and any role the person already holds that is neither, so saving
// the form never drops a role it does not know.
func roleOptions(apps []db.App, held []string) []string {
	out := []string{"owner", "admin", "moderator", "support"}
	for _, ap := range apps {
		if ap.Slug != "" && !contains(out, "admin:"+ap.Slug) {
			out = append(out, "admin:"+ap.Slug)
		}
	}
	for _, role := range held {
		if role != "" && !contains(out, role) {
			out = append(out, role)
		}
	}
	return out
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
		u.Profile["username"], u.Profile["name"], u.Profile["email"] = alias, alias, ""
		raw = nil
	}

	// Each app's own view of the person, fetched side by side. An app that has
	// no account for this HOP id answers not_found; any other error is shown as
	// such, so a broken app never reads as "not linked".
	summaries := map[string]*appSummary{"pixelhop": {}, "notehop": {}, "filehop": {}}
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

	apps, _ := a.db.Apps(r.Context())
	a.render(w, r, "user.html", "@"+str(u.Profile["username"]), map[string]any{
		"ID": id, "U": u, "Raw": string(raw), "IsAdmin": isAdmin(current(r)), "CanNote": canNote(current(r)),
		"Protected": u.protected(),
		"PixelUser": summaries["pixelhop"].User, "PixelErr": summaries["pixelhop"].Err,
		"NoteUser": summaries["notehop"].User, "NoteErr": summaries["notehop"].Err,
		"FileUser": summaries["filehop"].User, "FileErr": summaries["filehop"].Err,
		"RoleOptions": roleOptions(apps, u.Roles),
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

// userBulkActions are the account actions offered for many users at once. Delete
// (typed confirmation per account), roles and notes stay on the user page.
var userBulkActions = []string{"suspend", "activate", "revoke_sessions", "revoke_tokens"}

// POST /users/actions: one account action over many HOP ID users (admins only).
// Superadmin accounts and the operator's own account are skipped, never sent.
func (a *App) usersBulkAction(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	_ = r.ParseForm()
	back := "/users?" + url.Values{"q": {r.FormValue("q")}, "status": {r.FormValue("status")}, "role": {r.FormValue("role")}}.Encode()
	action := strings.TrimSpace(r.FormValue("action"))
	// A form whose action is still on the empty "choose" option must say so, not report
	// a missing role: the default used to be Suspend, so an empty submit suspended people.
	if action == "" {
		a.flashError(w, a.T("bulk.no_action"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	if !isAdmin(s) || !contains(userBulkActions, action) {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	ids := uniquePicks(r.Form["pick"])
	if len(ids) == 0 {
		a.flashError(w, a.T("bulk.none"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	// Rows are counted before the confirmation is judged: with nothing selected the
	// message must be "No rows selected.", not "this changes many rows".
	if r.FormValue("confirm") != "yes" && (confirmAction(action) || action == "revoke_sessions" || action == "revoke_tokens") {
		a.flashError(w, a.T("bulk.need_confirm"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	if len(ids) > maxBulk {
		a.flashError(w, a.T("bulk.too_many"))
		http.Redirect(w, r, back, http.StatusFound)
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	done, skipped, failed := 0, 0, 0
	for _, id := range ids {
		audit := func(err error, detail map[string]any) {
			detail["reason"], detail["bulk"], detail["error"] = reason, true, errString(err)
			a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), App: "hub", Action: "user." + action, Target: id,
				Detail: detail, OK: err == nil && detail["skipped"] == nil})
		}
		if id == s.Sub {
			skipped++
			audit(nil, map[string]any{"skipped": "self"})
			continue
		}
		var target user360
		if _, err := a.callHop(r.Context(), s, "GET", "/users/"+url.PathEscape(id), nil, &target); err != nil {
			failed++
			audit(err, map[string]any{})
			continue
		}
		if target.protected() {
			skipped++
			audit(nil, map[string]any{"skipped": "protected"})
			continue
		}
		_, err := a.callHop(r.Context(), s, "POST", "/users/"+url.PathEscape(id)+"/actions", map[string]any{"action": action, "reason": reason, "params": map[string]any{}}, nil)
		audit(err, map[string]any{})
		if err != nil {
			failed++
		} else {
			done++
		}
	}
	if failed > 0 {
		a.flashError(w, fmt.Sprintf(a.T("bulk.summary_failed"), done, failed, skipped))
	} else {
		a.flash(w, fmt.Sprintf(a.T("bulk.summary"), done, skipped))
	}
	http.Redirect(w, r, back, http.StatusFound)
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
	apps = visibleApps(current(r), apps)
	a.mu.Lock()
	health := map[string]Health{}
	for k, v := range a.health {
		health[k] = v
	}
	a.mu.Unlock()
	a.render(w, r, "apps.html", a.T("apps.title"), map[string]any{"Apps": apps, "Health": health, "HopURL": a.cfg.HopURL, "Global": globalRole(current(r)), "CanSync": isAdmin(current(r))})
}

func (a *App) appsSync(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	if !isAdmin(s) {
		a.flashError(w, a.T("err.forbidden"))
		http.Redirect(w, r, "/apps", http.StatusFound)
		return
	}
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
