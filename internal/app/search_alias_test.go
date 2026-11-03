package app

import (
	"strings"
	"testing"
)

// The alias hides the owner's real identity on every screen, search results included (IMP-08).
func TestSearchHidesAliasedIdentity(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	a.cfg.Aliases = envAliasesFrom("rani=superadmin")
	hop.searchLabel = "@rani Rani Dewi <r@x>" // what HOP ID returns: @username name <email>
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	_, b := get(t, c, srv.URL+"/search?q=rani")
	if strings.Contains(b, "r@x") || strings.Contains(b, "Rani") {
		t.Fatalf("search leaks the aliased identity:\n%s", b)
	}
	if !strings.Contains(b, "superadmin") {
		t.Fatal("search should show the alias")
	}
	hop.searchLabel = "@budi Budi <b@x>"
	_, b = get(t, c, srv.URL+"/search?q=budi")
	if !strings.Contains(b, "@budi Budi &lt;b@x&gt;") {
		t.Fatalf("a person without an alias must keep the full label:\n%s", b)
	}
}
