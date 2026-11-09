package app

import (
	"strings"
	"testing"
)

// The sign-in page says Imperium belongs to Hop Easy Link.
func TestLoginFooterPartOfFamily(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newApp(t, hop)
	defer srv.Close()
	res, err := client(t).Get(srv.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	if page := body(res); !strings.Contains(page, `class="part-of" href="https://hel.ink"`) || !strings.Contains(page, "Part of <b>Hop Easy Link</b>") {
		t.Fatal("footer row missing on the sign-in page")
	}
}
