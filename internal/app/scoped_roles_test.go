package app

import (
	"net/url"
	"strings"
	"testing"
)

func scopedTicket(sub, name string, roles ...string) map[string]any {
	return map[string]any{"sub": sub, "username": name, "name": name, "email": name + "@x", "roles": roles, "mfa": true, "status": "active"}
}

// IMP-03: admin:<app> administers that app only (hub contract, HEL.ink
// User.php roleNames). It must not reach hub user data or other apps.
func TestScopedAdminHasNoHubUserAccess(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("27", "dewi", "admin:linkhop")
	c := client(t)
	body(login(t, c, srv))
	for _, path := range []string{"/users", "/users/26", "/search?q=rani", "/audit"} {
		res, _ := c.Get(srv.URL + path)
		b := body(res)
		if res.StatusCode != 403 || strings.Contains(b, "@rani") {
			t.Errorf("admin:linkhop GET %s: status %d, want 403 without user data", path, res.StatusCode)
		}
	}
	hop.actions = nil
	res, _ := c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"note"}, "text": {"x"}})
	body(res)
	if len(hop.actions) != 0 {
		t.Fatalf("admin:linkhop reached HOP ID with a user action: %+v", hop.actions)
	}
}

func TestScopedAdminActsOnItsOwnAppOnly(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	post := func(roles ...string) int {
		hop.ticket = scopedTicket("27", "dewi", roles...)
		c := client(t)
		body(login(t, c, srv))
		hop.actions = nil
		res, _ := c.PostForm(srv.URL+"/apps/pixelhop/reports/r1/actions", url.Values{"action": {"dismiss"}, "reason": {"fine"}})
		body(res)
		return len(hop.actions)
	}
	if n := post("admin:linkhop"); n != 0 {
		t.Fatalf("admin:linkhop dismissed a pixelhop report (%d upstream actions)", n)
	}
	if n := post("admin:pixelhop"); n != 1 {
		t.Fatalf("admin:pixelhop must keep acting on pixelhop reports, got %d upstream actions", n)
	}
	for _, role := range []string{"moderator", "admin", "owner"} {
		if n := post(role); n != 1 {
			t.Fatalf("%s must act on any app, got %d upstream actions", role, n)
		}
	}
}

func TestScopedAdminSeesNoActionButtonsElsewhere(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("27", "dewi", "admin:linkhop")
	c := client(t)
	body(login(t, c, srv))
	res, _ := c.Get(srv.URL + "/apps/pixelhop/reports")
	if b := body(res); strings.Contains(b, `value="dismiss"`) {
		t.Fatal("admin:linkhop is offered report actions on pixelhop")
	}
	hop.ticket = scopedTicket("28", "eka", "admin:pixelhop")
	c2 := client(t)
	body(login(t, c2, srv))
	res, _ = c2.Get(srv.URL + "/apps/pixelhop/reports")
	if b := body(res); !strings.Contains(b, `value="dismiss"`) {
		t.Fatal("admin:pixelhop lost report actions on its own app")
	}
}

func TestGlobalRolesKeepHubUserRead(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	for _, role := range []string{"owner", "admin", "moderator", "support"} {
		hop.ticket = scopedTicket("5", "cs", role)
		c := client(t)
		body(login(t, c, srv))
		for _, path := range []string{"/users", "/users/26", "/search?q=rani", "/audit"} {
			res, _ := c.Get(srv.URL + path)
			body(res)
			if res.StatusCode != 200 {
				t.Errorf("%s GET %s: status %d, want 200", role, path, res.StatusCode)
			}
		}
	}
}
