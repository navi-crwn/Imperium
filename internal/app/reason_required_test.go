package app

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

// Every action on a row opens the confirmation dialog, and a reason is mandatory: the field is required in the
// page, and the server refuses a request without one (empty after trimming) before anything is sent or logged.

// auditActions counts the audit lines of one action name and returns the detail and target of the newest.
func auditCount(t *testing.T, a *App, action string) (n int, last map[string]any, target string) {
	t.Helper()
	rows, err := a.db.Audits(t.Context(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Action == action {
			if n == 0 {
				last, target = r.Detail, r.Target
			}
			n++
		}
	}
	return
}

// dialogOf returns the markup of the dialog with this id.
func dialogOf(t *testing.T, page, id string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<dialog class="confirm" id="` + regexp.QuoteMeta(id) + `".*?</dialog>`)
	m := re.FindString(page)
	if m == "" {
		t.Fatalf("no dialog %s", id)
	}
	return m
}

func TestContentRowActionNeedsAReason(t *testing.T) {
	_, up, a, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content?type=link"))
	d := dialogOf(t, page, "dlg-0-link-11-disable")
	if !strings.Contains(page, `href="#dlg-0-link-11-disable"`) || !strings.Contains(d, `name="reason" required`) {
		t.Fatal("a non-destructive action should open a dialog with a required reason")
	}
	if strings.Contains(d, "destructive") {
		t.Error("a non-destructive dialog must not be red")
	}
	post, vals := menuForm(t, page, "disable", "/apps/linkhop/content/link/11/actions")
	for _, reason := range []string{"", "   "} {
		vals.Set("reason", reason)
		res, err := c.PostForm(srv.URL+post, vals)
		if err != nil {
			t.Fatal(err)
		}
		body(res)
	}
	if len(up.seen()) != 0 {
		t.Fatalf("an action without a reason reached the app: %+v", up.seen())
	}
	if n, _, _ := auditCount(t, a, "link.disable"); n != 0 {
		t.Fatalf("a refused action left %d audit lines", n)
	}
	vals.Set("reason", "  spam wave ")
	res, _ := c.PostForm(srv.URL+post, vals)
	body(res)
	calls := up.seen()
	if len(calls) != 1 || calls[0]["action"] != "disable" || calls[0]["reason"] != "spam wave" || !strings.HasSuffix(calls[0]["path"].(string), "/content/link/11/actions") {
		t.Fatalf("calls = %+v, want one disable on link 11 with the reason", calls)
	}
	n, detail, target := auditCount(t, a, "link.disable")
	if n != 1 || target != "11" || detail["reason"] != "spam wave" {
		t.Fatalf("audit = %d lines, target %q, detail %+v", n, target, detail)
	}
}

func TestUserRowActionNeedsAReason(t *testing.T) {
	hop, _, a, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/users"))
	if !strings.Contains(dialogOf(t, page, "dlg-0-u-26-suspend"), `name="reason" required`) {
		t.Fatal("the suspend dialog needs a required reason")
	}
	post, vals := menuForm(t, page, "suspend", "/users/26/actions")
	vals.Set("reason", " ")
	res, _ := c.PostForm(srv.URL+post, vals)
	body(res)
	if len(hop.actions) != 0 {
		t.Fatalf("suspend without a reason reached HOP ID: %+v", hop.actions)
	}
	if n, _, _ := auditCount(t, a, "user.suspend"); n != 0 {
		t.Fatalf("a refused action left %d audit lines", n)
	}
	vals.Set("reason", "abuse report 7")
	res, _ = c.PostForm(srv.URL+post, vals)
	body(res)
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "suspend" || hop.actions[0]["reason"] != "abuse report 7" {
		t.Fatalf("forwarded = %+v", hop.actions)
	}
	n, detail, target := auditCount(t, a, "user.suspend")
	if n != 1 || target != "26" || detail["reason"] != "abuse report 7" {
		t.Fatalf("audit = %d lines, target %q, detail %+v", n, target, detail)
	}
}

func TestReportRowActionNeedsAReason(t *testing.T) {
	_, up, a, srv := newBulkSetup(t)
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/reports"))
	post, vals := menuForm(t, page, "restore", "/apps/linkhop/reports/a1/actions")
	if vals.Get("kind") != "flag" {
		t.Fatalf("the row form should carry its own kind: %v", vals)
	}
	res, _ := c.PostForm(srv.URL+post, vals)
	body(res)
	if len(up.seen()) != 0 {
		t.Fatalf("restore without a reason reached the app: %+v", up.seen())
	}
	if n, _, _ := auditCount(t, a, "report.restore"); n != 0 {
		t.Fatalf("a refused action left %d audit lines", n)
	}
	vals.Set("reason", "not abuse")
	res, _ = c.PostForm(srv.URL+post, vals)
	body(res)
	if calls := up.seen(); len(calls) != 1 || calls[0]["reason"] != "not abuse" || !strings.HasSuffix(calls[0]["path"].(string), "/reports/flag/a1/actions") {
		t.Fatalf("calls = %+v", calls)
	}
	n, detail, target := auditCount(t, a, "report.restore")
	if n != 1 || target != "flag:a1" || detail["reason"] != "not abuse" {
		t.Fatalf("audit = %d lines, target %q, detail %+v", n, target, detail)
	}
}

func TestBulkActionsNeedAReason(t *testing.T) {
	// content
	_, up, a, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content?type=link"))
	if !strings.Contains(page, `id="bulk-reason" type="text" name="reason" required`) {
		t.Error("the reason of the bulk bar must be required")
	}
	res, _ := c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "action": {"disable"}, "pick": {"11", "12"}, "confirm": {"yes"}, "reason": {"  "}})
	body(res)
	if len(up.seen()) != 0 {
		t.Errorf("a content bulk without a reason reached the app: %+v", up.seen())
	}
	if n, _, _ := auditCount(t, a, "link.disable"); n != 0 {
		t.Errorf("a refused bulk left %d audit lines", n)
	}
	// users
	_, acted, ua, usrv := usersBulkHop(t)
	uc := client(t)
	body(login(t, uc, usrv))
	res, _ = uc.PostForm(usrv.URL+"/users/actions", url.Values{"action": {"suspend"}, "pick": {"26", "28"}, "confirm": {"yes"}})
	body(res)
	if len(*acted) != 0 {
		t.Errorf("a users bulk without a reason reached HOP ID: %v", *acted)
	}
	if n, _, _ := auditCount(t, ua, "user.suspend"); n != 0 {
		t.Errorf("a refused users bulk left %d audit lines", n)
	}
	// reports
	_, rup, ra, rsrv := newBulkSetup(t)
	rc := client(t)
	body(login(t, rc, rsrv))
	res, _ = rc.PostForm(rsrv.URL+"/apps/linkhop/reports/actions", url.Values{"action": {"dismiss"}, "pick": {"a1|flag", "b1|report"}, "confirm": {"yes"}})
	body(res)
	if len(rup.seen()) != 0 {
		t.Errorf("a reports bulk without a reason reached the app: %+v", rup.seen())
	}
	if n, _, _ := auditCount(t, ra, "report.dismiss"); n != 0 {
		t.Errorf("a refused reports bulk left %d audit lines", n)
	}
	// with a reason the same content bulk runs once per row
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "action": {"disable"}, "pick": {"11", "12"}, "confirm": {"yes"}, "reason": {"sweep"}})
	body(res)
	if len(up.seen()) != 2 {
		t.Errorf("a bulk with a reason should make two calls: %+v", up.seen())
	}
}

// namesExtSetup is a LinkHop with the extensions ip and names and a record of their action calls.
func namesExtSetup(t *testing.T) (*App, *httptest.Server, *[]map[string]any) {
	t.Helper()
	hop := newFakeHop(t)
	var calls []map[string]any
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	api := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, b, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				return
			}
			h(w, r, b)
		}
	}
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api"],"content_types":[],"extensions":["ip","names"]}}`))
	}))
	mux.HandleFunc("GET /api/hop/x/names", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"holds":[{"slug":"bad-slug","reason":"abuse"}]}}`))
	}))
	mux.HandleFunc("GET /api/hop/x/ip", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"banned":[]}}`))
	}))
	record := func(w http.ResponseWriter, r *http.Request, b []byte) {
		calls = append(calls, map[string]any{"path": r.URL.Path, "body": string(b)})
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}
	mux.HandleFunc("POST /api/hop/x/names/holds/bad-slug/actions", api(record))
	mux.HandleFunc("POST /api/hop/x/ip/actions", api(record))
	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	t.Cleanup(hop.srv.Close)
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	return a, srv, &calls
}

