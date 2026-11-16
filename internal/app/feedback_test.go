package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/fakehub"
)

// The Feedback pages against the fake hub (internal/fakehub/feedback.go: the routes of /api/hop/feedback).

const (
	ipPixel   = "203.0.113.77"
	ipLink    = "198.51.100.9"
	xssText   = "Upload fails\n<script>alert(1)</script> see https://contoh.test/x"
	jpegBytes = "\xff\xd8\xff\xe0\x00\x10JFIF-test-image"
)

type fbIDs struct{ px, lh, hub int }

func (e *annEnv) seedFeedback() fbIDs {
	f := e.hop.fb
	var ids fbIDs
	ids.hub = f.Seed(fakehub.Report{Kind: "other", Status: "done", App: "hub", Message: "日本語のテキスト Привет", Page: "home", Lang: "ja", CreatedAt: time.Now().Add(-48 * time.Hour)})
	ids.lh = f.Seed(fakehub.Report{Kind: "idea", Status: "read", App: "linkhop", Message: "Please add folders for links.", Page: "dashboard.links", AppVersion: "2.4.1", IP: ipLink, UserAgent: "Firefox 130", Viewport: "390x844", Lang: "en", CreatedAt: time.Now().Add(-24 * time.Hour)})
	ids.px = f.Seed(fakehub.Report{Kind: "problem", Status: "new", App: "pixelhop", Message: xssText, Page: "/i/{id}", AppVersion: "3.7.0", UserID: 7, Username: "helena", IP: ipPixel, UserAgent: "Edge 130 <b>x</b>", Viewport: "1440x900", Lang: "id", Screenshot: []byte(jpegBytes), CreatedAt: time.Now().Add(-2 * time.Hour)})
	e.a.fbClear() // the sign-in already counted an empty inbox
	return ids
}

func (e *annEnv) fbPost(t *testing.T, path string, v url.Values) (int, string) {
	t.Helper()
	code, loc, _ := e.post(t, path, v)
	return code, loc
}

func hubCalls(f *fakehub.Feedback, method string) int {
	n := 0
	for _, c := range f.Calls() {
		if c.Method == method {
			n++
		}
	}
	return n
}

func hubPosted(f *fakehub.Feedback, action string) bool {
	for _, c := range f.Calls() {
		if c.Method == "POST" && c.Body["action"] == action {
			return true
		}
	}
	return false
}

func TestFeedbackListShape(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	code, page := e.get(t, "/feedback")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{"Upload fails", "Please add folders", "日本語のテキスト", "PixelHop", "helena", "Guest", "fb-problem", "fb-new", "fb-done", `href="/feedback/` + fmt.Sprint(ids.px), "Image", `name="app"`, `name="kind"`, `name="status"`} {
		if !strings.Contains(page, want) {
			t.Errorf("list lacks %q", want)
		}
	}
	for _, no := range []string{ipPixel, ipLink} {
		if strings.Contains(page, no) {
			t.Errorf("the list shows an IP address %s", no)
		}
	}
	if strings.Contains(page, "<script>alert") || strings.Contains(page, `href="https://contoh.test`) {
		t.Error("report text on the list must be plain text")
	}
	// the filter is passed to the hub, only values it knows
	e.hop.fb.ResetCalls()
	_, page = e.get(t, "/feedback?app=pixelhop&kind=problem&status=new&cursor=&x=1")
	if !strings.Contains(page, "Upload fails") || strings.Contains(page, "Please add folders") {
		t.Error("PixelHop · Problem · New leaves one report")
	}
	last := lastFbCall(t, e.hop.fb)
	for _, want := range []string{"app=pixelhop", "kind=problem", "status=new", "limit=25"} {
		if !strings.Contains(last.Path, want) {
			t.Errorf("hub call %s lacks %s", last.Path, want)
		}
	}
	e.hop.fb.ResetCalls()
	e.get(t, "/feedback?app=nope&kind=%3Cx%3E&status=zzz")
	if p := lastFbCall(t, e.hop.fb).Path; strings.Contains(p, "nope") || strings.Contains(p, "zzz") || strings.Contains(p, "kind=") {
		t.Errorf("unknown filter values reached the hub: %s", p)
	}
	_, page = e.get(t, "/feedback?status=done&kind=idea")
	if !strings.Contains(page, `class="muted">`+"Nothing") && !strings.Contains(page, "No feedback yet.") {
		t.Error("an empty result shows the empty message")
	}
}

