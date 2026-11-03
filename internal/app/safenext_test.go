package app

import "testing"

func TestSafeNextRejectsControlCharacters(t *testing.T) {
	for next, want := range map[string]string{
		"/apps":          "/apps",
		"/search?q=a#x":  "/search?q=a#x",
		"//evil.test":    "/",
		"/\\evil.test":   "/",
		"/\t/evil.test":  "/", // browsers drop the tab: it would become //evil.test
		"/\n/evil.test":  "/",
		"/\r/evil.test":  "/",
		"/a\x00b":        "/",
		"https://x.test": "/",
		"":               "/",
	} {
		if got := safeNext(next); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", next, got, want)
		}
	}
}
