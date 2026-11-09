package app

import (
	"net/http"
	"strings"
	"testing"
)

// The optional Clean style is on only for hop_ui=clean; no cookie or any other value is the original HOP look.
func TestCleanStyleFollowsTheHopUiCookie(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("5", "boss", "owner")
	c := client(t)
	body(login(t, c, srv))
	for _, tc := range []struct {
		cookie string
		clean  bool
	}{{"", false}, {"hop", false}, {"CLEAN", false}, {"clean", true}} {
		req, _ := http.NewRequest("GET", srv.URL+"/me", nil)
		if tc.cookie != "" {
			req.AddCookie(&http.Cookie{Name: "hop_ui", Value: tc.cookie})
		}
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		page := body(res)
		if got := strings.Contains(page, `data-ui="clean"`); got != tc.clean {
			t.Errorf("hop_ui=%q: data-ui present=%v, want %v", tc.cookie, got, tc.clean)
		}
		if !strings.Contains(page, `data-ui-select`) || !strings.Contains(page, "Interface style") {
			t.Errorf("hop_ui=%q: /me lacks the style selector", tc.cookie)
		}
	}
	req, _ := http.NewRequest("GET", srv.URL+"/login", nil)
	req.AddCookie(&http.Cookie{Name: "hop_ui", Value: "clean"})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body(res), `data-ui="clean"`) {
		t.Error("the sign-in page ignored hop_ui=clean")
	}
}
