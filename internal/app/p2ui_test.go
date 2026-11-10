package app

import (
	"io/fs"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/cmd/imperium/webfs"
)

// The dashboard test of 2026-11-08 (P2) found the bulk panel on /users defaulting to
// "Suspend" and its Apply button live with no rows ticked, so an empty submit looked
// like it would suspend everybody and the server answered "this changes many rows".
// The select now starts on an empty "choose" option and Apply is disabled until a row
// is picked; the server says which of the two is missing, and it counts rows before it
// judges the confirmation.

func TestBulkPanelStartsWithoutAnAction(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))

	for _, path := range []string{"/users", "/apps/pixelhop/content?type=image", "/apps/pixelhop/reports"} {
		code, page := get(t, c, srv.URL+path)
		if code != 200 {
			t.Fatalf("%s: status %d", path, code)
		}
		if !strings.Contains(page, `data-bulk-action`) {
			t.Errorf("%s: the action select lacks data-bulk-action", page)
			continue
		}
		if !strings.Contains(page, `<option value="" selected>`) {
			t.Errorf("%s: the action select does not start on the empty choice", path)
		}
		if strings.Contains(page, `data-bulk-apply>`) {
			t.Errorf("%s: the Apply button is rendered enabled", path)
		}
		if !strings.Contains(page, `data-bulk-apply disabled`) {
			t.Errorf("%s: the Apply button is not disabled before any row is ticked", path)
		}
		// The first option must not be an action: a form submitted before JavaScript
		// runs would otherwise send Suspend as the action.
		firstOption := page[strings.Index(page, `data-bulk-action`):]
		firstOption = firstOption[strings.Index(firstOption, "<option"):]
		firstOption = firstOption[:strings.Index(firstOption, "</option>")]
		if !strings.Contains(firstOption, `value=""`) {
			t.Errorf("%s: the first option is an action, not the empty choice: %s", path, firstOption)
		}
	}
}

func TestUsersBulkSaysNoActionBeforeConfirmation(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	hop.actions = nil

	// An empty action with a ticked row: the answer is "choose an action", not "no rows".
	res, err := c.PostForm(srv.URL+"/users/actions", url.Values{"pick": {"26"}})
	if err != nil {
		t.Fatal(err)
	}
	page := body(res)
	if !strings.Contains(page, escapeForTest("Choose an action first.")) {
		t.Errorf("empty action should say so: %q", page)
	}
	// Zero rows and a destructive action: the answer is "no rows", not "many rows".
	res, err = c.PostForm(srv.URL+"/users/actions", url.Values{"action": {"suspend"}})
	if err != nil {
		t.Fatal(err)
	}
	page = body(res)
	if !strings.Contains(page, escapeForTest("No rows selected.")) {
		t.Errorf("zero rows should say so: %q", page)
	}
	if strings.Contains(page, escapeForTest("changes many rows")) {
		t.Errorf("zero rows must not be reported as a large change: %q", page)
	}
	if len(hop.actions) != 0 {
		t.Fatalf("nothing may reach HOP ID, got %+v", hop.actions)
	}
}

func TestContentBulkSaysNoActionAndCountsRowsFirst(t *testing.T) {
	hop, up, _, srv := newContentBulkSetup(t, map[string]bool{})
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))

	res, err := c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "pick": {"11"}})
	if err != nil {
		t.Fatal(err)
	}
	if page := body(res); !strings.Contains(page, escapeForTest("Choose an action first.")) {
		t.Errorf("empty action should say so: %q", page)
	}
	res, err = c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "action": {"delete"}})
	if err != nil {
		t.Fatal(err)
	}
	if page := body(res); !strings.Contains(page, escapeForTest("No rows selected.")) {
		t.Errorf("zero rows should say so: %q", page)
	}
	if n := len(up.seen()); n != 0 {
		t.Fatalf("no upstream call may happen, got %d", n)
	}
}

func TestReportsBulkSaysNoActionAndCountsRowsFirst(t *testing.T) {
	hop, up, _, srv := newBulkSetup(t)
	defer hop.srv.Close()
	c := client(t)
	body(login(t, c, srv))

	res, err := c.PostForm(srv.URL+"/apps/linkhop/reports/actions", url.Values{"pick": {"a1|flag"}})
	if err != nil {
		t.Fatal(err)
	}
	if page := body(res); !strings.Contains(page, escapeForTest("Choose an action first.")) {
		t.Errorf("empty action should say so: %q", page)
	}
	res, err = c.PostForm(srv.URL+"/apps/linkhop/reports/actions", url.Values{"action": {"resolve"}})
	if err != nil {
		t.Fatal(err)
	}
	if page := body(res); !strings.Contains(page, escapeForTest("No rows selected.")) {
		t.Errorf("zero rows should say so: %q", page)
	}
	if n := len(up.seen()); n != 0 {
		t.Fatalf("no upstream call may happen, got %d", n)
	}
}

