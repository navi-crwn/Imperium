package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

// digest builds a valid guest.digest report from linkhop, as LinkHop's
// links:guest-digest sends it.
func digest(mod func(m map[string]any, d map[string]any)) map[string]any {
	d := map[string]any{
		"guest_links": 42, "pending": 7, "disabled": 3, "flags": 2, "creators": 19,
		"spikes":      []any{"evil-kit.icu"},
		"top_domains": []any{[]any{"evil-kit.icu", 15}, []any{"example.com", 9}, []any{"xn--bcher-kva.example", 4}, []any{"?", 1}},
	}
	m := map[string]any{"id": "dg-" + strconv.FormatInt(time.Now().UnixNano(), 36), "type": "guest.digest", "at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"app": "linkhop", "env": "production", "data": d}
	if mod != nil {
		mod(m, d)
	}
	return m
}

func TestGuestDigestIsAcceptedAndStored(t *testing.T) {
	a, srv, _ := newInbox(t)
	out := mustAccept(t, srv, "linkhop", lhOwnSecret, digest(nil))
	if data := out["data"].(map[string]any); data["duplicate"] != false || data["group"] == "" {
		t.Fatalf("reply: %v", out)
	}
	g := groups(t, a)
	if len(g) != 1 {
		t.Fatalf("groups: %+v", g)
	}
	n := g[0]
	if n.Kind != "guest.digest" || n.App != "linkhop" || n.Env != "production" || n.Title != "guest.digest" || n.Count != 1 || n.Status != "new" {
		t.Fatalf("group: %+v", n)
	}
	dg := n.Sample.Digest
	if dg == nil {
		t.Fatal("no digest on the sample")
	}
	if dg.Date != time.Now().UTC().Format("2006-01-02") || dg.GuestLinks != 42 || dg.Pending != 7 || dg.Disabled != 3 || dg.Flags != 2 || dg.Creators != 19 {
		t.Errorf("digest: %+v", dg)
	}
	if len(dg.Spikes) != 1 || dg.Spikes[0] != "evil-kit.icu" || len(dg.TopDomains) != 4 || dg.TopDomains[0] != (db.DomainCount{Domain: "evil-kit.icu", Count: 15}) || dg.TopDomains[3].Domain != "?" {
		t.Errorf("domains: %+v %+v", dg.Spikes, dg.TopDomains)
	}
	if n.Sample.Route != "" || n.Sample.Origin != "" || n.Sample.Ref != "" {
		t.Errorf("a digest has no error fields: %+v", n.Sample)
	}
	if n.Fingerprint != db.DigestFingerprint("linkhop", "production", "guest.digest", dg.Date) {
		t.Error("a digest is grouped by app, env, type and day")
	}
	// An empty digest (no guest links) is valid too.
	mustAccept(t, srv, "linkhop", lhOwnSecret, digest(func(m, d map[string]any) {
		for _, k := range []string{"guest_links", "pending", "disabled", "flags", "creators"} {
			d[k] = 0
		}
		d["spikes"], d["top_domains"] = []any{}, []any{}
	}))
	mustAccept(t, srv, "pixelhop", pxSecret, digest(func(m, d map[string]any) { m["app"] = "pixelhop" }))
}

func TestGuestDigestRefusesBadFieldsAndTypes(t *testing.T) {
	a, srv, _ := newInbox(t)
	many := make([]any, 51)
	for i := range many {
		many[i] = "d" + strconv.Itoa(i) + ".example"
	}
	tops := make([]any, 11)
	for i := range tops {
		tops[i] = []any{"d" + strconv.Itoa(i) + ".example", i}
	}
	cases := []struct {
		name  string
		field string
		mod   func(m, d map[string]any)
	}{
		{"unknown type", "type", func(m, d map[string]any) { m["type"] = "guest.weekly" }},
		{"error fields in a digest", "data", func(m, d map[string]any) { d["class"] = "panic" }},
		{"a list of addresses", "data", func(m, d map[string]any) { d["urls"] = []any{"https://evil-kit.icu/x?t=1"} }},
		{"an ip field", "data", func(m, d map[string]any) { d["ips"] = []any{"203.0.113.7"} }},
		{"digest fields in an error report", "body", func(m, d map[string]any) {
			m["type"] = "error.500"
			d["class"], d["origin"], d["route"], d["method"], d["ref"] = "panic", "unknown", "unmatched", "GET", "7f3a9c1e2b4d"
		}},
		{"data is a string", "data", func(m, d map[string]any) { m["data"] = "x" }},
		{"data missing", "data.guest_links", func(m, d map[string]any) { delete(m, "data") }},
		{"count missing", "data.pending", func(m, d map[string]any) { delete(d, "pending") }},
		{"count negative", "data.flags", func(m, d map[string]any) { d["flags"] = -1 }},
		{"count a string", "data", func(m, d map[string]any) { d["creators"] = "19" }},
		{"count a fraction", "data", func(m, d map[string]any) { d["disabled"] = 1.5 }},
		{"spike upper case", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"Evil-Kit.icu"} }},
		{"spike with a path", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"evil-kit.icu/login"} }},
		{"spike with a scheme", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"https://evil-kit.icu"} }},
		{"spike with a port", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"evil-kit.icu:8080"} }},
		{"spike with userinfo", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"rani@evil-kit.icu"} }},
		{"spike without a dot", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"localhost"} }},
		{"spike is an ip", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"203.0.113.7"} }},
		{"spike with a leading hyphen", "data.spikes", func(m, d map[string]any) { d["spikes"] = []any{"-evil.icu"} }},
		{"spike not a string", "data", func(m, d map[string]any) { d["spikes"] = []any{7} }},
		{"too many spikes", "data.spikes", func(m, d map[string]any) { d["spikes"] = many }},
		{"too many top domains", "data.top_domains", func(m, d map[string]any) { d["top_domains"] = tops }},
		{"top entry with three parts", "data.top_domains", func(m, d map[string]any) { d["top_domains"] = []any{[]any{"a.com", 1, "x"}} }},
		{"top entry a single value", "data", func(m, d map[string]any) { d["top_domains"] = []any{"a.com"} }},
		{"top entry an object", "data", func(m, d map[string]any) { d["top_domains"] = []any{map[string]any{"domain": "a.com", "count": 1}} }},
		{"top domain not a string", "data.top_domains", func(m, d map[string]any) { d["top_domains"] = []any{[]any{1, 1}} }},
		{"top domain a url", "data.top_domains", func(m, d map[string]any) { d["top_domains"] = []any{[]any{"https://a.com/x?token=abc", 1}} }},
		{"top count negative", "data.top_domains", func(m, d map[string]any) { d["top_domains"] = []any{[]any{"a.com", -1}} }},
		{"top count a string", "data.top_domains", func(m, d map[string]any) { d["top_domains"] = []any{[]any{"a.com", "1"}} }},
		{"app differs from the sender", "app", func(m, d map[string]any) { m["app"] = "pixelhop" }},
		{"time two days old", "at", func(m, d map[string]any) {
			m["at"] = time.Now().Add(-48 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
		}},
	}
	for _, c := range cases {
		res, out := send(t, srv, "linkhop", lhOwnSecret, digest(c.mod))
		if res.StatusCode != 422 || out["error"] != "bad_payload" {
			t.Errorf("%s: want 422 bad_payload, got %d %v", c.name, res.StatusCode, out)
			continue
		}
		msg, _ := out["message"].(string)
		if !strings.HasPrefix(msg, c.field+":") {
			t.Errorf("%s: refused for the wrong reason (want field %s): %q", c.name, c.field, msg)
		}
		for _, leak := range []string{"evil-kit", "203.0.113", "rani@", "token=abc", "https://"} {
			if strings.Contains(msg, leak) {
				t.Errorf("%s: the error echoes the value: %q", c.name, msg)
			}
		}
	}
	if n := len(groups(t, a)); n != 0 {
		t.Fatalf("%d groups stored from refused digests", n)
	}
}