func TestExtensionIrreversibleActionsNeedTheDialogAndAReason(t *testing.T) {
	a, srv, calls := namesExtSetup(t)
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/x/names"))
	d := dialogOf(t, page, "dlg-0-x-names-release")
	if !strings.Contains(d, `name="ack" value="yes"`) || !strings.Contains(d, `name="reason" required`) || !strings.Contains(d, `name="path" required`) {
		t.Fatalf("the release dialog lacks ack, a required reason or its target: %s", d)
	}
	if strings.Contains(strings.SplitN(page, `<details class="sect run-action">`, 2)[1], `<option value="release">`) {
		t.Error("release must not stay in the plain form")
	}
	post := "/apps/linkhop/x/names/actions"
	// no ack: refused, nothing logged
	res, _ := c.PostForm(srv.URL+post, url.Values{"action": {"release"}, "path": {"holds/bad-slug"}, "reason": {"appeal"}})
	body(res)
	// ack but no reason: refused too
	res, _ = c.PostForm(srv.URL+post, url.Values{"action": {"release"}, "path": {"holds/bad-slug"}, "ack": {"yes"}, "reason": {" "}})
	body(res)
	if len(*calls) != 0 {
		t.Fatalf("a release without ack or without a reason reached the app: %+v", *calls)
	}
	if n, _, _ := auditCount(t, a, "names.release"); n != 0 {
		t.Fatalf("refused releases left %d audit lines", n)
	}
	res, _ = c.PostForm(srv.URL+post, url.Values{"action": {"release"}, "path": {"holds/bad-slug"}, "ack": {"yes"}, "reason": {"appeal"}})
	body(res)
	if len(*calls) != 1 || (*calls)[0]["path"] != "/api/hop/x/names/holds/bad-slug/actions" {
		t.Fatalf("calls = %+v", *calls)
	}
	n, detail, _ := auditCount(t, a, "names.release")
	if n != 1 || detail["reason"] != "appeal" {
		t.Fatalf("audit = %d lines, detail %+v", n, detail)
	}
	// ban needs the ack as well
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/x/ip/actions", url.Values{"action": {"ban"}, "p_ip": {"203.0.113.9"}, "reason": {"scan"}})
	body(res)
	if len(*calls) != 1 {
		t.Fatalf("ban without ack reached the app: %+v", *calls)
	}
	// the plain actions stay one form
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/x/ip/actions", url.Values{"action": {"unban"}, "p_ip": {"203.0.113.9"}})
	body(res)
	if len(*calls) != 2 {
		t.Fatalf("unban should still run from the plain form: %+v", *calls)
	}
}

