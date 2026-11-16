package app

import (
	"fmt"
	"strings"
)

// reportSubject is the subject cell of a report row. For a bio page it names the page the moderator is about
// to act on (its slug and address, or that it no longer exists); every other subject reads "type id".
func reportSubject(l string, sub map[string]any) string {
	typ := str(sub["type"])
	if typ == "bio_page" {
		slug, addr := str(sub["slug"]), str(sub["url"])
		switch {
		case sub["id"] == nil: // BioHop sends a null id once the page is gone
			name := slug
			if name == "" {
				name = typ
			}
			return name + " · " + tl(l, "reports.page_deleted")
		case slug != "" && addr != "":
			return slug + " · " + addr
		case slug != "" || addr != "":
			return slug + addr
		}
	}
	return strings.TrimSpace(typ + " " + fmt.Sprint(sub["id"]))
}
