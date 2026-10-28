// fakehop plays HOP ID, PixelHop and NoteHop on localhost so Imperium's UI can
// be developed and screenshotted without the real servers. Sign-in always
// succeeds as an owner with 2FA. Not for production.
//
//	go run ./scripts/fakehop -addr 127.0.0.1:9101 -px 127.0.0.1:9102 -nh 127.0.0.1:9103
//	HOP_URL=http://127.0.0.1:9101 HOP_SSO_SECRET=<secret> ... bin/imperium -addr 127.0.0.1:9100
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/navi-crwn/imperium/internal/hopproto"
)

var (
	addr     = flag.String("addr", "127.0.0.1:9101", "HOP ID (LinkHop) address")
	pxAddr   = flag.String("px", "127.0.0.1:9102", "PixelHop address")
	nhAddr   = flag.String("nh", "127.0.0.1:9103", "NoteHop address")
	secret   = flag.String("secret", "imperium-dev-secret-that-is-long-enough-1234", "Imperium's secret in the registry")
	pxSecret = flag.String("px-secret", "pixelhop-dev-secret-that-is-long-enough-12", "PixelHop's secret")
	nhSecret = flag.String("nh-secret", "notehop-dev-secret-that-is-long-enough-123", "NoteHop's secret")
	nonces   hopproto.Nonces
)

func ok(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
}

func page(w http.ResponseWriter, data any, total int) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data, "page": map[string]any{"next": nil, "total": total}})
}

// signed verifies the HOP signature with the given secret, then logs and serves.
func signed(sec string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := nonces.Verify(r, body, "imperium", []string{sec}, time.Minute); err != nil {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		log.Printf("%s %s actor=%s", r.Method, r.URL.RequestURI(), r.Header.Get("X-HOP-Actor"))
		h(w, r)
	}
}

func day(i int) string { return time.Now().UTC().AddDate(0, 0, -i).Format("2006-01-02") }

func series(vals ...int) [][2]any {
	out := make([][2]any, 0, len(vals))
	for i, v := range vals {
		out = append(out, [2]any{day(len(vals) - 1 - i), v})
	}
	return out
}

func main() {
	flag.Parse()
	go func() { log.Fatal(http.ListenAndServe(*pxAddr, pixelhop())) }()
	go func() { log.Fatal(http.ListenAndServe(*nhAddr, notehop())) }()
	log.Printf("fakehop: HOP ID %s, PixelHop %s, NoteHop %s", *addr, *pxAddr, *nhAddr)
	log.Fatal(http.ListenAndServe(*addr, hopID()))
}

