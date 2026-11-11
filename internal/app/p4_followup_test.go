package app

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

func TestSyncIsLimitedToGlobalAdmins(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()

	logins := 0
	for _, role := range []string{"admin:pixelhop", "moderator", "support"} {
		hop.ticket = scopedTicket("27", "scoped", role)
		c := client(t)
		body(login(t, c, srv))
		logins++
		waitBackgroundSync(t, a, hop, logins)
		_ = a.db.UpsertApp(t.Context(), registryAppForTest())

		registryBefore := hop.registryReads()
		res, err := c.PostForm(srv.URL+"/apps/sync", url.Values{})
		if err != nil {
			t.Fatal(err)
		}
		page := body(res)
		if res.Request.URL.Path != "/apps" || !strings.Contains(page, escapeForTest("Your role may not do this.")) {
			t.Errorf("%s sync was not refused: %s %q", role, res.Request.URL, page)
		}
		if got := hop.registryReads() - registryBefore; got != 0 {
			t.Errorf("%s sync reached the hub registry %d time(s)", role, got)
		}
		apps, _ := a.db.Apps(t.Context())
		for _, ap := range apps {
			if ap.Slug == "pixelhop" && ap.URL != "https://before.example" {
				t.Errorf("%s changed registry URL to %q", role, ap.URL)
			}
		}
		_, page = get(t, c, srv.URL+"/apps")
		if strings.Contains(page, `action="/apps/sync"`) {
			t.Errorf("%s still sees the Sync form", role)
		}
		if role == "admin:pixelhop" && strings.Contains(page, `action="/search"`) {
			t.Error("a scoped admin still sees global Search")
		}
	}

	for _, role := range []string{"owner", "admin"} {
		hop.ticket = scopedTicket("5", "boss", role)
		c := client(t)
		body(login(t, c, srv))
		logins++
		waitBackgroundSync(t, a, hop, logins)
		_ = a.db.UpsertApp(t.Context(), registryAppForTest())
		registryBefore := hop.registryReads()
		res, err := c.PostForm(srv.URL+"/apps/sync", url.Values{})
		if err != nil {
			t.Fatal(err)
		}
		page := body(res)
		if !strings.Contains(page, "Registry refreshed.") {
			t.Errorf("%s could not sync: %q", role, page)
		}
		if got := hop.registryReads() - registryBefore; got != 1 {
			t.Errorf("%s sync made %d signed registry reads, want exactly 1", role, got)
		}
		_, page = get(t, c, srv.URL+"/apps")
		if !strings.Contains(page, `method="post" action="/apps/sync"`) || !strings.Contains(page, `action="/search"`) {
			t.Errorf("%s lost Sync or Search", role)
		}
	}
}

