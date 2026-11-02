package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

// ---- harness ----

// newInbox starts Imperium with the registry synced, so linkhop and pixelhop
// have their own secrets stored, and the hub signs with Imperium's secret.
func newInbox(t *testing.T) (*App, *httptest.Server, *fakeHop) {
	t.Helper()
	hop := newFakeHop(t)
	t.Cleanup(func() { hop.srv.Close(); hop.px.Close() })
	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	if err := a.syncApps(nil); err != nil {
		t.Fatal(err)
	}
	return a, srv, hop
}

func report(app string, mod func(m map[string]any, d map[string]any)) map[string]any {
	d := map[string]any{
		"class": `Illuminate\Database\QueryException`, "message": "SQLSTATE[HY000]: General error", "origin": "app/Http/Controllers/LinkController.php:88",
		"route": "GET /l/{code}", "method": "GET", "ref": "7f3a9c1e2b4d", "version": "abc1234",
	}
	m := map[string]any{"id": "rep-" + strconv.FormatInt(time.Now().UnixNano(), 36), "type": "error.500", "at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"app": app, "env": "production", "data": d}
	if mod != nil {
		mod(m, d)
	}
	return m
}

// post signs body as `app` with `secret` and sends it to /hop/notify.
func post(t *testing.T, srv *httptest.Server, app, secret string, body []byte, mod func(r *http.Request)) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/hop/notify", bytes.NewReader(body))
	for k, v := range hopproto.Headers(app, secret, "system", "POST", "/hop/notify", body) {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/json")
	if mod != nil {
		mod(req)
	}
	return do(t, req)
}

func do(t *testing.T, req *http.Request) (*http.Response, map[string]any) {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	raw := body(res)
	_ = json.Unmarshal([]byte(raw), &out)
	return res, out
}

func send(t *testing.T, srv *httptest.Server, app, secret string, rep map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(rep)
	return post(t, srv, app, secret, b, nil)
}

func mustAccept(t *testing.T, srv *httptest.Server, app, secret string, rep map[string]any) map[string]any {
	t.Helper()
	res, out := send(t, srv, app, secret, rep)
	if res.StatusCode != 202 {
		t.Fatalf("want 202, got %d %v", res.StatusCode, out)
	}
	return out
}

func groups(t *testing.T, a *App) []db.Notification {
	t.Helper()
	rows, _, err := a.db.ListNotifications(context.Background(), db.NotifScope{All: true}, db.NotifFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// ---- signature ----

func TestNotifySignatureGoodAndBad(t *testing.T) {
	a, srv, _ := newInbox(t)
	rep := report("linkhop", nil)
	out := mustAccept(t, srv, "linkhop", lhOwnSecret, rep)
	data := out["data"].(map[string]any)
	if data["group"] == "" || data["duplicate"] != false {
		t.Fatalf("reply: %v", out)
	}
	if g := groups(t, a); len(g) != 1 || g[0].App != "linkhop" || g[0].Count != 1 || g[0].Status != "new" {
		t.Fatalf("stored: %+v", g)
	}
	cases := []struct {
		name   string
		app    string
		secret string
		want   int
		code   string
	}{
		{"wrong secret", "linkhop", "not-the-secret-not-the-secret-not-the-secret", 401, "bad_signature"},
		{"imperium's own outbound secret is not linkhop's key", "linkhop", secret, 401, "bad_signature"},
		{"another app's secret", "linkhop", pxSecret, 401, "bad_signature"},
		{"unknown app", "ghost", lhOwnSecret, 401, "bad_signature"},
		{"imperium itself", "imperium", secret, 401, "bad_signature"},
		{"empty app", "", lhOwnSecret, 401, "bad_signature"},
	}
	for _, c := range cases {
		res, out := send(t, srv, c.app, c.secret, report(c.app, nil))
		if res.StatusCode != c.want || out["error"] != c.code {
			t.Errorf("%s: %d %v", c.name, res.StatusCode, out)
		}
	}
	if g := groups(t, a); len(g) != 1 || g[0].Count != 1 {
		t.Fatalf("a refused report must store nothing: %+v", g)
	}
}

func TestNotifyHubSignsWithImperiumsSecretAndPixelhopWithItsOwn(t *testing.T) {
	a, srv, _ := newInbox(t)
	mustAccept(t, srv, "hub", secret, report("hub", func(m, d map[string]any) { d["route"] = "GET /{$}" }))
	mustAccept(t, srv, "pixelhop", pxSecret, report("pixelhop", nil))
	apps := map[string]bool{}
	for _, g := range groups(t, a) {
		apps[g.App] = true
	}
	if !apps["hub"] || !apps["pixelhop"] {
		t.Fatalf("groups: %v", apps)
	}
	// An app whose registry entry has no secret (or a short one) cannot sign anything.
	if err := a.db.UpsertApp(context.Background(), &db.App{Slug: "newapp", Name: "New", URL: "https://n", Status: "active", OwnSecret: "short"}); err != nil {
		t.Fatal(err)
	}
	if res, _ := send(t, srv, "newapp", "short", report("newapp", nil)); res.StatusCode != 401 {
		t.Fatalf("short secret accepted: %d", res.StatusCode)
	}
}

func TestSyncKeepsTheAppsOwnSecretForInboundAndImperiumsForOutbound(t *testing.T) {
	a, _, _ := newInbox(t)
	ctx := context.Background()
	lh, err := a.db.App(ctx, "linkhop")
	if err != nil {
		t.Fatal(err)
	}
	if lh.OwnSecret != lhOwnSecret {
		t.Fatal("the registry secret of linkhop must now be stored for inbound verification")
	}
	if lh.Secret != secret || a.target(*lh).Secret != secret {
		t.Fatal("outbound calls to linkhop must still use Imperium's own secret")
	}
	px, _ := a.db.App(ctx, "pixelhop")
	if px.OwnSecret != pxSecret || px.Secret != pxSecret {
		t.Fatalf("pixelhop: %+v", px)
	}
}

func TestNotifyStaleAndReplay(t *testing.T) {
	a, srv, _ := newInbox(t)
	b, _ := json.Marshal(report("linkhop", nil))

	// stale: a timestamp from five minutes ago
	res, out := post(t, srv, "linkhop", lhOwnSecret, b, func(r *http.Request) {
		r.Header.Set("X-HOP-Timestamp", strconv.FormatInt(time.Now().Add(-5*time.Minute).Unix(), 10))
	})
	if res.StatusCode != 401 || out["error"] != "stale_request" {
		t.Fatalf("stale: %d %v", res.StatusCode, out)
	}

	// replay: the very same signed request sent twice
	hdr := hopproto.Headers("linkhop", lhOwnSecret, "system", "POST", "/hop/notify", b)
	same := func() (*http.Response, map[string]any) {
		req, _ := http.NewRequest("POST", srv.URL+"/hop/notify", bytes.NewReader(b))
		req.Header = hdr.Clone()
		req.Header.Set("Content-Type", "application/json")
		return do(t, req)
	}
	if res, out := same(); res.StatusCode != 202 {
		t.Fatalf("first send: %d %v", res.StatusCode, out)
	}
	if res, out := same(); res.StatusCode != 401 || out["error"] != "replay" {
		t.Fatalf("replay: %d %v", res.StatusCode, out)
	}

	// a body changed after signing
	res, out = post(t, srv, "linkhop", lhOwnSecret, b, func(r *http.Request) {
		r.Body = nopCloser(bytes.NewReader(append([]byte(" "), b...)))
		r.ContentLength = int64(len(b) + 1)
	})
	if res.StatusCode != 401 || out["error"] != "bad_signature" {
		t.Fatalf("tampered body: %d %v", res.StatusCode, out)
	}
	// a signature for another path
	req, _ := http.NewRequest("POST", srv.URL+"/hop/notify", bytes.NewReader(b))
	req.Header = hopproto.Headers("linkhop", lhOwnSecret, "system", "POST", "/hop/other", b)
	req.Header.Set("Content-Type", "application/json")
	if res, out := do(t, req); res.StatusCode != 401 {
		t.Fatalf("signature for another path: %d %v", res.StatusCode, out)
	}
	if n := len(groups(t, a)); n != 1 {
		t.Fatalf("groups: %d", n)
	}
}

func TestNotifyIsIdempotentOnTheReportID(t *testing.T) {
	a, srv, _ := newInbox(t)
	rep := report("linkhop", nil)
	first := mustAccept(t, srv, "linkhop", lhOwnSecret, rep)["data"].(map[string]any)
	second := mustAccept(t, srv, "linkhop", lhOwnSecret, rep)["data"].(map[string]any) // a retry: new nonce, same id
	if first["group"] != second["group"] || second["duplicate"] != true {
		t.Fatalf("first %v second %v", first, second)
	}
	if g := groups(t, a); len(g) != 1 || g[0].Count != 1 {
		t.Fatalf("a retried report must count once: %+v", g)
	}
}

// ---- transport rules ----

func TestNotifyTransportRules(t *testing.T) {
	a, srv, _ := newInbox(t)
	b, _ := json.Marshal(report("linkhop", nil))
	cases := []struct {
		name string
		mod  func(r *http.Request)
		want int
		code string
	}{
		{"X-Forwarded-For (a proxy door)", func(r *http.Request) { r.Header.Set("X-Forwarded-For", "203.0.113.5") }, 403, "forbidden_door"},
		{"CF-Connecting-IP (the Cloudflare door)", func(r *http.Request) { r.Header.Set("CF-Connecting-IP", "203.0.113.5") }, 403, "forbidden_door"},
		{"X-HOP-Door (the nginx door)", func(r *http.Request) { r.Header.Set("X-HOP-Door", "tailscale") }, 403, "forbidden_door"},
		{"a cookie", func(r *http.Request) { r.Header.Set("Cookie", "imp_session=abc") }, 403, "forbidden_door"},
		{"wrong content type", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415, "unsupported_media_type"},
		{"no content type", func(r *http.Request) { r.Header.Del("Content-Type") }, 415, "unsupported_media_type"},
		{"json with charset is fine", func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=utf-8") }, 202, ""},
	}
	for _, c := range cases {
		// a fresh report each time so the idempotency key never interferes
		b, _ = json.Marshal(report("linkhop", func(m, d map[string]any) { m["id"] = "tr-" + strconv.Itoa(len(c.name)) + "-" + c.name[:3] }))
		res, out := post(t, srv, "linkhop", lhOwnSecret, b, c.mod)
		if res.StatusCode != c.want || (c.code != "" && out["error"] != c.code) {
			t.Errorf("%s: %d %v", c.name, res.StatusCode, out)
		}
	}

	// a query string is refused even when it is signed
	req, _ := http.NewRequest("POST", srv.URL+"/hop/notify?x=1", bytes.NewReader(b))
	req.Header = hopproto.Headers("linkhop", lhOwnSecret, "system", "POST", "/hop/notify?x=1", b)
	req.Header.Set("Content-Type", "application/json")
	if res, out := do(t, req); res.StatusCode != 400 {
		t.Errorf("query string: %d %v", res.StatusCode, out)
	}
	// only POST
	res, err := http.Get(srv.URL + "/hop/notify")
	if err != nil || res.StatusCode != 405 {
		t.Errorf("GET: %v %v", res, err)
	}
	_ = a
}

func TestNotifyBodyLimit(t *testing.T) {
	a, srv, _ := newInbox(t)
	big := report("linkhop", func(m, d map[string]any) { d["message"] = strings.Repeat("x", 9000) })
	b, _ := json.Marshal(big)
	res, out := post(t, srv, "linkhop", lhOwnSecret, b, nil)
	if res.StatusCode != 413 || out["error"] != "too_large" {
		t.Fatalf("declared length: %d %v", res.StatusCode, out)
	}
	// without a Content-Length (chunked) the reader still stops at the limit
	res, out = post(t, srv, "linkhop", lhOwnSecret, b, func(r *http.Request) {
		r.ContentLength = -1
		r.Body = nopCloser(bytes.NewReader(b))
	})
	if res.StatusCode != 413 {
		t.Fatalf("chunked: %d %v", res.StatusCode, out)
	}
	if len(groups(t, a)) != 0 {
		t.Fatal("nothing may be stored")
	}
}

// ---- payload validation ----

func TestNotifyValidation(t *testing.T) {
	a, srv, _ := newInbox(t)
	fields := map[string]string{
		"unknown top-level field (user id)": "body",
		"cookies":                           "body",
		"request body":                      "body",
		"query string field":                "body",
		"ip field":                          "body",
		"user_id field":                     "body",
		"headers field":                     "body",
		"trace field":                       "body",
		"wrong type":                        "type",
		"app differs from the sender":       "app",
		"missing id":                        "id",
		"short id":                          "id",
		"id with a space":                   "id",
		"time without Z":                    "at",
		"time not a time":                   "at",
		"time in the future":                "at",
		"time two days old":                 "at",
		"env with capitals":                 "env",
		"class empty":                       "data.class",
		"class with a space":                "data.class",
		"class with an email":               "data.class",
		"message over 200 characters":       "data.message",
		"origin absolute":                   "data.origin",
		"origin traversal":                  "data.origin",
		"origin without a line":             "data.origin",
		"origin with an IP":                 "data.origin",
		"route empty":                       "data.route",
		"route with a query string":         "data.route",
		"route with an email":               "data.route",
		"route with a real token":           "data.route",
		"route with a long mixed token":     "data.route",
		"route with an IP":                  "data.route",
		"route with an unknown method word": "data.route",
		"route with a fragment":             "data.route",
		"route with percent encoding":       "data.route",
		"method lower case":                 "data.method",
		"method missing":                    "data.method",
		"ref not hex":                       "data.ref",
		"ref missing":                       "data.ref",
		"version with a space":              "data.version",
		"data is a string":                  "body",
		"null instead of an object":         "data.class",
		"number where a string is expected": "body",
	}
	bad := map[string]func(m, d map[string]any){
		"unknown top-level field (user id)": func(m, d map[string]any) { m["user"] = "26" },
		"cookies":                           func(m, d map[string]any) { d["cookies"] = "a=b" },
		"request body":                      func(m, d map[string]any) { d["body"] = "{\"email\":\"a@b.c\"}" },
		"query string field":                func(m, d map[string]any) { d["query"] = "token=abc" },
		"ip field":                          func(m, d map[string]any) { d["ip"] = "203.0.113.7" },
		"user_id field":                     func(m, d map[string]any) { d["user_id"] = "26" },
		"headers field":                     func(m, d map[string]any) { d["headers"] = map[string]string{"a": "b"} },
		"trace field":                       func(m, d map[string]any) { d["trace"] = "#0 ..." },
		"wrong type":                        func(m, d map[string]any) { m["type"] = "error.404" },
		"app differs from the sender":       func(m, d map[string]any) { m["app"] = "pixelhop" },
		"missing id":                        func(m, d map[string]any) { delete(m, "id") },
		"short id":                          func(m, d map[string]any) { m["id"] = "abc" },
		"id with a space":                   func(m, d map[string]any) { m["id"] = "abc def ghi" },
		"time without Z":                    func(m, d map[string]any) { m["at"] = "2026-11-02T14:03:22+01:00" },
		"time not a time":                   func(m, d map[string]any) { m["at"] = "yesterday" },
		"time in the future":                func(m, d map[string]any) { m["at"] = time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05Z") },
		"time two days old": func(m, d map[string]any) {
			m["at"] = time.Now().Add(-48 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
		},
		"env with capitals":                 func(m, d map[string]any) { m["env"] = "Production!" },
		"class empty":                       func(m, d map[string]any) { d["class"] = "" },
		"class with a space":                func(m, d map[string]any) { d["class"] = "Some Error" },
		"class with an email":               func(m, d map[string]any) { d["class"] = "a@b.com" },
		"message over 200 characters":       func(m, d map[string]any) { d["message"] = strings.Repeat("é", 201) },
		"origin absolute":                   func(m, d map[string]any) { d["origin"] = "/home/carawin/app/X.php:1" },
		"origin traversal":                  func(m, d map[string]any) { d["origin"] = "app/../../etc/passwd:1" },
		"origin without a line":             func(m, d map[string]any) { d["origin"] = "app/X.php" },
		"origin with an IP":                 func(m, d map[string]any) { d["origin"] = "10.0.0.1:80" },
		"route empty":                       func(m, d map[string]any) { d["route"] = "" },
		"route with a query string":         func(m, d map[string]any) { d["route"] = "GET /l/{code}?utm=1" },
		"route with an email":               func(m, d map[string]any) { d["route"] = "/u/a@b.com" },
		"route with a real token":           func(m, d map[string]any) { d["route"] = "GET /n/9f8e7d6c5b4a39281716" },
		"route with a long mixed token":     func(m, d map[string]any) { d["route"] = "GET /l/aZ3kP9qLm2Xv7Rt5Yb1Nc8" },
		"route with an IP":                  func(m, d map[string]any) { d["route"] = "GET /ban/203.0.113.7" },
		"route with an unknown method word": func(m, d map[string]any) { d["route"] = "FETCH /x" },
		"route with a fragment":             func(m, d map[string]any) { d["route"] = "GET /a#b" },
		"route with percent encoding":       func(m, d map[string]any) { d["route"] = "GET /a%20b" },
		"method lower case":                 func(m, d map[string]any) { d["method"] = "get" },
		"method missing":                    func(m, d map[string]any) { delete(d, "method") },
		"ref not hex":                       func(m, d map[string]any) { d["ref"] = "zzzzzzzzzzzz" },
		"ref missing":                       func(m, d map[string]any) { delete(d, "ref") },
		"version with a space":              func(m, d map[string]any) { d["version"] = "a b" },
		"data is a string":                  func(m, d map[string]any) { m["data"] = "x" },
		"null instead of an object":         func(m, d map[string]any) { m["data"] = nil },
		"number where a string is expected": func(m, d map[string]any) { d["ref"] = 123456789012 },
	}
	for name, mod := range bad {
		res, out := send(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) {
			m["id"] = "v-" + strconv.Itoa(len(name)) + "-" + strings.NewReplacer(" ", "", "(", "", ")", "", "'", "").Replace(name)[:6]
			mod(m, d)
		}))
		if res.StatusCode != 422 || out["error"] != "bad_payload" {
			t.Errorf("%s: want 422 bad_payload, got %d %v", name, res.StatusCode, out)
		}
		msg, _ := out["message"].(string)
		if !strings.HasPrefix(msg, fields[name]+":") {
			t.Errorf("%s: refused for the wrong reason (want field %s): %q", name, fields[name], msg)
		}
		if strings.Contains(msg, "203.0.113") || strings.Contains(msg, "a@b.com") {
			t.Errorf("%s: the error echoes the value: %q", name, msg)
		}
	}
	if n := len(groups(t, a)); n != 0 {
		t.Fatalf("%d groups stored from invalid reports", n)
	}

	// Things that are valid.
	ok := map[string]func(m, d map[string]any){
		"no message (NoteHop)":          func(m, d map[string]any) { delete(d, "message") },
		"no version":                    func(m, d map[string]any) { delete(d, "version") },
		"unmatched route":               func(m, d map[string]any) { d["route"] = "unmatched" },
		"laravel route without a slash": func(m, d map[string]any) { d["route"] = "l/{code}" },
		"optional parameter":            func(m, d map[string]any) { d["route"] = "GET users/{id?}" },
		"go catch-all":                  func(m, d map[string]any) { d["route"] = "GET /static/{path...}" },
		"go panic":                      func(m, d map[string]any) { d["class"] = "panic"; d["origin"] = "internal/app/handlers.go:212" },
		"unknown origin":                func(m, d map[string]any) { d["origin"] = "unknown" },
		"200 characters":                func(m, d map[string]any) { d["message"] = strings.Repeat("é", 200) },
		"long readable route":           func(m, d map[string]any) { d["route"] = "GET /admin/settings/notification-preferences-overview" },
	}
	for name, mod := range ok {
		n := name
		res, out := send(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) {
			m["id"] = "ok-" + strconv.Itoa(len(n)) + "-" + strings.NewReplacer(" ", "", "(", "", ")", "").Replace(n)[:5]
			mod(m, d)
		}))
		if res.StatusCode != 202 {
			t.Errorf("%s: want 202, got %d %v", name, res.StatusCode, out)
		}
	}
	// Trailing data after the object is not one JSON value.
	b, _ := json.Marshal(report("linkhop", nil))
	if res, _ := post(t, srv, "linkhop", lhOwnSecret, append(b, []byte(`{"x":1}`)...), nil); res.StatusCode != 422 {
		t.Errorf("trailing data: %d", res.StatusCode)
	}
	if res, _ := post(t, srv, "linkhop", lhOwnSecret, []byte(`not json`), nil); res.StatusCode != 422 {
		t.Errorf("not json: %d", res.StatusCode)
	}
}

func TestScrubMessage(t *testing.T) {
	cases := map[string]string{
		"duplicate entry for user rani@example.com":                  "duplicate entry for user [redacted]",
		"Authorization: Bearer abcDEF123.xyz failed":                 "Authorization: Bearer [redacted] failed",
		"GET https://hel.ink/l/x?token=secret123&u=1 failed":         "GET https://hel.ink/l/x?[redacted] failed",
		"fetch https://user:pw@db.internal/x failed":                 "fetch [redacted] failed",
		"client 203.0.113.9 refused":                                 "client [redacted] refused",
		"session 9f8e7d6c5b4a39281716ffeedd was invalid":             "session [redacted] was invalid",
		"key aZ3kP9qLm2Xv7Rt5Yb1Nc8Wd4 rejected":                     "key [redacted] rejected",
		"Call to undefined method LinkController::missingMethodName": "Call to undefined method LinkController::missingMethodName",
		"line one\nline two\ttab":                                    "line one line two tab",
	}
	for in, want := range cases {
		if got := scrubMessage(in); got != want {
			t.Errorf("scrub(%q) = %q, want %q", in, got, want)
		}
	}
	long := scrubMessage(strings.Repeat("ab ", 200))
	if len([]rune(long)) > 200 {
		t.Errorf("not cut: %d", len([]rune(long)))
	}
}

func TestNotifyStoresTheScrubbedMessageOnly(t *testing.T) {
	a, srv, _ := newInbox(t)
	mustAccept(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) {
		d["message"] = "failed for rani@example.com from 203.0.113.9"
	}))
	g := groups(t, a)[0]
	if strings.Contains(g.Sample.Message, "rani") || strings.Contains(g.Sample.Message, "203.0.113.9") || !strings.Contains(g.Sample.Message, "[redacted]") {
		t.Fatalf("stored message: %q", g.Sample.Message)
	}
}