func hopID() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /sso/{app}", func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Unix()
		p := map[string]any{"sub": "1", "username": "superadmin", "name": "Superadmin", "email": "owner@example.test", "roles": []string{"owner"}, "mfa": true, "status": "active",
			"state": r.URL.Query().Get("state"), "iat": now, "exp": now + 120, "jti": fmt.Sprint(now, time.Now().Nanosecond()), "aud": r.PathValue("app"), "v": 2}
		js, _ := json.Marshal(p)
		body := hopproto.B64(js)
		mac := hmac.New(sha256.New, []byte(*secret))
		mac.Write([]byte(body))
		http.Redirect(w, r, r.URL.Query().Get("return_to")+"?ticket="+body+"."+hopproto.B64(mac.Sum(nil)), http.StatusFound)
	})
	s := func(p string, h http.HandlerFunc) { m.HandleFunc(p, signed(*secret, h)) }
	s("GET /api/hop/apps", func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{
			{"slug": "pixelhop", "name": "PixelHop", "url": "http://" + *pxAddr, "status": "active", "icon": "image", "capabilities": []string{"account_api", "admin_api", "events", "metrics"}, "secret": *pxSecret, "sort_order": 10},
			{"slug": "notehop", "name": "NoteHop", "url": "http://" + *nhAddr, "status": "active", "icon": "note", "capabilities": []string{"admin_api", "events", "metrics", "zero_knowledge"}, "secret": *nhSecret, "sort_order": 20},
			{"slug": "remotehop", "name": "RemoteHop", "url": "http://127.0.0.1:9", "status": "active", "capabilities": []string{"account_api"}, "sort_order": 30},
			{"slug": "imperium", "name": "Imperium", "url": "x", "status": "active", "hidden": true},
		})
	})
	s("GET /api/hop/health", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"app": "linkhop", "version": "8a22b3d", "db": "ok"})
	})
	s("GET /api/hop/manifest", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"app": "linkhop", "name": "LinkHop", "icon": "link", "capabilities": []string{"admin_api", "account_server", "reports", "settings", "search", "metrics"},
			"content_types": []map[string]any{{"type": "link", "label": "Links", "actions": []string{"disable", "enable", "delete", "transfer", "rescan"}}, {"type": "bio", "label": "Bio pages", "actions": []string{"unpublish", "publish", "delete", "transfer"}}},
			"extensions":    []string{"ip", "domains", "queue"}, "settings_schema": []map[string]any{{"key": "seo.site_title", "type": "string"}}})
	})
	s("GET /api/hop/stats", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"kpi": map[string]any{"users": 74, "users_new_7d": 6, "users_suspended": 2, "links": 1532, "links_live": 1377, "bio_pages": 41, "clicks_30d": 48213, "clicks_today": 1206, "open_reports": 3, "open_flags": 5},
			"series": map[string]any{"signups": series(1, 0, 2, 3, 1, 0, 4, 2, 1, 0, 1, 3, 2, 1), "clicks": series(1400, 1520, 1310, 1780, 2100, 1900, 1650, 1720, 1580, 1660, 1840, 1990, 1750, 1206)}})
	})
	s("GET /api/hop/search", func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{{"kind": "user", "id": "26", "label": "@rani", "sub": "Rani Dewi"}})
	})
	s("GET /api/hop/users", func(w http.ResponseWriter, r *http.Request) {
		page(w, []map[string]any{
			{"sub": "26", "username": "rani", "name": "Rani Dewi", "email": "rani@example.test", "status": "active", "roles": []string{}, "verified": true, "created_at": "2026-09-02", "last_login_at": "2026-10-27", "links": 38, "bio_pages": 1},
			{"sub": "31", "username": "budi", "name": "Budi Santoso", "email": "budi@example.test", "status": "suspended", "roles": []string{}, "verified": true, "created_at": "2026-09-14", "last_login_at": "2026-10-20", "links": 410, "bio_pages": 0},
			{"sub": "40", "username": "sari", "name": "Sari", "email": "sari@example.test", "status": "active", "roles": []string{"moderator"}, "verified": false, "created_at": "2026-10-21", "last_login_at": "2026-10-28", "links": 3, "bio_pages": 2},
		}, 74)
	})
	s("GET /api/hop/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"profile": map[string]any{"sub": r.PathValue("id"), "username": "rani", "name": "Rani Dewi", "email": "rani@example.test", "status": "active", "verified": true},
			"counts": map[string]any{"links": 38, "bio_pages": 1, "clicks_30d": 912}, "sessions": map[string]any{"count": 2}, "logins": []any{}, "activity": []any{}, "roles": []string{}, "notes": []any{}, "recent_links": []any{}})
	})
	s("GET /api/hop/audit", func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{{"created_at": day(0) + "T08:12:00Z", "actor": "1:superadmin", "app_slug": "imperium", "action": "link.disable", "target_type": "link", "target_id": "9", "reason": "phishing"}})
	})
	s("GET /api/hop/content", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "bio" {
			page(w, []map[string]any{{"id": "7", "type": "bio", "title": "Rani · tautan", "owner": map[string]any{"sub": "26", "username": "rani"}, "status": "published", "views": 1200, "created_at": day(9) + "T10:00:00Z", "url": "https://hel.ink/b/rani"}}, 41)
			return
		}
		page(w, []map[string]any{
			{"id": "1532", "type": "link", "title": "/promo-oktober", "owner": map[string]any{"sub": "26", "username": "rani"}, "status": "live", "clicks": 312, "created_at": day(0) + "T07:40:00Z", "url": "https://hel.ink/promo-oktober"},
			{"id": "1531", "type": "link", "title": "/x9k2", "owner": nil, "status": "live", "clicks": 4, "created_at": day(0) + "T06:12:00Z", "url": "https://hel.ink/x9k2", "flags": []string{"guest"}},
			{"id": "1530", "type": "link", "title": "/login-bank-verify", "owner": map[string]any{"sub": "31", "username": "budi"}, "status": "disabled", "clicks": 88, "created_at": day(1) + "T22:05:00Z", "url": "https://hel.ink/login-bank-verify", "flags": []string{"phishing", "password"}},
			{"id": "1529", "type": "link", "title": "/undangan-nikah", "owner": map[string]any{"sub": "40", "username": "sari"}, "status": "live", "clicks": 1911, "created_at": day(2) + "T13:30:00Z", "url": "https://hel.ink/undangan-nikah"},
			{"id": "1528", "type": "link", "title": "/menu-warung", "owner": map[string]any{"sub": "40", "username": "sari"}, "status": "live", "clicks": 230, "created_at": day(3) + "T09:00:00Z", "url": "https://hel.ink/menu-warung"},
		}, 1532)
	})
	s("GET /api/hop/content/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"id": r.PathValue("id"), "type": r.PathValue("type"), "title": "/login-bank-verify", "slug": "login-bank-verify", "target_url": "https://bad.example/verify", "owner": map[string]any{"sub": "31", "username": "budi"}, "status": "disabled", "flags": []string{"phishing"}, "clicks": 88})
	})
	s("GET /api/hop/reports", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") == "closed" {
			page(w, []any{}, 0)
			return
		}
		page(w, []map[string]any{
			{"kind": "flag", "id": "3", "subject": map[string]any{"type": "link", "id": "1530"}, "score": 80, "summary": "phishing words, password field", "status": "open", "created_at": day(1) + "T22:06:00Z"},
			{"kind": "report", "id": "12", "subject": map[string]any{"type": "link", "id": "1531"}, "reason": "spam", "reporter": "guest", "status": "open", "created_at": day(0) + "T05:00:00Z"},
			{"kind": "flag", "id": "4", "subject": map[string]any{"type": "bio", "id": "7"}, "score": 35, "summary": "adult keywords", "status": "open", "created_at": day(2) + "T01:00:00Z"},
		}, 3)
	})
	s("GET /api/hop/settings", func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{{"key": "seo.site_title", "type": "string", "value": "LinkHop", "description": "Site & SEO: site title"}, {"key": "turnstile", "type": "bool", "value": true, "readonly": true, "description": "Cloudflare Turnstile"}})
	})
	s("GET /api/hop/x/ip", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"bans": []map[string]any{{"id": 1, "ip": "203.0.113.9", "reason": "spam burst", "expires_at": nil, "created_at": day(3) + "T00:00:00Z"}},
			"watchlist": []map[string]any{{"id": 5, "ip": "198.51.100.4", "user": "budi", "reason": "failed logins", "attempts": 14, "last_attempt_at": day(0) + "T02:00:00Z"}}})
	})
	s("GET /api/hop/x/domains", func(w http.ResponseWriter, r *http.Request) {
		page(w, []map[string]any{{"id": 1, "domain": "bad.example", "match_type": "wildcard", "category": "phishing", "notes": "", "created_at": day(5) + "T00:00:00Z"}}, 1)
	})
	s("GET /api/hop/x/queue", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"worker": "ok", "heartbeat_at": time.Now().UTC().Add(-40 * time.Second).Format(time.RFC3339), "pending": 2, "failed": 0, "abuse_scan_at": time.Now().UTC().Add(-50 * time.Minute).Format(time.RFC3339)})
	})
	m.HandleFunc("POST /api/hop/", signed(*secret, func(w http.ResponseWriter, r *http.Request) { ok(w, map[string]any{}) }))
	m.HandleFunc("PUT /api/hop/", signed(*secret, func(w http.ResponseWriter, r *http.Request) { ok(w, map[string]any{}) }))
	return m
}

