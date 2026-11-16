package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"time"

	"github.com/navi-crwn/imperium/internal/fakehub"
)

// mockShot is a picture that stands for a screenshot: a page with a header, a menu, cards and a box that looks like an error.
func mockShot(w, h int, accent color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{c}, image.Point{}, draw.Src)
	}
	fill(0, 0, w, h, color.RGBA{244, 244, 248, 255})
	fill(0, 0, w, h/12, color.RGBA{255, 255, 255, 255})
	fill(0, 0, w/6, h, color.RGBA{236, 236, 244, 255})
	for i := 0; i < 5; i++ {
		fill(w/40, h/6+i*h/12, w/6-w/40, h/6+i*h/12+h/40, color.RGBA{200, 200, 216, 255})
	}
	for i := 0; i < 3; i++ {
		x := w/5 + i*(w/4)
		fill(x, h/6, x+w/5, h/6+h/4, color.RGBA{255, 255, 255, 255})
		fill(x+w/40, h/6+h/20, x+w/10, h/6+h/12, accent)
	}
	fill(w/5, h/2, w-w/12, h/2+h/6, color.RGBA{255, 226, 226, 255})
	fill(w/5+w/50, h/2+h/20, w/2, h/2+h/12, color.RGBA{214, 64, 64, 255})
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 80})
	return b.Bytes()
}

// feedback seeds HOP ID's feedback inbox: seven apps, three kinds, three states, with and without a picture, from
// guests and from a member, with and without an IP address (removed after 30 days), long, markup-like, link-like
// and non-Latin text. With -bulk enough for several pages.
func feedback() *fakehub.Feedback {
	f := fakehub.NewFeedback()
	now := time.Now().UTC().Truncate(time.Minute)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	h := time.Hour
	if *bulk {
		for i := 1; i <= 40; i++ {
			f.Seed(fakehub.Report{Kind: fakehub.FeedbackKinds[i%3], Status: fakehub.FeedbackStatuses[i%3], App: fakehub.FeedbackApps[i%7], Message: fmt.Sprintf("Older report number %d for the paging check.", i),
				IP: "198.51.100.20", UserAgent: "Mozilla/5.0 (X11; Linux x86_64) Firefox/130.0", Viewport: "1280x720", Lang: "en", CreatedAt: ago(time.Duration(200+i) * h)})
		}
	}
	long := "The editor loses what I typed when I switch between the preview and the code tab on a slow connection. " +
		"It happened three times today. Steps: open a note, write two paragraphs, press the preview tab, wait about ten seconds, then go back. " +
		"The second paragraph is gone and the cursor jumps to the top. I use a phone on mobile data, so the connection drops now and then. " +
		"Please keep a local copy of the draft, or warn before leaving the tab. Thank you for the great work, I use NoteHop every day."
	f.Seed(fakehub.Report{Kind: "idea", Status: "read", App: "payhop", Message: "Could the bill page show amounts with the local currency symbol?", Page: "bill.share", AppVersion: "1.4.0", UserID: 12, Username: "budi", IP: "198.51.100.31", UserAgent: "Safari 17 iPhone", Viewport: "390x844", Lang: "id", CreatedAt: ago(30 * h)})
	f.Seed(fakehub.Report{Kind: "other", Status: "done", App: "hub", Message: "日本語で書いても大丈夫ですか?とても使いやすいです。ありがとうございます。", Page: "home", AppVersion: "2.9.0", UserAgent: "Chrome 130", Viewport: "1920x1080", Lang: "ja", CreatedAt: ago(70 * h)})
	f.Seed(fakehub.Report{Kind: "problem", Status: "new", App: "notehop", Message: long, Page: "/n/{id}", AppVersion: "1.3.2", UserID: 7, Username: "helena", IP: "203.0.113.45", UserAgent: "Mozilla/5.0 (Linux; Android 14) Chrome/130", Viewport: "390x844", Lang: "en", Screenshot: mockShot(780, 1688, color.RGBA{59, 79, 216, 255}), CreatedAt: ago(3 * h)})
	f.Seed(fakehub.Report{Kind: "problem", Status: "new", App: "pixelhop", Message: "Upload fails with a spinner that never stops.\n<script>alert(1)</script>\nSee https://contoh.test/x for the file.", Page: "dashboard.upload", AppVersion: "3.7.0", IP: "203.0.113.77", UserAgent: "Edge 130 on Windows", Viewport: "1440x900", Lang: "en", Screenshot: mockShot(1280, 800, color.RGBA{214, 64, 64, 255}), CreatedAt: ago(2 * h)})
	f.Seed(fakehub.Report{Kind: "idea", Status: "new", App: "linkhop", Message: "Folders for links, please.", Page: "dashboard.links", AppVersion: "2.4.1", UserID: 7, Username: "helena", IP: "203.0.113.45", UserAgent: "Firefox 130", Viewport: "1366x768", Lang: "de", CreatedAt: ago(90 * time.Minute)})
	f.Seed(fakehub.Report{Kind: "problem", Status: "read", App: "filehop", Message: "Download link says the file expired after one day, but I set seven days.", Page: "dl", AppVersion: "1.1.0", UserAgent: "Chrome 130", Viewport: "1536x864", Lang: "ru", CreatedAt: ago(45 * 24 * h)})
	f.Seed(fakehub.Report{Kind: "other", Status: "new", App: "biohop", Message: "Привет! Страница профиля открывается медленно по вечерам.", Page: "bio.show", AppVersion: "2.0.3", IP: "203.0.113.8", UserAgent: "Opera 114", Viewport: "1280x720", Lang: "ru", CreatedAt: ago(40 * time.Minute)})
	return f
}
