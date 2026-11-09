package app

import (
	"context"
	"testing"
	"time"
)

// At sign-in the registry is copied from HOP ID in the background, one app at a time (see ssoCallback), so a
// page asked for right after login can find none of the apps, or only some. Tests that read the registry
// wait here until it holds what the fake hub lists.
func waitForRegistry(t *testing.T, a *App, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		apps, _ := a.db.Apps(context.Background())
		if len(apps) >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the registry holds %d app(s) after 5s, want %d", len(apps), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitRegistrySynced waits for the sign-in's background registry sync to have
// COMPLETED, not merely started: syncApps upserts LinkHop first and PixelHop
// last, so PixelHop's row carrying the fake hub's URL is the sync's final
// persisted write. A row count (waitForRegistry) can see the intermediate
// state; tests that snapshot upstream counters or render app pages wait here.
func waitRegistrySynced(t *testing.T, a *App, hop *fakeHop) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		apps, _ := a.db.Apps(context.Background())
		for _, ap := range apps {
			if ap.Slug == "pixelhop" && ap.URL == hop.px.URL {
				return
			}
		}
		if time.Now().After(deadline) {
			apps, _ := a.db.Apps(context.Background())
			t.Fatalf("the registry sync did not persist PixelHop within 5s: %+v", apps)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