func pixelhop() http.Handler {
	m := http.NewServeMux()
	s := func(p string, h http.HandlerFunc) { m.HandleFunc(p, signed(*pxSecret, h)) }
	s("GET /hop/health", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"app": "pixelhop", "version": "3.7.0", "db": "ok"})
	})
	s("GET /hop/manifest", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"app": "pixelhop", "name": "PixelHop", "icon": "image", "capabilities": []string{"admin_api", "events", "account_api", "metrics"},
			"content_types":   []map[string]any{{"type": "image", "label": "Images", "actions": []string{"hide", "unhide", "delete", "ban_ip", "transfer"}}, {"type": "album", "label": "Albums", "actions": []string{"delete", "transfer"}}},
			"extensions":      []string{"ip"},
			"settings_schema": []map[string]any{{"key": "guest_uploads", "type": "bool", "description": "Let visitors upload without an account"}, {"key": "announcement", "type": "string", "description": "Banner shown on every page"}},
			"actions":         map[string]any{"user": []string{"suspend", "lock", "activate"}, "report": []string{"dismiss", "resolve", "delete"}, "content": []string{"hide", "unhide", "delete", "ban_ip", "transfer"}}})
	})
	s("GET /hop/stats", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"kpi": map[string]any{"users": 212, "images": 8431, "videos": 120, "pdfs": 44, "content": 8595, "albums": 310, "bytes": 23_622_320_128, "views": 1_204_550, "uploads_today": 37, "open_reports": 2, "bans": 4, "guest_uploads": 1, "disk_free_bytes": 61_000_000_000, "disk_total_bytes": 160_000_000_000},
			"series": map[string]any{"uploads": series(40, 52, 31, 60, 72, 48, 55, 61, 44, 39, 58, 70, 66, 37), "bytes": series(90e6, 120e6, 70e6, 150e6, 160e6, 110e6, 130e6, 140e6, 100e6, 95e6, 125e6, 170e6, 150e6, 80e6)}, "step": "day"})
	})
	s("GET /hop/content", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "album" {
			page(w, []map[string]any{{"id": "a1", "type": "album", "title": "Liburan Bali", "owner": map[string]any{"sub": "26", "username": "rani"}, "status": "public", "size": 420_000_000, "created_at": day(4) + "T10:00:00Z"}}, 310)
			return
		}
		page(w, []map[string]any{
			{"id": "k3f9a", "type": "image", "title": "sunset.jpg", "owner": map[string]any{"sub": "26", "username": "rani"}, "status": "live", "size": 2_480_000, "views": 310, "created_at": day(0) + "T07:10:00Z", "url": "http://" + *pxAddr + "/i/k3f9a"},
			{"id": "p0q2z", "type": "image", "title": "IMG_2231.png", "owner": nil, "status": "hidden", "size": 5_100_000, "views": 12, "created_at": day(0) + "T05:42:00Z", "flags": []string{"reported"}},
			{"id": "m7x1c", "type": "image", "title": "menu.pdf", "owner": map[string]any{"sub": "40", "username": "sari"}, "status": "live", "size": 880_000, "views": 1_920, "created_at": day(1) + "T19:00:00Z"},
			{"id": "d4r8t", "type": "image", "title": "clip.mp4", "owner": map[string]any{"sub": "31", "username": "budi"}, "status": "live", "size": 48_000_000, "views": 77, "created_at": day(2) + "T12:00:00Z"},
		}, 8431)
	})
	s("GET /hop/content/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"id": r.PathValue("id"), "type": r.PathValue("type"), "title": "IMG_2231.png", "owner": nil, "status": "hidden", "size": 5_100_000, "views": 12, "sha256": "ab12…", "backend": "local"})
	})
	s("GET /hop/reports", func(w http.ResponseWriter, r *http.Request) {
		page(w, []map[string]any{
			{"id": 41, "kind": "image", "image": map[string]any{"id": "p0q2z", "url": "http://" + *pxAddr + "/i/p0q2z"}, "reason": "nudity", "note": "", "ip": "198.51.100.7", "status": "open", "created": day(0) + "T05:50:00Z"},
			{"id": 40, "kind": "image", "image": map[string]any{"id": "d4r8t", "url": "http://" + *pxAddr + "/i/d4r8t"}, "reason": "copyright", "note": "film clip", "ip": "203.0.113.2", "status": "open", "created": day(1) + "T15:00:00Z"},
		}, 2)
	})
	s("GET /hop/settings", func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{{"key": "guest_uploads", "type": "bool", "value": true, "description": "Let visitors upload without an account"}, {"key": "announcement", "type": "string", "value": "", "description": "Banner shown on every page"}})
	})
	s("GET /hop/x/ip", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"bans": []map[string]any{{"ip": "203.0.113.2", "reason": "copyright spam", "created_at": day(1) + "T15:30:00Z"}, {"ip": "198.51.100.7", "reason": "nudity uploads", "created_at": day(0) + "T06:00:00Z"}}})
	})
	m.HandleFunc("POST /hop/", signed(*pxSecret, func(w http.ResponseWriter, r *http.Request) { ok(w, map[string]any{}) }))
	m.HandleFunc("PUT /hop/", signed(*pxSecret, func(w http.ResponseWriter, r *http.Request) { ok(w, map[string]any{}) }))
	return m
}

