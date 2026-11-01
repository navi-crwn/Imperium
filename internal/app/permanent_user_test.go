package app

import (
	"net/url"
	"strings"
	"testing"
)

func TestPermanentOwnerHasNoActionsAndCannotBeMutated(t *testing.T) {
	for _, actor := range []string{"1", "26"} {
		t.Run(actor, func(t *testing.T) {
			hop := newFakeHop(t)
			defer hop.srv.Close()
			hop.protectedUser = true
			_, srv := newApp(t, hop)
			defer srv.Close()
			hop.ticket = map[string]any{"sub": actor, "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
			c := client(t)
			body(login(t, c, srv))
			res, _ := c.Get(srv.URL + "/users/26")
			html := body(res)
			if !strings.Contains(html, "permanent and read-only") || strings.Contains(html, `action="/users/26/actions"`) {
				t.Fatal("owner actions are not read-only")
			}
			for _, action := range []string{"suspend", "activate", "delete", "set_roles", "revoke_sessions", "revoke_tokens", "note"} {
				res, _ = c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {action}, "text": {"note"}})
				if html := body(res); !strings.Contains(html, "permanent and read-only") {
					t.Fatalf("%s refusal missing", action)
				}
			}
			if len(hop.actions) != 0 {
				t.Fatalf("protected actions forwarded: %v", hop.actions)
			}
		})
	}
}
