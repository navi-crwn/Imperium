package app

import (
	"net/http"
	"strings"
	"testing"
)

// Imperium's CSP allows only 'self' for styles and scripts, so the HOP family
// fonts and floating tiles are served from its own copy (scripts/sync-hop-assets.sh).
func TestHopFamilyAssetsServedLocally(t *testing.T) {
	_, srv := newApp(t, newFakeHop(t))
	res, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	page := body(res)
	for _, want := range []string{`href="/static/hop/fonts.css"`, `href="/static/hop/hop-float.css"`, `src="/static/hop/hop-float.js"`, `data-hop-float`} {
		if !strings.Contains(page, want) {
			t.Errorf("login page lacks %q", want)
		}
	}
	if strings.Contains(page, "hel.ink/hop-assets") {
		t.Error("login page must not load hel.ink assets: the CSP is 'self' only")
	}
	for _, path := range []string{"/static/hop/fonts.css", "/static/hop/hop-float.js", "/static/hop/hop-float.css", "/static/hop/apps/linkhop/mark-white.svg", "/static/hop/fonts/syne-latin-wght-normal.woff2"} {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Errorf("%s: status %d", path, r.StatusCode)
		}
	}
}
