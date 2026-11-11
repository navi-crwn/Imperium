package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newContentBulkSetup: a LinkHop-shaped app with one content type ("link") whose
// manifest lists both plain and parameter actions, three rows, and a record of
// every action call. Ids in fail answer 502.
func newContentBulkSetup(t *testing.T, fail map[string]bool) (*fakeHop, *bulkUpstream, *App, *httptest.Server) {
	t.Helper()
	hop := newFakeHop(t)
	up := &bulkUpstream{fail: fail}
	api := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, body, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				return
			}
			h(w, r, body)
		}
	}
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api"],"content_types":[{"type":"link","label":"Links","actions":["disable","enable","transfer","set_expiry","delete"]}]}}`))
	}))
	mux.HandleFunc("GET /api/hop/content", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[
			{"type":"link","id":"11","title":"kit-1","status":"live"},
			{"type":"link","id":"12","title":"kit-2","status":"live"},
			{"type":"link","id":"13","title":"ok","status":"live"}],"page":{"total":3}}`))
	}))
	mux.HandleFunc("POST /api/hop/content/link/{id}/actions", api(up.handle))
	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	t.Cleanup(hop.srv.Close)
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	return hop, up, a, srv
}

func TestContentPageOffersBulkWithoutParamActions(t *testing.T) {
	_, _, _, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content?type=link"))
	for _, want := range []string{`id="bulk-form"`, `action="/apps/linkhop/content/actions"`, `name="pick" value="11"`, `name="pick" value="13"`, `<option value="disable">`, `<option value="delete">`, `data-bulk-all`} {
		if !strings.Contains(page, want) {
			t.Errorf("content page missing %q", want)
		}
	}
	for _, not := range []string{`<option value="transfer">`, `<option value="set_expiry">`} {
		if strings.Contains(page, not) {
			t.Errorf("an action that needs a value per item must not be offered in bulk: %s", not)
		}
	}
	if strings.Count(page, "<form") != strings.Count(page, "</form>") {
		t.Errorf("unbalanced forms")
	}
}

func TestContentBulkActionPartialFailureAndAudit(t *testing.T) {
	_, up, a, srv := newContentBulkSetup(t, map[string]bool{"12": true})
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{
		"type": {"link"}, "pick": {"11", "12", "11", " "}, "action": {"disable"}, "confirm": {"yes"}, "reason": {"kit sweep"}, "q": {"kit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if got := res.Request.URL.String(); !strings.Contains(got, "/apps/linkhop/content?") || !strings.Contains(got, "q=kit") || !strings.Contains(got, "type=link") {
		t.Errorf("should return to the same filtered list, got %s", got)
	}
	calls := up.seen()
	if len(calls) != 2 {
		t.Fatalf("two distinct ids, two calls expected, got %d: %+v", len(calls), calls)
	}
	if calls[0]["path"] != "/api/hop/content/link/11/actions" || calls[0]["action"] != "disable" || calls[0]["reason"] != "kit sweep" {
		t.Errorf("call not forwarded as on the item page: %+v", calls[0])
	}
	audits, _ := a.db.Audits(t.Context(), 0, 20)
	var ok, failed int
	for _, au := range audits {
		if au.Action != "link.disable" {
			continue
		}
		if au.OK {
			ok++
		} else {
			failed++
		}
	}
	if ok != 1 || failed != 1 {
		t.Fatalf("one audit line per item expected (1 ok, 1 failed), got %d ok %d failed", ok, failed)
	}
}

func TestContentBulkActionRefusals(t *testing.T) {
	_, up, _, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	cases := []url.Values{
		{"type": {"link"}, "pick": {"11"}, "action": {"disable"}},                      // destructive, no confirmation
		{"type": {"link"}, "pick": {"11"}, "action": {"transfer"}, "confirm": {"yes"}}, // needs a value per item
		{"type": {"link"}, "pick": {"11"}, "action": {"shutdown"}, "confirm": {"yes"}}, // not in the manifest
		{"type": {"photo"}, "pick": {"11"}, "action": {"disable"}, "confirm": {"yes"}}, // unknown type
		{"type": {"link"}, "action": {"enable"}},                                       // nothing picked
	}
	over := url.Values{"type": {"link"}, "action": {"enable"}}
	for i := 0; i <= maxBulk; i++ {
		over.Add("pick", "x"+strings.Repeat("y", i%5)+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	cases = append(cases, over)
	for i, v := range cases {
		res, err := c.PostForm(srv.URL+"/apps/linkhop/content/actions", v)
		if err != nil {
			t.Fatal(err)
		}
		_ = body(res)
		if n := len(up.seen()); n != 0 {
			t.Fatalf("case %d must not reach the app, got %d calls", i, n)
		}
	}
	// enable is not destructive: no confirmation needed.
	res, err := c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "pick": {"13"}, "action": {"enable"}, "reason": {"back on"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if n := len(up.seen()); n != 1 {
		t.Fatalf("enable should run once, got %d calls", n)
	}
}

func TestContentBulkHiddenForReadOnlyRole(t *testing.T) {
	hop, up, _, srv := newContentBulkSetup(t, map[string]bool{})
	hop.ticket["roles"] = []string{"support"}
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content?type=link"))
	if strings.Contains(page, `id="bulk-form"`) || strings.Contains(page, `bulk-pick`) {
		t.Fatal("support is read-only: no bulk controls")
	}
	res, err := c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "pick": {"11"}, "action": {"enable"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if n := len(up.seen()); n != 0 {
		t.Fatalf("support must not act, got %d calls", n)
	}
}

// usersBulkHop adds users 28 (normal) and 99 (superadmin) to the fake HOP ID and
// records every account action by sub.
func usersBulkHop(t *testing.T) (*fakeHop, *[]string, *App, *httptest.Server) {
	t.Helper()
	hop := newFakeHop(t)
	var acted []string
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	signed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, b, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				return
			}
			r.Body = io.NopCloser(strings.NewReader(string(b)))
			h(w, r)
		}
	}
	mux.HandleFunc("POST /api/hop/users/{sub}/actions", signed(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		acted = append(acted, r.PathValue("sub")+":"+m["action"].(string))
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	hop.member = func(sub string) (int, string) {
		switch sub {
		case "28":
			return 200, `{"ok":true,"data":{"profile":{"sub":"28","username":"tono","status":"active"},"roles":[]}}`
		case "99":
			return 200, `{"ok":true,"data":{"profile":{"sub":"99","username":"root","status":"active","owner":true},"roles":["owner"]}}`
		}
		return 404, `{"ok":false,"error":"not_found"}`
	}
	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	t.Cleanup(hop.srv.Close)
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	return hop, &acted, a, srv
}

func TestUsersPageOffersBulkToAdmins(t *testing.T) {
	_, _, _, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/users"))
	for _, want := range []string{`id="bulk-form"`, `action="/users/actions"`, `name="pick" value="26"`, `<option value="suspend">`, `<option value="revoke_sessions">`} {
		if !strings.Contains(page, want) {
			t.Errorf("users page missing %q", want)
		}
	}
	for _, not := range []string{`<option value="delete">`, `<option value="set_roles">`} {
		if strings.Contains(page, not) {
			t.Errorf("%s must stay on the user page", not)
		}
	}
}

func TestUsersBulkSkipsSelfAndSuperadmin(t *testing.T) {
	hop, acted, a, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/users/actions", url.Values{
		"pick": {"26", "28", "99", "1"}, "action": {"suspend"}, "confirm": {"yes"}, "reason": {"spam wave"}, "status": {"active"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if got := res.Request.URL.String(); !strings.Contains(got, "/users?") || !strings.Contains(got, "status=active") {
		t.Errorf("should return to the filtered list, got %s", got)
	}
	// 26 goes to fakeHop's own handler (more specific pattern), 28 to ours.
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "suspend" || hop.actions[0]["reason"] != "spam wave" {
		t.Errorf("user 26 not suspended once: %+v", hop.actions)
	}
	if strings.Join(*acted, ",") != "28:suspend" {
		t.Errorf("only 28 should reach the generic handler, got %v", *acted)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 20)
	var ok, skipped int
	for _, au := range audits {
		if au.Action != "user.suspend" {
			continue
		}
		if au.OK {
			ok++
		} else {
			skipped++
		}
	}
	if ok != 2 || skipped != 2 {
		t.Fatalf("2 applied and 2 skipped (self, superadmin) audit lines expected, got %d ok %d not", ok, skipped)
	}
}

func TestUsersBulkRefusals(t *testing.T) {
	hop, acted, _, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	for i, v := range []url.Values{
		{"pick": {"28"}, "action": {"suspend"}},                       // no confirmation
		{"pick": {"28"}, "action": {"revoke_sessions"}},               // no confirmation
		{"pick": {"28"}, "action": {"delete"}, "confirm": {"yes"}},    // not a bulk action
		{"pick": {"28"}, "action": {"set_roles"}, "confirm": {"yes"}}, // not a bulk action
		{"action": {"activate"}},                                      // nothing picked
	} {
		res, err := c.PostForm(srv.URL+"/users/actions", v)
		if err != nil {
			t.Fatal(err)
		}
		_ = body(res)
		if len(*acted) != 0 || len(hop.actions) != 0 {
			t.Fatalf("case %d must not reach HOP ID: %v %v", i, *acted, hop.actions)
		}
	}
	// A moderator sees users but may not run account actions.
	hop.ticket["roles"] = []string{"moderator"}
	c2 := client(t)
	body(login(t, c2, srv))
	if page := body(bulkGet(t, c2, srv.URL+"/users")); strings.Contains(page, `id="bulk-form"`) {
		t.Error("moderators get no account bulk controls")
	}
	res, err := c2.PostForm(srv.URL+"/users/actions", url.Values{"pick": {"28"}, "action": {"activate"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if len(*acted) != 0 {
		t.Fatalf("a moderator must not act on accounts: %v", *acted)
	}
}
