package app

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/navi-crwn/imperium/cmd/imperium/webfs"
)

func prefsLogin(t *testing.T, lang string) (*http.Client, string, func()) {
	t.Helper()
	hop := newFakeHop(t)
	_, srv := newAppWithLang(t, hop, lang)
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	return c, srv.URL, func() { srv.Close(); hop.srv.Close() }
}

func mustGet(t *testing.T, c *http.Client, u string) string {
	t.Helper()
	res, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	return body(res)
}

func postPrefs(t *testing.T, c *http.Client, base string, v url.Values) (string, *http.Response) {
	t.Helper()
	res, err := c.PostForm(base+"/prefs", v)
	if err != nil {
		t.Fatal(err)
	}
	return body(res), res
}

// The language is per reader (cookie hop_lang); IMPERIUM_LANG is only the default.
func TestLanguageIsPerReaderAndConfigIsTheDefault(t *testing.T) {
	c, base, done := prefsLogin(t, "en")
	defer done()
	if b := mustGet(t, c, base+"/users"); !strings.Contains(b, "HOP ID users") || !strings.Contains(b, `<html lang="en"`) {
		t.Fatal("without a cookie the instance default (en) must apply")
	}
	b, res := postPrefs(t, c, base, url.Values{"lang": {"id"}, "next": {"/users"}})
	if res.Request.URL.Path != "/users" || !strings.Contains(b, "Pengguna HOP ID") || !strings.Contains(b, `<html lang="id"`) {
		t.Fatalf("after choosing id the page must be Indonesian, got %s", res.Request.URL)
	}
	if other := mustGet(t, client(t), base+"/login"); strings.Contains(other, `<html lang="id"`) {
		t.Fatal("another browser must not inherit the choice")
	}
	// Junk is ignored, and the next target stays on this site.
	_, res = postPrefs(t, c, base, url.Values{"lang": {"ar"}, "theme": {"neon"}, "next": {"https://evil.example/"}})
	if res.Request.URL.Host != strings.TrimPrefix(base, "http://") {
		t.Fatalf("redirected off-site to %s", res.Request.URL)
	}
	if b := mustGet(t, c, base+"/users"); !strings.Contains(b, `<html lang="id"`) || strings.Contains(b, "data-theme=") {
		t.Fatal("an unknown language or theme must change nothing")
	}
}

func TestThemeCookieSetsDataThemeAndPickerListsNineLanguages(t *testing.T) {
	c, base, done := prefsLogin(t, "en")
	defer done()
	page := mustGet(t, c, base+"/users")
	if strings.Contains(page, "data-theme=") {
		t.Fatal("no theme chosen: the page must follow the system")
	}
	for _, code := range []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
		if !strings.Contains(page, `<option value="`+code+`"`) {
			t.Errorf("language picker lacks %s", code)
		}
	}
	if strings.Contains(page, `value="ar"`) {
		t.Error("Arabic must not be offered")
	}
	postPrefs(t, c, base, url.Values{"theme": {"dark"}, "next": {"/users"}})
	if page = mustGet(t, c, base+"/users"); !strings.Contains(page, `data-theme="dark"`) {
		t.Fatal("theme=dark must render data-theme")
	}
	postPrefs(t, c, base, url.Values{"theme": {"system"}, "next": {"/users"}})
	if page = mustGet(t, c, base+"/users"); strings.Contains(page, "data-theme=") {
		t.Fatal("theme=system must clear the choice")
	}
}

func TestPrefKeysExistInEveryLanguage(t *testing.T) {
	for _, l := range langOrder {
		for _, k := range []string{"pref.language", "pref.theme", "pref.system", "pref.light", "pref.dark", "pref.apply", "nav.account", "nav.profile"} {
			if _, ok := catalog[l][k]; !ok {
				t.Errorf("%s lacks %s", l, k)
			}
		}
	}
}

// U-1: the family lists kept in code must equal the hop-assets manifest that is
// embedded next to them (apps.json), so a seventh app cannot be half added.
func TestFamilyListsMatchManifest(t *testing.T) {
	raw, err := fs.ReadFile(webfs.FS, "web/static/hop/apps.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Apps []struct {
			Slug string `json:"slug"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	syncSh, err := os.ReadFile("../../scripts/sync-hop-assets.sh")
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, ap := range m.Apps {
		slugs = append(slugs, ap.Slug)
		if ap.Slug == "imperium" {
			continue
		}
		mark := appMark(ap.Slug)
		if mark == "/static/imperium.svg" {
			t.Errorf("appMark has no mark for manifest app %s", ap.Slug)
			continue
		}
		if _, err := fs.Stat(webfs.FS, "web"+mark); err != nil {
			t.Errorf("%s: embedded mark %s is missing: %v", ap.Slug, mark, err)
		}
		if !strings.Contains(string(syncSh), strings.TrimPrefix(mark, "/static/hop/")) {
			t.Errorf("sync-hop-assets.sh does not copy %s", mark)
		}
		if _, ok := kpiOrder[ap.Slug]; !ok {
			t.Errorf("kpiOrder has no entry for %s", ap.Slug)
		}
	}
	for slug := range kpiOrder {
		if !contains(slugs, slug) {
			t.Errorf("kpiOrder lists %s, which is not in the manifest", slug)
		}
	}
}

func TestPrefsKeepTheQueryAndRefuseCrossOrigin(t *testing.T) {
	c, base, done := prefsLogin(t, "en")
	defer done()
	page := mustGet(t, c, base+"/users?q=ann&page=2")
	if !strings.Contains(page, `name="next" value="/users?q=ann&amp;page=2"`) {
		t.Fatal("the preference forms must carry the query string")
	}
	_, res := postPrefs(t, c, base, url.Values{"lang": {"de"}, "next": {"/users?q=ann&page=2"}})
	if got := res.Request.URL.RequestURI(); got != "/users?q=ann&page=2" {
		t.Fatalf("came back to %s", got)
	}
	for _, ck := range res.Request.Response.Cookies() {
		if ck.Name == langCookie && (!ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode) {
			t.Errorf("hop_lang must be HttpOnly and SameSite=Lax: %+v", ck)
		}
	}
	req, _ := http.NewRequest("POST", base+"/prefs", strings.NewReader("lang=fr"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	r2, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if r2.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin POST /prefs = %d, want 403", r2.StatusCode)
	}
}
