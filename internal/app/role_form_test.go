package app

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/navi-crwn/imperium/internal/db"
)

func TestRoleOptions(t *testing.T) {
	apps := []db.App{{Slug: "linkhop"}, {Slug: "pixelhop"}, {Slug: ""}}
	got := roleOptions(apps, []string{"admin", "admin:pixelhop", "admin:retiredapp"})
	want := []string{"owner", "admin", "moderator", "support", "admin:linkhop", "admin:pixelhop", "admin:retiredapp"}
	if !slices.Equal(got, want) {
		t.Fatalf("roleOptions = %v, want %v", got, want)
	}
}

// The role form offered only the four hub-wide roles, so admin:<app> could be given only from a console on
// the hub (dashboard test 2026-11-08, finding #10). The form lists admin:<app> for every registered app;
// saving sends it to HOP ID, and only an owner or admin may save.
func TestScopedAdminRoleCanBeGivenFromTheUserPage(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()

	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	_, page := get(t, c, srv.URL+"/users/26")
	for _, role := range []string{"owner", "admin", "moderator", "support", "admin:pixelhop", "admin:linkhop"} {
		if !strings.Contains(page, `name="roles" value="`+role+`"`) {
			t.Errorf("user page lacks the role checkbox %s", role)
		}
	}
	if _, list := get(t, c, srv.URL+"/users"); !strings.Contains(list, ">admin:pixelhop</option>") {
		t.Error("the role filter of the user list lacks admin:pixelhop")
	}

	hop.actions = nil
	res, err := c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"set_roles"}, "roles": {"moderator", "admin:pixelhop"}})
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "set_roles" {
		t.Fatalf("owner set_roles: upstream actions %+v", hop.actions)
	}
	params, _ := hop.actions[0]["params"].(map[string]any)
	if got := fmt.Sprint(params["roles"]); got != "[moderator admin:pixelhop]" {
		t.Errorf("roles sent to HOP ID: %s, want [moderator admin:pixelhop]", got)
	}

	// Not for the roles below admin, and not for an admin of one app.
	for _, role := range []string{"moderator", "support", "admin:pixelhop"} {
		hop.ticket = scopedTicket("27", "dewi", role)
		c := client(t)
		body(login(t, c, srv))
		hop.actions = nil
		res, err := c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"set_roles"}, "roles": {"admin:pixelhop"}})
		if err != nil {
			t.Fatal(err)
		}
		body(res)
		if len(hop.actions) != 0 {
			t.Errorf("%s changed roles: %+v", role, hop.actions)
		}
	}
}