func notehop() http.Handler {
	m := http.NewServeMux()
	s := func(p string, h http.HandlerFunc) { m.HandleFunc(p, signed(*nhSecret, h)) }
	s("GET /hop/health", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"app": "notehop", "version": "29f463b", "db": "ok"})
	})
	s("GET /hop/manifest", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"app": "notehop", "name": "NoteHop", "icon": "note", "capabilities": []string{"admin_api", "events", "metrics", "zero_knowledge"},
			"content_types":   []map[string]any{{"type": "note", "label": "Notes", "actions": []string{"delete", "takedown", "freeze", "set_expiry", "release_slug"}}, {"type": "file", "label": "Attachments", "actions": []string{"delete"}}},
			"extensions":      []string{"names", "files", "realtime", "integrations"},
			"settings_schema": []map[string]any{{"key": "rate_limit_per_min", "type": "int"}},
			"actions":         map[string]any{"user": []string{"revoke_sessions", "purge"}, "report": []string{"dismiss", "resolve", "takedown"}}})
	})
	s("GET /hop/stats", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"kpi": map[string]any{"users": 58, "users_sso": 51, "notes": 3120, "notes_encrypted": 2890, "notes_public": 230, "notes_owned": 1402, "notes_burn": 96, "files": 410, "files_bytes": 1_395_864_371, "files_budget_bytes": 10_737_418_240, "live_rooms": 3, "slug_holds": 2, "reports_open": 1, "takedowns": 4},
			"series": map[string]any{"notes_created": series(20, 31, 18, 42, 55, 39, 27, 33, 48, 29, 36, 61, 44, 25)}})
	})
	s("GET /hop/content", func(w http.ResponseWriter, r *http.Request) {
		page(w, []map[string]any{
			{"id": "3120", "type": "note", "title": "(terenkripsi)", "owner": map[string]any{"sub": "26", "username": "rani"}, "status": "encrypted", "size": 2_048, "views": 3, "created_at": day(0) + "T08:00:00Z", "flags": []string{"burn"}},
			{"id": "3119", "type": "note", "title": "Daftar belanja", "owner": nil, "status": "public", "size": 512, "views": 41, "created_at": day(0) + "T06:30:00Z", "url": "http://" + *nhAddr + "/n/3119"},
			{"id": "3118", "type": "note", "title": "(terenkripsi)", "owner": map[string]any{"sub": "40", "username": "sari"}, "status": "encrypted", "size": 90_000, "views": 0, "created_at": day(1) + "T21:00:00Z"},
		}, 3120)
	})
	s("GET /hop/content/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"id": r.PathValue("id"), "type": "note", "title": "Daftar belanja", "status": "public", "views": 41})
	})
	s("GET /hop/reports", func(w http.ResponseWriter, r *http.Request) {
		page(w, []map[string]any{{"id": "9", "kind": "note", "subject": map[string]any{"type": "note", "id": "3119"}, "reason": "spam", "status": "open", "created_at": day(0) + "T07:00:00Z"}}, 1)
	})
	s("GET /hop/settings", func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{{"key": "rate_limit_per_min", "type": "int", "value": 60, "description": "Requests per minute per IP"}})
	})
	s("GET /hop/x/names", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"holds": []map[string]any{{"slug": "konten-buruk", "reason": "takedown", "until": day(-20)}, {"slug": "admin", "reason": "reserved", "until": nil}}, "reserved": []string{"admin", "api", "login"}})
	})
	s("GET /hop/x/files", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"count": 410, "bytes": 1_395_864_371, "budget_bytes": 10_737_418_240, "largest": []map[string]any{{"id": "f1", "name": "backup.zip", "bytes": 240_000_000, "note": "3002"}}})
	})
	s("GET /hop/x/realtime", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"rooms": 3, "clients": 7, "messages_1h": 412, "uptime_s": 86_400 * 3})
	})
	s("GET /hop/x/integrations", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"webhooks": []map[string]any{{"id": "w1", "url": "https://hooks.example/notehop", "events": "note.created", "failures": 0}}})
	})
	m.HandleFunc("POST /hop/", signed(*nhSecret, func(w http.ResponseWriter, r *http.Request) { ok(w, map[string]any{}) }))
	m.HandleFunc("PUT /hop/", signed(*nhSecret, func(w http.ResponseWriter, r *http.Request) { ok(w, map[string]any{}) }))
	return m
}
