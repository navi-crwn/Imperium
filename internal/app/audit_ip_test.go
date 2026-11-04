package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/navi-crwn/imperium/internal/db"
)

// lastAudit returns the newest audit row with the given action.
func lastAudit(t *testing.T, a *App, action string) db.Audit {
	t.Helper()
	rows, err := a.db.Audits(t.Context(), 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Action == action {
			return r
		}
	}
	t.Fatalf("no %s audit row among %d rows", action, len(rows))
	return db.Audit{}
}

// noteWithXFF posts a user note as an admin with the given X-Forwarded-For
// header lines and returns the IP the audit row recorded.
func noteWithXFF(t *testing.T, trust bool, xff ...string) string {
	t.Helper()
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	a.cfg.TrustProxy = trust
	hop.ticket = map[string]any{"sub": "26", "username": "rani", "name": "Rani", "email": "r@x", "roles": []string{"admin"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	req, _ := http.NewRequest("POST", srv.URL+"/users/26/actions", strings.NewReader(url.Values{"action": {"note"}, "text": {"x"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, v := range xff {
		req.Header.Add("X-Forwarded-For", v)
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	return lastAudit(t, a, "user.note").IP
}

// IMP-02: nginx overwrites X-Forwarded-For with $remote_addr, so the audit IP
// is the last entry; anything before it was typed by the client.
func TestAuditIPIgnoresClientPlantedXFF(t *testing.T) {
	if ip := noteWithXFF(t, true, "203.0.113.9, 198.51.100.7"); ip != "198.51.100.7" {
		t.Fatalf("audit IP %q, want the last X-Forwarded-For entry 198.51.100.7", ip)
	}
	if ip := noteWithXFF(t, true, "203.0.113.9", "198.51.100.7"); ip != "198.51.100.7" {
		t.Fatalf("audit IP %q with two header lines, want 198.51.100.7", ip)
	}
}

func TestAuditIPKeepsPeerWhenXFFIsNotAnIP(t *testing.T) {
	ip := noteWithXFF(t, true, "<script>")
	if ip != "127.0.0.1" {
		t.Fatalf("audit IP %q, want the peer address 127.0.0.1 when the header is not an IP", ip)
	}
}

func TestAuditIPIgnoresXFFWithoutTrustProxy(t *testing.T) {
	if ip := noteWithXFF(t, false, "203.0.113.9"); ip != "127.0.0.1" {
		t.Fatalf("audit IP %q, want the peer address 127.0.0.1 when TrustProxy is off", ip)
	}
}