// ---- grouping over HTTP ----

func TestNotifyGroupsAndCounts(t *testing.T) {
	a, srv, _ := newInbox(t)
	for i := 0; i < 3; i++ {
		mustAccept(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) { m["id"] = "group-0" + strconv.Itoa(i); d["ref"] = fmt.Sprintf("%012x", i) }))
	}
	mustAccept(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) { d["route"] = "GET /b/{slug}" }))
	g := groups(t, a)
	if len(g) != 2 {
		t.Fatalf("groups: %d", len(g))
	}
	counts := map[string]int64{}
	for _, x := range g {
		counts[x.Sample.Route] = x.Count
	}
	if counts["GET /l/{code}"] != 3 || counts["GET /b/{slug}"] != 1 {
		t.Fatalf("counts: %v", counts)
	}
}

// ---- rate limit ----

func TestNotifyRateLimitPerAppAndGlobal(t *testing.T) {
	a, srv, _ := newInbox(t)
	clock := time.Now()
	a.inbox.perApp = newRateLimiter(1, 3) // burst 3, 1 a second
	a.inbox.perApp.now = func() time.Time { return clock }
	a.inbox.global = newRateLimiter(100, 100)

	send1 := func(app, secret string, i int) (*http.Response, map[string]any) {
		return send(t, srv, app, secret, report(app, func(m, d map[string]any) {
			m["id"] = fmt.Sprintf("rl-%s-%d", app, i)
			d["ref"] = fmt.Sprintf("%012x", i)
		}))
	}
	for i := 0; i < 3; i++ {
		if res, out := send1("linkhop", lhOwnSecret, i); res.StatusCode != 202 {
			t.Fatalf("report %d: %d %v", i, res.StatusCode, out)
		}
	}
	res, out := send1("linkhop", lhOwnSecret, 3)
	if res.StatusCode != 429 || out["error"] != "rate_limited" || res.Header.Get("Retry-After") == "" {
		t.Fatalf("over the limit: %d %v retry=%q", res.StatusCode, out, res.Header.Get("Retry-After"))
	}
	if ra, _ := strconv.Atoi(res.Header.Get("Retry-After")); ra < 1 || ra > 2 {
		t.Fatalf("Retry-After: %d", ra)
	}
	// The refused report counted on its group without being stored; the count stays at 3.
	g := groups(t, a)[0]
	if g.Count != 3 || g.Suppressed != 1 {
		t.Fatalf("count %d suppressed %d", g.Count, g.Suppressed)
	}
	// Another app has its own bucket.
	if res, out := send1("pixelhop", pxSecret, 0); res.StatusCode != 202 {
		t.Fatalf("other app: %d %v", res.StatusCode, out)
	}
	// Time passes: a token comes back.
	clock = clock.Add(1100 * time.Millisecond)
	if res, out := send1("linkhop", lhOwnSecret, 4); res.StatusCode != 202 {
		t.Fatalf("after waiting: %d %v", res.StatusCode, out)
	}

	// The global cap holds across apps.
	a.inbox.perApp = newRateLimiter(100, 100)
	a.inbox.global = newRateLimiter(0.001, 2)
	codes := []int{}
	for i := 0; i < 4; i++ {
		app, sec := "linkhop", lhOwnSecret
		if i%2 == 1 {
			app, sec = "pixelhop", pxSecret
		}
		res, _ := send1(app, sec, 10+i)
		codes = append(codes, res.StatusCode)
	}
	if fmt.Sprint(codes) != "[202 202 429 429]" {
		t.Fatalf("global limit: %v", codes)
	}
}

