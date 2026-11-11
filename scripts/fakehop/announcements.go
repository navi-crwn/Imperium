package main

import (
	"fmt"
	"time"

	"github.com/navi-crwn/imperium/internal/fakehub"
)

// announcements seeds HOP ID's announcements: one of every state, and with -bulk enough for several pages.
func announcements() *fakehub.Store {
	s := fakehub.New("linkhop", "pixelhop", "notehop", "filehop", "payhop", "remotehop")
	now := time.Now().UTC().Truncate(time.Minute)
	at := func(d time.Duration) *time.Time { t := now.Add(d); return &t }
	h := time.Hour
	if *bulk { // older and first, so the named notices below are the newest and open the first page
		for i := 1; i <= 40; i++ {
			r := fakehub.Row{Title: fmt.Sprintf("Notice number %d", i), Body: "Filler for the paging check.", Level: []string{"info", "warning", "critical"}[i%3], Author: "1", CreatedAt: now.Add(-time.Duration(100+i) * h)}
			if i%4 != 0 {
				r.PublishedAt = at(-time.Duration(90+i) * h)
			}
			s.Seed(r)
		}
	}
	s.Seed(fakehub.Row{Title: "Short links move to /l/ on hel.ink", Body: "From 1 December short links are served under /l/.\nOld links keep working for a year.", Level: "warning", PublishedAt: at(-72 * h), Author: "1", CreatedAt: now.Add(-80 * h)})
	s.Seed(fakehub.Row{Title: "Planned maintenance on Saturday", Body: "PixelHop is read-only for about 20 minutes from 02:00 UTC.", Level: "info", AppSlug: "pixelhop", PublishedAt: at(2 * 24 * h), ExpiresAt: at(3 * 24 * h), Author: "1", CreatedAt: now.Add(-5 * h)})
	s.Seed(fakehub.Row{Title: "Sign-in codes by e-mail were delayed", Body: "Codes sent between 08:00 and 09:30 UTC arrived late. Fixed.", Level: "critical", PublishedAt: at(-30 * 24 * h), ExpiresAt: at(-20 * 24 * h), Author: "1", CreatedAt: now.Add(-31 * 24 * h)})
	s.Seed(fakehub.Row{Title: "NoteHop file sharing: draft text", Body: "Not final.", Level: "info", AppSlug: "notehop", Author: "1", CreatedAt: now.Add(-2 * h)})
	s.Seed(fakehub.Row{Title: "PayHop is open to everyone", Body: "Create bills and share them with a QR code.", Level: "info", AppSlug: "payhop", PublishedAt: at(-6 * h), Author: "1", CreatedAt: now.Add(-7 * h)})
	return s
}