func lastFbCall(t *testing.T, f *fakehub.Feedback) fakehub.Call {
	t.Helper()
	c := f.Calls()
	if len(c) == 0 {
		t.Fatal("no call reached HOP ID")
	}
	return c[len(c)-1]
}

func TestFeedbackListPaging(t *testing.T) {
	e := newAnnEnv(t, "owner")
	for i := 1; i <= 30; i++ {
		e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: fmt.Sprintf("Report number %02d", i)})
	}
	_, page := e.get(t, "/feedback?app=pixelhop")
	if !strings.Contains(page, "Report number 30") || strings.Contains(page, "Report number 05") || !strings.Contains(page, "cursor=6") {
		t.Fatalf("first page of 25 newest, with a link to the next")
	}
	_, page = e.get(t, "/feedback?app=pixelhop&cursor=6")
	if !strings.Contains(page, "Report number 05") || strings.Contains(page, "Report number 30") || !strings.Contains(page, "First page") {
		t.Error("second page holds the rest and a way back")
	}
	// the detail link carries the filters, so Back returns to the same page
	if !strings.Contains(page, "f=app%3Dpixelhop%26cursor%3D6") {
		t.Error("detail links keep the filter and the page")
	}
	_, one := e.get(t, "/feedback/1?f=app%3Dpixelhop%26cursor%3D6%26evil%3D1")
	if !strings.Contains(one, `href="/feedback?app=pixelhop&amp;cursor=6"`) {
		t.Errorf("Back to the list keeps the filter: %s", regexp.MustCompile(`crumb.*`).FindString(one))
	}
}

