package main

// Generated lists for screenshots and layout measurements (-bulk): enough rows to
// show how a page behaves when it is long, all made up (example.test addresses).

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var bulk = flagBool("bulk", false, "serve long generated lists (users, content, names, domains, audit) instead of the 3-5 row samples")

var firstNames = []string{"rani", "budi", "sari", "dewi", "agus", "wulan", "joko", "maya", "putri", "bayu", "nina", "tono"}

func offset(r *http.Request) (int, int) {
	off, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
	lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	return off, lim
}

// pageOf writes rows[off:off+lim] with the cursor of the next page.
func pageOf(w http.ResponseWriter, rows []map[string]any, r *http.Request) {
	off, lim := offset(r)
	if off > len(rows) {
		off = len(rows)
	}
	end := off + lim
	var next any
	if end < len(rows) {
		next = strconv.Itoa(end)
	} else {
		end = len(rows)
	}
	w.Header().Set("Content-Type", "application/json")
	jsonOut(w, map[string]any{"ok": true, "data": rows[off:end], "page": map[string]any{"next": next, "total": len(rows)}})
}

func bulkUsers() []map[string]any {
	var rows []map[string]any
	for i := 0; i < 120; i++ {
		u := fmt.Sprintf("%s%d", firstNames[i%len(firstNames)], i+1)
		st := "active"
		if i%17 == 5 {
			st = "suspended"
		}
		roles := []string{}
		if i == 2 {
			roles = []string{"moderator"}
		}
		rows = append(rows, map[string]any{"sub": strconv.Itoa(100 + i), "username": u, "name": "Pengguna Contoh " + u, "email": u + "@example.test", "status": st, "roles": roles, "verified": i%5 != 0,
			"created_at": day(3 + i), "last_login_at": day(i % 9), "links": 3 + i*7%40, "bio_pages": i % 3})
	}
	return rows
}

func bulkLinks() []map[string]any {
	var rows []map[string]any
	for i := 0; i < 120; i++ {
		row := map[string]any{"id": strconv.Itoa(2000 - i), "type": "link", "title": fmt.Sprintf("/kampanye-musim-semi-2026-halaman-promo-%d", i), "status": []string{"live", "live", "disabled"}[i%3],
			"clicks": 10 + i*13%900, "created_at": day(i%20) + "T07:40:00Z", "url": fmt.Sprintf("https://hel.ink/kampanye-musim-semi-%d", i)}
		if i%4 != 3 {
			row["owner"] = map[string]any{"sub": strconv.Itoa(100 + i%30), "username": firstNames[i%len(firstNames)] + strconv.Itoa(i%30+1)}
		}
		if i%9 == 0 {
			row["flags"] = []string{"guest"}
		}
		rows = append(rows, row)
	}
	return rows
}

func bulkAudit() []map[string]any {
	var rows []map[string]any
	for i := 0; i < 51; i++ {
		rows = append(rows, map[string]any{"created_at": time.Now().UTC().Add(-time.Duration(i) * 97 * time.Minute).Format(time.RFC3339), "actor": "1:superadmin", "app_slug": []string{"imperium", "linkhop", "filehop"}[i%3],
			"action": []string{"link.disable", "user.suspend", "settings.update"}[i%3], "target_type": "link", "target_id": strconv.Itoa(1500 + i), "reason": "pemeriksaan rutin"})
	}
	return rows
}

func bulkNames() map[string]any {
	var res []string
	for i := 0; i < 189; i++ {
		res = append(res, fmt.Sprintf("nama-dilindungi-%03d", i))
	}
	holds := []map[string]any{{"slug": "konten-buruk", "reason": "takedown", "until": day(-20)}, {"slug": "admin", "reason": "reserved", "until": nil}}
	return map[string]any{"holds": holds, "reserved": res}
}

func bulkDomains() []map[string]any {
	var rows []map[string]any
	for i := 0; i < 55; i++ {
		rows = append(rows, map[string]any{"id": i + 1, "domain": fmt.Sprintf("situs-berbahaya-%d.example", i), "match_type": "wildcard", "category": "phishing", "notes": "dilaporkan oleh pengguna dan diverifikasi", "created_at": day(i%30) + "T00:00:00Z"})
	}
	return rows
}

func bulkSettings() []map[string]any {
	var rows []map[string]any
	for i := 0; i < 20; i++ {
		rows = append(rows, map[string]any{"key": fmt.Sprintf("seo.page_setting_%d", i), "type": []string{"string", "int", "bool"}[i%3], "value": []any{"LinkHop", 10 + i, i%2 == 0}[i%3],
			"description": "Pengaturan contoh untuk mengukur tata letak tabel pengaturan dengan keterangan yang cukup panjang"})
	}
	return rows
}
