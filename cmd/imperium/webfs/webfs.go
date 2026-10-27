// Package webfs embeds the templates and static files so both the binary and
// the tests use the same copy.
package webfs

import "embed"

//go:embed all:web
var FS embed.FS
