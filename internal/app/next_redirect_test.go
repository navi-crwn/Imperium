package app

import (
	"strings"
	"testing"
)

// IMP-05: the sign-in redirect must escape next so its query survives.
func TestMemberRedirectKeepsQueryOfNext(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("26", "rani", "admin")
	c := client(t)
	res, _ := c.Get(srv.URL + "/search?q=a&x=1")
	if got := res.Request.URL.Query().Get("next"); got != "/search?q=a&x=1" {
		t.Fatalf("login next = %q, want /search?q=a&x=1 (url %s)", got, res.Request.URL)
	}
	if res.Request.URL.Query().Get("x") != "" {
		t.Fatalf("the tail of next leaked into /login parameters: %s", res.Request.URL)
	}
	page := body(res)
	i := strings.Index(page, `href="/auth/sso?next=`)
	if i < 0 {
		t.Fatal("sign-in button not found")
	}
	rest := page[i+len(`href="/auth/sso?next=`):]
	res2, _ := c.Get(srv.URL + "/auth/sso?next=" + rest[:strings.Index(rest, `"`)])
	if res2.Request.URL.Path != "/search" || res2.Request.URL.RawQuery != "q=a&x=1" {
		t.Fatalf("after sign-in landed on %s, want /search?q=a&x=1", res2.Request.URL)
	}
}
