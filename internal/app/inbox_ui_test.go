package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

// seed stores one group per call directly and returns its id.
func seed(t *testing.T, a *App, app, route string, mod func(*db.Report)) int64 {
	t.Helper()
	r := db.Report{ID: fmt.Sprintf("seed-%s-%d", app, time.Now().UnixNano()), Kind: "error.500", App: app, Env: "production", At: time.Now().UTC().Truncate(time.Second),
		Class: `Illuminate\Database\QueryException`, Message: "SQLSTATE[HY000]", Origin: "app/Http/Controllers/LinkController.php:88",
		Route: route, Method: "GET", Ref: "7f3a9c1e2b4d", Version: "abc1234"}
	if mod != nil {
		mod(&r)
	}
	res, err := a.db.RecordNotification(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return res.GroupID
}

// signedIn returns a client signed in with the given roles.
func signedIn(t *testing.T, hop *fakeHop, srv *httptest.Server, username string, roles ...string) *http.Client {
	t.Helper()
	hop.ticket = map[string]any{"sub": "1", "username": username, "name": username, "email": username + "@x", "roles": roles, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	return c
}

func get(t *testing.T, c *http.Client, u string) (int, string) {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, body(res)
}

func act(t *testing.T, c *http.Client, srv *httptest.Server, id int64, action, reason string) (int, string) {
	t.Helper()
	res, err := c.PostForm(fmt.Sprintf("%s/notifications/%d/actions", srv.URL, id), url.Values{"action": {action}, "reason": {reason}})
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, body(res)
}

func statusOf(t *testing.T, a *App, id int64) string {
	t.Helper()
	n, err := a.db.Notification(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return n.Status
}

func auditActions(t *testing.T, a *App) []db.Audit {
	t.Helper()
	rows, err := a.db.Audits(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []db.Audit
	for _, r := range rows {
		if strings.HasPrefix(r.Action, "notification.") {
			out = append(out, r)
		}
	}
	return out
}

func TestNotificationPagesNeedASession(t *testing.T) {
	a, srv, _ := newInbox(t)
	id := seed(t, a, "linkhop", "GET /l/{code}", nil)
	c := client(t)
	for _, u := range []string{"/notifications", fmt.Sprintf("/notifications/%d", id)} {
		res, err := c.Get(srv.URL + u)
		if err != nil {
			t.Fatal(err)
		}
		b := body(res)
		if res.Request.URL.Path != "/login" || strings.Contains(b, "QueryException") {
			t.Errorf("%s without a session landed on %s", u, res.Request.URL)
		}
	}
	res, err := c.PostForm(fmt.Sprintf("%s/notifications/%d/actions", srv.URL, id), url.Values{"action": {"resolve"}})
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if statusOf(t, a, id) != "new" || len(auditActions(t, a)) != 0 {
		t.Fatal("an anonymous request changed something")
	}
}

func TestOwnerSeesListDetailBadgeAndFilters(t *testing.T) {
	a, srv, hop := newInbox(t)
	lh := seed(t, a, "linkhop", "GET /l/{code}", func(r *db.Report) { r.Message = "<script>alert(1)</script>" })
	seed(t, a, "pixelhop", "GET /i/{id}", func(r *db.Report) { r.Class = "panic"; r.Origin = "internal/app/x.go:5" })
	resolved := seed(t, a, "biohop", "GET /b/{slug}", nil)
	if _, err := a.db.ChangeNotification(context.Background(), resolved, "resolve", "", db.Audit{ActorSub: "9", ActorName: "x"}); err != nil {
		t.Fatal(err)
	}
	c := signedIn(t, hop, srv, "boss", "owner")

	code, b := get(t, c, srv.URL+"/notifications")
	if code != 200 {
		t.Fatalf("list: %d", code)
	}
	for _, want := range []string{"Notifications", "GET /l/{code}", "GET /i/{id}", `href="/notifications/` + fmt.Sprint(lh) + `"`, `class="badge"`, ">2</span>", "New · 2", "Resolved · 1", "All · 3"} {
		if !strings.Contains(b, want) {
			t.Errorf("list lacks %q", want)
		}
	}
	if strings.Contains(b, "GET /b/{slug}") {
		t.Error("the default list shows new groups only; the resolved one belongs to the Resolved filter")
	}
	_, b = get(t, c, srv.URL+"/notifications?status=resolved")
	if !strings.Contains(b, "GET /b/{slug}") || strings.Contains(b, "GET /l/{code}") {
		t.Error("resolved filter")
	}
	_, b = get(t, c, srv.URL+"/notifications?status=all&app=pixelhop")
	if !strings.Contains(b, "GET /i/{id}") || strings.Contains(b, "GET /l/{code}") || strings.Contains(b, "GET /b/{slug}") {
		t.Error("app filter")
	}
	_, b = get(t, c, srv.URL+"/notifications?status=bogus&app=%3Cx%3E")
	if !strings.Contains(b, "GET /l/{code}") || strings.Contains(b, "<x>") {
		t.Error("unknown filter values fall back to the default")
	}

	code, b = get(t, c, fmt.Sprintf("%s/notifications/%d", srv.URL, lh))
	if code != 200 {
		t.Fatalf("detail: %d", code)
	}
	for _, want := range []string{`Illuminate\Database\QueryException`, "app/Http/Controllers/LinkController.php:88", "7f3a9c1e2b4d", "abc1234", "&lt;script&gt;alert(1)&lt;/script&gt;", `value="ack"`, `value="resolve"`, `value="mute"`, "UTC"} {
		if !strings.Contains(b, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	if strings.Contains(b, "<script>alert(1)") || strings.Contains(b, `value="reopen"`) {
		t.Error("detail: message must be escaped, and a new group cannot be reopened")
	}
	if code, _ := get(t, c, srv.URL+"/notifications/99999"); code != 404 {
		t.Errorf("missing group: %d", code)
	}
	if code, _ := get(t, c, srv.URL+"/notifications/abc"); code != 404 {
		t.Errorf("bad id: %d", code)
	}
}

func TestNotificationActionsAreAudited(t *testing.T) {
	a, srv, hop := newInbox(t)
	id := seed(t, a, "linkhop", "GET /l/{code}", nil)
	c := signedIn(t, hop, srv, "boss", "owner")

	// ack
	code, b := act(t, c, srv, id, "ack", "looking")
	if code != 200 || !strings.Contains(b, "Done.") || statusOf(t, a, id) != "seen" {
		t.Fatalf("ack: %d %s", code, statusOf(t, a, id))
	}
	// the same action again is not a valid transition and says so
	_, b = act(t, c, srv, id, "ack", "")
	if !strings.Contains(b, "not possible") {
		t.Error("ack twice should explain itself")
	}
	// mute needs a reason
	_, b = act(t, c, srv, id, "mute", "")
	if !strings.Contains(b, "reason to mute") || statusOf(t, a, id) != "seen" {
		t.Errorf("mute without a reason: %s", statusOf(t, a, id))
	}
	_, _ = act(t, c, srv, id, "mute", strings.Repeat("x", 201))
	if statusOf(t, a, id) != "seen" {
		t.Error("a reason over 200 characters must be refused")
	}
	if _, _ = act(t, c, srv, id, "mute", "bot traffic"); statusOf(t, a, id) != "muted" {
		t.Fatal("mute")
	}
	if _, _ = act(t, c, srv, id, "reopen", ""); statusOf(t, a, id) != "new" {
		t.Fatal("reopen")
	}
	if _, _ = act(t, c, srv, id, "resolve", "fixed in abc1234"); statusOf(t, a, id) != "resolved" {
		t.Fatal("resolve")
	}
	// an action that does not exist is a bad request and writes nothing
	if code, _ := act(t, c, srv, id, "delete", ""); code != 400 {
		t.Errorf("unknown action: %d", code)
	}

	audits := auditActions(t, a) // newest first
	var seq []string
	for i := len(audits) - 1; i >= 0; i-- {
		seq = append(seq, audits[i].Action)
	}
	if strings.Join(seq, " ") != "notification.ack notification.mute notification.reopen notification.resolve" {
		t.Fatalf("audit trail: %v", seq)
	}
	mute := audits[2]
	if mute.ActorSub != "1" || mute.ActorName != "boss" || mute.App != "linkhop" || mute.Target != fmt.Sprint(id) || !mute.OK ||
		mute.Detail["reason"] != "bot traffic" || mute.Detail["from"] != "seen" || mute.Detail["to"] != "muted" {
		t.Fatalf("mute audit row: %+v", mute)
	}
	// and the audit page shows them
	_, b = get(t, c, srv.URL+"/audit")
	if !strings.Contains(b, "notification.mute") {
		t.Error("audit page")
	}
}

func TestNotificationActionFailsLoudlyWhenTheAuditRowCannotBeWritten(t *testing.T) {
	a, srv, hop := newInbox(t)
	logs := captureLog(t)
	id := seed(t, a, "linkhop", "GET /l/{code}", nil)
	c := signedIn(t, hop, srv, "boss", "owner")
	if _, err := a.db.Exec(`CREATE TRIGGER audit_fail BEFORE INSERT ON audit WHEN NEW.action LIKE 'notification.%' BEGIN SELECT RAISE(ABORT, 'disk full'); END`); err != nil {
		t.Fatal(err)
	}
	_, b := act(t, c, srv, id, "resolve", "x")
	if !strings.Contains(b, "Nothing was changed") || statusOf(t, a, id) != "new" {
		t.Fatalf("status %s, page: %q", statusOf(t, a, id), b)
	}
	if !strings.Contains(logs.String(), "disk full") {
		t.Error("the cause must reach the log")
	}
}

func TestNotificationRolesPerApp(t *testing.T) {
	a, srv, hop := newInbox(t)
	lh := seed(t, a, "linkhop", "GET /l/{code}", nil)
	px := seed(t, a, "pixelhop", "GET /i/{id}", nil)
	detail := func(id int64) string { return fmt.Sprintf("%s/notifications/%d", srv.URL, id) }

	// admin:pixelhop sees and changes pixelhop's reports only
	c := signedIn(t, hop, srv, "pxadmin", "admin:pixelhop")
	_, b := get(t, c, srv.URL+"/notifications?status=all")
	if !strings.Contains(b, "GET /i/{id}") || strings.Contains(b, "GET /l/{code}") {
		t.Error("an app admin must only list their own app")
	}
	_, b = get(t, c, srv.URL+"/notifications?status=all&app=linkhop")
	if strings.Contains(b, "GET /l/{code}") {
		t.Error("the app filter must not widen the scope")
	}
	if !strings.Contains(b, `<option value="pixelhop"`) || strings.Contains(b, `<option value="linkhop"`) {
		t.Error("the app drop-down must list only the visible apps")
	}
	if code, b := get(t, c, detail(lh)); code != 404 || strings.Contains(b, "QueryException") {
		t.Errorf("another app's report: %d", code)
	}
	if code, _ := act(t, c, srv, lh, "resolve", ""); code != 404 || statusOf(t, a, lh) != "new" {
		t.Errorf("acting on another app's report: %d %s", code, statusOf(t, a, lh))
	}
	if code, b := get(t, c, detail(px)); code != 200 || !strings.Contains(b, `value="ack"`) {
		t.Errorf("own app's report: %d", code)
	}
	if _, _ = act(t, c, srv, px, "ack", ""); statusOf(t, a, px) != "seen" {
		t.Error("an app admin can ack their own app's report")
	}
	_, b = get(t, c, srv.URL+"/")
	if strings.Contains(b, `class="badge"`) {
		t.Error("the badge must count only visible groups (pixelhop's is acknowledged now)")
	}

	// the role names another app: nothing of pixelhop
	c = signedIn(t, hop, srv, "lhadmin", "admin:linkhop")
	if code, _ := get(t, c, detail(px)); code != 404 {
		t.Errorf("admin:linkhop on pixelhop's report: %d", code)
	}
	// admin: with nothing after it grants nothing
	c = signedIn(t, hop, srv, "empty", "admin:")
	if _, b := get(t, c, srv.URL+"/notifications?status=all"); strings.Contains(b, "QueryException") || strings.Contains(b, "GET /l/") {
		t.Error("a bare admin: role must see nothing")
	}

	// support reads everything and changes nothing
	c = signedIn(t, hop, srv, "sup", "support")
	_, b = get(t, c, srv.URL+"/notifications?status=all")
	if !strings.Contains(b, "GET /l/{code}") || !strings.Contains(b, "GET /i/{id}") {
		t.Error("support should list every app")
	}
	_, b = get(t, c, detail(lh))
	if strings.Contains(b, `value="resolve"`) || !strings.Contains(b, "can read these reports but not change them") {
		t.Error("support must get a read-only detail page")
	}
	_, b = act(t, c, srv, lh, "resolve", "")
	if !strings.Contains(b, "may not change") || statusOf(t, a, lh) != "new" {
		t.Errorf("support changed a report: %s", statusOf(t, a, lh))
	}

	// moderator and admin act on any app
	c = signedIn(t, hop, srv, "mod", "moderator")
	if _, _ = act(t, c, srv, lh, "ack", ""); statusOf(t, a, lh) != "seen" {
		t.Error("moderator")
	}
	c = signedIn(t, hop, srv, "adm", "admin")
	if _, _ = act(t, c, srv, lh, "resolve", ""); statusOf(t, a, lh) != "resolved" {
		t.Error("admin")
	}
	// every change above was audited, and nothing was audited for the refused ones
	var who []string
	for _, r := range auditActions(t, a) {
		who = append(who, r.ActorName+":"+r.Action)
	}
	if got := strings.Join(who, " "); got != "adm:notification.resolve mod:notification.ack pxadmin:notification.ack" {
		t.Fatalf("audit: %s", got)
	}
}

func TestCrossSitePostIsRefused(t *testing.T) {
	a, srv, hop := newInbox(t)
	id := seed(t, a, "linkhop", "GET /l/{code}", nil)
	c := signedIn(t, hop, srv, "boss", "owner")
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/notifications/%d/actions", srv.URL, id), strings.NewReader(url.Values{"action": {"resolve"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if res.StatusCode != 403 || statusOf(t, a, id) != "new" {
		t.Fatalf("cross-site action: %d %s", res.StatusCode, statusOf(t, a, id))
	}
}

func TestHomeShowsNewErrors(t *testing.T) {
	a, srv, hop := newInbox(t)
	seed(t, a, "linkhop", "GET /l/{code}", nil)
	c := signedIn(t, hop, srv, "boss", "owner")
	_, b := get(t, c, srv.URL+"/")
	if !strings.Contains(b, "New server errors") || !strings.Contains(b, `href="/notifications?status=new"`) {
		t.Error("home page lacks the new-errors card")
	}
}

func TestPagination(t *testing.T) {
	a, srv, hop := newInbox(t)
	for i := 0; i < notifPageSize+3; i++ {
		seed(t, a, "linkhop", fmt.Sprintf("GET /r%d", i), nil)
	}
	c := signedIn(t, hop, srv, "boss", "owner")
	_, b := get(t, c, srv.URL+"/notifications")
	if !strings.Contains(b, "p=2") || strings.Count(b, `href="/notifications/`) != notifPageSize {
		t.Errorf("page 1: %d links", strings.Count(b, `href="/notifications/`))
	}
	_, b = get(t, c, srv.URL+"/notifications?p=2")
	if strings.Count(b, `href="/notifications/`) != 3 || strings.Contains(b, "Next") {
		t.Errorf("page 2: %d links", strings.Count(b, `href="/notifications/`))
	}
}

func TestPruneRunsFromTheSweepJob(t *testing.T) {
	a, _, _ := newInbox(t)
	old := seed(t, a, "linkhop", "GET /old", func(r *db.Report) { r.At = time.Now().UTC().Add(-23 * time.Hour) })
	if _, err := a.db.Exec(`UPDATE notifications SET last_seen=?, first_seen=? WHERE id=?`, time.Now().Add(-100*24*time.Hour).Unix(), time.Now().Add(-100*24*time.Hour).Unix(), old); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE notification_events SET at=? WHERE group_id=?`, time.Now().Add(-100*24*time.Hour).Unix(), old); err != nil {
		t.Fatal(err)
	}
	keep := seed(t, a, "linkhop", "GET /new", nil)
	a.pruneNotifications(context.Background())
	if _, err := a.db.Notification(context.Background(), old); err == nil {
		t.Error("a group not seen for 100 days must be pruned")
	}
	if _, err := a.db.Notification(context.Background(), keep); err != nil {
		t.Error("a fresh group must stay")
	}
}
