package app

import (
	"fmt"
	"strings"
	"testing"
)

var nineLanguages = []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"}

// Every notification text exists in all nine languages, and Arabic is not one of them.
func TestNotificationTextsInNineLanguages(t *testing.T) {
	if len(notifTexts) != len(nineLanguages) {
		t.Fatalf("%d notification catalogs, want %d", len(notifTexts), len(nineLanguages))
	}
	for _, lang := range nineLanguages {
		texts, ok := notifTexts[lang]
		if !ok {
			t.Fatalf("no notification texts for %s", lang)
		}
		for key, en := range notifTexts["en"] {
			v := texts[key]
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s lacks %s", lang, key)
			}
			// Short words may match English ("Route", "Version"); a long identical text is an untranslated sentence.
			if lang != "en" && v == en && len(en) > 24 {
				t.Errorf("%s/%s looks untranslated: %q", lang, key, v)
			}
		}
		for key := range texts {
			if _, ok := notifTexts["en"][key]; !ok {
				t.Errorf("%s has %s which English lacks", lang, key)
			}
		}
	}
	for _, banned := range []string{"ar", "ar-SA", "ar-EG"} {
		if _, ok := catalog[banned]; ok {
			t.Errorf("Arabic catalog %q must never exist", banned)
		}
	}
}

func TestNormLang(t *testing.T) {
	for in, want := range map[string]string{"zh": "zh-CN", "zh-cn": "zh-CN", "ZH_cn": "zh-CN", "pt": "pt-BR", "pt-br": "pt-BR", "DE": "de", "ru": "ru", "en": "en", "id": "id", "xx": "xx", "": ""} {
		if got := normLang(in); got != want {
			t.Errorf("normLang(%q) = %q, want %q", in, got, want)
		}
	}
}

// Each language renders both pages with no raw key and a matching <html lang>.
func TestNotificationPagesRenderInEveryLanguage(t *testing.T) {
	a, srv, hop := newInbox(t)
	id := seed(t, a, "linkhop", "GET /l/{code}", nil)
	c := signedIn(t, hop, srv, "boss", "owner")
	for _, lang := range nineLanguages {
		a.cfg.Lang = lang
		for _, path := range []string{"/notifications", fmt.Sprintf("/notifications/%d", id), "/"} {
			code, b := get(t, c, srv.URL+path)
			if code != 200 {
				t.Fatalf("%s %s: %d", lang, path, code)
			}
			if !strings.Contains(b, `<html lang="`+lang+`">`) {
				t.Errorf("%s %s: wrong html lang", lang, path)
			}
			if strings.Contains(b, "notif.") || strings.Contains(b, "nav.notifications") {
				t.Errorf("%s %s: a text key is shown raw", lang, path)
			}
		}
		_, b := get(t, c, srv.URL+"/notifications")
		if want := notifTexts[lang]["notif.intro"]; !strings.Contains(b, strings.NewReplacer("'", "&#39;").Replace(want)) {
			t.Errorf("%s: list page lacks its intro text", lang)
		}
	}
	// A flash message in another language, from an action.
	a.cfg.Lang = "ja"
	_, b := act(t, c, srv, id, "ack", "")
	if !strings.Contains(b, "完了しました。") {
		t.Error("ja flash")
	}
	// A language with no catalog falls back to English.
	a.cfg.Lang = "xx"
	if _, b := get(t, c, srv.URL+"/notifications"); !strings.Contains(b, "Server errors (HTTP 500) and daily guest link digests reported") {
		t.Error("fallback to English")
	}
}
