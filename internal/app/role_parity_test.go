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

func TestSameAccountReadsIdsAsNumbers(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"26", "26", true}, {"026", "26", true}, {"0026", " 26", true}, {"0", "000", true}, {"26", "27", false}, {"", "", false}, {"26", "", false}, {"abc", "abc", true}, {"abc", "ABC", false}, {"2 6", "26", false}} {
		if got := sameAccount(c.a, c.b); got != c.want {
			t.Errorf("sameAccount(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestSelfChecksNormaliseTheId(t *testing.T) {
	e := newAnnEnv(t, "admin")
	e.hop.ticket = scopedTicket("026", "rani", "admin") // signed in as 26, written with a leading zero
	c := client(t)
	body(login(t, c, e.srv))
	e.c = c
	for _, act := range []string{"suspend", "delete"} {
		if _, _ = e.userPost(t, url.Values{"action": {act}, "reason": {"oops"}, "ack": {"yes"}, "confirm": {"@rani"}}); len(e.hop.actions) != 0 {
			t.Errorf("%s on yourself (026 = 26) reached HOP ID: %v", act, upstreamActions(e))
		}
	}
	e.hop.actions = nil
	e.post(t, "/users/actions", url.Values{"action": {"suspend"}, "pick": {"26"}, "confirm": {"yes"}, "reason": {"wave"}})
	if len(e.hop.actions) != 0 {
		t.Errorf("bulk suspend skipped nobody: %v", upstreamActions(e))
	}
	if st, _, _ := e.post(t, "/users/actions", url.Values{"action": {"suspend"}, "pick": {"0026"}, "confirm": {"yes"}, "reason": {"wave"}}); st != http.StatusFound || len(e.hop.actions) != 0 {
		t.Errorf("bulk suspend with 0026: %v", upstreamActions(e))
	}
}

// HOP ID already deletes the tokens when it suspends; a refusal of the extra revoke_tokens is not a failure.
func TestSuspendIsDoneEvenIfTheTokenFollowUpIsRefused(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.hop.failAction = "revoke_tokens"
	e.hop.actions = nil
	res, err := e.c.PostForm(e.srv.URL+"/users/26/actions", url.Values{"action": {"suspend"}, "reason": {"abuse"}, "ack": {"yes"}})
	if err != nil {
		t.Fatal(err)
	}
	page := body(res)
	if !strings.Contains(page, "Done.") || strings.Contains(page, "failed") {
		t.Errorf("the suspension succeeded and must read as done")
	}
	if n, _, _ := auditCount(t, e.a, "user.suspend"); n != 1 {
		t.Errorf("suspend audit %d", n)
	}
}

func TestUserPageListsTokensWithoutSecrets(t *testing.T) {
	e := newAnnEnv(t, "owner")
	_, page := e.get(t, "/users/26?tab=sessions")
	for _, want := range []string{"API tokens", "deploy script", "never used", "2026-11-10"} {
		if !strings.Contains(page, want) {
			t.Errorf("sessions tab lacks %q", want)
		}
	}
}

// HOP ID refuses suspend, delete and set_roles unless X-HOP-Actor is a number without leading zeros, optionally
// followed by ":<username>" with no space or hyphen. Every user action Imperium sends, single or bulk, has that shape.
func TestEveryUserActionCarriesAnActorHopIdAccepts(t *testing.T) {
	for _, tc := range []struct{ sub, user, want string }{{"5", "boss", "5:boss"}, {"05", "boss", "5:boss"}, {"5", "a-b", "5"}, {"005", "o brien", "5"}, {"5", "", "5"}} {
		e := newAnnEnv(t, "owner")
		e.hop.ticket = scopedTicket(tc.sub, tc.user, "owner")
		c := client(t)
		body(login(t, c, e.srv))
		e.c = c
		e.hop.actions = nil
		for _, v := range []url.Values{
			{"action": {"suspend"}, "reason": {"r"}, "ack": {"yes"}},
			{"action": {"set_roles"}, "roles": {"support"}, "reason": {"r"}},
			{"action": {"delete"}, "reason": {"r"}, "ack": {"yes"}, "confirm": {"@rani"}},
		} {
			e.post(t, "/users/26/actions", v)
		}
		e.post(t, "/users/actions", url.Values{"action": {"suspend"}, "pick": {"26"}, "confirm": {"yes"}, "reason": {"r"}})
		if len(e.hop.actions) < 5 {
			t.Fatalf("%+v: only %d actions reached HOP ID", tc, len(e.hop.actions))
		}
		for _, a := range e.hop.actions {
			if a["actor"] != tc.want || !actorShape.MatchString(fmt.Sprint(a["actor"])) {
				t.Errorf("%+v: action %v carried actor %v, want %s", tc, a["action"], a["actor"], tc.want)
			}
			if a["error"] != nil {
				t.Errorf("unexpected %v", a)
			}
		}
	}
	if got := actorOf(nil); got != "system" {
		t.Errorf("no session: %s", got)
	}
}
