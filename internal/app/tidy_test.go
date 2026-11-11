package app

import (
	"fmt"
	"strings"
	"testing"
)

// Every label of the tidied dashboard exists in all nine languages (the catalogue test checks the
// keys; this one checks that the table was written without a missing or extra value).
func TestTidyLabelsHaveOneValuePerKeyInEveryLanguage(t *testing.T) {
	for _, lang := range []string{"en", "zh-CN", "de", "id", "ja", "pt-BR", "es", "fr", "ru"} {
		if len(tidyVals[lang]) != len(tidyKeys) {
			t.Errorf("%s has %d tidy values for %d keys", lang, len(tidyVals[lang]), len(tidyKeys))
		}
	}
}

// A key an app reports is never shown with its underscores.
func TestKeyLabelSpellsKeysOut(t *testing.T) {
	if got := keyLabel("en", "api_tokens"); got != "API tokens" {
		t.Errorf("api_tokens = %q", got)
	}
	if got := keyLabel("de", "api_tokens"); got != "API-Token" {
		t.Errorf("api_tokens in German = %q", got)
	}
	if got := keyLabel("en", "some_new_counter"); got != "Some new counter" {
		t.Errorf("an unlisted key should be spelled out, got %q", got)
	}
	if got := keyLabel("en", ""); got != "" {
		t.Errorf("empty key = %q", got)
	}
	if got := extLabel("en", "analytics"); got != "Analytics" {
		t.Errorf("ext analytics = %q", got)
	}
}

// Rows of cards spread evenly, so the last row is never a lone card.
func TestStatColsLeavesNoLoneCard(t *testing.T) {
	cases := map[[2]int]string{{6, 6}: "c6", {7, 6}: "c4", {8, 6}: "c4", {12, 6}: "c6", {4, 6}: "c4", {1, 6}: "c1", {6, 4}: "c3", {5, 6}: "c5", {13, 6}: "c5"}
	for in, want := range cases {
		if got := statCols(in[0], in[1]); got != want {
			t.Errorf("statCols(%d, %d) = %s, want %s", in[0], in[1], got, want)
		}
	}
	for n := 2; n <= 24; n++ {
		var cols int
		fmt.Sscanf(statCols(n, 6), "c%d", &cols)
		if n%cols == 1 {
			t.Errorf("%d cards in %d columns leave a lone card", n, cols)
		}
	}
}

// Every extension page goes under "More"; the other tabs stay.
func TestExtensionTabsGroupUnderMore(t *testing.T) {
	items := []navItem{{Label: "Dashboard", Kind: "dash"}, {Label: "Content", Kind: "content"}, {Label: "IP", Kind: "ext"}, {Label: "Queue", Kind: "ext"}}
	if m, more := splitTabs(items); len(more) != 2 || len(m) != 2 {
		t.Errorf("extensions go under More: main=%d more=%d", len(m), len(more))
	}
}

func TestPageTablesFiltersAndCuts(t *testing.T) {
	var rows [][]string
	for i := 0; i < 60; i++ {
		rows = append(rows, []string{fmt.Sprintf("slug-%02d", i)})
	}
	got := pageTables([]table{{Name: "reserved", Cols: []string{"reserved"}, Rows: rows}}, "", 25)
	if len(got[0].Rows) != 25 || got[0].Total != 60 {
		t.Errorf("rows=%d total=%d, want 25 of 60", len(got[0].Rows), got[0].Total)
	}
	got = pageTables([]table{{Name: "reserved", Cols: []string{"reserved"}, Rows: rows}}, "SLUG-5", 25)
	if len(got[0].Rows) != 10 || got[0].Total != 0 {
		t.Errorf("filter: rows=%d total=%d, want 10 and no cut", len(got[0].Rows), got[0].Total)
	}
	for _, r := range got[0].Rows {
		if !strings.Contains(r[0], "5") {
			t.Errorf("row %v does not match", r)
		}
	}
}
