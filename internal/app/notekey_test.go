package app

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// An 500 report must not carry an encrypted NoteHop key (#k= / #a= of a note link) into Imperium (fake key only).
var keyParam = regexp.MustCompile(`(?i)(#|%23|&|;)\s*(k|a)=[A-Za-z0-9_%-]{8,}`)

const fakeNoteKey = "FAKEKEY0notehop0s2audit0000000000000000000"

func TestNotifyDropsNoteKeysFromMessageAndRefusesThemInFixedFields(t *testing.T) {
	a, srv, _ := newInbox(t)
	for name, msg := range map[string]string{
		"https link":     "failed fetching https://note.hel.ink/AbC123xyz0#k=" + fakeNoteKey + " for the user",
		"owner link":     "bad https://n.hel.ink/AbC123xyz0#k=" + fakeNoteKey + "&a=ADMINTOKEN0FAKE000000000000000000000000000",
		"scheme-less":    "see note.hel.ink/AbC123xyz0#k=" + fakeNoteKey,
		"escaped hash":   "see https://note.hel.ink/AbC123xyz0%23k=" + fakeNoteKey,
		"bare fragment":  "fragment was #k=" + fakeNoteKey,
		"query":          "url https://note.hel.ink/AbC123xyz0?x=1#k=" + fakeNoteKey,
		"bare letters":   "fragment was #k=" + strings.Repeat("abcdefghij", 4),
		"pure letters":   "key https://note.hel.ink/AbC123xyz0#k=" + strings.Repeat("abcdefghij", 4),
		"full-width key": "https://ｎote.hel.ink/AbC123xyz0#k=" + strings.Repeat("abcdefghij", 4),
	} {
		rep := report("linkhop", func(m map[string]any, d map[string]any) { d["message"] = msg; d["ref"] = "7f3a9c1e2b4d" })
		mustAccept(t, srv, "linkhop", lhOwnSecret, rep)
		for _, g := range groups(t, a) {
			raw := fmt.Sprintf("%+v", g)
			if keyParam.MatchString(raw) || strings.Contains(raw, fakeNoteKey) || strings.Contains(raw, "abcdefghijabcdefghij") {
				t.Errorf("%s: the key reached the stored report: %s", name, raw)
			}
		}
	}
	// A route or an origin can not hold a fragment at all: those fields are refused.
	for _, field := range []string{"route", "origin"} {
		rep := report("linkhop", func(m map[string]any, d map[string]any) { d[field] = "GET /AbC123xyz0#k=" + fakeNoteKey })
		if res, _ := send(t, srv, "linkhop", lhOwnSecret, rep); res.StatusCode != 422 {
			t.Errorf("%s with a fragment: want 422, got %d", field, res.StatusCode)
		}
	}
}