// Every value of a report is text: no markup and no link from what the report holds.
func TestFeedbackDetailEscapesReportText(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	_, page := e.get(t, fmt.Sprintf("/feedback/%d", ids.px))
	if strings.Contains(page, "<script>alert(1)") || strings.Contains(page, "<b>x</b>") {
		t.Error("markup of the report is not escaped")
	}
	for _, want := range []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "https://contoh.test/x", "pre-wrap", "Upload fails\n", "Edge 130 &lt;b&gt;x&lt;/b&gt;", "1440x900", "3.7.0"} {
		if !strings.Contains(page, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	if strings.Contains(page, `href="https://contoh.test`) || strings.Contains(page, `href="http://contoh`) {
		t.Error("an address in the text became a link")
	}
	for _, a := range regexp.MustCompile(`<a [^>]*>`).FindAllString(page, -1) {
		if strings.Contains(a, "contoh") {
			t.Errorf("link from report text: %s", a)
		}
	}
	// the non-Latin report and one without user, page or IP
	_, page = e.get(t, fmt.Sprintf("/feedback/%d", ids.hub))
	for _, want := range []string{"日本語のテキスト Привет", "Guest", "Removed after 30 days"} {
		if !strings.Contains(page, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
}

// F-5: who sees the IP address, who may act, which app's reports.
func TestFeedbackRoleMatrix(t *testing.T) {
	type want struct {
		ip, act, del bool
	}
	roles := []struct {
		role string
		w    want
	}{
		{"owner", want{true, true, true}},
		{"admin", want{true, true, true}},
		{"moderator", want{false, true, false}},
		{"support", want{false, false, false}},
		{"admin:pixelhop", want{false, true, false}},
	}
	for _, c := range roles {
		t.Run(c.role, func(t *testing.T) {
			e := newAnnEnv(t, c.role)
			ids := e.seedFeedback()
			code, page := e.get(t, fmt.Sprintf("/feedback/%d", ids.px))
			if code != 200 {
				t.Fatalf("status %d", code)
			}
			if got := strings.Contains(page, ipPixel); got != c.w.ip {
				t.Errorf("IP in the page = %v, want %v", got, c.w.ip)
			}
			if got := strings.Contains(page, "Mark done"); got != c.w.act {
				t.Errorf("Mark done = %v, want %v", got, c.w.act)
			}
			if got := strings.Contains(page, `value="delete"`); got != c.w.del {
				t.Errorf("Delete dialog = %v, want %v", got, c.w.del)
			}
			// the list never has the address, whoever looks
			_, list := e.get(t, "/feedback")
			if strings.Contains(list, ipPixel) || strings.Contains(list, ipLink) {
				t.Error("an IP address is on the list")
			}
			// reading: those who act mark it read, support does not
			if got := hubPosted(e.hop.fb, "read"); got != c.w.act {
				t.Errorf("opened the report: read action sent = %v, want %v", got, c.w.act)
			}
			st, _ := e.hop.fb.Get(ids.px)
			if wantStatus := map[bool]string{true: "read", false: "new"}[c.w.act]; st.Status != wantStatus {
				t.Errorf("status after opening = %s, want %s", st.Status, wantStatus)
			}
			// the action itself
			e.hop.fb.ResetCalls()
			code, _ = e.fbPost(t, fmt.Sprintf("/feedback/%d/actions", ids.px), url.Values{"action": {"done"}})
			if c.w.act && code != 302 || !c.w.act && code != 403 {
				t.Errorf("Mark done answered %d", code)
			}
			if !c.w.act && hubCalls(e.hop.fb, "POST") != 0 {
				t.Error("a refused action reached the hub")
			}
			e.hop.fb.ResetCalls()
			code, _ = e.fbPost(t, fmt.Sprintf("/feedback/%d/actions", ids.px), url.Values{"action": {"delete"}, "ack": {"yes"}, "reason": {"spam"}})
			if c.w.del && code != 302 || !c.w.del && code != 403 {
				t.Errorf("Delete answered %d", code)
			}
			if !c.w.del && hubCalls(e.hop.fb, "POST") != 0 {
				t.Error("a refused delete reached the hub")
			}
			if _, still := e.hop.fb.Get(ids.px); still == c.w.del {
				t.Errorf("report still there = %v after Delete by %s", still, c.role)
			}
		})
	}
}

func TestFeedbackScopedAdminSeesItsAppOnly(t *testing.T) {
	e := newAnnEnv(t, "admin:pixelhop")
	ids := e.seedFeedback()
	_, list := e.get(t, "/feedback")
	if !strings.Contains(list, "Upload fails") || strings.Contains(list, "Please add folders") {
		t.Error("the list is forced to the admin's own app")
	}
	if p := lastFbCall(t, e.hop.fb).Path; !strings.Contains(p, "app=pixelhop") {
		t.Errorf("the hub was not asked for one app: %s", p)
	}
	_, list = e.get(t, "/feedback?app=linkhop") // asking for another app changes nothing
	if strings.Contains(list, "Please add folders") || !strings.Contains(list, "Upload fails") {
		t.Error("another app's reports leaked into the list")
	}
	for _, id := range []int{ids.lh, ids.hub} {
		if code, _ := e.get(t, fmt.Sprintf("/feedback/%d", id)); code != 404 {
			t.Errorf("another app's report %d answered %d, want 404", id, code)
		}
		if code, _ := e.get(t, fmt.Sprintf("/feedback/%d/screenshot", id)); code != 404 {
			t.Errorf("another app's picture %d answered %d, want 404", id, code)
		}
		e.hop.fb.ResetCalls()
		if code, _ := e.fbPost(t, fmt.Sprintf("/feedback/%d/actions", id), url.Values{"action": {"done"}}); code != 404 {
			t.Errorf("done on another app's report %d answered %d, want 404", id, code)
		}
		if hubPosted(e.hop.fb, "done") {
			t.Error("a done for another app reached the hub")
		}
	}
	if st, _ := e.hop.fb.Get(ids.lh); st.Status != "read" {
		t.Error("another app's report was changed")
	}
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d", ids.px)); code != 200 {
		t.Errorf("its own report answered %d", code)
	}
	// only the number of its own app on the menu
	_, home := e.get(t, "/feedback")
	if !strings.Contains(home, `ico-feedback"></span>Feedback`) {
		t.Error("the menu has Feedback")
	}
}

func TestFeedbackReopenAndRepeat(t *testing.T) {
	e := newAnnEnv(t, "moderator")
	ids := e.seedFeedback()
	path := fmt.Sprintf("/feedback/%d/actions", ids.px)
	for i := 0; i < 2; i++ { // twice: the second is harmless
		if code, loc := e.fbPost(t, path, url.Values{"action": {"done"}, "f": {"app=pixelhop"}}); code != 302 || loc != fmt.Sprintf("/feedback/%d?f=app%%3Dpixelhop", ids.px) {
			t.Fatalf("done: %d %s", code, loc)
		}
	}
	if st, _ := e.hop.fb.Get(ids.px); st.Status != "done" || st.DoneAt == nil {
		t.Errorf("not done: %+v", st)
	}
	_, page := e.get(t, fmt.Sprintf("/feedback/%d", ids.px))
	if !strings.Contains(page, "Reopen") || strings.Contains(page, "Mark done") {
		t.Error("a done report offers Reopen only")
	}
	e.fbPost(t, path, url.Values{"action": {"reopen"}})
	if st, _ := e.hop.fb.Get(ids.px); st.Status != "read" {
		t.Errorf("reopened status = %s, want read", st.Status)
	}
	if code, _ := e.fbPost(t, path, url.Values{"action": {"explode"}}); code != 400 {
		t.Errorf("unknown action answered %d", code)
	}
}

// Delete asks for the dialog's confirmation and a reason; a second tab that deletes again, or opens the report, gets a clear answer.
func TestFeedbackDeleteAndStaleTab(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	path := fmt.Sprintf("/feedback/%d/actions", ids.px)
	e.hop.fb.ResetCalls()
	e.fbPost(t, path, url.Values{"action": {"delete"}, "reason": {"spam"}}) // no ack
	e.fbPost(t, path, url.Values{"action": {"delete"}, "ack": {"yes"}})     // no reason
	if hubCalls(e.hop.fb, "POST") != 0 {
		t.Fatal("a delete without confirmation or reason reached the hub")
	}
	if code, loc := e.fbPost(t, path, url.Values{"action": {"delete"}, "ack": {"yes"}, "reason": {"spam"}, "f": {"status=new"}}); code != 302 || loc != "/feedback?status=new" {
		t.Fatalf("delete: %d %s", code, loc)
	}
	if e.hop.fb.Len() != 2 {
		t.Error("the report is still at the hub")
	}
	// the other tab: delete again and open
	if code, loc := e.fbPost(t, path, url.Values{"action": {"delete"}, "ack": {"yes"}, "reason": {"again"}}); code != 302 || loc != "/feedback" {
		t.Errorf("second delete: %d %s", code, loc)
	}
	code, page := e.get(t, fmt.Sprintf("/feedback/%d", ids.px))
	if code != 404 || !strings.Contains(page, "no longer exists") {
		t.Errorf("a deleted report: %d", code)
	}
	// audit: who, what, which report and app; never the text or the address
	rows, _ := e.a.db.Audits(context.Background(), 0, 20)
	found := false
	for _, a := range rows {
		blob := fmt.Sprint(a.Action, a.Target, a.App, a.Detail, a.ActorName)
		if a.Action == "feedback.delete" && a.Target == fmt.Sprint(ids.px) && a.App == "pixelhop" && a.ActorName == "boss" {
			found = true
		}
		if strings.HasPrefix(a.Action, "feedback.") && (strings.Contains(blob, ipPixel) || strings.Contains(blob, "Upload fails") || strings.Contains(blob, "alert(1)")) {
			t.Errorf("the audit holds report content: %s", blob)
		}
	}
	if !found {
		t.Error("the delete is not in the audit trail")
	}
}

func TestFeedbackAuditOfDoneAndReopen(t *testing.T) {
	e := newAnnEnv(t, "moderator")
	ids := e.seedFeedback()
	path := fmt.Sprintf("/feedback/%d/actions", ids.lh)
	e.fbPost(t, path, url.Values{"action": {"done"}})
	e.fbPost(t, path, url.Values{"action": {"reopen"}})
	rows, _ := e.a.db.Audits(context.Background(), 0, 20)
	got := map[string]bool{}
	for _, a := range rows {
		if a.Target == fmt.Sprint(ids.lh) && a.App == "linkhop" && a.OK {
			got[a.Action] = true
		}
		if a.Action == "feedback.read" {
			t.Error("opening a report is not audited")
		}
	}
	if !got["feedback.done"] || !got["feedback.reopen"] {
		t.Errorf("audit actions: %v", got)
	}
}

func TestFeedbackScreenshotRoute(t *testing.T) {
	e := newAnnEnv(t, "support")
	ids := e.seedFeedback()
	e.hop.fb.ResetCalls()
	_, page := e.get(t, fmt.Sprintf("/feedback/%d", ids.px))
	if strings.Contains(page, `<img class="fb-shot"`) || !strings.Contains(page, "Show image") || !strings.Contains(page, "image=1") {
		t.Error("the picture is behind Show image")
	}
	for _, c := range e.hop.fb.Calls() {
		if strings.Contains(c.Path, "screenshot") {
			t.Error("the picture was asked from the hub before Show image")
		}
	}
	_, page = e.get(t, fmt.Sprintf("/feedback/%d?image=1", ids.px))
	if !strings.Contains(page, fmt.Sprintf(`<img class="fb-shot" src="/feedback/%d/screenshot"`, ids.px)) {
		t.Error("Show image shows the picture")
	}
	res, err := e.c.Get(fmt.Sprintf("%s/feedback/%d/screenshot", e.srv.URL, ids.px))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(b) != jpegBytes {
		t.Fatalf("picture: %d %q", res.StatusCode, b)
	}
	for k, v := range map[string]string{"Content-Type": "image/jpeg", "X-Content-Type-Options": "nosniff", "Cache-Control": "private, no-store"} {
		if res.Header.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, res.Header.Get(k), v)
		}
	}
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d/screenshot", ids.lh)); code != 404 {
		t.Errorf("a report without a picture answered %d", code)
	}
	if code, _ := e.get(t, "/feedback/999/screenshot"); code != 404 {
		t.Errorf("a missing report answered %d", code)
	}
}

