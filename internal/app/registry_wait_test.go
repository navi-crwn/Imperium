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