func TestItemURLIsALinkOnlyForHTTP(t *testing.T) {
	a := &App{cfg: Config{Lang: "en"}}
	for _, tc := range []struct {
		url  string
		link bool
	}{{"https://x.example/a", true}, {"http://x.example/a", true}, {"javascript:alert(1)", false}, {"data:text/html,x", false}, {"//evil.example", false}} {
		p := a.itemDetailPage(httptest.NewRequest("GET", "/", nil), &db.App{Slug: "linkhop", Name: "LinkHop"}, &Manifest{}, "link", "1", row{ID: "1", URL: tc.url, Status: "live"}, "", false)
		var got *uiKV
		for i := range p.Sections[0].KV {
			if p.Sections[0].KV[i].K == tl("en", "col.url") {
				got = &p.Sections[0].KV[i]
			}
		}
		if got == nil {
			t.Fatalf("%s: no URL row", tc.url)
		}
		if linked := strings.Contains(string(got.V), `<a href="`+tc.url+`"`); linked != tc.link {
			t.Errorf("%s: cell %s, link wanted = %v", tc.url, got.V, tc.link)
		}
		if !tc.link && (strings.Contains(string(got.V), "<a") || !strings.Contains(string(got.V), template.HTMLEscapeString(tc.url))) {
			t.Errorf("%s: not shown as escaped plain text: %s", tc.url, got.V)
		}
	}
}

// A rendered detail page of an item whose URL is javascript: must not carry an href to it.
func TestItemPageDoesNotLinkJavascriptURL(t *testing.T) {
	hop := newFakeHop(t)
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	api := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, b, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api"],"content_types":[{"type":"link","label":"Links","actions":["hide","unhide","disable","enable"]}]}}`))
	}))
	mux.HandleFunc("GET /api/hop/content/link/{id}", api(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"7","type":"link","title":"x","status":"under-review","url":"javascript:alert(1)"}}`))
	}))
	_, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	t.Cleanup(hop.srv.Close)
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content/link/7"))
	if strings.Contains(page, `href="javascript:`) || strings.Contains(page, `href='javascript:`) {
		t.Fatal("a javascript: URL was rendered as a link")
	}
	if !strings.Contains(page, "javascript:alert(1)") {
		t.Error("the address should still be shown as text")
	}
	// the status is not one the menu knows: both halves of every pair are offered
	for _, act := range []string{"hide", "unhide", "disable", "enable"} {
		if !strings.Contains(page, `id="dlg-0-link-7-`+act+`"`) {
			t.Errorf("an unknown status should offer %s", act)
		}
	}
}

func TestUnknownStatusOffersBothHalves(t *testing.T) {
	acts := []string{"hide", "unhide", "disable", "enable", "delete"}
	for _, st := range []string{"", "weird", "pending-review"} {
		got := pairedActions(acts, st, nil)
		if len(got) != len(acts) {
			t.Errorf("status %q: %v, want every action", st, got)
		}
	}
	if got := pairedActions(acts, "live", nil); contains(got, "unhide") || !contains(got, "hide") {
		t.Errorf("a live item offers Hide, not Show: %v", got)
	}
	if got := pairedActions(acts, "hidden", nil); !contains(got, "unhide") || contains(got, "hide") {
		t.Errorf("a hidden item offers Show, not Hide: %v", got)
	}
}

func TestDialogIDsAreUniquePerRow(t *testing.T) {
	// two ids that differ only in characters idSafe replaces must still get two dialogs
	x, y := dlgID(0, "link", "a.b", "hide"), dlgID(1, "link", "a-b", "hide")
	if x == y {
		t.Fatalf("collision: %s", x)
	}
}
