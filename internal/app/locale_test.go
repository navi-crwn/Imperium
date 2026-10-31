package app

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/cmd/imperium/webfs"
	"github.com/navi-crwn/imperium/internal/db"
)

// Locale configuration for Imperium: the default language is English, an
// operator may explicitly set IMPERIUM_LANG=id, and the rendered <html lang>
// must always match the catalog actually in use.

func TestDefaultLanguageIsEnglish(t *testing.T) {
	c := LoadConfig("test")
	if c.Lang != "en" {
		t.Fatalf("default IMPERIUM_LANG should resolve to en, got %q", c.Lang)
	}
}

func TestExplicitIndonesianConfigStillWorks(t *testing.T) {
	t.Setenv("IMPERIUM_LANG", "id")
	c := LoadConfig("test")
	if c.Lang != "id" {
		t.Fatalf("an explicit IMPERIUM_LANG=id must be preserved, got %q", c.Lang)
	}
	// The Indonesian catalog is still reachable for a page render.
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newAppWithLang(t, hop, "id")
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c2 := client(t)
	body(login(t, c2, srv))
	res, _ := c2.Get(srv.URL + "/users")
	b := body(res)
	if !strings.Contains(b, "Pengguna HOP ID") {
		t.Fatal("with IMPERIUM_LANG=id the users page should render the Indonesian catalog")
	}
	if !strings.Contains(b, `<html lang="id">`) {
		t.Fatal("an Indonesian page must declare lang=\"id\"")
	}
}

func TestUnknownLanguageFallsBackToEnglish(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newAppWithLang(t, hop, "fr")
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	res, _ := c.Get(srv.URL + "/users")
	b := body(res)
	if !strings.Contains(b, "HOP ID users") {
		t.Fatal("an unknown language should fall back to the English catalog")
	}
	if a.T("nav.home") != "Home" {
		t.Fatalf("unknown language should fall back to English strings, got %q", a.T("nav.home"))
	}
}

func TestRenderedHtmlLangMatchesEffectiveLanguage(t *testing.T) {
	// Default (English) configuration.
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newAppWithLang(t, hop, "")
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	// Signed-in page (the "head" template).
	res, _ := c.Get(srv.URL + "/users")
	b := body(res)
	if !strings.Contains(b, `<html lang="en">`) {
		t.Fatal("a default-configuration page must declare lang=\"en\"")
	}
	if strings.Contains(b, `<html lang="id">`) {
		t.Fatal("no page should hard-code lang=\"id\" when the effective language is English")
	}
	// The login page is reachable without a session.
	res, _ = c.Get(srv.URL + "/login")
	if b := body(res); !strings.Contains(b, `<html lang="en">`) {
		t.Fatal("the login page must declare lang=\"en\" under the default configuration")
	}
}

func newAppWithLang(t *testing.T, hop *fakeHop, lang string) (*App, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Addr: "127.0.0.1:0", HopURL: hop.srv.URL, HopSecret: secret, HopClient: "imperium", SessionTTL: time.Hour, RequireMFA: true, Version: "test"}
	if lang != "" {
		cfg.Lang = lang
	}
	a, err := New(cfg, store, webfs.FS)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	a.cfg.BaseURL = srv.URL
	return a, srv
}

func TestLoginDeniedPageLanguageFollowsConfig(t *testing.T) {
	// English default: the denial page is English.
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newAppWithLang(t, hop, "")
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "26", "username": "rani", "name": "Rani", "email": "r@x", "roles": []string{}, "mfa": true, "status": "active"}
	c := client(t)
	res := login(t, c, srv)
	if b := body(res); !strings.Contains(b, "no admin role") || !strings.Contains(b, `<html lang="en">`) {
		t.Fatal("with the default configuration the denial page should be English and declare lang=\"en\"")
	}
}
