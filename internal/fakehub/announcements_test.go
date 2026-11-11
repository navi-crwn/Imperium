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
	// expires_at is checked against the submitted published_at, before publish_now replaces it.
	if st, out = call(t, srv, "POST", "/api/hop/announcements", `{`+base+`,"publish_now":true,"expires_at":"2031-01-01T00:00:00Z"}`); st != 422 || !strings.Contains(out, "expires_at must be after published_at") {
		t.Errorf("publish_now without published_at: %d %s", st, out)
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
