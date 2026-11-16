package fakehub

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveFeedback(t *testing.T) (*Feedback, *httptest.Server) {
	f := NewFeedback()
	m := http.NewServeMux()
	f.Register(m, "/api/hop", func(h Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { b, _ := io.ReadAll(r.Body); h(w, r, b) }
	})
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	return f, srv
}

// The four routes answer in the shape the contract (F-4) fixes.
func TestFeedbackRoutesShape(t *testing.T) {
	f, srv := serveFeedback(t)
	id := f.Seed(Report{Kind: "idea", App: "pixelhop", Message: "a\nb  c", UserID: 3, Username: "helena", IP: "203.0.113.5", Screenshot: []byte("\xff\xd8\xff")})
	f.Seed(Report{App: "linkhop", Message: "guest one"})
	code, out := call(t, srv, "GET", "/api/hop/feedback?status=new&limit=1", "")
	if code != 200 || !strings.Contains(out, `"total":2`) || !strings.Contains(out, `"next":"2"`) || strings.Count(out, `"excerpt"`) != 1 {
		t.Errorf("list: %d %s", code, out)
	}
	for _, key := range []string{`"id":"1"`, `"kind":"idea"`, `"has_screenshot":true`, `"username":"helena"`, `"excerpt":"a b c"`, `"created_at":"`} {
		_, one := call(t, srv, "GET", "/api/hop/feedback?app=pixelhop", "")
		if !strings.Contains(one, key) {
			t.Errorf("list row lacks %s: %s", key, one)
		}
	}
	if _, out = call(t, srv, "GET", "/api/hop/feedback?app=pixelhop", ""); strings.Contains(out, "203.0.113.5") {
		t.Error("the list must not carry the IP address")
	}
	_, out = call(t, srv, "GET", "/api/hop/feedback/1", "")
	for _, key := range []string{`"ip":"203.0.113.5"`, `"user":{"id":3,"username":"helena"}`, `"message":"a\nb  c"`, `"read_at":null`, `"done_at":null`} {
		if !strings.Contains(out, key) {
			t.Errorf("detail lacks %s: %s", key, out)
		}
	}
	_, out = call(t, srv, "GET", "/api/hop/feedback/2", "")
	if !strings.Contains(out, `"user":null`) || !strings.Contains(out, `"ip":null`) || !strings.Contains(out, `"has_screenshot":false`) {
		t.Errorf("a guest report with no address and no picture: %s", out)
	}
	for _, bad := range []string{"app=nope", "kind=x", "status=zzz", "limit=abc", "limit=0"} {
		if code, _ = call(t, srv, "GET", "/api/hop/feedback?"+bad, ""); code != 422 {
			t.Errorf("%s answered %d, want 422", bad, code)
		}
	}
	if code, _ = call(t, srv, "GET", "/api/hop/feedback?limit=500", ""); code != 200 {
		t.Errorf("a big limit is lowered, not refused (%d)", code)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/hop/feedback/1/screenshot", nil)
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || res.Header.Get("Cache-Control") != "private, no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("picture: %d %v", res.StatusCode, res.Header)
	}
	if code, _ = call(t, srv, "GET", "/api/hop/feedback/2/screenshot", ""); code != 404 {
		t.Errorf("no picture answered %d", code)
	}
	_ = id
}

func TestFeedbackActions(t *testing.T) {
	f, srv := serveFeedback(t)
	f.Seed(Report{App: "hub", Message: "x"})
	post := func(body string) (int, string) { return call(t, srv, "POST", "/api/hop/feedback/1/actions", body) }
	if code, out := post(`{"action":"read"}`); code != 200 || !strings.Contains(out, `"status":"read"`) {
		t.Errorf("read: %d %s", code, out)
	}
	if code, out := post(`{"action":"done"}`); code != 200 || !strings.Contains(out, `"status":"done"`) || strings.Contains(out, `"done_at":null`) {
		t.Errorf("done: %d %s", code, out)
	}
	if code, out := post(`{"action":"read"}`); code != 200 || !strings.Contains(out, `"status":"done"`) {
		t.Errorf("read only moves new to read: %d %s", code, out)
	}
	if code, out := post(`{"action":"reopen"}`); code != 200 || !strings.Contains(out, `"status":"read"`) || !strings.Contains(out, `"done_at":null`) {
		t.Errorf("reopen: %d %s", code, out)
	}
	if code, out := post(`{"action":"explode"}`); code != 400 || !strings.Contains(out, "bad_action") {
		t.Errorf("unknown action: %d %s", code, out)
	}
	if code, out := post(`{"action":"delete"}`); code != 200 || !strings.Contains(out, `"ok":true`) || f.Len() != 0 {
		t.Errorf("delete: %d %s", code, out)
	}
	if code, out := post(`{"action":"done"}`); code != 404 || !strings.Contains(out, "not_found") {
		t.Errorf("a deleted report: %d %s", code, out)
	}
	f.SetMissing(true)
	if code, out := call(t, srv, "GET", "/api/hop/feedback", ""); code != 404 || strings.Contains(out, `"ok"`) {
		t.Errorf("a hub without the routes: %d %s", code, out)
	}
}
