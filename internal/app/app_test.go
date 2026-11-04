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
const pxSecret = "px-secret-that-is-long-enough-123456789"

// lhOwnSecret is what LinkHop holds for itself in the registry (Imperium signs its own calls to LinkHop with its own secret).
const lhOwnSecret = "linkhop-own-secret-that-is-long-enough-1"

// fakeHop plays HOP ID: issues tickets, answers the admin API, records calls.
type fakeHop struct {
	srv            *httptest.Server
	px             *httptest.Server // PixelHop, an app with its own admin API
	pxmux          *http.ServeMux
	ticket         map[string]any
	actions        []map[string]any
	protectedUser  bool
	contentQueries []string
	nonces         hopproto.Nonces
	pxReports      string                         // overrides PixelHop's /hop/reports answer when set
	member         func(sub string) (int, string) // HOP ID's answer for GET /api/hop/users/{sub}; nil means 404 not_found
	searchLabel    string                         // overrides the label of the user hit in /api/hop/search
	memberCalls    []string                       // the subs Imperium asked about
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
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"slug":"linkhop","name":"LinkHop","url":"` + f.srv.URL + `","status":"active","capabilities":["admin_api"],"secret":"` + lhOwnSecret + `"},{"slug":"pixelhop","name":"PixelHop","url":"` + f.px.URL + `","status":"active","capabilities":["account_api","admin_api"],"secret":"` + pxSecret + `"},{"slug":"imperium","name":"Imperium","url":"x","status":"active","hidden":true}]}`))
	}))
	f.srv = httptest.NewServer(mux)
	// PixelHop's admin API (prefix /hop), signed with its own secret, on its own server.
	pxmux := http.NewServeMux()
	f.pxmux = pxmux
	px := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := f.nonces.Verify(r, body, "imperium", []string{pxSecret}, time.Minute); err != nil {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
				return
			}
			h(w, r, body)
		}
	}
	pxmux.HandleFunc("GET /hop/health", px(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"pixelhop","version":"3.7.0","db":"ok"}}`))
	}))
	pxmux.HandleFunc("GET /hop/manifest", px(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"pixelhop","name":"PixelHop","capabilities":["admin_api","events","account_api","metrics"],"content_types":[{"type":"image","label":"Images","actions":["hide","unhide","delete","ban_ip","transfer"]},{"type":"album","label":"Albums","actions":["delete","transfer"]}],"settings_schema":[{"key":"guest_uploads","type":"bool"}],"actions":{"user":["suspend"],"report":["dismiss","resolve","delete"],"content":["hide"]}}}`))
	}))
	pxmux.HandleFunc("GET /hop/stats", px(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"kpi":{"users":5,"images":1234,"bytes":1073741824,"open_reports":1,"uploads_today":3},"series":{"uploads":[["2026-10-20",2],["2026-10-21",8]],"bytes":[["2026-10-20",1024],["2026-10-21",2048]]},"step":"day"}}`))
	}))
	pxmux.HandleFunc("GET /hop/content", px(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		f.contentQueries = append(f.contentQueries, r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"img1","type":"image","title":"cat.jpg","owner":{"sub":"26","username":"rani"},"status":"live","size":20480,"views":7,"created_at":"2026-10-21T10:00:00Z","url":"` + f.px.URL + `/i/img1","preview":"` + f.px.URL + `/t/img1"}],"page":{"next":"","total":1}}`))
	}))
	pxmux.HandleFunc("GET /hop/reports", px(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if f.pxReports != "" {
			_, _ = w.Write([]byte(f.pxReports))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"r1","kind":"image","image":{"id":"img1","url":"` + f.px.URL + `/i/img1"},"reason":"nudity","status":"open","created":"2026-10-21T11:00:00Z"}],"page":{"next":"","total":1}}`))
	}))
	pxmux.HandleFunc("POST /hop/reports/r1/actions", px(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		f.actions = append(f.actions, m)
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	mux.HandleFunc("GET /api/hop/stats", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"kpi":{"users":74,"open_reports":2},"series":{"signups":[["2026-10-20",1],["2026-10-21",3]],"clicks":[["2026-10-20",10]]}}}`))
	}))
	mux.HandleFunc("GET /api/hop/health", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","version":"abc123","db":"ok"}}`))
	}))
	mux.HandleFunc("GET /api/hop/search", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		label := "@rani"
		if f.searchLabel != "" {
			label = f.searchLabel
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"kind":"user","id":"26","label":"` + label + `"}]}`))
	}))
	mux.HandleFunc("GET /api/hop/users", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"sub":"26","username":"rani","name":"Rani","email":"r@x","status":"active","roles":[],"verified":true,"links":3}],"page":{"next":"26"}}`))
	}))
	mux.HandleFunc("GET /api/hop/users/26", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if f.protectedUser {
			_, _ = w.Write([]byte(`{"ok":true,"data":{"profile":{"sub":"26","username":"rani","status":"active","owner":true},"roles":["owner"]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"profile":{"sub":"26","username":"rani","name":"Rani","email":"r@x","status":"active","verified":true},"counts":{"links":3},"sessions":{"count":1},"logins":[],"activity":[],"roles":[],"notes":[],"recent_links":[]}}`))
	}))
	mux.HandleFunc("GET /api/hop/users/27", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"profile":{"sub":"27","username":"budi","name":"Budi","email":"b@x","status":"active","verified":true},"counts":{},"sessions":{"count":0},"logins":[],"activity":[],"roles":[],"notes":[],"recent_links":[]}}`))
	}))
	mux.HandleFunc("GET /api/hop/users/{sub}", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		sub := r.PathValue("sub")
		f.memberCalls = append(f.memberCalls, sub)
		code, out := 404, `{"ok":false,"error":"not_found"}`
		if f.member != nil {
			code, out = f.member(sub)
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(out))
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
	pxmux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	f.px = httptest.NewServer(pxmux)
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
		if len(apps) == 2 && apps[0].Secret != "" && apps[1].Secret != "" {
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

func TestAliasHidesOwnerIdentity(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	a.cfg.Aliases = envAliasesFrom("rani=superadmin, R@X=superadmin")
	hop.ticket = map[string]any{"sub": "26", "username": "rani", "name": "Rani Dewi", "email": "r@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	res := login(t, c, srv)
	if res.Request.URL.Path != "/users" {
		t.Fatalf("owner should be in, got %s", res.Request.URL)
	}
	for _, path := range []string{"/users", "/me", "/users/26", "/audit"} {
		res, _ := c.Get(srv.URL + path)
		b := body(res)
		if strings.Contains(b, "r@x") || strings.Contains(b, "Rani") {
			t.Fatalf("%s leaks the real identity:\n%s", path, b)
		}
		if !strings.Contains(b, "superadmin") {
			t.Fatalf("%s should show the alias", path)
		}
	}
	rows, _ := a.db.Audits(t.Context(), 0, 5)
	if len(rows) == 0 || rows[0].ActorName != "superadmin" {
		t.Fatalf("audit should carry the alias: %+v", rows)
	}
}

func envAliasesFrom(v string) map[string]string {
	t := map[string]string{}
	for _, pair := range strings.Split(v, ",") {
		k, a, _ := strings.Cut(pair, "=")
		t[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(a)
	}
	return t
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
	for _, want := range []string{"HOP ID", "LinkHop", "PixelHop", "Healthy", ">74<", "Open reports", "abc123"} {
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
	if s := got(); len(s) != 1 || !strings.Contains(s[0], "🔴 HOP ID") {
		t.Fatalf("telegram alerts: %q", s)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 5)
	if len(audits) == 0 || audits[0].Action != "health.down" {
		t.Fatalf("audit: %+v", audits)
	}
}

// The explorer sidebar lists every app with its health and modules; clicking
// one opens a dashboard built from the app's stats, content, reports and
// operational extension.
func TestSidebarAndAppDashboards(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	var extActions []string
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
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api","reports","settings"],"content_types":[{"type":"link","label":"Links","actions":["disable"]},{"type":"bio","label":"Bio pages","actions":["unpublish"]}],"extensions":["ip","queue","names"]}}`))
	}))
	mux.HandleFunc("GET /api/hop/content", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"9","type":"link","title":"spam","owner":{"sub":"26","username":"rani"},"status":"live","clicks":5,"created_at":"2026-10-01T00:00:00Z"}],"page":{"next":"","total":1}}`))
	}))
	mux.HandleFunc("GET /api/hop/reports", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"kind":"flag","id":"3","subject":{"type":"link","id":"9"},"score":80,"summary":"phishing words","status":"open","created_at":"2026-10-02T00:00:00Z"}]}`))
	}))
	mux.HandleFunc("GET /api/hop/x/queue", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"worker":"stale","heartbeat_at":"2026-10-20T00:00:00Z","pending":4,"failed":0,"abuse_scan_at":null}}`))
	}))
	mux.HandleFunc("GET /api/hop/x/names", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"holds":[{"slug":"bad-slug","reason":"abuse"}]}}`))
	}))
	mux.HandleFunc("POST /api/hop/x/names/holds/bad-slug/actions", api(func(w http.ResponseWriter, r *http.Request, body []byte) {
		extActions = append(extActions, r.URL.Path)
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))

	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	// home: the sidebar names both apps and their dashboards; the registry cards link to them
	res, _ := c.Get(srv.URL + "/")
	b := body(res)
	for _, want := range []string{`class="tree"`, `href="/apps/linkhop"`, `href="/apps/pixelhop"`, "PixelHop", "HOP ID", "data-side-toggle", `class="card card-link" href="/apps/pixelhop"`, `class="h100"`} {
		if !strings.Contains(b, want) {
			t.Errorf("home missing %q", want)
		}
	}
	if strings.Contains(b, "style=") {
		t.Error("home has an inline style, which the CSP blocks")
	}
	// PixelHop dashboard: KPIs (bytes humanised), attention, series, latest images, open reports with the manifest's report actions
	res, _ = c.Get(srv.URL + "/apps/pixelhop")
	b = body(res)
	for _, want := range []string{"1.0 GB", ">1.234<", "Open reports", "Uploads today", "Bytes uploaded", `class="h100"`, "cat.jpg", "/apps/pixelhop/content/image/img1", "@rani", "nudity", `value="dismiss"`, `value="resolve"`,
		`class="chip active" href="/apps/pixelhop"`, `href="/apps/pixelhop/content?type=image"`, `href="/apps/pixelhop/content?type=album"`, `href="/apps/pixelhop/reports"`, `href="/apps/pixelhop/settings"`, "3.7.0"} {
		if !strings.Contains(b, want) {
			t.Errorf("pixelhop dashboard missing %q", want)
		}
	}
	if !strings.Contains(strings.Join(hop.contentQueries, " "), "type=image&limit=8") {
		t.Errorf("dashboard should ask for the latest images: %v", hop.contentQueries)
	}
	// a report action from the dashboard reaches PixelHop and is audited
	res, _ = c.PostForm(srv.URL+"/apps/pixelhop/reports/r1/actions", url.Values{"kind": {"image"}, "action": {"dismiss"}})
	body(res)
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "dismiss" {
		t.Fatalf("report action: %+v", hop.actions)
	}
	// LinkHop dashboard: queue worker on the dashboard, extension labels in the sidebar, users under the app
	res, _ = c.Get(srv.URL + "/apps/linkhop")
	b = body(res)
	for _, want := range []string{">74<", "phishing words", `value="confirm"`, "heartbeat_at", "stale", `href="/apps/linkhop/x/queue"`, `href="/apps/linkhop/x/names"`, `href="/users"`, "Bio pages", ">spam<"} {
		if !strings.Contains(b, want) {
			t.Errorf("linkhop dashboard missing %q", want)
		}
	}
	// The sidebar stays flat even after all app manifests have been cached.
	res, _ = c.Get(srv.URL + "/audit")
	if b := body(res); strings.Contains(b, `href="/apps/pixelhop/content?type=album"`) || strings.Contains(b, `href="/apps/linkhop/x/queue"`) || strings.Contains(b, `class="tree-app"`) {
		t.Error("sidebar must not duplicate per-app tabs")
	}
	// an extension action may address a nested resource (NoteHop-style slug holds), but not escape the extension
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/x/names/actions", url.Values{"action": {"release"}, "path": {"holds/bad-slug"}})
	body(res)
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/x/names/actions", url.Values{"action": {"release"}, "path": {"../ip"}})
	body(res)
	if len(extActions) != 1 || extActions[0] != "/api/hop/x/names/holds/bad-slug/actions" {
		t.Fatalf("ext sub-path actions: %v", extActions)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 10)
	var seen []string
	for _, x := range audits {
		seen = append(seen, x.Action+"@"+x.App)
	}
	for _, want := range []string{"report.dismiss@pixelhop", "names.release@linkhop"} {
		if !contains(seen, want) {
			t.Errorf("audit missing %s in %v", want, seen)
		}
	}
	// support may look but gets no moderation buttons on a dashboard
	hop.ticket = map[string]any{"sub": "5", "username": "cs", "name": "CS", "email": "c@x", "roles": []string{"support"}, "mfa": true, "status": "active"}
	c2 := client(t)
	body(login(t, c2, srv))
	res, _ = c2.Get(srv.URL + "/apps/pixelhop")
	if b := body(res); strings.Contains(b, `name="action"`) || !strings.Contains(b, "nudity") {
		t.Error("support should see the reports but no action buttons")
	}
}