func TestGuestDigestIsOneGroupPerAppAndDay(t *testing.T) {
	a, srv, _ := newInbox(t)
	rep := digest(nil)
	first := mustAccept(t, srv, "linkhop", lhOwnSecret, rep)["data"].(map[string]any)
	again := mustAccept(t, srv, "linkhop", lhOwnSecret, rep)["data"].(map[string]any) // a retry: new nonce, same id
	if first["group"] != again["group"] || again["duplicate"] != true {
		t.Fatalf("first %v again %v", first, again)
	}
	// Another digest the same day (a new id, say after a re-run) joins the group and becomes its sample.
	later := mustAccept(t, srv, "linkhop", lhOwnSecret, digest(func(m, d map[string]any) { d["guest_links"] = 43 }))["data"].(map[string]any)
	if later["group"] != first["group"] || later["duplicate"] != false {
		t.Fatalf("same day: %v vs %v", later, first)
	}
	g := groups(t, a)
	if len(g) != 1 || g[0].Count != 2 || g[0].Sample.Digest == nil || g[0].Sample.Digest.GuestLinks != 43 {
		t.Fatalf("stored: %+v", g)
	}
	// Another app's digest, and the next day's, are their own groups.
	mustAccept(t, srv, "pixelhop", pxSecret, digest(func(m, d map[string]any) { m["app"] = "pixelhop" }))
	ctx := context.Background()
	for _, date := range []string{"2001-01-01", "2001-01-02"} {
		r := db.Report{ID: "day-" + date, Kind: db.KindDigest, App: "linkhop", Env: "production", At: time.Now().UTC(), Class: db.KindDigest, Digest: &db.Digest{Date: date, GuestLinks: 1}}
		if _, err := a.db.RecordNotification(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if g := groups(t, a); len(g) != 4 {
		t.Fatalf("want 4 groups (two days, two apps), got %d: %+v", len(g), g)
	}
	// A duplicate id across days is still the same report.
	r := db.Report{ID: "day-2001-01-01", Kind: db.KindDigest, App: "linkhop", Env: "production", At: time.Now().UTC(), Class: db.KindDigest, Digest: &db.Digest{Date: "2001-01-03"}}
	if res, err := a.db.RecordNotification(ctx, r); err != nil || !res.Duplicate {
		t.Fatalf("duplicate id: %+v %v", res, err)
	}
}

// The day of a digest is the UTC day of "at", so a digest sent just after
// midnight for the previous day still groups with that day.
func TestGuestDigestDayComesFromAt(t *testing.T) {
	now := time.Date(2026, 11, 6, 0, 30, 0, 0, time.UTC)
	for at, want := range map[string]string{"2026-11-05T23:50:00Z": "2026-11-05", "2026-11-06T00:10:00Z": "2026-11-06"} {
		b, _ := json.Marshal(digest(func(m, d map[string]any) { m["at"] = at }))
		p, err := decodeNotify(b)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := validateNotify(p, "linkhop", now)
		if err != nil {
			t.Fatal(err)
		}
		if rep.Digest.Date != want {
			t.Errorf("%s: day %s, want %s", at, rep.Digest.Date, want)
		}
	}
}

// A digest never sends the Telegram "server error" alert; an error report
// after it still does.
func TestGuestDigestDoesNotAlert(t *testing.T) {
	_, srv, tg := telegramApp(t)
	mustAccept(t, srv, "linkhop", lhOwnSecret, digest(nil))
	tg.expectNone(t, "a digest is not a failure")
	mustAccept(t, srv, "linkhop", lhOwnSecret, report("linkhop", nil))
	if msg := tg.expect(t); !strings.Contains(msg, "HTTP 500") {
		t.Fatalf("error alert: %q", msg)
	}
}

// The list shows a digest as a digest, the detail page shows the domains with
// their counts and marks the spikes, and both read in every language.
func TestGuestDigestPagesRender(t *testing.T) {
	a, srv, hop := newInbox(t)
	id := mustAccept(t, srv, "linkhop", lhOwnSecret, digest(func(m, d map[string]any) {
		d["spikes"] = []any{"evil-kit.icu", "other-kit.top"} // the second spike is not among the top domains
	}))["data"].(map[string]any)["group"].(string)
	day := time.Now().UTC().Format("2006-01-02")
	c := signedIn(t, hop, srv, "boss", "owner")

	code, b := get(t, c, srv.URL+"/notifications")
	if code != 200 {
		t.Fatalf("list: %d", code)
	}
	for _, want := range []string{"Guest link digest", day, `Guest links: <span class="mono">42</span>`, `Spikes: <span class="mono">2</span>`, `href="/notifications/` + id + `"`, ">linkhop</b>"} {
		if !strings.Contains(b, want) {
			t.Errorf("list lacks %q", want)
		}
	}
	if strings.Contains(b, ">guest.digest<") {
		t.Error("the list shows the raw type instead of its label")
	}

	code, b = get(t, c, srv.URL+"/notifications/"+id)
	if code != 200 {
		t.Fatalf("detail: %d", code)
	}
	for _, want := range []string{"<title>Guest link digest", "Guest link digest <span class=\"mono\">" + day + "</span>", "evil-kit.icu", ">15<", "example.com", ">9<", "xn--bcher-kva.example",
		`class="chip spike"`, "Awaiting check", ">7<", "Disabled", ">3<", "Abuse flags", ">2<", "Creators", ">19<", "no domain", "Top destination domains",
		"Spikes outside the top domains", "other-kit.top", "registrable domains and counts", `value="ack"`, `value="resolve"`, `value="mute"`} {
		if !strings.Contains(b, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	if strings.Count(b, `class="chip spike"`) != 1 {
		t.Errorf("exactly one top domain spikes: %d chips", strings.Count(b, `class="chip spike"`))
	}
	for _, no := range []string{"Recent occurrences", "Origin (file:line)", ">guest.digest<", "notif."} {
		if strings.Contains(b, no) {
			t.Errorf("detail shows %q", no)
		}
	}
	// A digest can be acknowledged like any group.
	act(t, c, srv, mustID(t, id), "ack", "")
	if statusOf(t, a, mustID(t, id)) != "seen" {
		t.Error("ack")
	}
	// Every language renders both pages without a raw key.
	for _, lang := range nineLanguages {
		a.cfg.Lang = lang
		for _, path := range []string{"/notifications?status=all", "/notifications/" + id} {
			code, b := get(t, c, srv.URL+path)
			if code != 200 || strings.Contains(b, "notif.") || !strings.Contains(b, "evil-kit.icu") && path != "/notifications?status=all" {
				t.Errorf("%s %s: %d, raw key or missing domain", lang, path, code)
			}
			if !strings.Contains(b, strings.NewReplacer("'", "&#39;").Replace(notifTexts[lang]["notif.digest.title"])) {
				t.Errorf("%s %s: lacks the digest title", lang, path)
			}
		}
	}
}

func mustID(t *testing.T, s string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// LinkHop lists the extension as guest_digest and serves it at /x/guest-digest:
// the dashboard shows it next to the queue, capped at ten domains, and the
// extension page shows all of it.
func TestLinkHopDashboardShowsTheGuestDigest(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	a, srv := newApp(t, hop)
	defer srv.Close()
	var (
		mu    sync.Mutex // the dashboard calls several endpoints at once
		paths []string
	)
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	api := func(h func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, body, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
				return
			}
			mu.Lock()
			paths = append(paths, r.URL.Path)
			mu.Unlock()
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api"],"content_types":[{"type":"link","label":"Links","actions":["disable"]}],"extensions":["ip","queue","guest_digest"]}}`))
	}))
	mux.HandleFunc("GET /api/hop/content", api(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":[],"page":{"next":"","total":0}}`))
	}))
	mux.HandleFunc("GET /api/hop/x/queue", api(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"worker":"ok","heartbeat_at":"2026-11-06T06:00:00Z","pending":0,"failed":0}}`))
	}))
	mux.HandleFunc("GET /api/hop/x/guest-digest", api(func(w http.ResponseWriter, r *http.Request) {
		var domains []string
		for i := 0; i < 12; i++ {
			domains = append(domains, fmt.Sprintf(`{"domain":"site-%02d.example","links":%d,"pending":0,"disabled":0,"spike":%v}`, i, 20-i, i == 0))
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"generated_at":"2026-11-06T06:00:00Z","window":{"from":"2026-11-05T06:00:00Z","to":"2026-11-06T06:00:00Z","hours":24},"guest_links":99,"pending":4,"cleared":90,"disabled":5,"flags":2,"creators":31,"spike_threshold":10,"spikes":["site-00.example"],"domains":[` + strings.Join(domains, ",") + `]}}`))
	}))
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))

	_, b := get(t, c, srv.URL+"/apps/linkhop")
	for _, want := range []string{"heartbeat_at", "Operations · Queue", "Operations · Guest link digest", `href="/apps/linkhop/x/guest_digest"`, "guest_links", ">99<", "spike_threshold",
		"site-00.example", "site-09.example", `<th>spikes</th>`, `<th>domain</th>`, `<th>links</th>`} {
		if !strings.Contains(b, want) {
			t.Errorf("dashboard lacks %q", want)
		}
	}
	if strings.Contains(b, "site-10.example") {
		t.Error("the dashboard shows at most ten domains")
	}
	_, b = get(t, c, srv.URL+"/apps/linkhop/x/guest_digest")
	for _, want := range []string{"site-00.example", "site-11.example", ">99<", "Guest link digest"} {
		if !strings.Contains(b, want) {
			t.Errorf("extension page lacks %q", want)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if !contains(paths, "/api/hop/x/guest-digest") || contains(paths, "/api/hop/x/guest_digest") {
		t.Errorf("imperium must call /x/guest-digest: %v", paths)
	}
	_ = a
}