func TestFeedbackScreenshotNeedsSessionAndRole(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := anon.Get(fmt.Sprintf("%s/feedback/%d/screenshot", e.srv.URL, ids.px))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == 200 || strings.Contains(string(b), "JFIF") {
		t.Errorf("without a session the picture answered %d", res.StatusCode)
	}
	for _, path := range []string{"/feedback", fmt.Sprintf("/feedback/%d", ids.px)} {
		res, _ = anon.Get(e.srv.URL + path)
		res.Body.Close()
		if res.StatusCode != 302 {
			t.Errorf("%s without a session answered %d", path, res.StatusCode)
		}
	}
}

func TestFeedbackScreenshotRefusesWhatIsNotJPEGOrTooBig(t *testing.T) {
	e := newAnnEnv(t, "owner")
	id := e.hop.fb.Seed(fakehub.Report{App: "hub", Message: "html pretending", Screenshot: []byte("<html><script>alert(1)</script>")})
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d/screenshot", id)); code != http.StatusBadGateway {
		t.Errorf("a non-JPEG answer was passed on (%d)", code)
	}
	big := make([]byte, fbShotMax+10)
	copy(big, "\xff\xd8\xff")
	id = e.hop.fb.Seed(fakehub.Report{App: "hub", Message: "huge", Screenshot: big})
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d/screenshot", id)); code != http.StatusBadGateway {
		t.Errorf("an oversize picture was passed on (%d)", code)
	}
}

