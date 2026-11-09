package app

import (
	"strings"
	"testing"
)

// The alias replaced an aliased person's name and email, but the username stayed on screen: "@rani" in the
// user list, on the user page and as the owner of content rows, where an admin of one app reads it too
// (dashboard test 2026-11-08, finding #10). The username is as telling as the name; the alias covers it.
func TestAliasHidesTheUsernameToo(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	hubPages := []string{"/users", "/users/26"}
	appPages := []string{"/apps/pixelhop", "/apps/pixelhop/content", "/apps/pixelhop/content/image/img1"}

	// Control: without an alias every one of these pages names the person. This is what makes the checks
	// below mean something (the raw JSON panel, for one, escapes its quotes).
	hop.ticket = scopedTicket("5", "boss", "admin")
	c := client(t)
	body(login(t, c, srv))
	waitForRegistry(t, a, 2)
	for _, path := range append(append([]string{}, hubPages...), appPages...) {
		if code, b := get(t, c, srv.URL+path); code != 200 || !strings.Contains(b, "rani") {
			t.Fatalf("control, admin GET %s: status %d, names the person: %v", path, code, strings.Contains(b, "rani"))
		}
	}

	a.cfg.Aliases = envAliasesFrom("rani=superadmin")
	shown := func(who, path, b string) {
		t.Helper()
		for _, leak := range []string{"rani", "Rani", "r@x"} {
			if strings.Contains(b, leak) {
				t.Errorf("%s GET %s shows %q of an aliased person", who, path, leak)
			}
		}
		if !strings.Contains(b, "superadmin") {
			t.Errorf("%s GET %s does not show the alias", who, path)
		}
	}
	for _, path := range append(append([]string{}, hubPages...), appPages...) {
		code, b := get(t, c, srv.URL+path)
		if code != 200 {
			t.Fatalf("admin GET %s: status %d", path, code)
		}
		shown("admin", path, b)
	}

	// An admin of one app reads content owners as well.
	hop.ticket = scopedTicket("27", "dewi", "admin:pixelhop")
	c = client(t)
	body(login(t, c, srv))
	for _, path := range appPages {
		code, b := get(t, c, srv.URL+path)
		if code != 200 {
			t.Fatalf("admin:pixelhop GET %s: status %d", path, code)
		}
		shown("admin:pixelhop", path, b)
	}
}
