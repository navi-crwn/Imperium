package app

import (
	"net/http"
	"strings"
	"testing"
)

// IMP-07: HTML is not cached; static assets keep their own caching.
func TestHTMLIsNoStoreAndStaticStaysCached(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	res, _ := http.Get(srv.URL + "/login")
	body(res)
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("/login Cache-Control = %q, want no-store", cc)
	}
	res, _ = http.Get(srv.URL + "/static/hop/fonts.css")
	body(res)
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "max-age=3600") {
		t.Fatalf("static Cache-Control = %q, want the static caching kept", cc)
	}
}