func TestRateLimiterRefill(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newRateLimiter(0.5, 2)
	l.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow("a"); !ok {
			t.Fatal("burst")
		}
	}
	ok, wait := l.allow("a")
	if ok || wait != 2*time.Second {
		t.Fatalf("ok=%v wait=%v", ok, wait)
	}
	now = now.Add(2 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Fatal("refill")
	}
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("keys are separate")
	}
}

func TestInboxFullAnswers429(t *testing.T) {
	a, srv, _ := newInbox(t)
	a.db.Limits = db.NotifLimits{KeepEvents: 20, MaxGroups: 1, MaxGroupsPerApp: 1}
	mustAccept(t, srv, "linkhop", lhOwnSecret, report("linkhop", nil))
	res, out := send(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) { d["route"] = "GET /other" }))
	if res.StatusCode != 429 || out["error"] != "inbox_full" {
		t.Fatalf("%d %v", res.StatusCode, out)
	}
}

// ---- secrets stay out of the log ----

func TestNotifyRejectsAreLoggedWithoutSecrets(t *testing.T) {
	a, srv, _ := newInbox(t)
	logs := captureLog(t)
	b, _ := json.Marshal(report("linkhop", nil))
	post(t, srv, "linkhop", "wrong-secret-wrong-secret-wrong-secret-xx", b, nil)
	send(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) { d["ref"] = "nothex" }))
	out := logs.String()
	if !strings.Contains(out, "notify: refused app=linkhop reason=bad_signature") {
		t.Fatalf("expected a reject line, got %q", out)
	}
	for _, leak := range []string{lhOwnSecret, secret, pxSecret, "wrong-secret", "X-HOP-Signature", "nothex"} {
		if strings.Contains(out, leak) {
			t.Errorf("log contains %q", leak)
		}
	}
	_ = a
}

type nopCloserT struct{ *bytes.Reader }

func (nopCloserT) Close() error { return nil }

func nopCloser(r *bytes.Reader) nopCloserT { return nopCloserT{r} }