func TestFeedbackHubDownOldAndGone(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	e.hop.fb.FailWith(&fakehub.ErrSpec{Status: 500, Code: "server_error", Message: "boom"})
	code, page := e.get(t, "/feedback")
	if code != 200 || !strings.Contains(page, "HOP ID could not be reached") {
		t.Errorf("hub down: list %d", code)
	}
	if code, _ := e.get(t, "/"); code != 200 {
		t.Errorf("another page failed while the hub is down: %d", code)
	}
	code, page = e.get(t, fmt.Sprintf("/feedback/%d", ids.px))
	if code != 502 || !strings.Contains(page, "HOP ID could not be reached") {
		t.Errorf("hub down: detail %d", code)
	}
	e.hop.fb.FailWith(nil)
	e.hop.fb.SetMissing(true) // a hub that has no such routes yet
	code, page = e.get(t, "/feedback")
	if code != 200 || !strings.Contains(page, "does not have the feedback inbox yet") || strings.Contains(page, "http_404") {
		t.Errorf("old hub: %d, message missing or raw", code)
	}
	e.hop.fb.SetMissing(false)
	code, page = e.get(t, "/feedback/9999")
	if code != 404 || !strings.Contains(page, "no longer exists") {
		t.Errorf("a missing report: %d", code)
	}
}

