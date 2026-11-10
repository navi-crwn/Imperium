package app

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// A row's menu must run only the action written on its button, on that row only. The buttons use
// form="rowf-<id>", a form of their own outside the bulk form, so the bulk form's action select,
// reason and ticks never travel with it. rowSubmit rebuilds what a browser sends for one click.
func rowSubmit(t *testing.T, page, id, action string) (post string, vals url.Values) {
	t.Helper()
	if !strings.Contains(page, `form="rowf-`+id+`"`) {
		t.Fatalf("no row button for %s", id)
	}
	m := regexp.MustCompile(`<form id="rowf-` + regexp.QuoteMeta(id) + `" method="post" action="([^"]+)">(.*?)</form>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no row form for %s", id)
	}
	vals = url.Values{}
	for _, in := range regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`).FindAllStringSubmatch(m[2], -1) {
		vals.Add(in[1], in[2])
	}
	if !regexp.MustCompile(`<button[^>]*form="rowf-` + regexp.QuoteMeta(id) + `"[^>]*name="action" value="` + action + `"`).MatchString(page) {
		t.Fatalf("no %s button for row %s", action, id)
	}
	vals.Set("action", action)
	return strings.ReplaceAll(m[1], "&amp;", "&"), vals
}

// insideBulkForm says whether the first occurrence of marker lies between <form id="bulk-form"> and its end.
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
	if insideBulkForm(page, `id="rowf-11"`) {
		t.Error("the row form sits inside the bulk form")
	}
	post, vals := rowSubmit(t, page, "11", "disable")
	// The bulk select, the reason and a tick may hold anything: none of it is in the row form.
	for _, k := range []string{"pick", "reason", "confirm", "type"} {
		if _, ok := vals[k]; ok {
			t.Errorf("row form carries the bulk field %q", k)
		}
	}
	res, err := c.PostForm(srv.URL+post, vals)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(up.calls) != 1 || up.calls[0]["action"] != "disable" || !strings.HasSuffix(up.calls[0]["path"].(string), "/content/link/11/actions") {
		t.Fatalf("calls = %+v, want one disable on link 11", up.calls)
	}
	if res.Request.URL.Path != "/apps/linkhop/content" {
		t.Errorf("the row action should return to the list, ended at %s", res.Request.URL)
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
}

func TestUsersRowActionIsItsOwnForm(t *testing.T) {
	hop, _, _, srv := usersBulkHop(t)
	c := client(t)
	body(login(t, c, srv))
	page := body(bulkGet(t, c, srv.URL+"/users"))
	if strings.Contains(page, "formaction") {
		t.Error("a row button must not borrow the bulk form with formaction")
	}
	if insideBulkForm(page, `id="rowf-26"`) {
		t.Error("the row form sits inside the bulk form")
	}
	post, vals := rowSubmit(t, page, "26", "suspend")
	for _, k := range []string{"pick", "reason", "confirm"} {
		if _, ok := vals[k]; ok {
			t.Errorf("row form carries the bulk field %q", k)
		}
	}
	res, err := c.PostForm(srv.URL+post, vals)
	if err != nil {
		t.Fatal(err)
	}
	body(res)
	if len(hop.actions) != 1 || hop.actions[0]["action"] != "suspend" || hop.actions[0]["reason"] != "" {
		t.Fatalf("forwarded = %+v, want only suspend on 26 with no reason from the bulk form", hop.actions)
	}
	if res.Request.URL.Path != "/users" {
		t.Errorf("the row action should return to the list, ended at %s", res.Request.URL)
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
