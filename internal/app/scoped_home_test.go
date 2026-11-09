package app

import (
	"html"
	"strings"
	"testing"
)

// The home page showed HOP ID's own numbers (accounts, new accounts, suspended, the sign-up chart) to an
// admin of one app (dashboard test 2026-11-08, finding #10). Those are hub-wide data: admin:<app> gets the
// cards of its apps and nothing of the hub, and HOP ID is not asked for them on its behalf.
func TestScopedAdminHomeHasNoHubNumbers(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()

	hop.ticket = scopedTicket("27", "dewi", "admin:pixelhop")
	c := client(t)
	body(login(t, c, srv))
	hop.statsCalls = 0
	code, b := get(t, c, srv.URL+"/")
	if code != 200 || !strings.Contains(b, "PixelHop") {
		t.Fatalf("admin:pixelhop home: status %d, PixelHop card present: %v", code, strings.Contains(b, "PixelHop"))
	}
	for _, leak := range []string{html.EscapeString(a.T("home.kpi")), html.EscapeString(a.T("home.attention")), html.EscapeString(a.T("kpi.users_new_7d")), ">74<", html.EscapeString(a.T("series.signups"))} {
		if strings.Contains(b, leak) {
			t.Errorf("admin:pixelhop home shows hub-wide data: %q", leak)
		}
	}
	if hop.statsCalls != 0 {
		t.Errorf("HOP ID was asked for its numbers %d time(s) on behalf of admin:pixelhop", hop.statsCalls)
	}
	if strings.Contains(b, "LinkHop") {
		t.Error("admin:pixelhop home shows another app's card")
	}

	// The hub-wide roles keep them.
	for _, role := range []string{"owner", "admin", "moderator", "support"} {
		hop.ticket = scopedTicket("5", "boss", role)
		c := client(t)
		body(login(t, c, srv))
		hop.statsCalls = 0
		code, b := get(t, c, srv.URL+"/")
		if code != 200 || !strings.Contains(b, html.EscapeString(a.T("home.kpi"))) || !strings.Contains(b, html.EscapeString(a.T("kpi.users_new_7d"))) || !strings.Contains(b, ">74<") || hop.statsCalls != 1 {
			t.Errorf("%s home: status %d, numbers shown: %v, stats calls %d", role, code, strings.Contains(b, ">74<"), hop.statsCalls)
		}
	}
}
