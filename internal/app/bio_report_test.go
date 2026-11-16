package app

import (
	"net/url"
	"strings"
	"testing"
)

// BioHop lists a visitor's report of a bio page as kind "bio_report" (id "report-<n>") and offers
// investigate, close and unpublish_page. These are that app's answers, as the fake serves them.
const bioManifest = `{"ok":true,"data":{"app":"linkhop","name":"BioHop","capabilities":["admin_api","reports"],"content_types":[],"actions":{"report":["investigate","close","unpublish_page"]}}}`

const bioReports = `{"ok":true,"data":[
	{"kind":"bio_report","id":"report-7","reason":"scam page","subject":{"type":"bio_page","id":3,"slug":"promo"},"status":"open","created_at":"2026-11-16T00:00:00Z"}]}`

func TestUnpublishPageLabelAndConfirmation(t *testing.T) {
	if !confirmAction("unpublish_page") {
		t.Error("unpublish_page must need confirmation, like unpublish")
	}
	if destructiveAction("unpublish_page") != destructiveAction("unpublish") {
		t.Error("unpublish_page is as reversible as unpublish: the page goes back to a draft")
	}
	if confirmAction("investigate") || confirmAction("close") {
		t.Error("investigate and close stay without the bulk tick")
	}
	raw := "unpublish page"
	for i, code := range langOrder {
		got := actionLabel(code, "unpublish_page")
		if got == raw || strings.TrimSpace(got) == "" {
			t.Errorf("%s: unpublish_page shows %q, not a label", code, got)
		}
		if i > 0 && got == actionLabel("en", "unpublish_page") {
			t.Errorf("%s: unpublish_page is still the English label", code)
		}
	}
	for lang, want := range map[string]string{"en": "Unpublish page", "id": "Batalkan publikasi halaman"} {
		if got := actionLabel(lang, "unpublish_page"); got != want {
			t.Errorf("%s: got %q, want %q", lang, got, want)
		}
	}
}

func TestBioReportRowShowsUnpublishPage(t *testing.T) {
	hop, _, _, srv := newBulkSetupWith(t, bioManifest, bioReports)
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/reports"))
	for _, want := range []string{
		`name="pick" value="report-7|bio_report"`,
		`<form method="post" action="/apps/linkhop/reports/report-7/actions">`,
		`<input type="hidden" name="action" value="unpublish_page">`,
		`<option value="unpublish_page">Unpublish page</option>`,
		">Unpublish page</a>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("reports page lacks %q", want)
		}
	}
	if strings.Contains(page, "unpublish page") {
		t.Error("an action name must never be shown raw")
	}
}

// One row, one click: the call goes to the kind-less route BioHop answers, with the reason.
func TestBioReportUnpublishPageSingle(t *testing.T) {
	hop, up, a, srv := newBulkSetupWith(t, bioManifest, bioReports)
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/apps/linkhop/reports/report-7/actions", url.Values{
		"kind": {"bio_report"}, "action": {"unpublish_page"}, "reason": {"scam"}, "ack": {"yes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	calls := up.seen()
	if len(calls) != 1 || calls[0]["path"] != "/api/hop/reports/report-7/actions" || calls[0]["action"] != "unpublish_page" || calls[0]["reason"] != "scam" {
		t.Fatalf("one signed call to the report route expected, got %+v", calls)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 5)
	if len(audits) == 0 || audits[0].Action != "report.unpublish_page" || !audits[0].OK {
		t.Errorf("the audit trail should name the action, got %+v", audits)
	}
}

// Bulk: without the confirmation nothing leaves Imperium; with it each ticked row is one call.
func TestBioReportUnpublishPageBulkNeedsConfirmation(t *testing.T) {
	hop, up, a, srv := newBulkSetupWith(t, bioManifest, bioReports)
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))
	form := url.Values{"pick": {"report-7|bio_report"}, "action": {"unpublish_page"}, "reason": {"scam"}}
	res, err := c.PostForm(srv.URL+"/apps/linkhop/reports/actions", form)
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	if n := len(up.seen()); n != 0 {
		t.Fatalf("unpublish_page without confirm=yes must not reach the app, got %d calls", n)
	}
	audits, _ := a.db.Audits(t.Context(), 0, 5)
	for _, au := range audits {
		if au.Action == "report.unpublish_page" {
			t.Fatal("a refused bulk action must not be audited as if it happened")
		}
	}
	form.Set("confirm", "yes")
	res, err = c.PostForm(srv.URL+"/apps/linkhop/reports/actions", form)
	if err != nil {
		t.Fatal(err)
	}
	_ = body(res)
	calls := up.seen()
	if len(calls) != 1 || calls[0]["path"] != "/api/hop/reports/report-7/actions" || calls[0]["action"] != "unpublish_page" {
		t.Fatalf("confirmed bulk action should be one call to the report route, got %+v", calls)
	}
}

// Support only reads: it gets no buttons, and a forged post reaches nothing.
func TestBioReportUnpublishPageRefusedForSupport(t *testing.T) {
	hop, up, _, srv := newBulkSetupWith(t, bioManifest, bioReports)
	defer hop.srv.Close()
	hop.ticket["roles"] = []string{"support"}
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/reports"))
	if strings.Contains(page, `value="unpublish_page"`) {
		t.Error("a read-only role must not be offered unpublish_page")
	}
	for _, path := range []string{"/apps/linkhop/reports/report-7/actions", "/apps/linkhop/reports/actions"} {
		res, err := c.PostForm(srv.URL+path, url.Values{
			"pick": {"report-7|bio_report"}, "kind": {"bio_report"}, "action": {"unpublish_page"}, "reason": {"x"}, "confirm": {"yes"}, "ack": {"yes"},
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = body(res)
	}
	if n := len(up.seen()); n != 0 {
		t.Fatalf("support must not reach the app, got %d calls", n)
	}
}
