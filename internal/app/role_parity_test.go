package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The user actions of HOP ID's own /admin pages, and what Imperium does the same way (docs/announcements-contract.md,
// "role parity"): only an owner changes roles, ownership is never given here, nobody suspends or deletes their own
// account, and suspending also takes the API tokens, as HOP ID's page does.

func (e *annEnv) userPost(t *testing.T, v url.Values) (int, string) {
	t.Helper()
	e.hop.actions = nil
	st, loc, _ := e.post(t, "/users/26/actions", v)
	return st, loc
}

func upstreamActions(e *annEnv) []string {
	var out []string
	for _, a := range e.hop.actions {
		out = append(out, fmt.Sprint(a["action"]))
	}
	return out
}

func TestOnlyAnOwnerChangesRoles(t *testing.T) {
	e := newAnnEnv(t, "owner")
	if st, _ := e.userPost(t, url.Values{"action": {"set_roles"}, "roles": {"support", "admin:pixelhop"}, "reason": {"rota"}}); st != http.StatusFound || fmt.Sprint(upstreamActions(e)) != "[set_roles]" {
		t.Fatalf("an owner sets roles: %d %v", st, upstreamActions(e))
	}
	if params, _ := e.hop.actions[0]["params"].(map[string]any); fmt.Sprint(params["roles"]) != "[support admin:pixelhop]" || e.hop.actions[0]["reason"] != "rota" {
		t.Errorf("sent %+v", e.hop.actions[0])
	}
	if n, d, target := auditCount(t, e.a, "user.set_roles"); n != 1 || target != "26" || d["reason"] != "rota" {
		t.Errorf("audit n=%d target=%s detail=%v", n, target, d)
	}

	a := newAnnEnv(t, "admin")
	_, page := a.get(t, "/users/26")
	if strings.Contains(page, `value="set_roles"`) || !strings.Contains(page, `name="roles" value="support" disabled`) {
		t.Error("an admin sees the roles, read-only, without a Save button")
	}
	if st, _ := a.userPost(t, url.Values{"action": {"set_roles"}, "roles": {"support"}, "reason": {"x"}}); st != http.StatusFound || len(a.hop.actions) != 0 {
		t.Errorf("an admin changed roles: %d %v", st, upstreamActions(a))
	}
}

func TestRolesFormAsksForAReasonAndNeverGivesOwnership(t *testing.T) {
	e := newAnnEnv(t, "owner")
	_, page := e.get(t, "/users/26")
	if !strings.Contains(page, `id="roles-reason" type="text" name="reason" required`) {
		t.Error("the roles form asks for a reason")
	}
	if !strings.Contains(page, `name="roles" value="owner" disabled`) {
		t.Error("the owner checkbox is shown but cannot be ticked")
	}
	if st, _ := e.userPost(t, url.Values{"action": {"set_roles"}, "roles": {"support"}}); st != http.StatusFound || len(e.hop.actions) != 0 {
		t.Errorf("no reason, no change: %v", upstreamActions(e))
	}
	if st, _ := e.userPost(t, url.Values{"action": {"set_roles"}, "roles": {"support"}, "reason": {"  "}}); st != http.StatusFound || len(e.hop.actions) != 0 {
		t.Errorf("a blank reason is no reason: %v", upstreamActions(e))
	}
	// A hand-made request that tries to give ownership is refused before HOP ID is asked (its API would allow it).
	if st, _ := e.userPost(t, url.Values{"action": {"set_roles"}, "roles": {"owner", "admin"}, "reason": {"sneaky"}}); st != http.StatusFound || len(e.hop.actions) != 0 {
		t.Errorf("ownership was sent to HOP ID: %v", upstreamActions(e))
	}
	if n, _, _ := auditCount(t, e.a, "user.set_roles"); n != 0 {
		t.Errorf("refused role changes are not audited (%d)", n)
	}
}

func TestNobodySuspendsOrDeletesTheirOwnAccount(t *testing.T) {
	e := newAnnEnv(t, "admin")
	e.hop.ticket = scopedTicket("26", "rani", "admin") // the fake's user 26 is the signed-in admin
	c := client(t)
	body(login(t, c, e.srv))
	e.c = c
	for _, act := range []string{"suspend", "delete"} {
		if st, _ := e.userPost(t, url.Values{"action": {act}, "reason": {"oops"}, "ack": {"yes"}, "confirm": {"@rani"}}); st != http.StatusFound || len(e.hop.actions) != 0 {
			t.Errorf("%s on yourself reached HOP ID: %v", act, upstreamActions(e))
		}
		if n, _, _ := auditCount(t, e.a, "user."+act); n != 0 {
			t.Errorf("%s refused, so nothing is audited", act)
		}
	}
	// Other actions on yourself stay possible (HOP ID's page allows them too).
	if _, _ = e.userPost(t, url.Values{"action": {"revoke_sessions"}, "reason": {"lost laptop"}}); fmt.Sprint(upstreamActions(e)) != "[revoke_sessions]" {
		t.Errorf("revoke_sessions on yourself: %v", upstreamActions(e))
	}
}

func TestSuspendAlsoRevokesApiTokens(t *testing.T) {
	e := newAnnEnv(t, "owner")
	if st, _ := e.userPost(t, url.Values{"action": {"suspend"}, "reason": {"abuse"}, "ack": {"yes"}}); st != http.StatusFound {
		t.Fatalf("status %d", st)
	}
	if got := fmt.Sprint(upstreamActions(e)); got != "[suspend revoke_tokens]" {
		t.Fatalf("suspend must take the tokens too, upstream: %s", got)
	}
	if e.hop.actions[1]["reason"] != "abuse" {
		t.Errorf("the token call carries the reason: %+v", e.hop.actions[1])
	}
	if n, d, _ := auditCount(t, e.a, "user.revoke_tokens"); n != 1 || d["with"] != "suspend" {
		t.Errorf("the second step has its own audit line: %d %v", n, d)
	}
	if n, _, _ := auditCount(t, e.a, "user.suspend"); n != 1 {
		t.Errorf("suspend audit: %d", n)
	}
	// Activate does not touch tokens.
	e.userPost(t, url.Values{"action": {"activate"}, "reason": {"appeal"}})
	if got := fmt.Sprint(upstreamActions(e)); got != "[activate]" {
		t.Errorf("activate upstream: %s", got)
	}
	// Bulk suspend does the same for every account it suspends.
	e.hop.actions = nil
	st, _, _ := e.post(t, "/users/actions", url.Values{"action": {"suspend"}, "pick": {"26"}, "confirm": {"yes"}, "reason": {"spam wave"}})
	if st != http.StatusFound || fmt.Sprint(upstreamActions(e)) != "[suspend revoke_tokens]" {
		t.Errorf("bulk suspend upstream: %v", upstreamActions(e))
	}
}
