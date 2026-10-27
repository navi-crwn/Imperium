package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/cmd/imperium/webfs"
	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

const secret = "imperium-secret-that-is-long-enough-1234"

// fakeHop plays HOP ID: issues tickets, answers the admin API, records calls.
type fakeHop struct {
	srv     *httptest.Server
	ticket  map[string]any
	actions []map[string]any
	nonces  hopproto.Nonces
}

func newFakeHop(t *testing.T) *fakeHop {
	f := &fakeHop{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sso/imperium", func(w http.ResponseWriter, r *http.Request) {
		p := map[string]any{}
		for k, v := range f.ticket {
			p[k] = v
		}
		now := time.Now().Unix()
		p["state"], p["iat"], p["exp"], p["jti"], p["aud"], p["v"] = r.URL.Query().Get("state"), now, now+120, time.Now().Format(time.RFC3339Nano), "imperium", 2
		js, _ := json.Marshal(p)
		body := hopproto.B64(js)
		m := hmac.New(sha256.New, []byte(secret))
		m.Write([]byte(body))
		http.Redirect(w, r, r.URL.Query().Get("return_to")+"?ticket="+body+"."+hopproto.B64(m.Sum(nil)), http.StatusFound)
	})
	api := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := f.nonces.Verify(r, body, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
				return
			}
			h(w, r, body)
		}
	}
	mux.HandleFunc("GET /api/hop/apps", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"slug":"pixelhop","name":"PixelHop","url":"` + f.srv.URL + `/px","status":"active","capabilities":["account_api"],"secret":"px-secret-that-is-long-enough-123456789"},{"slug":"imperium","name":"Imperium","url":"x","status":"active","hidden":true}]}`))
	}))
	mux.HandleFunc("GET /api/hop/stats", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"kpi":{"users":74,"open_reports":2},"series":{"signups":[["2026-10-20",1],["2026-10-21",3]],"clicks":[["2026-10-20",10]]}}}`))
	}))
	mux.HandleFunc("GET /api/hop/health", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","version":"abc123","db":"ok"}}`))
	}))
	mux.HandleFunc("GET /api/hop/search", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"kind":"user","id":"26","label":"@rani"}]}`))
	}))
	mux.HandleFunc("GET /api/hop/users", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"sub":"26","username":"rani","name":"Rani","email":"r@x","status":"active","roles":[],"verified":true,"links":3}],"page":{"next":"26"}}`))
	}))
	mux.HandleFunc("GET /api/hop/users/26", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"profile":{"sub":"26","username":"rani","name":"Rani","email":"r@x","status":"active","verified":true},"counts":{"links":3},"sessions":{"count":1},"logins":[],"activity":[],"roles":[],"notes":[],"recent_links":[]}}`))
	}))
	mux.HandleFunc("POST /api/hop/users/26/actions", api(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		m["actor"] = r.Header.Get("X-HOP-Actor")
		f.actions = append(f.actions, m)
		if m["action"] == "delete" {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"ok":false,"error":"owner_protected"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	mux.HandleFunc("GET /api/hop/audit", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	mux.HandleFunc("GET /px/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	f.srv = httptest.NewServer(mux)
	return f
}

func newApp(t *testing.T, hop *fakeHop) (*App, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Addr: "127.0.0.1:0", DBPath: "", HopURL: hop.srv.URL, HopSecret: secret, HopClient: "imperium", SessionTTL: time.Hour, RequireMFA: true, Lang: "en", Version: "test"}
	a, err := New(cfg, store, webfs.FS)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	a.cfg.BaseURL = srv.URL
	return a, srv
}

func client(t *testing.T) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func login(t *testing.T, c *http.Client, srv *httptest.Server) *http.Response {
	t.Helper()
	res, err := c.Get(srv.URL + "/auth/sso?next=/users")
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func body(res *http.Response) string {
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return string(b)
}

func TestLoginRolesAndMFA(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	_ = a
	// plain user: denied
	hop.ticket = map[string]any{"sub": "26", "username": "rani", "name": "Rani", "email": "r@x", "roles": []string{}, "mfa": true, "status": "active"}
	c := client(t)
	res := login(t, c, srv)
	if res.Request.URL.Path != "/login" || !strings.Contains(body(res), "no admin role") {
		t.Fatalf("plain user should be denied, landed on %s", res.Request.URL)
	}
	// admin without 2FA: denied
	hop.ticket["roles"], hop.ticket["mfa"] = []string{"admin"}, false
	res = login(t, c, srv)
	if !strings.Contains(body(res), "2FA") {
		t.Fatal("admin without mfa should be told to enable 2FA")
	}
	// owner with 2FA: in, lands on next
	hop.ticket["roles"], hop.ticket["mfa"] = []string{"owner"}, true
	res = login(t, c, srv)
	if res.Request.URL.Path != "/users" {
		t.Fatalf("owner should land on /users, got %s", res.Request.URL)
	}
	if b := body(res); !strings.Contains(b, "@rani") || !strings.Contains(b, "owner") {
		t.Fatalf("users page: %s", b[:200])
	}
	// registry synced on login
	deadline := time.Now().Add(3 * time.Second)
	for {
		apps, _ := a.db.Apps(t.Context())
		if len(apps) == 1 && apps[0].Slug == "pixelhop" && apps[0].Secret != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("registry not synced: %+v", apps)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// logout ends the session
	res, _ = c.PostForm(srv.URL+"/logout", url.Values{})
	body(res)
	res, _ = c.Get(srv.URL + "/users")
	if res.Request.URL.Path != "/login" {
		t.Fatal("still signed in after logout")
	}
}

func TestHomeUserActionsAudit(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	// home: health of HOP ID and PixelHop, KPIs, attention
	res, _ := c.Get(srv.URL + "/")
	b := body(res)
	for _, want := range []string{"LinkHop (HOP ID)", "PixelHop", "Healthy", ">74<", "Open reports", "abc123"} {
		if !strings.Contains(b, want) {
			t.Errorf("home missing %q", want)
		}
	}
	// search and user 360
	res, _ = c.Get(srv.URL + "/search?q=rani")
	if b := body(res); !strings.Contains(b, "/users/26") {
		t.Fatal("search result should link to the user")
	}
	res, _ = c.Get(srv.URL + "/users/26")
	if b := body(res); !strings.Contains(b, "Suspend") || !strings.Contains(b, "Delete account") {
		t.Fatal("owner should see the actions")
	}
	// an action is forwarded with the actor and recorded locally
	res, _ = c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"suspend"}, "reason": {"spam"}})
	if b := body(res); !strings.Contains(b, "Done.") {
		t.Fatalf("action flash missing: %s", b[:300])
	}
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "suspend" || hop.actions[0]["reason"] != "spam" || hop.actions[0]["actor"] != "1:boss" {
		t.Fatalf("forwarded action: %+v", hop.actions)
	}
	// a refused action shows the error and is logged as failed
	res, _ = c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"delete"}, "confirm": {"rani"}})
	if b := body(res); !strings.Contains(b, "owner_protected") {
		t.Fatal("refusal should be shown")
	}
	audits, _ := a.db.Audits(t.Context(), 0, 10)
	var ok, failed int
	for _, x := range audits {
		if x.Action == "user.suspend" && x.OK {
			ok++
		}
		if x.Action == "user.delete" && !x.OK {
			failed++
		}
	}
	if ok != 1 || failed != 1 {
		t.Fatalf("audit rows: %+v", audits)
	}
	res, _ = c.Get(srv.URL + "/audit")
	if b := body(res); !strings.Contains(b, "user.suspend") {
		t.Fatal("audit page should list the action")
	}
	// CSRF: a cross-site POST is refused
	req, _ := http.NewRequest("POST", srv.URL+"/users/26/actions", strings.NewReader("action=suspend"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res, _ = c.Do(req)
	if res.StatusCode != 403 {
		t.Fatalf("cross-site POST got %d", res.StatusCode)
	}
	body(res)
}

func TestSupportIsReadOnly(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "5", "username": "cs", "name": "CS", "email": "c@x", "roles": []string{"support"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	res, _ := c.Get(srv.URL + "/users/26")
	if b := body(res); strings.Contains(b, "Delete account") {
		t.Fatal("support must not see destructive actions")
	}
	res, _ = c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"suspend"}})
	if b := body(res); !strings.Contains(b, "may not") || len(hop.actions) != 0 {
		t.Fatal("support action should be refused before reaching HOP ID")
	}
}

func TestMain(m *testing.M) { os.Exit(m.Run()) }

// The generic modules follow the app's manifest: content lists, actions, reports, settings, extensions.
func TestContentModulesFollowTheManifest(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	var contentActions, reportActions, settingsPuts, extActions []map[string]any
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	api := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, body, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
				return
			}
			h(w, r, body)
		}
	}
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api","reports","settings"],"content_types":[{"type":"link","label":"Links","actions":["disable","enable","delete"]},{"type":"bio","label":"Bio pages","actions":["unpublish"]}],"extensions":["ip"],"settings_schema":[{"key":"seo.site_title","type":"string"}]}}`))
	}))
	mux.HandleFunc("GET /api/hop/content", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.URL.Query().Get("type") != "link" || r.URL.Query().Get("status") != "live" {
			_, _ = w.Write([]byte(`{"ok":true,"data":[],"page":{"next":null,"total":0}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"9","type":"link","title":"spam","slug":"spam","owner":{"sub":"26","username":"rani"},"status":"live","clicks":5,"created_at":"2026-10-01T00:00:00Z","url":"https://hel.ink/spam","flags":["password"]}],"page":{"next":"9","total":1}}`))
	}))
	mux.HandleFunc("GET /api/hop/content/link/9", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"9","type":"link","title":"spam","slug":"spam","target_url":"https://bad.example","owner":{"sub":"26","username":"rani"},"status":"live","findings":[]}}`))
	}))
	mux.HandleFunc("POST /api/hop/content/link/9/actions", api(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		contentActions = append(contentActions, m)
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"9","status":"disabled"}}`))
	}))
	mux.HandleFunc("GET /api/hop/reports", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"kind":"flag","id":"3","subject":{"type":"link","id":"9"},"score":80,"summary":"phishing words","status":"open","created_at":"2026-10-02T00:00:00Z"}]}`))
	}))
	mux.HandleFunc("POST /api/hop/reports/flag/3/actions", api(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		reportActions = append(reportActions, m)
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	mux.HandleFunc("GET /api/hop/settings", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"key":"seo.site_title","type":"string","value":"LinkHop","readonly":false},{"key":"turnstile","type":"bool","value":true,"readonly":true}]}`))
	}))
	mux.HandleFunc("PUT /api/hop/settings", api(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		settingsPuts = append(settingsPuts, m)
		_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	mux.HandleFunc("GET /api/hop/x/ip", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"bans":[{"ip":"203.0.113.9","reason":"spam","expires_at":null}],"watchlist":[]}}`))
	}))
	mux.HandleFunc("POST /api/hop/x/ip/actions", api(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		extActions = append(extActions, m)
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))

	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	// list follows the manifest: first content type by default, filters passed through
	res, _ := c.Get(srv.URL + "/apps/linkhop/content?status=live")
	b := body(res)
	for _, want := range []string{"Links", "Bio pages", ">spam<", "@rani", "password", "/apps/linkhop/content/link/9", "/apps/linkhop/x/ip"} {
		if !strings.Contains(b, want) {
			t.Errorf("content list missing %q", want)
		}
	}
	// detail shows the actions the manifest allows, and an action is forwarded + audited
	res, _ = c.Get(srv.URL + "/apps/linkhop/content/link/9")
	if b := body(res); !strings.Contains(b, `value="disable"`) || strings.Contains(b, `value="unpublish"`) || !strings.Contains(b, "bad.example") {
		t.Fatal("detail should offer only the link actions")
	}
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/content/link/9/actions", url.Values{"action": {"disable"}, "reason": {"phishing"}})
	body(res)
	if len(contentActions) != 1 || contentActions[0]["action"] != "disable" || contentActions[0]["reason"] != "phishing" {
		t.Fatalf("content action: %+v", contentActions)
	}
	// an action the manifest does not list is refused locally
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/content/link/9/actions", url.Values{"action": {"nuke"}})
	body(res)
	if len(contentActions) != 1 {
		t.Fatal("unlisted action must not reach the app")
	}
	// reports queue + action with kind in the path
	res, _ = c.Get(srv.URL + "/apps/linkhop/reports")
	if b := body(res); !strings.Contains(b, "phishing words") || !strings.Contains(b, `value="restore"`) {
		t.Fatal("reports page")
	}
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/reports/3/actions", url.Values{"kind": {"flag"}, "action": {"restore"}})
	body(res)
	if len(reportActions) != 1 || reportActions[0]["action"] != "restore" {
		t.Fatalf("report action: %+v", reportActions)
	}
	// settings: editable field saved, read-only shown, bool typed
	res, _ = c.Get(srv.URL + "/apps/linkhop/settings")
	if b := body(res); !strings.Contains(b, `name="s_seo.site_title"`) || !strings.Contains(b, "read-only") {
		t.Fatal("settings page")
	}
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/settings", url.Values{"t_seo.site_title": {"string"}, "s_seo.site_title": {"New title"}})
	body(res)
	if len(settingsPuts) != 1 || settingsPuts[0]["seo.site_title"] != "New title" {
		t.Fatalf("settings put: %+v", settingsPuts)
	}
	// extension renders tables and forwards actions
	res, _ = c.Get(srv.URL + "/apps/linkhop/x/ip")
	if b := body(res); !strings.Contains(b, "203.0.113.9") || !strings.Contains(b, "<th>reason</th>") {
		t.Fatal("extension table")
	}
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/x/ip/actions", url.Values{"action": {"unban"}, "p_ip": {"203.0.113.9"}})
	body(res)
	if len(extActions) != 1 || extActions[0]["params"].(map[string]any)["ip"] != "203.0.113.9" {
		t.Fatalf("ext action: %+v", extActions)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 20)
	var kinds []string
	for _, x := range audits {
		kinds = append(kinds, x.Action)
	}
	for _, want := range []string{"link.disable", "report.restore", "settings.update", "ip.unban"} {
		if !contains(kinds, want) {
			t.Errorf("audit missing %s in %v", want, kinds)
		}
	}
}

func TestHealthFlipsSendTelegram(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	var mu sync.Mutex
	var sent []string
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		sent = append(sent, r.FormValue("text"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	got := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), sent...) }
	defer tg.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	a.cfg.TelegramToken, a.cfg.TelegramChat, a.cfg.TelegramAPI = "tok", "42", tg.URL
	// first probe: nothing known yet, no alert
	a.probeAndAlert(t.Context())
	// HOP ID goes away: alert
	hop.srv.Close()
	a.probeAndAlert(t.Context())
	deadline := time.Now().Add(3 * time.Second)
	for len(got()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if s := got(); len(s) != 1 || !strings.Contains(s[0], "🔴 LinkHop (HOP ID)") {
		t.Fatalf("telegram alerts: %q", s)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 5)
	if len(audits) == 0 || audits[0].Action != "health.down" {
		t.Fatalf("audit: %+v", audits)
	}
}
