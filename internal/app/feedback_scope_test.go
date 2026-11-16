package app

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/imperium/internal/fakehub"
)

// Checks of who may see and change what, bad ids, broken hub answers and the menu count.

func countCalls(f *fakehub.Feedback, pred func(c fakehub.Call) bool) int {
	n := 0
	for _, c := range f.Calls() {
		if pred(c) {
			n++
		}
	}
	return n
}

// An admin of one app never sees another app's report, even a new one with a picture, and Imperium does not even
// ask the hub for its picture or mark it read.
func TestFeedbackScopedAdminAndAnotherAppsNewReport(t *testing.T) {
	e := newAnnEnv(t, "admin:pixelhop")
	ids := e.seedFeedback()
	e.hop.fb.ResetCalls()
	id := ids.lhNew
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d", id)); code != 404 {
		t.Errorf("detail: %d", code)
	}
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d/screenshot", id)); code != 404 {
		t.Errorf("picture: %d", code)
	}
	for _, act := range []string{"done", "reopen", "delete"} {
		if code, _ := e.fbPost(t, fmt.Sprintf("/feedback/%d/actions", id), url.Values{"action": {act}, "ack": {"yes"}, "reason": {"x"}}); code != 404 {
			t.Errorf("%s on another app's report: %d, want 404", act, code)
		}
	}
	if n := countCalls(e.hop.fb, func(c fakehub.Call) bool { return c.Method == "POST" || strings.Contains(c.Path, "screenshot") }); n != 0 {
		t.Errorf("%d hub calls changed or fetched another app's report", n)
	}
	if st, ok := e.hop.fb.Get(id); !ok || st.Status != "new" {
		t.Error("another app's report was changed")
	}
	// a hub that ignores the app filter does not leak the other apps into the list
	e.hop.fb.SetIgnoreApp(true)
	_, list := e.get(t, "/feedback")
	for _, leak := range []string{"Link of another app", "Please add folders", "日本語のテキスト Привет"} {
		if strings.Contains(list, leak) {
			t.Errorf("the list shows %q of another app", leak)
		}
	}
	if !strings.Contains(list, "Upload fails") {
		t.Error("its own report is missing")
	}
}

// Ids that are not the shape of an id never reach the hub.
func TestFeedbackBadIdsAnswer404WithoutAskingTheHub(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.seedFeedback()
	e.hop.fb.ResetCalls()
	for _, id := range []string{"%252e%252e", "a.b", "a%20b", strings.Repeat("a", 65), "%2e%2e"} {
		if code, _ := e.get(t, "/feedback/"+id); code != 404 {
			t.Errorf("detail %q: %d", id, code)
		}
		if code, _ := e.get(t, "/feedback/"+id+"/screenshot"); code != 404 {
			t.Errorf("picture %q: %d", id, code)
		}
		if code, _ := e.fbPost(t, "/feedback/"+id+"/actions", url.Values{"action": {"done"}}); code != 404 {
			t.Errorf("action %q: %d", id, code)
		}
	}
	for _, c := range e.hop.fb.Calls() {
		if !strings.Contains(c.Path, "limit=1") { // only the menu count may have asked
			t.Errorf("a bad id reached the hub: %s %s", c.Method, c.Path)
		}
	}
}

// A report that is gone and another app's report look the same to an admin of one app.
func TestFeedbackMissingAndForeignLookTheSame(t *testing.T) {
	e := newAnnEnv(t, "admin:pixelhop")
	ids := e.seedFeedback()
	stamp := regexp.MustCompile(`\d{4}-\d\d-\d\d \d\d:\d\d`)
	post := func(id int, act string) (int, string) {
		res, err := e.noFollow().PostForm(fmt.Sprintf("%s/feedback/%d/actions", e.srv.URL, id), url.Values{"action": {act}, "ack": {"yes"}, "reason": {"x"}})
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, regexp.MustCompile(`/feedback/[0-9]+/actions`).ReplaceAllString(stamp.ReplaceAllString(body(res), ""), "/feedback/X/actions")
	}
	for _, act := range []string{"done", "reopen", "delete"} {
		c1, b1 := post(9999, act)
		c2, b2 := post(ids.lhNew, act)
		if c1 != 404 || c2 != 404 || b1 != b2 {
			t.Errorf("%s: missing %d and foreign %d differ", act, c1, c2)
		}
	}
}

// Refusals not covered by the matrix: Reopen by support, Reopen and Delete of another app's report.
func TestFeedbackMoreRefusals(t *testing.T) {
	e := newAnnEnv(t, "support")
	ids := e.seedFeedback()
	e.hop.fb.ResetCalls()
	if code, _ := e.fbPost(t, fmt.Sprintf("/feedback/%d/actions", ids.hub), url.Values{"action": {"reopen"}}); code != 403 {
		t.Errorf("support Reopen: %d", code)
	}
	if hubCalls(e.hop.fb, "POST") != 0 {
		t.Error("support Reopen reached the hub")
	}
	e2 := newAnnEnv(t, "admin:pixelhop")
	ids2 := e2.seedFeedback()
	e2.hop.fb.ResetCalls()
	for _, act := range []string{"reopen", "delete"} {
		if code, _ := e2.fbPost(t, fmt.Sprintf("/feedback/%d/actions", ids2.hub), url.Values{"action": {act}, "ack": {"yes"}, "reason": {"x"}}); code != 404 {
			t.Errorf("admin:pixelhop %s of the hub's report: %d", act, code)
		}
	}
	if hubCalls(e2.hop.fb, "POST") != 0 {
		t.Error("a refused action reached the hub")
	}
	// and Delete of its own app's report is still not its to do
	if code, _ := e2.fbPost(t, fmt.Sprintf("/feedback/%d/actions", ids2.px), url.Values{"action": {"delete"}, "ack": {"yes"}, "reason": {"x"}}); code != 403 {
		t.Errorf("admin:pixelhop Delete of its own report: %d, want 403", code)
	}
}

