package app

import (
	"strings"
	"testing"
)

// A report row without a string "kind" (a buggy or older app) used to abort the template
// halfway: the admin got HTTP 200 and a page cut off after the table header.
func TestReportsPageSurvivesRowWithoutKind(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	hop.pxReports = `{"ok":true,"data":[{"id":"r2","reason":"spam","status":"open"},{"id":"r3","kind":5,"status":"open"}],"page":{"next":"","total":2}}`
	c := client(t)
	body(login(t, c, srv))
	for _, path := range []string{"/apps/pixelhop/reports", "/apps/pixelhop"} {
		res, err := c.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b := body(res)
		if res.StatusCode != 200 || !strings.Contains(b, "</html>") {
			t.Errorf("%s: status %d, page complete=%v (len %d)", path, res.StatusCode, strings.Contains(b, "</html>"), len(b))
		}
		if !strings.Contains(b, "spam") {
			t.Errorf("%s: the report row is missing", path)
		}
	}
}