// Times used to be printed in the server's own zone (CET in the test round) with no
// label at all, and raw ISO text in some tables. Every clock is now UTC with the zone
// named, and carries the instant so the browser can convert it.
func TestTimesSayTheirZoneAndCarryTheInstant(t *testing.T) {
	at := time.Date(2026, 11, 8, 16, 37, 0, 0, time.UTC)
	got := string(dateWithZone("en", at))
	for _, want := range []string{`datetime="2026-11-08T16:37:00Z"`, ">2026-11-08 16:37<", `class="time-zone">UTC<`} {
		if !strings.Contains(got, want) {
			t.Errorf("dateWithZone lacks %q: %s", want, got)
		}
	}
	// A zero time shows a dash, never "0001-01-01".
	if z := string(dateWithZone("en", time.Time{})); strings.Contains(z, "0001") {
		t.Errorf("zero time rendered: %s", z)
	}
	// Text from another app is parsed when it is a time, and shown as given when it is not.
	if v := string(datesz("en", "2026-11-08T16:37:00Z")); !strings.Contains(v, `datetime="2026-11-08T16:37:00Z"`) {
		t.Errorf("an RFC 3339 string was not parsed: %s", v)
	}
	if v := string(datesz("en", "2026-11-08 16:37:00")); !strings.Contains(v, "<time ") {
		t.Errorf("a naive timestamp was not parsed: %s", v)
	}
	if v := string(datesz("en", "not a time")); v != "not a time" {
		t.Errorf("unparseable text must be shown as it came, got %s", v)
	}
	if v := string(datesz("en", "")); strings.Contains(v, "<time ") {
		t.Errorf("an empty value must not render a clock: %s", v)
	}
	// The Indonesian catalogue names the zone in its own words only if it says UTC.
	if v := string(dateWithZone("id", at)); !strings.Contains(v, "UTC") {
		t.Errorf("the zone label is missing in Indonesian: %s", v)
	}
}

// Every screen that shows a clock now names the zone. The audit page is the one the
// test round read the raw CET values on.
func TestAuditPageNamesTheTimeZone(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	code, page := get(t, c, srv.URL+"/audit")
	if code != 200 {
		t.Fatalf("audit: status %d", code)
	}
	if !strings.Contains(page, `class="time-zone"`) && !strings.Contains(page, "UTC") {
		t.Error("the audit page does not name the time zone")
	}
	if !strings.Contains(page, "Times are UTC") {
		t.Error("the audit page does not explain the zone at the foot")
	}
}

// The seven languages other than en and id were empty, so IMPERIUM_LANG=de showed
// English. Every key of the catalogue must exist in every one of the nine languages.
func TestEveryLanguageHasEveryKey(t *testing.T) {
	want := []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"}
	for _, lang := range want {
		if _, ok := catalog[lang]; !ok {
			t.Fatalf("no catalogue for %s", lang)
		}
	}
	// The base catalogue keys are the ones the templates ask for; notifications add theirs.
	for _, lang := range want {
		for key := range catalog["en"] {
			if v, ok := catalog[lang][key]; !ok || v == "" {
				t.Errorf("%s lacks %q", lang, key)
			}
		}
	}
}

func TestNewLanguagesRenderTheirOwnWords(t *testing.T) {
	for lang, want := range map[string]string{
		"de":    "Startseite",
		"ja":    "ホーム",
		"zh-CN": "首页",
		"ru":    "Главная",
	} {
		a := &App{cfg: Config{Lang: lang}}
		if got := a.T("nav.home"); got != want {
			t.Errorf("%s nav.home = %q, want %q", lang, got, want)
		}
		if got := a.lang(); got != lang {
			t.Errorf("%s lang() = %q", lang, got)
		}
	}
	// A key the language does not carry would fall back to English, but the test above
	// already proves every key is there. A page must also declare the language it uses.
	hop := newFakeHop(t)
	_, srv := newAppWithLang(t, hop, "de")
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	code, page := get(t, c, srv.URL+"/users")
	if code != 200 {
		t.Fatalf("/users in German: status %d", code)
	}
	if !strings.Contains(page, `<html lang="de">`) {
		t.Error("a German page must declare lang=\"de\"")
	}
	if strings.Contains(page, "HOP ID users") || strings.Contains(page, "Nothing matches.") {
		t.Error("a German page still shows English table text")
	}
	if !strings.Contains(page, "HOP ID") {
		t.Error("the product name must stay as it is")
	}
}

// A settings column that is 766px wide for "10" and 56px for a paragraph: the table
// now has a class of its own, a label column that takes the rest, a field column that
// is capped, and a narrow column for numbers; on a phone the rows stack.
func TestSettingsTemplateAndStylesheetFixTheColumnWidth(t *testing.T) {
	tpl, err := fs.ReadFile(webfs.FS, "web/templates/settings.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(tpl)
	for _, want := range []string{`class="set-table"`, `class="narrow"`, `only_admin`} {
		if !strings.Contains(s, want) {
			t.Errorf("settings.html lacks %q", want)
		}
	}
	css, err := fs.ReadFile(webfs.FS, "web/static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	c := string(css)
	for _, want := range []string{".set-table", "table-layout:fixed", "--hop-field-max:28rem", ".set-table td.narrow", "@media (max-width:720px)"} {
		if !strings.Contains(c, want) {
			t.Errorf("app.css lacks %q", want)
		}
	}
	// The stacking rule must come after the desktop widths, or it never wins.
	if strings.Index(c, ".set-table td.narrow { width:auto; }") < strings.Index(c, ".set-table td { width:50%; }") {
		t.Error("the phone rule is declared before the desktop widths")
	}
}

func escapeForTest(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(s)
}
