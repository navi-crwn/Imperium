package app

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestRoleMatrix drives testdata/role-matrix.tsv: every role against every kind of page and action.
func TestRoleMatrix(t *testing.T) {
	raw, err := os.ReadFile("testdata/role-matrix.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	type rowT struct {
		method, path, action string
		want                 []string
	}
	var rows []rowT
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cells := strings.Split(line, "\t")
		if cells[0] == "endpoint" {
			roles = cells[1:]
			continue
		}
		method, rest, _ := strings.Cut(cells[0], " ")
		path, action, _ := strings.Cut(rest, " ")
		rows = append(rows, rowT{method, path, action, cells[1:]})
	}
	if len(roles) != 6 || len(rows) < 10 {
		t.Fatalf("matrix not read: %d roles, %d rows", len(roles), len(rows))
	}
	for i, role := range roles {
		hop := newFakeHop(t)
		a, srv := newApp(t, hop)
		defer srv.Close()
		hop.ticket = scopedTicket("27", "mx"+strconv.Itoa(i), role)
		c := client(t)
		body(login(t, c, srv))
		waitRegistrySynced(t, a, hop) // the sign-in's background sync has fully landed before the counters reset
		for _, row := range rows {
			want, _ := strconv.Atoi(row.want[i])
			if row.method == "GET" {
				res, err := c.Get(srv.URL + row.path)
				if err != nil {
					t.Fatal(err)
				}
				body(res)
				if res.StatusCode != want {
					t.Errorf("%-14s GET %s: status %d, want %d", role, row.path, res.StatusCode, want)
				}
				continue
			}
			// How many operations of this kind reached the other side: user/report/content actions,
			// a signed registry read for /apps/sync, or a signed settings PUT for /apps/pixelhop/settings.
			// The counters are reset and read through the fake's mutex: its handlers run on
			// server goroutines while this test goroutine snapshots them.
			hop.actions = nil
			hop.resetUpstreamCounters()
			form := url.Values{"action": {row.action}, "reason": {"matrix"}, "text": {"matrix"}}
			res, err := c.PostForm(srv.URL+row.path, form)
			if err != nil {
				t.Fatal(err)
			}
			body(res)
			got := len(hop.actions)
			if row.action == "suspend" && got == 2 && hop.actions[1]["action"] == "revoke_tokens" {
				got = 1 // suspending also takes the API tokens, as HOP ID's own page does: one allowed action, two calls
			}
			switch row.path {
			case "/apps/sync":
				got = hop.registryReads()
			case "/apps/pixelhop/settings":
				got = hop.settingsPuts()
			}
			if got != want {
				t.Errorf("%-14s POST %s [%s]: %d upstream operations, want %d", role, row.path, row.action, got, want)
			}
		}
	}
}

// admin:<app> sees its own app in the lists and nothing else; the global roles see every app.
func TestScopedAdminSeesOnlyItsOwnApp(t *testing.T) {
	hop := newFakeHop(t)
	_, srv := newApp(t, hop)
	defer srv.Close()
	hop.ticket = scopedTicket("27", "dewi", "admin:pixelhop")
	c := client(t)
	body(login(t, c, srv))
	for _, path := range []string{"/apps", "/"} {
		_, b := get(t, c, srv.URL+path)
		if !strings.Contains(b, "PixelHop") {
			t.Errorf("%s: admin:pixelhop does not see its own app", path)
		}
		for _, other := range []string{"LinkHop", `<span class="chip">HOP ID</span>`, `href="/apps/linkhop"`, `href="/apps/hub"`} {
			if strings.Contains(b, other) {
				t.Errorf("%s: admin:pixelhop sees %q of another app or of the hub", path, other)
			}
		}
	}
	hop.ticket = scopedTicket("5", "boss", "owner")
	c2 := client(t)
	body(login(t, c2, srv))
	_, b := get(t, c2, srv.URL+"/apps")
	if !strings.Contains(b, "PixelHop") || !strings.Contains(b, "LinkHop") {
		t.Error("owner lost part of the app list")
	}
}
