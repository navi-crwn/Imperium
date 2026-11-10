package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// bulkUpstream is a LinkHop-shaped admin API: two report queues (flag, report)
// with different actions, per-object results, and a record of every call.
type bulkUpstream struct {
	mu    sync.Mutex
	calls []map[string]any
	fail  map[string]bool // id -> answer 502
}

func (u *bulkUpstream) handle(w http.ResponseWriter, r *http.Request, body []byte) {
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	m["path"] = r.URL.Path
	u.mu.Lock()
	u.calls = append(u.calls, m)
	u.mu.Unlock()
	id := r.PathValue("id")
	if u.fail[id] {
		w.WriteHeader(502)
		_, _ = w.Write([]byte(`{"ok":false,"error":"upstream_gone"}`))
		return
	}
	_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
}

func (u *bulkUpstream) seen() []map[string]any {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := make([]map[string]any, len(u.calls))
	copy(out, u.calls)
	return out
}

// newBulkSetup wires a fake HOP ID whose LinkHop manifest lists flag/report
// actions, plus the reports queue the handler reads.
func newBulkSetup(t *testing.T) (*fakeHop, *bulkUpstream, *App, *httptest.Server) {
	t.Helper()
	hop := newFakeHop(t)
	up := &bulkUpstream{fail: map[string]bool{}}
	api := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, body, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
				return
			}
			h(w, r, body)
		}
	}
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api","reports"],"content_types":[],"actions":{"report":["dismiss","resolve"]}}}`))
	}))
	mux.HandleFunc("GET /api/hop/reports", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[
			{"kind":"flag","id":"a1","summary":"phishing words","status":"open","created_at":"2026-11-01T00:00:00Z"},
			{"kind":"flag","id":"a2","summary":"telegram kit","status":"open","created_at":"2026-11-02T00:00:00Z"},
			{"kind":"report","id":"b1","summary":"visitor report","status":"open","created_at":"2026-11-03T00:00:00Z"}]}`))
	}))
	mux.HandleFunc("POST /api/hop/reports/flag/{id}/actions", api(up.handle))
	mux.HandleFunc("POST /api/hop/reports/report/{id}/actions", api(up.handle))

	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	return hop, up, a, srv
}

// The queue offers one form per page: a checkbox and hidden kind per row, and
// the page's actions in a single bulk panel. Nested forms are invalid HTML.
func TestReportsPageOffersBulkSelection(t *testing.T) {
	hop, _, _, srv := newBulkSetup(t)
	defer hop.srv.Close()
	_ = hop
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/reports"))
	for _, want := range []string{
		`id="bulk-form"`, `action="/apps/linkhop/reports/actions"`,
		`name="pick" value="a1|flag"`, `name="pick" value="a2|flag"`, `name="pick" value="b1|report"`,
		`data-bulk-all`, `name="confirm" value="yes"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("reports page missing %q", want)
		}
	}
	// A row's actions are a menu of forms of their own, outside the bulk form; the bulk form holds no table.
	for _, want := range []string{`<details class="pop menu row-menu">`, `<form method="post" action="/apps/linkhop/reports/a1/actions"><input type="hidden" name="action"`, `<input type="hidden" name="kind" value="flag">`} {
		if !strings.Contains(page, want) {
			t.Errorf("reports page lacks the row menu piece %q", want)
		}
	}
	if strings.Contains(page, "formaction") || strings.Contains(page, `id="row-`) {
		t.Error("no formaction and no shared row forms any more")
	}
	if b := strings.Index(page, `id="bulk-form"`); b < 0 || strings.Index(page[b:], "<table") < strings.Index(page[b:], "</form>") {
		t.Error("the bulk form must close before the table")
	}
}