func TestHumanBytesAndBarSteps(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 1073741824: "1.0 GB"} {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
	pts := series([][2]any{{"2026-10-20", float64(1)}, {"2026-10-21", float64(3)}, {"2026-10-22", float64(2)}})
	if pts[0].Pct != 35 || pts[1].Pct != 100 || pts[2].Pct != 65 || pts[0].Label != "10-20" {
		t.Errorf("bar steps: %+v", pts)
	}
}

func TestBioHopAppDashboard(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()

	// BioHop shares HOP ID's registry and verifies the caller "imperium" with
	// the imperium row's secret, not with the biohop row's own secret.
	const bhSecret = "biohop-secret-that-is-long-enough-123456"
	var bhActions []map[string]any
	bhmux := http.NewServeMux()
	bh := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
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
	bhmux.HandleFunc("GET /api/hop/health", bh(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"biohop","version":"1.0","db":"ok"}}`))
	}))
	bhmux.HandleFunc("GET /api/hop/manifest", bh(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"biohop","name":"BioHop","icon":"id-card","capabilities":["admin_api","events","metrics","reports","settings","search"],"content_types":[{"type":"page","label":"Bio Pages","actions":["unpublish","publish","delete","transfer","clear_custom_css","mark_nsfw","unmark_nsfw"]},{"type":"block","label":"Bio Blocks","actions":["disable","enable","delete"]}],"settings_schema":[{"key":"bio.allow_custom_css","type":"bool"}]}}`))
	}))
	bhmux.HandleFunc("GET /api/hop/stats", bh(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"totals":{"pages":41,"pages_published":38,"pages_draft":3,"pages_adult":1,"blocks":190,"views_total":4200,"clicks_total":980,"reports_open":2},"series":{"pages_created":[["2026-10-20",2],["2026-10-21",5]],"views":[["2026-10-20",100],["2026-10-21",250]]}}}`))
	}))
	bhmux.HandleFunc("GET /api/hop/content", bh(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.URL.Query().Get("type") == "block" {
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"12","type":"block","title":"Portfolio","owner":{"sub":"26","username":"rani"},"status":"active","clicks":44,"created_at":"2026-10-21T10:00:00Z","url":"https://example.com"}],"page":{"next":"","total":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"7","type":"page","title":"Rani Bio","slug":"rani","owner":{"sub":"26","username":"rani"},"status":"published","views":1200,"created_at":"2026-10-20T10:00:00Z","url":"https://hel.ink/b/rani","flags":["custom_css"]}],"page":{"next":"","total":1}}`))
	}))
	bhmux.HandleFunc("GET /api/hop/content/page/7", bh(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"7","type":"page","title":"Rani Bio","slug":"rani","owner":{"sub":"26","username":"rani"},"status":"published","views":1200,"url":"https://hel.ink/b/rani","flags":["custom_css"]}}`))
	}))
	bhmux.HandleFunc("POST /api/hop/content/page/7/actions", bh(func(w http.ResponseWriter, r *http.Request, body []byte) {
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		bhActions = append(bhActions, m)
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"7","status":"draft"}}`))
	}))
	bhmux.HandleFunc("GET /api/hop/reports", bh(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[{"kind":"flag","id":"4","subject":{"type":"bio_page","id":"7"},"score":35,"summary":"adult keywords","status":"open","created_at":"2026-10-21T11:00:00Z"}],"page":{"next":"","total":1}}`))
	}))
	bhsrv := httptest.NewServer(bhmux)
	defer bhsrv.Close()

	_ = a.db.UpsertApp(t.Context(), &db.App{
		Slug:         "biohop",
		Name:         "BioHop",
		URL:          bhsrv.URL,
		Prefix:       "/hop",
		Secret:       bhSecret,
		Status:       "active",
		Capabilities: []string{"account_api", "admin_api", "events", "reports", "settings", "metrics"},
		SortOrder:    15,
	})

	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))

	res, _ := c.Get(srv.URL + "/apps/biohop")
	b := body(res)
	for _, want := range []string{"BioHop", ">41<", "Published pages", "Draft pages", "Adult pages", "Blocks", "Total views", "Total clicks", "Pages created", "Rani Bio", "/apps/biohop/content/page/7", `href="/apps/biohop/content?type=page"`, `href="/apps/biohop/content?type=block"`, "adult keywords"} {
		if !strings.Contains(b, want) {
			t.Errorf("biohop dashboard missing %q", want)
		}
	}
	if strings.Contains(b, "style=") {
		t.Error("biohop dashboard has an inline style, which the CSP blocks")
	}

	res, _ = c.Get(srv.URL + "/apps/biohop/content/page/7")
	if b := body(res); !strings.Contains(b, `value="clear_custom_css"`) || !strings.Contains(b, `value="mark_nsfw"`) {
		t.Fatal("biohop page detail should offer bio page moderation actions")
	}
	res, _ = c.PostForm(srv.URL+"/apps/biohop/content/page/7/actions", url.Values{"action": {"clear_custom_css"}, "reason": {"broken layout"}})
	body(res)
	if len(bhActions) != 1 || bhActions[0]["action"] != "clear_custom_css" {
		t.Fatalf("biohop action not forwarded: %+v", bhActions)
	}
}

func TestUser360AppSummaries(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	defer hop.px.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()

	// Sync pixelhop app into DB with pxSecret
	_ = a.db.UpsertApp(t.Context(), &db.App{
		Slug: "pixelhop", Name: "PixelHop", URL: hop.px.URL, Prefix: "/hop", Secret: pxSecret, Status: "active",
	})

	// Add signed GET /hop/users/26 handler to px
	hop.pxmux.HandleFunc("GET /hop/users/26", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := hop.nonces.Verify(r, body, "imperium", []string{pxSecret}, time.Minute); err != nil {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"role":"admin","status":"active","images":12,"albums":3,"storage_bytes":1048576,"quota_bytes":1073741824,"sessions":2}}`))
	})

	// NoteHop mock server
	const ntSecret = "nt-secret-that-is-long-enough-123456789"
	ntmux := http.NewServeMux()
	ntmux.HandleFunc("GET /hop/users/26", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := hop.nonces.Verify(r, body, "imperium", []string{ntSecret}, time.Minute); err != nil {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"role":"user","status":"active","notes":42,"notes_public":5,"vault_set":true,"sessions":1}}`))
	})
	ntSrv := httptest.NewServer(ntmux)
	defer ntSrv.Close()

	_ = a.db.UpsertApp(t.Context(), &db.App{
		Slug: "notehop", Name: "NoteHop", URL: ntSrv.URL, Prefix: "/hop", Secret: ntSecret, Status: "active",
	})

	hop.ticket = map[string]any{"sub": "26", "username": "rani", "name": "Rani", "email": "r@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	login(t, c, srv)

	res, err := c.Get(srv.URL + "/users/26")
	if err != nil {
		t.Fatal(err)
	}
	b := body(res)
	if !strings.Contains(b, "PixelHop") || !strings.Contains(b, "12") || !strings.Contains(b, "1.0 MB") {
		t.Fatalf("user page missing pixelhop summary: %s", b)
	}
	if !strings.Contains(b, "NoteHop") || !strings.Contains(b, "42") || !strings.Contains(b, "5 public") {
		t.Fatalf("user page missing notehop summary: %s", b)
	}
	// An app that does not know the person says "not linked"; an app that fails says so instead.
	hop.pxmux.HandleFunc("GET /hop/users/27", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"ok":false,"error":"not_found"}`))
	})
	ntmux.HandleFunc("GET /hop/users/27", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"ok":false,"error":"db_locked"}`))
	})
	res, err = c.Get(srv.URL + "/users/27")
	if err != nil {
		t.Fatal(err)
	}
	b = body(res)
	if !strings.Contains(b, "Not linked") {
		t.Fatalf("pixelhop without the account should read as not linked: %s", b)
	}
	if !strings.Contains(b, "Could not reach this app") || !strings.Contains(b, "db_locked") {
		t.Fatalf("a failing notehop must not look like an unlinked account: %s", b)
	}
}

// The example file is public-facing documentation: its alias line shows the format with placeholders only.
func TestEnvExampleAliasesArePlaceholders(t *testing.T) {
	b, err := os.ReadFile("../../deploy/imperium.env.example")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		if _, v, ok := strings.Cut(line, "IMPERIUM_ALIASES="); ok {
			found = true
			for k := range envAliasesFrom(v) {
				if k != "your_username" && k != "you@example.com" {
					t.Error("the alias example names a real-looking account instead of your_username or you@example.com")
				}
			}
		}
	}
	if !found {
		t.Error("the example no longer shows IMPERIUM_ALIASES")
	}
}
