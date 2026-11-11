package app

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// The design: a row's actions are a menu of forms of their own (each in a dialog that asks for a reason), outside the bulk form. A plain action used to be a
// one-click form; an action that cannot be undone opens a dialog whose form sends ack=yes, and the server
// refuses it without that. A click can only run the action written on its button, on that row only.

// menuForm finds, in a page, the form of one menu entry (or dialog) for an action on a target URL and returns
// the fields a browser would send for it.
func menuForm(t *testing.T, page, action, targetPath string) (string, url.Values) {
	t.Helper()
	re := regexp.MustCompile(`(?s)<form method="post" action="` + regexp.QuoteMeta(targetPath) + `">(.*?)</form>`)
	for _, m := range re.FindAllStringSubmatch(page, -1) {
		if !strings.Contains(m[1], `name="action" value="`+action+`"`) {
			continue
		}
		vals := url.Values{}
		for _, in := range regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(m[1], -1) {
			vals.Add(in[1], in[2])
		}
		return targetPath, vals
	}
	t.Fatalf("no %s form for %s", action, targetPath)
	return "", nil
}

// insideBulkForm says whether marker lies between <form ... id="bulk-form"> and its end.
func insideBulkForm(page, marker string) bool {
	s := strings.Index(page, `id="bulk-form"`)
	if s < 0 {
		return false
	}
	e := strings.Index(page[s:], "</form>")
	i := strings.Index(page, marker)
	return e >= 0 && i > s && i < s+e
}

func TestContentRowActionIsItsOwnForm(t *testing.T) {
	_, up, _, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content?type=link"))
	if strings.Contains(page, "formaction") {
		t.Error("a row button must not borrow the bulk form with formaction")
	}
	if insideBulkForm(page, `class="pop menu row-menu"`) {
		t.Error("the row menu sits inside the bulk form")
	}
	post, vals := menuForm(t, page, "disable", "/apps/linkhop/content/link/11/actions")
	for _, k := range []string{"pick", "reason", "confirm", "type"} {
		if _, ok := vals[k]; ok {
			t.Errorf("the row form carries the bulk field %q", k)
		}
	}
	vals.Set("reason", "cleanup")
	res, err := c.PostForm(srv.URL+post, vals)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(up.calls) != 1 || up.calls[0]["action"] != "disable" || !strings.HasSuffix(up.calls[0]["path"].(string), "/content/link/11/actions") {
		t.Fatalf("calls = %+v, want one disable on link 11", up.calls)
	}
}

// delete cannot be undone: it sits in a dialog, and without the dialog's ack the server does nothing.
func TestIrreversibleContentActionNeedsTheDialogAck(t *testing.T) {
	_, up, _, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/apps/linkhop/content?type=link"))
	if !strings.Contains(page, `<dialog class="confirm" id="dlg-0-link-11-delete"`) || !strings.Contains(page, `href="#dlg-0-link-11-delete"`) {
		t.Fatal("delete should be a dialog reached from the menu (and by :target without JavaScript)")
	}
	post, vals := menuForm(t, page, "delete", "/apps/linkhop/content/link/11/actions")
	if vals.Get("ack") != "yes" {
		t.Fatal("the dialog form must send ack=yes")
	}
	// A request without the ack (a crafted form, a stale tab) is refused before it reaches the app.
	vals.Del("ack")
	res, err := c.PostForm(srv.URL+post, vals)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(up.calls) != 0 {
		t.Fatalf("a destructive action without the dialog reached the app: %+v", up.calls)
	}
	vals.Set("ack", "yes")
	vals.Set("reason", "spam")
	res, _ = c.PostForm(srv.URL+post, vals)
	body(res)
	if len(up.calls) != 1 || up.calls[0]["action"] != "delete" {
		t.Fatalf("with the ack the action runs once: %+v", up.calls)
	}
}

func TestContentBulkWithoutTicksStillRefused(t *testing.T) {
	_, up, _, srv := newContentBulkSetup(t, map[string]bool{})
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "action": {"disable"}, "confirm": {"yes"}})
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(up.calls) != 0 {
		t.Errorf("a bulk action with no rows reached the app: %+v", up.calls)
	}
	// Rows ticked but no confirmation: refused too.
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "action": {"disable"}, "pick": {"11"}})
	body(res)
	if len(up.calls) != 0 {
		t.Errorf("a bulk action without the confirmation reached the app: %+v", up.calls)
	}
	// Ticked, confirmed, with a reason: runs once per row.
	res, _ = c.PostForm(srv.URL+"/apps/linkhop/content/actions", url.Values{"type": {"link"}, "action": {"disable"}, "pick": {"11", "12"}, "confirm": {"yes"}, "reason": {"sweep"}})
	body(res)
	if len(up.calls) != 2 {
		t.Errorf("two ticked rows should make two calls: %+v", up.calls)
	}
}

func TestUsersRowActionIsItsOwnForm(t *testing.T) {
	hop, _, _, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/users"))
	if strings.Contains(page, "formaction") {
		t.Error("a row button must not borrow the bulk form with formaction")
	}
	if insideBulkForm(page, `class="pop menu row-menu"`) {
		t.Error("the row menu sits inside the bulk form")
	}
	post, vals := menuForm(t, page, "suspend", "/users/26/actions")
	for _, k := range []string{"pick", "reason", "confirm"} {
		if _, ok := vals[k]; ok {
			t.Errorf("the row form carries the bulk field %q", k)
		}
	}
	vals.Set("reason", "abuse")
	res, err := c.PostForm(srv.URL+post, vals)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "suspend" || hop.actions[0]["reason"] != "abuse" {
		t.Fatalf("forwarded = %+v, want only suspend on 26 with the reason typed in its own dialog", hop.actions)
	}
}

func TestUsersBulkWithoutTicksStillRefused(t *testing.T) {
	_, acted, _, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/users/actions", url.Values{"action": {"suspend"}, "confirm": {"yes"}})
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(*acted) != 0 {
		t.Errorf("a bulk action with no rows reached HOP ID: %v", *acted)
	}
}

// Deleting an account is a dialog that asks for the username; the server refuses it without the ack.
func TestUserDeleteNeedsTheDialogAck(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/users/26"))
	if !strings.Contains(page, `<dialog class="confirm" id="dlg-0-u-26-delete"`) || !strings.Contains(page, `name="confirm" required`) {
		t.Fatal("delete account should be a dialog that asks for the username")
	}
	res, _ := c.PostForm(srv.URL+"/users/26/actions", url.Values{"action": {"delete"}, "confirm": {"@rani"}})
	body(res)
	if len(hop.actions) != 0 {
		t.Fatalf("delete without the dialog's ack reached HOP ID: %+v", hop.actions)
	}
}