var badgeRE = regexp.MustCompile(`ico-feedback"></span>Feedback<span class="badge"[^>]*>(\d+)</span>`)

func badgeOf(page string) string {
	if m := badgeRE.FindStringSubmatch(page); m != nil {
		return m[1]
	}
	return ""
}

func TestFeedbackBadge(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: "a"})
	e.hop.fb.Seed(fakehub.Report{App: "linkhop", Message: "b"})
	e.hop.fb.Seed(fakehub.Report{App: "hub", Message: "c", Status: "done"})
	e.a.fbClear()
	_, page := e.get(t, "/")
	if badgeOf(page) != "2" {
		t.Errorf("badge = %q, want 2", badgeOf(page))
	}
	if q := lastFbCall(t, e.hop.fb).Path; !strings.Contains(q, "status=new") || !strings.Contains(q, "limit=1") {
		t.Errorf("badge asked %s", q)
	}
	// kept for a minute: the next pages do not ask again
	e.hop.fb.ResetCalls()
	e.get(t, "/")
	e.get(t, "/audit")
	if n := len(e.hop.fb.Calls()); n != 0 {
		t.Errorf("badge asked the hub %d more times within a minute", n)
	}
	// but taking an action refreshes it at once
	e.fbPost(t, "/feedback/1/actions", url.Values{"action": {"done"}})
	_, page = e.get(t, "/")
	if badgeOf(page) != "1" {
		t.Errorf("after Mark done the badge = %q, want 1", badgeOf(page))
	}
	// none new: no badge
	e2 := newAnnEnv(t, "moderator")
	_, page = e2.get(t, "/")
	if badgeOf(page) != "" || !strings.Contains(page, `href="/feedback"`) {
		t.Errorf("no new reports: badge %q", badgeOf(page))
	}
}

func TestFeedbackBadgeScopedToTheAppOfAnAdmin(t *testing.T) {
	e := newAnnEnv(t, "admin:pixelhop")
	e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: "a"})
	e.hop.fb.Seed(fakehub.Report{App: "linkhop", Message: "b"})
	e.hop.fb.Seed(fakehub.Report{App: "linkhop", Message: "c"})
	e.a.fbClear()
	_, page := e.get(t, "/feedback")
	if badgeOf(page) != "1" {
		t.Errorf("badge = %q, want 1 (its own app)", badgeOf(page))
	}
	if q := lastFbCall(t, e.hop.fb).Path; !strings.Contains(q, "app=pixelhop") {
		t.Errorf("badge call not scoped: %s", q)
	}
	// an admin of no app, and no role at all, has no menu entry
	e3 := newAnnEnv(t, "admin:remotehop")
	if _, page = e3.get(t, "/apps"); strings.Contains(page, `href="/feedback"`) {
		t.Error("an admin of an app that cannot send feedback has the menu entry")
	}
}

