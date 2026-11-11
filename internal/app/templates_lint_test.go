package app

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/navi-crwn/imperium/cmd/imperium/webfs"
)

var (
	labelRe = regexp.MustCompile(`<label\b([^>]*)>([^<]*)</label>`)
	tableRe = regexp.MustCompile(`(<div class="scroll-x">)?<table\b`)
)

// The list pages whose tables are wide; the small key/value cards need no wrapper.
var wideTables = map[string]bool{"web/templates/audit.html": true, "web/templates/list.html": true, "web/templates/search.html": true}

// A label that names no control is invisible to a screen reader, and a table that is not
// wrapped in .scroll-x makes the whole page scroll sideways on a phone (A2-S6-17, A2-S6-21).
func TestTemplatesLabelControlsAndWrapTables(t *testing.T) {
	files, err := fs.Glob(webfs.FS, "web/templates/*.html")
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates: %v", err)
	}
	for _, f := range files {
		raw, _ := fs.ReadFile(webfs.FS, f)
		src := string(raw)
		for _, m := range labelRe.FindAllStringSubmatch(src, -1) {
			if strings.Contains(m[1], "for=") || strings.TrimSpace(strings.ReplaceAll(m[2], "&nbsp;", "")) == "" {
				continue // associated, or a blank spacer that only aligns a button
			}
			t.Errorf("%s: <label> without for=: %s", f, m[0])
		}
		if !wideTables[f] {
			continue
		}
		for _, m := range tableRe.FindAllStringSubmatchIndex(src, -1) {
			if m[2] < 0 {
				t.Errorf("%s: <table> not wrapped in .scroll-x at line %d", f, 1+strings.Count(src[:m[0]], "\n"))
			}
		}
	}
}
