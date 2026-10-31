package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navi-crwn/imperium/internal/db"
)

func TestAccountHubAndLinkServiceHaveIndependentTargets(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	defer hop.px.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	links := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/hop/health":
			fmt.Fprint(w, `{"ok":true,"data":{"app":"linkhop","version":"links-v2","db":"ok"}}`)
		case "/api/hop/manifest":
			fmt.Fprint(w, `{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api"],"content_types":[{"type":"link","label":"Links"}]}}`)
		case "/api/hop/stats":
			fmt.Fprint(w, `{"ok":true,"data":{"kpi":{"links":19}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer links.Close()
	ctx := context.Background()
	err := a.db.UpsertApp(ctx, &db.App{Slug: "linkhop", Name: "LinkHop", URL: links.URL, Prefix: "/api/hop", Secret: secret, Status: "active", Capabilities: []string{"admin_api"}})
	if err != nil {
		t.Fatal(err)
	}
	hub, ok := a.source(ctx, "hub")
	if !ok || hub.Name != "HOP ID" || hub.URL != hop.srv.URL {
		t.Fatalf("wrong hub: %+v", hub)
	}
	link, ok := a.source(ctx, "linkhop")
	if !ok || link.URL != links.URL {
		t.Fatalf("wrong links: %+v", link)
	}
	states := a.probeAll(ctx, nil, []db.App{*link})
	if len(states) != 2 || states[0].Slug != "hub" || states[1].Slug != "linkhop" || states[0].Version != "abc123" || states[1].Version != "links-v2" {
		t.Fatalf("mixed health/version: %+v", states)
	}
	var accounts, content stats
	if err := a.call(ctx, nil, hub, "GET", "/stats", nil, &accounts); err != nil {
		t.Fatal(err)
	}
	if err := a.call(ctx, nil, link, "GET", "/stats", nil, &content); err != nil {
		t.Fatal(err)
	}
	if accounts.KPI["users"] != 74 || content.KPI["links"] != 19 || content.KPI["users"] != 0 {
		t.Fatalf("mixed statistics: %+v %+v", accounts, content)
	}
	nav := a.nav(httptest.NewRequest("GET", "/apps/linkhop", nil))
	if len(nav.Apps) != 2 || nav.Apps[0].Name != "HOP ID" || nav.Apps[1].Name != "LinkHop" || nav.Apps[1].Tag != "" {
		t.Fatalf("mixed navigation: %+v", nav.Apps)
	}
}

func TestHealthDoesNotTreatBrokenPayloadOrDatabaseAsHealthy(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	defer hop.px.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	for _, body := range []string{
		`{"ok":true,"data":{"db":"error","version":"test"}}`,
		`{"ok":true,"data":"not an object"}`,
		`{"ok":true}`,
	} {
		t.Run(body, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer upstream.Close()
			h := a.probe(context.Background(), nil, db.App{Slug: "linkhop", URL: upstream.URL, Secret: secret, Capabilities: []string{"admin_api"}})
			if h.OK || h.Detail == "" {
				t.Fatalf("unhealthy response reported healthy: %+v", h)
			}
		})
	}
}