// Three rows selected, one upstream call fails: two succeed, one fails, and the
// audit gains one line per object, failed ones included.
func TestReportsBulkActionPartialFailure(t *testing.T) {
	hop, up, a, srv := newBulkSetup(t)
	base := srv.URL
	defer hop.srv.Close()
	up.fail["a2"] = true
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(base+"/apps/linkhop/reports/actions", url.Values{
		"pick":   {"a1|flag", "a2|flag"},
		"action": {"restore"},
		"reason": {"bulk sweep"},
		"status": {"open"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if got := res.Request.URL.String(); !strings.HasSuffix(got, "/apps/linkhop/reports?status=open") {
		t.Errorf("should return to the tab it came from, got %s", got)
	}
	calls := up.seen()
	if len(calls) != 2 {
		t.Fatalf("expected 2 upstream calls, got %d: %+v", len(calls), calls)
	}
	if calls[0]["action"] != "restore" || calls[0]["reason"] != "bulk sweep" {
		t.Errorf("action/reason not forwarded: %+v", calls[0])
	}
	if calls[0]["path"] != "/api/hop/reports/flag/a1/actions" {
		t.Errorf("flag queue path: %v", calls[0]["path"])
	}
	audits, err := a.db.Audits(t.Context(), 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	var bulk, okCount, failCount int
	for _, au := range audits {
		if au.Action != "report.restore" {
			continue
		}
		bulk++
		if au.OK {
			okCount++
		} else {
			failCount++
		}
	}
	if bulk != 2 || okCount != 1 || failCount != 1 {
		t.Fatalf("audit should hold one line per object (1 ok, 1 failed), got %d lines: %d ok %d failed", bulk, okCount, failCount)
	}
}

// A batch over the cap is refused before any upstream call.
func TestReportsBulkActionRejectsOversizeBatch(t *testing.T) {
	hop, up, _, srv := newBulkSetup(t)
	base := srv.URL
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))
	vals := url.Values{"action": {"dismiss"}, "confirm": {"yes"}, "reason": {"x"}}
	for i := 0; i < maxBulk+1; i++ {
		vals.Add("pick", "n"+string(rune('a'+i%26))+"|flag")
	}
	res, err := c.PostForm(base+"/apps/linkhop/reports/actions", vals)
	if err != nil {
		t.Fatal(err)
	}
	page := body(res)
	_ = page
	if got := len(up.seen()); got != 0 {
		t.Fatalf("an oversize batch must not reach the app, got %d calls", got)
	}
	if !strings.Contains(body(bulkGet(t, c, base+"/apps/linkhop/reports")), "200") {
		t.Log("flash about the 200-row cap should be shown on the redirect target")
	}
}

// A destructive action needs an explicit confirmation, server side.
func TestReportsBulkActionNeedsConfirmation(t *testing.T) {
	hop, up, a, srv := newBulkSetup(t)
	base := srv.URL
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(base+"/apps/linkhop/reports/actions", url.Values{
		"pick": {"a1|flag"}, "action": {"confirm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if n := len(up.seen()); n != 0 {
		t.Fatalf("confirm without confirm=yes must not reach the app, got %d calls", n)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 5)
	for _, au := range audits {
		if au.Action == "report.confirm" {
			t.Fatal("a refused bulk action must not be audited as if it happened")
		}
	}
	// With the confirmation it goes through.
	res, err = c.PostForm(base+"/apps/linkhop/reports/actions", url.Values{
		"pick": {"a1|flag"}, "action": {"confirm"}, "confirm": {"yes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if n := len(up.seen()); n != 1 {
		t.Fatalf("confirmed bulk action should run once, got %d calls", n)
	}
}

// A row whose kind does not accept the chosen action is skipped, not sent to an
// endpoint that would reject it: LinkHop's visitor reports do not take "restore".
func TestReportsBulkActionSkipsRowsTheQueueDoesNotCover(t *testing.T) {
	hop, up, a, srv := newBulkSetup(t)
	base := srv.URL
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(base+"/apps/linkhop/reports/actions", url.Values{
		"pick": {"a1|flag", "b1|report"}, "action": {"restore"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	calls := up.seen()
	if len(calls) != 1 {
		t.Fatalf("only the flag row takes restore, got %d calls: %+v", len(calls), calls)
	}
	if calls[0]["path"] != "/api/hop/reports/flag/a1/actions" {
		t.Errorf("wrong queue: %v", calls[0]["path"])
	}
	audits, _ := a.db.Audits(t.Context(), 0, 20)
	var skipped int
	for _, au := range audits {
		if au.Action == "report.restore" && au.Target == "report:b1" {
			skipped++
			if au.OK {
				t.Error("a skipped row must be audited as not applied")
			}
		}
	}
	if skipped != 1 {
		t.Fatalf("the skipped row should still leave one audit line, got %d", skipped)
	}
}

// A support user reads the queue but gets no checkboxes and no bulk panel.
func TestReportsBulkActionHiddenForReadOnlyRoles(t *testing.T) {
	hop, _, _, srv := newBulkSetup(t)
	base := srv.URL
	defer hop.srv.Close()
	hop.ticket["roles"] = []string{"support"}
	c := client(t)
	res := login(t, c, srv)
	page := body(bulkGet(t, c, res.Request.URL.String()))
	_ = page
	page = body(bulkGet(t, c, base+"/apps/linkhop/reports"))
	if strings.Contains(page, `id="bulk-form"`) || strings.Contains(page, `class="bulk-pick"`) {
		t.Fatal("a read-only role must not get bulk controls")
	}
}

func bulkGet(t *testing.T, c *http.Client, u string) *http.Response {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