// A hub that is dead or slow costs the first page one short wait, never more, and no page fails.
func TestFeedbackBadgeSlowHubDoesNotHoldPagesBack(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: "a"})
	e.hop.fb.SetDelay(5 * time.Second)
	t0 := time.Now()
	code, page := e.get(t, "/")
	first := time.Since(t0)
	if code != 200 || badgeOf(page) != "" {
		t.Errorf("slow hub: %d, badge %q", code, badgeOf(page))
	}
	if first > 3*time.Second {
		t.Errorf("the first page waited %s for the hub", first)
	}
	t0 = time.Now()
	for i := 0; i < 3; i++ {
		if code, _ := e.get(t, "/audit"); code != 200 {
			t.Errorf("page %d: %d", i, code)
		}
	}
	if d := time.Since(t0); d > time.Second {
		t.Errorf("later pages waited %s for the hub", d)
	}
	e.hop.fb.SetDelay(0)
}

func TestFeedbackTextsInNineLanguages(t *testing.T) {
	langs := []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"}
	for key, vals := range feedbackText {
		for i, v := range vals {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s has no %s text", key, langs[i])
			}
			if catalog[langs[i]][key] != v {
				t.Errorf("%s (%s) is not in the catalogue", key, langs[i])
			}
		}
	}
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	for _, lang := range langs {
		for _, path := range []string{"/feedback", fmt.Sprintf("/feedback/%d", ids.px), fmt.Sprintf("/feedback/%d?image=1", ids.hub), "/feedback/9999"} {
			req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
			req.AddCookie(&http.Cookie{Name: "hop_lang", Value: lang})
			res, err := e.c.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			page := body(res)
			if want := map[bool]int{true: 404, false: 200}[path == "/feedback/9999"]; res.StatusCode != want {
				t.Errorf("%s %s: status %d", lang, path, res.StatusCode)
			}
			for key := range feedbackText {
				if strings.Contains(page, ">"+key) || strings.Contains(page, `"`+key+`"`) {
					t.Errorf("%s %s: raw key %s on the page", lang, path, key)
				}
			}
			if lang != "en" && path == "/feedback" && !strings.Contains(page, feedbackText["fb.col.message"][indexOf(langs, lang)]) {
				t.Errorf("%s list is not translated", lang)
			}
		}
	}
}

func indexOf(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return 0
}

// A list that could not be read must not also say "No feedback yet.".
func TestFeedbackHubDownDoesNotClaimTheInboxIsEmpty(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.seedFeedback()
	e.hop.fb.FailWith(&fakehub.ErrSpec{Status: 500, Code: "server_error", Message: "boom"})
	_, page := e.get(t, "/feedback")
	if strings.Contains(page, "No feedback yet.") {
		t.Error("an unreadable list says the inbox is empty")
	}
}

// IMPERIUM_ALIASES hides a real identity on every screen: the sender's name too.
func TestFeedbackSenderNameFollowsAliases(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	e.a.cfg.Aliases = map[string]string{"helena": "superadmin"}
	for _, path := range []string{"/feedback", fmt.Sprintf("/feedback/%d", ids.px)} {
		_, page := e.get(t, path)
		if path != "/feedback" {
			continue
		}
		if !strings.Contains(page, "superadmin") || strings.Contains(page, ">helena<") {
			t.Errorf("%s shows the real sender name", path)
		}
	}
	_, page := e.get(t, "/feedback/3")
	if strings.Contains(page, ">helena<") || !strings.Contains(page, "superadmin") {
		t.Error("the report page shows the real sender name")
	}
}
