package app

import (
	"html/template"
	"strings"
)

// partOf fills the name into the translated "Part of {name}" sentence; the name
// stays "Hop Easy Link" in every language.
func partOf(s string) template.HTML {
	return template.HTML(strings.Replace(template.HTMLEscapeString(s), "{name}", "<b>Hop Easy Link</b>", 1))
}
