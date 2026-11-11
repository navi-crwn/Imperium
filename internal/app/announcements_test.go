package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/fakehub"
)

// The announcements pages against the fake hub (internal/fakehub, the contract in docs/announcements-contract.md).

type annEnv struct {
	hop *fakeHop
	a   *App
	srv *httptest.Server
	c   *http.Client
}

func newAnnEnv(t *testing.T, roles ...string) *annEnv {
	t.Helper()
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	t.Cleanup(srv.Close)
	hop.ticket = scopedTicket("5", "boss", roles...)
	c := client(t)
	body(login(t, c, srv))
	return &annEnv{hop, a, srv, c}
}

// noFollow is the same session that shows the redirect itself.
func (e *annEnv) noFollow() *http.Client {
	return &http.Client{Jar: e.c.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (e *annEnv) get(t *testing.T, path string) (int, string) {
	t.Helper()
	res, err := e.c.Get(e.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, body(res)
}

func (e *annEnv) post(t *testing.T, path string, v url.Values) (int, string, string) {
	t.Helper()
	res, err := e.noFollow().PostForm(e.srv.URL+path, v)
	if err != nil {
		t.Fatal(err)
	}
	loc := res.Header.Get("Location")
	return res.StatusCode, loc, body(res)
}

func hoursAgo(h int) *time.Time {
	t := time.Now().UTC().Add(-time.Duration(h) * time.Hour).Truncate(time.Minute)
	return &t
}

// seedAll adds one announcement of every state and returns their ids: draft, live, scheduled, expired.
func (e *annEnv) seedAll() (draft, live, scheduled, expired int) {
	s := e.hop.ann
	draft = s.Seed(fakehub.Row{Title: "Draft notice", Body: "d", Level: "info"})
	live = s.Seed(fakehub.Row{Title: "Live notice", Body: "l", Level: "warning", PublishedAt: hoursAgo(5)})
	scheduled = s.Seed(fakehub.Row{Title: "Scheduled notice", Body: "s", Level: "info", AppSlug: "pixelhop", PublishedAt: hoursAgo(-48)})
	expired = s.Seed(fakehub.Row{Title: "Expired notice", Body: "e", Level: "critical", PublishedAt: hoursAgo(100), ExpiresAt: hoursAgo(50)})
	return
}

func validForm(extra url.Values) url.Values {
	v := url.Values{"title": {"Maintenance"}, "body": {"Line one\nLine two"}, "level": {"warning"}, "app_slug": {"pixelhop"}, "reason": {"planned work"}}
	for k, x := range extra {
		v[k] = x
	}
	return v
}

func lastCall(t *testing.T, s *fakehub.Store) fakehub.Call {
	t.Helper()
	c := s.Calls()
	if len(c) == 0 {
		t.Fatal("no call reached HOP ID")
	}
	return c[len(c)-1]
}

func TestAnnouncementsListShowsStatesAndMenus(t *testing.T) {
	e := newAnnEnv(t, "owner")
	draft, live, scheduled, expired := e.seedAll()
	code, page := e.get(t, "/announcements")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	for _, want := range []string{"Draft notice", "Live notice", "Scheduled notice", "Expired notice", "ann-draft", "ann-live", "ann-scheduled", "ann-expired", "ann-critical", `href="/announcements/new"`, "pixelhop"} {
		if !strings.Contains(page, want) {
			t.Errorf("list lacks %q", want)
		}
	}
	has := func(id int, act string) bool {
		return strings.Contains(page, fmt.Sprintf(`href="#dlg-`)) && strings.Contains(page, fmt.Sprintf(`/announcements/%d/actions`, id)) && strings.Contains(page, "-ann-"+fmt.Sprint(id)+"-"+act)
	}
	if !has(draft, "publish") || has(draft, "unpublish") {
		t.Error("a draft offers Publish, not Unpublish")
	}
	if !has(live, "unpublish") || has(live, "publish") {
		t.Error("a live notice offers Unpublish, not Publish")
	}
	if !has(scheduled, "publish") || !has(scheduled, "unpublish") {
		t.Error("a scheduled notice offers both")
	}
	if !has(expired, "unpublish") || has(expired, "publish") {
		t.Error("an expired notice cannot be published again")
	}
	for _, id := range []int{draft, live, scheduled, expired} {
		if !has(id, "delete") {
			t.Errorf("notice %d lacks Delete", id)
		}
	}
	d := dialogOf(t, page, fmt.Sprintf("dlg-0-ann-%d-delete", expired))
	if !strings.Contains(d, "destructive") || !strings.Contains(d, `name="reason" required`) {
		t.Error("Delete is the destructive variant and asks for a reason")
	}
	if c := e.hop.ann.Calls(); len(c) == 0 || c[0].Actor != "5:boss" {
		t.Errorf("the list call should carry the actor, got %+v", c)
	}
}

func TestAnnouncementsPagingAndFilters(t *testing.T) {
	e := newAnnEnv(t, "owner")
	for i := 0; i < 30; i++ {
		e.hop.ann.Seed(fakehub.Row{Title: fmt.Sprintf("N%02d", i), Body: "b", Level: "info", PublishedAt: hoursAgo(1)})
	}
	_, page := e.get(t, "/announcements")
	if !strings.Contains(page, "cursor=6") || !strings.Contains(page, "N29") || strings.Contains(page, "N04") {
		t.Error("the first page holds the newest 25 and links to the next with a cursor")
	}
	_, page2 := e.get(t, "/announcements?cursor=6")
	if !strings.Contains(page2, "N04") || strings.Contains(page2, "N29") || !strings.Contains(page2, "btn ghost sm") {
		t.Error("the second page holds the rest and links back to the first")
	}
	e.hop.ann.ResetCalls()
	e.get(t, "/announcements?state=draft&level=critical")
	if q := lastCall(t, e.hop.ann).Path; !strings.Contains(q, "state=draft") || !strings.Contains(q, "level=critical") {
		t.Errorf("filters not forwarded: %s", q)
	}
	e.hop.ann.ResetCalls()
	e.get(t, "/announcements?state=bogus&level=x")
	if q := lastCall(t, e.hop.ann).Path; strings.Contains(q, "bogus") {
		t.Errorf("an unknown filter value must not be forwarded: %s", q)
	}
}

func TestAnnouncementCreateSendsContractBodyAndAudits(t *testing.T) {
	e := newAnnEnv(t, "owner")
	code, page := e.get(t, "/announcements/new")
	if code != 200 || !strings.Contains(page, `name="title"`) || !strings.Contains(page, `name="reason" type="text" maxlength="255" required`) || !strings.Contains(page, `<option value="pixelhop">`) {
		t.Fatalf("form page: %d", code)
	}
	st, loc, _ := e.post(t, "/announcements", validForm(url.Values{"published_at": {"2026-12-01T09:30"}, "expires_at": {"2026-12-02T09:30"}}))
	if st != http.StatusFound || loc != "/announcements/1" {
		t.Fatalf("create: %d -> %q", st, loc)
	}
	call := lastCall(t, e.hop.ann)
	if call.Method != "POST" || call.Actor != "5:boss" {
		t.Errorf("call %+v", call)
	}
	b := call.Body
	if b["title"] != "Maintenance" || b["level"] != "warning" || b["app_slug"] != "pixelhop" || b["published_at"] != "2026-12-01T09:30:00Z" || b["expires_at"] != "2026-12-02T09:30:00Z" || b["publish_now"] != false || b["reason"] != "planned work" {
		t.Errorf("body to HOP ID: %+v", b)
	}
	row, ok := e.hop.ann.Get(1)
	if !ok || row.Author != "5:boss" || row.AppSlug != "pixelhop" {
		t.Errorf("stored: %+v", row)
	}
	n, detail, target := auditCount(t, e.a, "announcement.create")
	if n != 1 || target != "1" || detail["reason"] != "planned work" || detail["error"] != "" {
		t.Errorf("audit: n=%d target=%q detail=%v", n, target, detail)
	}
	// All apps and no dates: nulls, a draft.
	e.post(t, "/announcements", validForm(url.Values{"app_slug": {""}}))
	if b := lastCall(t, e.hop.ann).Body; b["app_slug"] != nil || b["published_at"] != nil || b["expires_at"] != nil {
		t.Errorf("empty fields must go as null: %+v", b)
	}
}

func TestAnnouncementFormValidationShowsFieldsAndSendsNothing(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.hop.ann.ResetCalls()
	cases := map[string]struct {
		extra url.Values
		want  string
	}{
		"title":    {url.Values{"title": {"  "}}, "Enter a title."},
		"long":     {url.Values{"title": {strings.Repeat("x", 161)}}, "at most 160"},
		"body":     {url.Values{"body": {""}}, "Enter the text"},
		"bodylong": {url.Values{"body": {strings.Repeat("y", 5001)}}, "at most 5000"},
		"level":    {url.Values{"level": {"loud"}}, "Pick info, warning or critical."},
		"app":      {url.Values{"app_slug": {"nosuchapp"}}, "registered apps"},
		"date":     {url.Values{"published_at": {"tomorrow"}}, "valid date"},
		"order":    {url.Values{"published_at": {"2026-12-02T09:00"}, "expires_at": {"2026-12-01T09:00"}}, "after the publish time"},
		"nopub":    {url.Values{"expires_at": {"2026-12-01T09:00"}}, "after the publish time"},
		"reason":   {url.Values{"reason": {" "}}, "reason"},
	}
	for name, tc := range cases {
		st, _, page := e.post(t, "/announcements", validForm(tc.extra))
		if st != http.StatusUnprocessableEntity || !strings.Contains(page, tc.want) || !strings.Contains(page, `class="field-err"`) {
			t.Errorf("%s: status %d, want 422 with %q", name, st, tc.want)
		}
	}
	if st, _, page := e.post(t, "/announcements", validForm(url.Values{"title": {""}, "body": {"typed text stays"}})); st != 422 || !strings.Contains(page, "typed text stays") {
		t.Error("what the person typed is kept")
	}
	if len(e.hop.ann.Calls()) != 0 || e.hop.ann.Len() != 0 {
		t.Errorf("an invalid form must send nothing: %+v", e.hop.ann.Calls())
	}
	if n, _, _ := auditCount(t, e.a, "announcement.create"); n != 0 {
		t.Errorf("nothing sent, nothing audited (%d)", n)
	}
}

func TestAnnouncementHubRefusalAndOutageAreShownAndAudited(t *testing.T) {
	e := newAnnEnv(t, "owner")
	e.hop.ann.FailWith(&fakehub.ErrSpec{Status: 422, Code: "bad_params", Message: "title is required"})
	st, _, page := e.post(t, "/announcements", validForm(nil))
	if st != 422 || !strings.Contains(page, "bad_params: title is required") || !strings.Contains(page, "Maintenance") {
		t.Errorf("a hub refusal is shown on the form with the input kept: %d", st)
	}
	e.hop.ann.FailWith(&fakehub.ErrSpec{Status: 500, Code: "http_500"})
	st, _, page = e.post(t, "/announcements", validForm(nil))
	if st != http.StatusBadGateway || !strings.Contains(page, "http_500") {
		t.Errorf("an outage is a 502 with the error: %d", st)
	}
	rows, _ := e.a.db.Audits(t.Context(), 0, 10)
	bad := 0
	for _, r := range rows {
		if r.Action == "announcement.create" && !r.OK && r.Detail["error"] != "" {
			bad++
		}
	}
	if bad != 2 {
		t.Errorf("both failures are in the audit trail as failed, got %d", bad)
	}
	// A list that fails says so and keeps the page.
	code, list := e.get(t, "/announcements")
	if code != 200 || !strings.Contains(list, "http_500") {
		t.Errorf("list on outage: %d", code)
	}
	e.hop.ann.FailWith(&fakehub.ErrSpec{Status: 404, Code: "not_found", Message: "No such announcement."})
	if _, p := e.get(t, "/announcements/99"); !strings.Contains(p, "not_found") {
		t.Error("a missing announcement shows the error")
	}
}

func TestAnnouncementEditPrefillsAndUpdates(t *testing.T) {
	e := newAnnEnv(t, "owner")
	_, live, _, _ := e.seedAll()
	_, page := e.get(t, fmt.Sprintf("/announcements/%d/edit", live))
	for _, want := range []string{`value="Live notice"`, `<option value="warning" selected>`, `name="published_at" type="datetime-local" value="`, `>l</textarea>`} {
		if !strings.Contains(page, want) {
			t.Errorf("edit form lacks %q", want)
		}
	}
	st, loc, _ := e.post(t, fmt.Sprintf("/announcements/%d", live), validForm(url.Values{"title": {"Live notice v2"}, "published_at": {"2026-01-01T00:00"}}))
	if st != http.StatusFound || loc != fmt.Sprintf("/announcements/%d", live) {
		t.Fatalf("update: %d -> %q", st, loc)
	}
	if c := lastCall(t, e.hop.ann); c.Method != "PUT" || !strings.HasSuffix(c.Path, fmt.Sprintf("/announcements/%d", live)) {
		t.Errorf("update must be a PUT: %+v", c)
	}
	if r, _ := e.hop.ann.Get(live); r.Title != "Live notice v2" {
		t.Errorf("stored title %q", r.Title)
	}
	if n, _, target := auditCount(t, e.a, "announcement.update"); n != 1 || target != fmt.Sprint(live) {
		t.Errorf("update audit n=%d target=%s", n, target)
	}
	// An announcement whose app left the registry stays editable.
	e.hop.ann.Apps = append(e.hop.ann.Apps, "retired") // HOP ID still knows it; Imperium's registry copy does not
	id := e.hop.ann.Seed(fakehub.Row{Title: "Old", Body: "b", Level: "info", AppSlug: "retired"})
	if st, _, _ := e.post(t, fmt.Sprintf("/announcements/%d", id), validForm(url.Values{"app_slug": {"retired"}, "app_was": {"retired"}})); st != http.StatusFound {
		t.Errorf("an unchanged retired app must not block the edit, got %d", st)
	}
	if st, _, _ := e.post(t, fmt.Sprintf("/announcements/%d", id), validForm(url.Values{"app_slug": {"retired"}})); st != 422 {
		t.Errorf("choosing an unknown app is refused, got %d", st)
	}
}

func TestAnnouncementActionsPublishUnpublishDelete(t *testing.T) {
	e := newAnnEnv(t, "owner")
	draft, live, _, expired := e.seedAll()
	do := func(id int, act, reason, ack string) (int, string) {
		v := url.Values{"action": {act}, "reason": {reason}}
		if ack != "" {
			v.Set("ack", ack)
		}
		st, loc, _ := e.post(t, fmt.Sprintf("/announcements/%d/actions", id), v)
		return st, loc
	}
	if st, loc := do(draft, "publish", "ready", "yes"); st != 302 || loc != fmt.Sprintf("/announcements/%d", draft) {
		t.Errorf("publish: %d %s", st, loc)
	}
	if r, _ := e.hop.ann.Get(draft); r.PublishedAt == nil {
		t.Error("publish did not publish")
	}
	if c := lastCall(t, e.hop.ann); c.Body["action"] != "publish" || c.Body["reason"] != "ready" || c.Actor != "5:boss" {
		t.Errorf("publish call %+v", c)
	}
	do(live, "unpublish", "typo", "yes")
	if r, _ := e.hop.ann.Get(live); r.PublishedAt != nil {
		t.Error("unpublish did not make it a draft")
	}
	if st, loc := do(expired, "delete", "obsolete", "yes"); st != 302 || loc != "/announcements" {
		t.Errorf("delete goes back to the list: %d %s", st, loc)
	}
	if _, ok := e.hop.ann.Get(expired); ok {
		t.Error("delete did not delete")
	}
	for act, n := range map[string]int{"announcement.publish": 1, "announcement.unpublish": 1, "announcement.delete": 1} {
		if got, d, _ := auditCount(t, e.a, act); got != n || d["reason"] == "" {
			t.Errorf("%s audit: %d %v", act, got, d)
		}
	}
	// A hub refusal (publishing an expired notice) is reported and audited as failed.
	e.hop.ann.FailWith(&fakehub.ErrSpec{Status: 409, Code: "expired", Message: "no"})
	do(draft, "publish", "again", "yes")
	rows, _ := e.a.db.Audits(t.Context(), 0, 20)
	failed := false
	for _, r := range rows {
		failed = failed || (r.Action == "announcement.publish" && !r.OK)
	}
	if !failed {
		t.Error("a refused action is audited as failed")
	}
}

func TestAnnouncementActionRefusedWithoutReasonOrAck(t *testing.T) {
	e := newAnnEnv(t, "owner")
	draft, _, _, _ := e.seedAll()
	e.hop.ann.ResetCalls()
	path := fmt.Sprintf("/announcements/%d/actions", draft)
	for _, tc := range []struct {
		name string
		v    url.Values
	}{
		{"no reason", url.Values{"action": {"publish"}, "ack": {"yes"}}},
		{"blank reason", url.Values{"action": {"publish"}, "ack": {"yes"}, "reason": {"   "}}},
		{"delete without ack", url.Values{"action": {"delete"}, "reason": {"x"}}},
		{"delete without reason", url.Values{"action": {"delete"}, "ack": {"yes"}}},
	} {
		st, loc, _ := e.post(t, path, tc.v)
		if st != http.StatusFound || loc != fmt.Sprintf("/announcements/%d", draft) {
			t.Errorf("%s: %d %s", tc.name, st, loc)
		}
	}
	if st, _, _ := e.post(t, path, url.Values{"action": {"explode"}, "reason": {"x"}, "ack": {"yes"}}); st != 400 {
		t.Errorf("unknown action: %d", st)
	}
	if len(e.hop.ann.Calls()) != 0 {
		t.Errorf("a refused request must not reach HOP ID: %+v", e.hop.ann.Calls())
	}
	rows, _ := e.a.db.Audits(t.Context(), 0, 20)
	for _, r := range rows {
		if strings.HasPrefix(r.Action, "announcement.") {
			t.Errorf("a refused request must not be audited: %s", r.Action)
		}
	}
	if r, ok := e.hop.ann.Get(draft); !ok || r.PublishedAt != nil {
		t.Error("the draft must be untouched")
	}
}

func TestAnnouncementRolesReadOnlyBelowAdmin(t *testing.T) {
	for _, role := range []string{"moderator", "support"} {
		e := newAnnEnv(t, role)
		draft, _, _, _ := e.seedAll()
		_, list := e.get(t, "/announcements")
		if !strings.Contains(list, "Draft notice") || strings.Contains(list, `href="/announcements/new"`) || strings.Contains(list, `<dialog class="confirm"`) {
			t.Errorf("%s reads the list without the New button or menus", role)
		}
		_, one := e.get(t, fmt.Sprintf("/announcements/%d", draft))
		if !strings.Contains(one, "Draft notice") || strings.Contains(one, "/edit") || strings.Contains(one, `<dialog class="confirm"`) {
			t.Errorf("%s reads one notice without Edit or dialogs", role)
		}
		if code, _ := e.get(t, "/announcements/new"); code != 403 {
			t.Errorf("%s opened the form: %d", role, code)
		}
		e.hop.ann.ResetCalls()
		st1, _, _ := e.post(t, "/announcements", validForm(nil))
		st2, _, _ := e.post(t, fmt.Sprintf("/announcements/%d", draft), validForm(nil))
		st3, _, _ := e.post(t, fmt.Sprintf("/announcements/%d/actions", draft), url.Values{"action": {"delete"}, "ack": {"yes"}, "reason": {"x"}})
		if st1 != 403 || st2 != 403 || st3 != 403 || len(e.hop.ann.Calls()) != 0 {
			t.Errorf("%s wrote: %d %d %d, calls %d", role, st1, st2, st3, len(e.hop.ann.Calls()))
		}
	}
	// admin:<app> is scoped to its app and has no hub pages.
	e := newAnnEnv(t, "admin:pixelhop")
	if code, _ := e.get(t, "/announcements"); code != 403 {
		t.Errorf("admin:pixelhop sees the announcements: %d", code)
	}
	// An admin below owner may write.
	e = newAnnEnv(t, "admin")
	if st, _, _ := e.post(t, "/announcements", validForm(nil)); st != http.StatusFound {
		t.Errorf("an admin may create: %d", st)
	}
}

func TestAnnouncementsInTheHubNavigationAndDetail(t *testing.T) {
	e := newAnnEnv(t, "owner")
	draft, _, _, _ := e.seedAll()
	_, page := e.get(t, "/announcements")
	if !strings.Contains(page, `href="/announcements"`) || !strings.Contains(page, ">Announcements<") {
		t.Error("the hub section links to Announcements")
	}
	_, one := e.get(t, fmt.Sprintf("/announcements/%d", draft))
	for _, want := range []string{"Draft notice", "Actions", fmt.Sprintf(`href="/announcements/%d/edit"`, draft), "pre-wrap"} {
		if !strings.Contains(one, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	e.hop.ann.Seed(fakehub.Row{Title: "<b>x</b>", Body: "<script>alert(1)</script>", Level: "info"})
	_, esc := e.get(t, "/announcements/5")
	if strings.Contains(esc, "<script>alert(1)</script>") || strings.Contains(esc, "<b>x</b></h1>") {
		t.Error("announcement text must be escaped")
	}
}

// Nine languages, no Arabic: every label has a value in each, the same %d placeholders, and every page renders
// in each language without a raw key left on it.
func TestAnnouncementTextsInNineLanguages(t *testing.T) {
	langs := []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"}
	if fmt.Sprint(langOrder) != fmt.Sprint(langs) {
		t.Fatalf("langOrder = %v", langOrder)
	}
	for key, vals := range announceText {
		for i, v := range vals {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s has no %s text", key, langs[i])
			}
			if strings.Count(v, "%d") != strings.Count(vals[0], "%d") {
				t.Errorf("%s (%s) has a different number of %%d than English", key, langs[i])
			}
			if catalog[langs[i]][key] != v {
				t.Errorf("%s (%s) is not in the catalogue", key, langs[i])
			}
		}
	}
	if _, ok := catalog["ar"]; ok {
		t.Error("Arabic must not be added")
	}
	e := newAnnEnv(t, "owner")
	draft, _, _, _ := e.seedAll()
	for _, lang := range langs {
		for _, path := range []string{"/announcements", "/announcements/new", fmt.Sprintf("/announcements/%d", draft), fmt.Sprintf("/announcements/%d/edit", draft), "/users/26"} {
			req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
			req.AddCookie(&http.Cookie{Name: "hop_lang", Value: lang})
			res, err := e.c.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			page := body(res)
			if res.StatusCode != 200 {
				t.Errorf("%s %s: status %d", lang, path, res.StatusCode)
			}
			for key := range announceText {
				if strings.Contains(page, ">"+key) || strings.Contains(page, `"`+key+`"`) {
					t.Errorf("%s %s: raw key %s on the page", lang, path, key)
				}
			}
			if lang != "en" && !strings.Contains(page, fmt.Sprintf(`<html lang="%s"`, lang)) {
				t.Errorf("%s %s: not rendered in %s", lang, path, lang)
			}
		}
	}
	// A validation message appears translated.
	req, _ := http.NewRequest("POST", e.srv.URL+"/announcements", strings.NewReader(validForm(url.Values{"title": {""}}).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "hop_lang", Value: "de"})
	res, _ := e.c.Do(req)
	if page := body(res); !strings.Contains(page, "Bitte einen Titel eingeben.") {
		t.Error("the German validation text is missing")
	}
}
