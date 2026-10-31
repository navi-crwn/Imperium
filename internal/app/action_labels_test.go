package app

import "testing"

func TestDestructiveProtocolActionsRequireConfirmation(t *testing.T) {
	for _, action := range []string{"delete", "purge", "takedown", "ban_ip", "suspend", "disable", "hide", "unpublish", "transfer", "clear_custom_css", "freeze", "set_expiry", "release_slug"} {
		if !confirmAction(action) {
			t.Errorf("%s must require confirmation", action)
		}
	}
	for _, action := range []string{"enable", "unhide", "publish", "restore"} {
		if confirmAction(action) {
			t.Errorf("%s is a restorative action", action)
		}
	}
}

func TestProtocolActionLabelsFollowLanguage(t *testing.T) {
	for _, tc := range []struct{ lang, want string }{{"en", "Delete"}, {"id", "Hapus"}} {
		a := &App{cfg: Config{Lang: tc.lang}}
		if got := a.actionLabel("delete"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.lang, got, tc.want)
		}
		if got := a.actionLabel("future_action"); got != "future action" {
			t.Errorf("unknown action fallback: %q", got)
		}
	}
}
