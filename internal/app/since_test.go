package app

import (
	"testing"
	"time"
)

// IMP-09: relative times follow the configured catalogue language.
func TestHumanSinceFollowsLanguage(t *testing.T) {
	two := time.Now().Add(-2 * time.Hour)
	for lang, want := range map[string]string{"en": "2 h", "id": "2 jam"} {
		a := &App{cfg: Config{Lang: lang}}
		if got := a.since(two); got != want {
			t.Errorf("lang %s: %q, want %q", lang, got, want)
		}
	}
	a := &App{cfg: Config{Lang: "en"}}
	if got := a.since(time.Now()); got != "just now" {
		t.Errorf("en just now = %q", got)
	}
	if got := a.since(time.Now().Add(-30 * time.Minute)); got != "30 min" {
		t.Errorf("en minutes = %q", got)
	}
	if got := a.since(time.Now().Add(-72 * time.Hour)); got != "3 d" {
		t.Errorf("en days = %q", got)
	}
	if got := a.since(time.Time{}); got != "–" {
		t.Errorf("zero time = %q", got)
	}
	// An unknown language falls back to English like the rest of the UI.
	if got := (&App{cfg: Config{Lang: "xx"}}).since(two); got != "2 h" {
		t.Errorf("fallback = %q", got)
	}
}
