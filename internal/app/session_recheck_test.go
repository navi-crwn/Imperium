package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// OP-05: a session is checked against HOP ID every few minutes, so a suspended or demoted admin is
// signed out within minutes instead of at the end of the session lifetime.

const hubActive = `{"ok":true,"data":{"profile":{"sub":"1","status":"active"},"roles":["owner"]}}`

func hubAnswer(status, roles string) func(string) (int, string) {
	return func(string) (int, string) {
		return 200, `{"ok":true,"data":{"profile":{"sub":"1","status":"` + status + `"},"roles":` + roles + `}}`
	}
}

// due makes every session look as if it was last checked long ago.
func due(a *App) {
	a.checkMu.Lock()
	a.checked = map[string]time.Time{}
	a.checkMu.Unlock()
}

func sessionCount(t *testing.T, a *App) int {
	t.Helper()
	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func recheckApp(t *testing.T) (*App, *httptest.Server, *fakeHop, *http.Client) {
	t.Helper()
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	c := signedIn(t, hop, srv, "boss", "owner")
	if code, _ := get(t, c, srv.URL+"/me"); code != 200 {
		t.Fatalf("setup: /me = %d", code)
	}
	hop.memberCalls = nil
	return a, srv, hop, c
}

func TestRecheckKeepsAnActiveAdminAndAsksOncePerWindow(t *testing.T) {
	a, srv, hop, c := recheckApp(t)
	hop.member = func(string) (int, string) { return 200, hubActive }
	if code, _ := get(t, c, srv.URL+"/me"); code != 200 || len(hop.memberCalls) != 0 {
		t.Fatalf("a fresh session must not call the hub: status %d, calls %v", code, hop.memberCalls)
	}
	due(a)
	for i := 0; i < 3; i++ {
		if code, _ := get(t, c, srv.URL+"/me"); code != 200 {
			t.Fatalf("active owner lost the session: %d", code)
		}
	}
	if len(hop.memberCalls) != 1 || hop.memberCalls[0] != "1" {
		t.Fatalf("want exactly one hub call for sub 1 in the window, got %v", hop.memberCalls)
	}
	if sessionCount(t, a) != 1 {
		t.Fatal("session row vanished")
	}
}

func TestRecheckEndsTheSession(t *testing.T) {
	for name, tc := range map[string]struct {
		code int
		body string
	}{
		"suspended":        {200, `{"ok":true,"data":{"profile":{"sub":"1","status":"suspended"},"roles":["owner"]}}`},
		"banned":           {200, `{"ok":true,"data":{"profile":{"sub":"1","status":"banned"},"roles":["owner"]}}`},
		"role revoked":     {200, `{"ok":true,"data":{"profile":{"sub":"1","status":"active"},"roles":[]}}`},
		"only a user role": {200, `{"ok":true,"data":{"profile":{"sub":"1","status":"active"},"roles":["editor"]}}`},
		"deleted":          {404, `{"ok":false,"error":"not_found"}`},
	} {
		t.Run(name, func(t *testing.T) {
			a, srv, hop, c := recheckApp(t)
			hop.member = func(string) (int, string) { return tc.code, tc.body }
			due(a)
			c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			res, err := c.Get(srv.URL + "/me")
			if err != nil {
				t.Fatal(err)
			}
			body(res)
			if res.StatusCode != 302 || !strings.HasPrefix(res.Header.Get("Location"), "/login") {
				t.Fatalf("want a redirect to /login, got %d %q", res.StatusCode, res.Header.Get("Location"))
			}
			if sessionCount(t, a) != 0 {
				t.Fatal("the session row must be deleted, not only hidden")
			}
			var n int
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM audit WHERE action='session.revoked' AND actor_sub='1'`).Scan(&n)
			if n != 1 {
				t.Fatalf("want one session.revoked audit row, got %d", n)
			}
		})
	}
}

func TestRecheckFailsSafeWhenTheHubCannotAnswer(t *testing.T) {
	for name, tc := range map[string]struct {
		code int
		body string
	}{
		"hub error":            {500, `{"ok":false,"error":"server_error"}`},
		"hub rejects our call": {401, `{"ok":false,"error":"bad_signature"}`},
		"html from a proxy":    {502, `<html>bad gateway</html>`},
		"empty answer":         {200, `{"ok":true,"data":{}}`},
		"no roles key":         {200, `{"ok":true,"data":{"profile":{"status":"active"}}}`},
	} {
		t.Run(name, func(t *testing.T) {
			a, srv, hop, c := recheckApp(t)
			hop.member = func(string) (int, string) { return tc.code, tc.body }
			due(a)
			if code, _ := get(t, c, srv.URL+"/me"); code != 200 {
				t.Fatalf("an unanswered check must not sign the admin out: %d", code)
			}
			if sessionCount(t, a) != 1 {
				t.Fatal("session deleted although the hub gave no verdict")
			}
			// and it asks again soon, not on every request
			get(t, c, srv.URL+"/me")
			if len(hop.memberCalls) != 1 {
				t.Fatalf("a failed check must back off, got %d calls", len(hop.memberCalls))
			}
		})
	}
}

func TestRecheckFailsSafeWhenTheHubIsDown(t *testing.T) {
	a, srv, hop, c := recheckApp(t)
	hop.srv.Close()
	due(a)
	start := time.Now()
	if code, _ := get(t, c, srv.URL+"/me"); code != 200 {
		t.Fatalf("hub down: the admin must keep the session until the TTL, got %d", code)
	}
	if time.Since(start) > 6*time.Second {
		t.Fatalf("a dead hub made the page wait %v", time.Since(start))
	}
	if sessionCount(t, a) != 1 {
		t.Fatal("session deleted while the hub was down")
	}
}

func TestRecheckFailsSafeWhenTheHubHangs(t *testing.T) {
	a, srv, hop, c := recheckApp(t)
	old := recheckTimeout
	recheckTimeout = 100 * time.Millisecond
	defer func() { recheckTimeout = old }()
	hop.member = func(string) (int, string) { time.Sleep(400 * time.Millisecond); return 200, hubActive }
	due(a)
	start := time.Now()
	if code, _ := get(t, c, srv.URL+"/me"); code != 200 {
		t.Fatalf("a hung hub must not sign the admin out: %d", code)
	}
	if d := time.Since(start); d > 350*time.Millisecond {
		t.Fatalf("the page waited %v for a hung hub", d)
	}
	if sessionCount(t, a) != 1 {
		t.Fatal("session deleted while the hub hung")
	}
}

func TestRecheckAppliesNewRoles(t *testing.T) {
	a, srv, hop, c := recheckApp(t)
	hop.member = hubAnswer("active", `["support"]`)
	due(a)
	if code, _ := get(t, c, srv.URL+"/me"); code != 200 {
		t.Fatalf("a demoted admin still has a role: %d", code)
	}
	var roles string
	_ = a.db.QueryRow(`SELECT roles FROM sessions`).Scan(&roles)
	if roles != `["support"]` {
		t.Fatalf("session roles = %s, want the hub's [\"support\"]", roles)
	}
	// owner-only power is gone at once: support may not suspend (the hub is not reached for it)
	hop.actions = nil
	res, _ := c.PostForm(srv.URL+"/users/26/actions", map[string][]string{"action": {"suspend"}})
	body(res)
	if len(hop.actions) != 0 {
		t.Fatalf("the demoted session still reached HOP ID with an action: %+v", hop.actions)
	}
}

func TestRecheckSkipsStaticAssets(t *testing.T) {
	a, srv, hop, c := recheckApp(t)
	due(a)
	get(t, c, srv.URL+"/static/app.css")
	if len(hop.memberCalls) != 0 {
		t.Fatalf("a static file triggered a hub call: %v", hop.memberCalls)
	}
}

func TestSessionLifetimeDefaultsToTwoHours(t *testing.T) {
	t.Setenv("IMPERIUM_SESSION_HOURS", "")
	if got := LoadConfig("t").SessionTTL; got != 2*time.Hour {
		t.Fatalf("default session lifetime = %v, want 2h", got)
	}
	t.Setenv("IMPERIUM_SESSION_HOURS", "0")
	if got := LoadConfig("t").SessionTTL; got != 2*time.Hour {
		t.Fatalf("a zero setting must fall back to 2h, got %v", got)
	}
	t.Setenv("IMPERIUM_SESSION_HOURS", "5")
	if got := LoadConfig("t").SessionTTL; got != 5*time.Hour {
		t.Fatalf("an explicit setting must win, got %v", got)
	}
}
