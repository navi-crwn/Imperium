package app

import (
	"net/url"
	"strings"
	"testing"
)

// IMP-19: support is read-only, so it cannot post a user note either.
func TestSupportCannotPostUserNote(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "cs", "support")
	c := client(t)
	body(login(t, c, srv))
	res, _ := c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"note"}, "text": {"hello"}})
	if b := body(res); !strings.Contains(b, "may not") || len(hop.actions) != 0 {
		t.Fatalf("support note must be refused before reaching HOP ID; upstream actions: %+v", hop.actions)
	}
	res, _ = c.Get(srv.URL + "/users/26")
	if b := body(res); strings.Contains(b, `value="note"`) {
		t.Fatal("support is offered the note form")
	}
	// Admin and moderator can still write one.
	hop.actions = nil
	for _, role := range []string{"admin", "moderator"} {
		hop.actions = nil
		hop.ticket = scopedTicket("1", "boss", role)
		c2 := client(t)
		body(login(t, c2, srv))
		res, _ = c2.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"note"}, "text": {"hello"}})
		body(res)
		if len(hop.actions) != 1 || hop.actions[0]["action"] != "note" {
			t.Fatalf("%s note not forwarded: %+v", role, hop.actions)
		}
	}
}
