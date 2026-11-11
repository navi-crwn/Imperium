package fakehub

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T) (*Store, *httptest.Server) {
	s := New("pixelhop")
	m := http.NewServeMux()
	s.Register(m, "/api/hop", func(h Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { b, _ := io.ReadAll(r.Body); h(w, r, b) }
	})
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	return s, srv
}

func call(t *testing.T, srv *httptest.Server, method, path, body string) (int, string) {
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return res.StatusCode, string(b)
}

// The behaviours of the real HOP ID that the first draft of the contract did not name.
func TestMatchesHopIdDetails(t *testing.T) {
	s, srv := serve(t)
	base := `"title":"T","body":"B","level":"info"`
	// Times: offsets and fractions are read, no offset is UTC, output is whole seconds with Z.
	st, out := call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"published_at":"2030-01-01T10:00:00.750+02:00"}`)
	if st != 201 || !strings.Contains(out, `"published_at":"2030-01-01T08:00:00Z"`) {
		t.Errorf("offset: %d %s", st, out)
	}
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"published_at":"2030-01-01T10:00"}`); st != 201 || !strings.Contains(out, `"published_at":"2030-01-01T10:00:00Z"`) {
		t.Errorf("no offset: %d %s", st, out)
	}
	// Validation order: the first problem wins.
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{"title":" ","body":" ","level":"x"}`); st != 422 || !strings.Contains(out, "title is required") {
		t.Errorf("order: %d %s", st, out)
	}
	// publish_now is applied first and expires_at is checked against "now": "publish now, ends later" is valid.
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"publish_now":true,"expires_at":"2099-01-01T00:00:00Z"}`); st != 201 {
		t.Errorf("publish_now with a later end must be accepted: %d %s", st, out)
	}
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"publish_now":true,"expires_at":"2001-01-01T00:00:00Z"}`); st != 422 || !strings.Contains(out, "expires_at must be after published_at") {
		t.Errorf("publish_now with an end in the past: %d %s", st, out)
	}
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"expires_at":"2099-01-01T00:00:00Z"}`); st != 422 {
		t.Errorf("an end without any start: %d %s", st, out)
	}
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"app_slug":5}`); st != 422 {
		t.Errorf("a non-string app_slug: %d %s", st, out)
	}
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"published_at":"2030-02-30T10:00"}`); st != 422 {
		t.Errorf("a date that does not exist: %d %s", st, out)
	}
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"published_at":"2030-01-01T10:00+0200"}`); st != 201 || !strings.Contains(out, `"published_at":"2030-01-01T08:00:00Z"`) {
		t.Errorf("an offset without a colon: %d %s", st, out)
	}
	if st, out = call(t, srv, "GET", "/api/hop/announcements?limit=abc", ""); st != 200 || strings.Count(out, `"title"`) != 1 {
		t.Errorf("a limit that is not a number is read as 0 and lifted to 1: %d %s", st, out)
	}
	// A draft whose end is already past cannot be published (409).
	past := time.Now().Add(-time.Hour)
	id := s.Seed(Row{Title: "D", Body: "b", Level: "info", ExpiresAt: &past})
	if st, out = call(t, srv, "POST", "/api/hop/announcements/"+strconv.Itoa(id)+"/actions", `{"action":"publish"}`); st != 409 || !strings.Contains(out, `"expired"`) {
		t.Errorf("draft with a past end: %d %s", st, out)
	}
	if st, _ = call(t, srv, "POST", "/api/hop/announcements/"+strconv.Itoa(id)+"/actions", `{"action":"explode"}`); st != 422 {
		t.Errorf("unknown action: %d", st)
	}
}
