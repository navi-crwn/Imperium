package app

import (
	"strings"
	"testing"
)

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
		if got := actionLabel(tc.lang, "delete"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.lang, got, tc.want)
		}
		if got := actionLabel(tc.lang, "future_action"); got != "future action" {
			t.Errorf("unknown action fallback: %q", got)
		}
	}
}

func TestActionLabelsCoverNineLanguages(t *testing.T) {
	if len(langOrder) != 9 {
		t.Fatalf("langOrder has %d languages, the labels hold 9", len(langOrder))
	}
	for key, vals := range actionLabels {
		for i, v := range vals {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s has no %s label", key, langOrder[i])
			}
			for _, r := range v {
				if r >= 0x0600 && r <= 0x06FF {
					t.Errorf("%s %s: Arabic text", key, langOrder[i])
				}
			}
		}
	}
	if got := actionLabel("zh-CN", "delete"); got != "删除" {
		t.Errorf("zh-CN delete = %q", got)
	}
	if got := actionLabel("xx", "delete"); got != "Delete" {
		t.Errorf("an unknown language should fall back to English, got %q", got)
	}
}