// The sender's address is in no other page of a report either: the opened picture, a missing report, a hub error.
func TestFeedbackIPNeverInOtherPagesForOtherRoles(t *testing.T) {
	for _, role := range []string{"moderator", "support", "admin:pixelhop"} {
		e := newAnnEnv(t, role)
		ids := e.seedFeedback()
		for _, path := range []string{fmt.Sprintf("/feedback/%d?image=1", ids.px), fmt.Sprintf("/feedback/%d?f=app%%3Dpixelhop", ids.px), fmt.Sprintf("/feedback/%d", ids.lhNew), "/feedback/9999", "/feedback"} {
			_, page := e.get(t, path)
			if strings.Contains(page, ipPixel) || strings.Contains(page, "198.51.100") {
				t.Errorf("%s %s: an IP address is in the page", role, path)
			}
		}
	}
}

// An old hub, or an answer that is not JSON, on the detail, an action and the picture: a clear page, never the raw text.
func TestFeedbackOldHubAndBrokenAnswers(t *testing.T) {
	e := newAnnEnv(t, "owner")
	ids := e.seedFeedback()
	cases := map[string]func(){
		"old hub":   func() { e.hop.fb.SetMissing(true) },
		"not json":  func() { e.hop.fb.SetRaw(200, "<html>oops {not json") },
		"html 502":  func() { e.hop.fb.SetRaw(502, "<html>bad gateway</html>") },
		"json html": func() { e.hop.fb.SetRaw(200, `{"ok":true,"data":"<script>x</script>"}`) },
	}
	for name, set := range cases {
		e.hop.fb.SetMissing(false)
		e.hop.fb.SetRaw(0, "")
		set()
		for _, path := range []string{fmt.Sprintf("/feedback/%d", ids.px), fmt.Sprintf("/feedback/%d/screenshot", ids.px), "/feedback"} {
			code, page := e.get(t, path)
			if code == 500 || strings.Contains(page, "oops") || strings.Contains(page, "<script>x") || strings.Contains(page, "bad gateway</html>") {
				t.Errorf("%s %s: %d with raw text", name, path, code)
			}
		}
		code, loc := e.fbPost(t, fmt.Sprintf("/feedback/%d/actions", ids.px), url.Values{"action": {"done"}})
		if code != 302 && code != 502 && code != 404 {
			t.Errorf("%s: action answered %d %s", name, code, loc)
		}
	}
	e.hop.fb.SetMissing(false)
	e.hop.fb.SetRaw(0, "")
	if st, _ := e.hop.fb.Get(ids.px); st.Status == "done" {
		t.Error("an action was taken against a broken answer")
	}
}

// An admin of two apps sees both and no other.
func TestFeedbackAdminOfTwoApps(t *testing.T) {
	e := newAnnEnv(t, "admin:pixelhop", "admin:linkhop")
	ids := e.seedFeedback()
	for _, app := range []string{"pixelhop", "linkhop"} {
		_, l := e.get(t, "/feedback?app="+app)
		if strings.Contains(l, "日本語のテキスト Привет") {
			t.Errorf("%s list holds the hub's report", app)
		}
	}
	_, l := e.get(t, "/feedback?app=hub")
	if strings.Contains(l, "日本語のテキスト Привет") || strings.Contains(l, `value="hub"`) {
		t.Error("the hub's reports or filter are offered")
	}
	for _, id := range []int{ids.px, ids.lh, ids.lhNew} {
		if code, _ := e.get(t, fmt.Sprintf("/feedback/%d", id)); code != 200 {
			t.Errorf("report %d: %d", id, code)
		}
	}
	if code, _ := e.get(t, fmt.Sprintf("/feedback/%d", ids.hub)); code != 404 {
		t.Errorf("the hub's report: %d", code)
	}
}

// An action renews the count of the one who acted and leaves the others' cache in place (old, not emptied), so
// nobody's next page waits for the hub.
func TestFeedbackActionKeepsTheBadgeCache(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: "a"})
	e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: "b"})
	e.a.fbClear()
	e.get(t, "/")
	e.a.fb.mu.Lock()
	e.a.fb.m["other"] = &fbBadge{n: 7}
	e.a.fb.mu.Unlock()
	e.fbPost(t, "/feedback/1/actions", url.Values{"action": {"done"}})
	e.a.fb.mu.Lock()
	other, ok := e.a.fb.m["other"]
	mine := e.a.fb.m["*"]
	e.a.fb.mu.Unlock()
	if !ok || other.n != 7 || !other.at.IsZero() {
		t.Error("the others' count was emptied instead of marked old")
	}
	if mine == nil || mine.n != 1 {
		t.Errorf("the count of the one who acted was not renewed: %+v", mine)
	}
}

// The hub tells the last page by its next being null; an exactly full last page has no Next link.
func TestFeedbackExactPageHasNoNext(t *testing.T) {
	e := newAnnEnv(t, "owner")
	for i := 0; i < 25; i++ {
		e.hop.fb.Seed(fakehub.Report{App: "pixelhop", Message: fmt.Sprintf("Exact %02d", i)})
	}
	_, page := e.get(t, "/feedback")
	if !strings.Contains(page, "Exact 24") || strings.Contains(page, "Next") && strings.Contains(page, "cursor=") {
		t.Error("an exactly full last page points to an empty one")
	}
}
