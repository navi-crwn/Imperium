package app

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// IMP-06: uptime_s is a plain JSON number of seconds.
func TestHealthzUptimeIsPlainSeconds(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	a.start = time.Now().Add(-133146 * time.Second)
	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		UptimeS float64 `json:"uptime_s"`
	}
	raw := body(res)
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("healthz is not JSON: %v: %s", err, raw)
	}
	if d := m.UptimeS - 133146; d < -2 || d > 2 {
		t.Fatalf("uptime_s = %v, want about 133146: %s", m.UptimeS, raw)
	}
}