// waitBackgroundSync waits until the sign-in's background registry sync has fully
// landed: `reads` signed registry reads have been counted (through the fake's
// mutex) and PixelHop's row carries the fake hub's URL (the sync's final
// persisted write), so counter deltas around a POST measure only that POST.
func waitBackgroundSync(t *testing.T, a *App, hop *fakeHop, reads int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if hop.registryReads() >= reads {
			apps, _ := a.db.Apps(context.Background())
			for _, ap := range apps {
				if ap.Slug == "pixelhop" && ap.URL == hop.px.URL {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("background registry sync did not finish (reads=%d, want %d)", hop.registryReads(), reads)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func registryAppForTest() *db.App {
	return &db.App{Slug: "pixelhop", Name: "PixelHop", URL: "https://before.example", Prefix: "/hop", Secret: pxSecret, Status: "active", Capabilities: []string{"admin_api"}}
}

func TestSettingsInputsFollowServerSideSaveScope(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()

	// Denied roles: the values are read from PixelHop's signed GET, shown instead of
	// inputs, the save button is gone, and a crafted save is refused before it is forwarded.
	for _, role := range []string{"admin:pixelhop", "moderator", "support"} {
		hop.ticket = scopedTicket("27", "scoped", role)
		c := client(t)
		body(login(t, c, srv))
		waitRegistrySynced(t, a, hop)
		code, page := get(t, c, srv.URL+"/apps/pixelhop/settings")
		if code != 200 || !strings.Contains(page, "Only an owner or a global admin can save these.") {
			t.Errorf("%s lacks the Settings scope explanation: %d %q", role, code, page)
		}
		if strings.Contains(page, `name="s_guest_uploads"`) || !strings.Contains(page, `class="mono">true</span>`) || strings.Contains(page, `class="btn primary" type="submit"`) {
			t.Errorf("%s can edit or submit Settings: %q", role, page)
		}
		res, err := c.PostForm(srv.URL+"/apps/pixelhop/settings", url.Values{"t_guest_uploads": {"bool"}, "s_guest_uploads": {"1"}})
		if err != nil {
			t.Fatal(err)
		}
		// a scoped admin gets a direct 403, a hub-wide non-admin a redirect with the error flash;
		// both must say why and neither may forward anything
		if page := body(res); !strings.Contains(page, "Your role may not do this.") {
			t.Errorf("%s Settings save was not refused: %d %q", role, res.StatusCode, page)
		}
	}
	if got := hop.settingsPuts(); got != 0 {
		t.Fatalf("unauthorized Settings saves reached PixelHop %d time(s)", got)
	}
	if got := hop.settingsGets(); got < 3 {
		t.Errorf("the settings page was not read from PixelHop for the denied roles (%d GETs)", got)
	}

	// Owner and admin keep the editable form; a save reaches PixelHop as one signed PUT.
	for _, role := range []string{"owner", "admin"} {
		hop.ticket = scopedTicket("5", "boss", role)
		c := client(t)
		body(login(t, c, srv))
		waitRegistrySynced(t, a, hop)
		code, page := get(t, c, srv.URL+"/apps/pixelhop/settings")
		if code != 200 || !strings.Contains(page, `name="s_guest_uploads"`) || !strings.Contains(page, `class="btn primary" type="submit"`) || strings.Contains(page, "Only an owner or a global admin can save these.") {
			t.Fatalf("%s lost the editable Settings: %d %q", role, code, page)
		}
		before := hop.settingsPuts()
		res, err := c.PostForm(srv.URL+"/apps/pixelhop/settings", url.Values{"t_guest_uploads": {"bool"}, "s_guest_uploads": {"0"}})
		if err != nil {
			t.Fatal(err)
		}
		page = body(res)
		if !strings.Contains(page, "Done.") {
			t.Errorf("%s Settings save not confirmed: %q", role, page)
		}
		if got := hop.settingsPuts() - before; got != 1 {
			t.Fatalf("%s Settings save made %d upstream PUT(s), want 1", role, got)
		}
		if got := hop.lastSettingsPut()["guest_uploads"]; got != false {
			t.Fatalf("%s Settings save sent guest_uploads=%v, want false", role, got)
		}
		// an upstream refusal is shown, not swallowed
		hop.setSettingsFail(true)
		res, err = c.PostForm(srv.URL+"/apps/pixelhop/settings", url.Values{"t_guest_uploads": {"bool"}, "s_guest_uploads": {"1"}})
		if err != nil {
			t.Fatal(err)
		}
		if page := body(res); !strings.Contains(page, "db_locked") {
			t.Errorf("%s did not see the upstream refusal: %q", role, page)
		}
		hop.setSettingsFail(false)
	}
}

func TestPixelHopContentStatusIsAllowlisted(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	waitRegistrySynced(t, a, hop)

	// Images: every listed status is forwarded to PixelHop. The statuses are
	// predicates, not stored states: a visible image may also be expiring.
	for _, status := range []string{"", "visible", "hidden", "private", "expiring"} {
		before := len(hop.contentQueries)
		code, page := get(t, c, srv.URL+"/apps/pixelhop/content?type=image&status="+url.QueryEscape(status))
		if code != 200 || len(hop.contentQueries) != before+1 {
			t.Fatalf("valid status %q did not reach PixelHop: %d", status, code)
		}
		query := hop.contentQueries[len(hop.contentQueries)-1]
		if !strings.Contains(query, "status="+url.QueryEscape(status)) {
			t.Errorf("valid status %q was not preserved in %q", status, query)
		}
		for _, want := range []string{`<select id="f-status"`, `value="visible"`, `value="hidden"`, `value="private"`, `value="expiring"`, `>Search</label>`, `>Status</label>`} {
			if !strings.Contains(page, want) {
				t.Errorf("PixelHop image filter lacks %q", want)
			}
		}
	}
	before := len(hop.contentQueries)
	code, page := get(t, c, srv.URL+"/apps/pixelhop/content?type=image&status=zzz")
	if len(hop.contentQueries) != before {
		t.Fatal("an unknown status reached PixelHop")
	}
	if code != 200 || !strings.Contains(page, "Choose a status from the list.") {
		t.Fatalf("unknown image status was not explained: %d %q", code, page)
	}

	// Albums: PixelHop ignores a status for albums and renders every album as
	// visible, so Imperium offers no status control and strips a smuggled one
	// instead of presenting a filter that does nothing.
	for _, status := range []string{"hidden", "zzz"} {
		before := len(hop.contentQueries)
		code, page := get(t, c, srv.URL+"/apps/pixelhop/content?type=album&status="+url.QueryEscape(status))
		if code != 200 || len(hop.contentQueries) != before+1 {
			t.Fatalf("album list was blocked by status %q: %d", status, code)
		}
		query := hop.contentQueries[len(hop.contentQueries)-1]
		if strings.Contains(query, "status="+url.QueryEscape(status)) {
			t.Errorf("album status %q was forwarded as a filter in %q", status, query)
		}
		if strings.Contains(page, "Choose a status from the list.") {
			t.Errorf("album status %q was refused although albums are not filtered", status)
		}
		if strings.Contains(page, `<select id="f-status"`) || strings.Contains(page, `<input id="f-status"`) {
			t.Errorf("albums are offered a status filter that does nothing: %q", page)
		}
	}
}

func TestCrossOriginPostLeavesNoTrace(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	waitRegistrySynced(t, a, hop)
	registryBefore := hop.registryReads()
	putsBefore := hop.settingsPuts()
	for _, path := range []string{"/apps/sync", "/apps/pixelhop/settings", "/logout"} {
		req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader("action=sync"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body(res)
		if res.StatusCode != 403 {
			t.Errorf("cross-site POST %s got %d, want 403", path, res.StatusCode)
		}
	}
	if got := hop.registryReads() - registryBefore; got != 0 {
		t.Errorf("a cross-site /apps/sync made %d registry read(s)", got)
	}
	if got := hop.settingsPuts() - putsBefore; got != 0 {
		t.Errorf("a cross-site settings save made %d PixelHop PUT(s)", got)
	}
	res, err := c.Get(srv.URL + "/users")
	if err != nil {
		t.Fatal(err)
	}
	code := res.StatusCode
	body(res)
	if code != 200 || res.Request.URL.Path != "/users" {
		t.Fatalf("a cross-site /logout left the session unusable: %s %d", res.Request.URL, code)
	}
}

func TestLoginDenialNamesTheAccountAndLogoutConfirms(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("26", "rani")
	hop.ticket["roles"] = []string{}
	c := client(t)
	res := login(t, c, srv)
	if page := body(res); !strings.Contains(page, "@rani") || !strings.Contains(page, "no admin role") {
		t.Fatalf("denial does not identify the account: %q", page)
	}

	// the 2FA denial also names the account
	hop.ticket = scopedTicket("26", "rani", "admin")
	hop.ticket["mfa"] = false
	res = login(t, c, srv)
	if page := body(res); !strings.Contains(page, "@rani") || !strings.Contains(page, "2FA") {
		t.Fatalf("the 2FA denial does not identify the account: %q", page)
	}

	hop.ticket = scopedTicket("5", "boss", "owner")
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/logout", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if page := body(res); !strings.Contains(page, "You have signed out of Imperium.") {
		t.Fatalf("logout confirmation missing: %q", page)
	}
}

func TestDevelopmentVersionAndSingleSecurityHeaders(t *testing.T) {
	a := &App{cfg: Config{Lang: "en"}}
	if got := versionLabel("en", "dev"); got != "development build" {
		t.Fatalf("development version = %q", got)
	}
	if got := versionLabel("en", "1"); got != "1" {
		t.Fatalf("an upstream release value was changed: %q", got)
	}
	hop := newFakeHop(t)
	app, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	waitRegistrySynced(t, app, hop)
	app.mu.Lock()
	app.health["pixelhop"] = Health{Slug: "pixelhop", OK: true, Version: "dev"}
	app.mu.Unlock()
	_, page := get(t, c, srv.URL+"/apps")
	if !strings.Contains(page, "development build") {
		t.Fatal("the app registry does not label a development version")
	}
	req := httptest.NewRequest("GET", "/healthz", nil)
	res := httptest.NewRecorder()
	a.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(res, req)
	for _, name := range []string{"X-Robots-Tag", "X-Frame-Options", "X-Content-Type-Options"} {
		if got := len(res.Header().Values(name)); got != 1 {
			t.Errorf("%s appears %d times", name, got)
		}
	}
	nginx, err := os.ReadFile("../../deploy/nginx-imperium.hel.ink.conf")
	if err != nil {
		t.Fatal(err)
	}
	// nginx sets each header once, for its own answers too (502 while the app is down), and drops the copy
	// the app sends, so a response never carries one twice. Measured with a real nginx; this pins the file.
	for _, name := range []string{"X-Robots-Tag", "X-Frame-Options", "X-Content-Type-Options"} {
		if n := strings.Count(string(nginx), "proxy_hide_header "+name+";"); n != 1 {
			t.Errorf("nginx hides the app's %s %d time(s), want once", name, n)
		}
		if n := strings.Count(string(nginx), "add_header "+name+" "); n != 1 {
			t.Errorf("nginx sets %s %d time(s), want once", name, n)
		}
	}
}

func TestCursorURLAlwaysSeparatesTheQuery(t *testing.T) {
	got := cursorURL("/users", url.Values{"role": {""}}, "a b&c")
	if got != "/users?cursor=a+b%26c&role=" {
		t.Fatalf("cursor URL = %q", got)
	}
}

// TestPaginationHasAFirstPageLink checks the first-page navigation
// semantically: the HTML-escaped href is unescaped and its query compared to
// the request's own filters minus the cursor — the link builder's contract
// (pageURL drops only "cursor"). This is independent of parameter order and of
// the &amp; spelling: an empty filter key stays empty, a missing one stays
// missing (the earlier expectation of a role= the request never sent was
// wrong), and a non-empty role filter keeps being preserved.
func TestPaginationHasAFirstPageLink(t *testing.T) {
	hop := newFakeHop(t)
	a, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	waitRegistrySynced(t, a, hop)
	for _, path := range []string{
		"/users?cursor=26&q=rani&status=active",
		"/users?cursor=26&q=rani&role=owner&status=active", // a non-empty role filter must survive too
		"/apps/pixelhop/content?type=image&cursor=50&q=cat&status=visible&owner=26",
	} {
		_, page := get(t, c, srv.URL+path)
		got := firstPageQuery(t, page)
		want, err := url.Parse(path)
		if err != nil {
			t.Fatal(err)
		}
		filters := want.Query()
		filters.Del("cursor")
		if !reflect.DeepEqual(got, filters) {
			t.Errorf("%s first-page link lost filters: got %v, want %v", path, got, filters)
		}
		if _, ok := got["cursor"]; ok {
			t.Errorf("%s first-page link carries a cursor: %v", path, got)
		}
	}
}

// firstPageQuery returns the query of the "First page" pagination link:
// HTML-unescaped (href carries &amp;) and parsed into url.Values. A page
// without the link is a test failure.
func firstPageQuery(t *testing.T, page string) url.Values {
	t.Helper()
	m := regexp.MustCompile(`href="([^"]*)"[^>]*>\s*First page`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no first-page navigation in %q", page)
	}
	u, err := url.Parse(html.UnescapeString(m[1]))
	if err != nil {
		t.Fatalf("first-page href %q: %v", m[1], err)
	}
	return u.Query()
}
