package app

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

// The flat sidebar lists Imperium pages and each app's health. Modules declared
// by an app's manifest appear in one navigation bar on its content pages.

type navItem struct {
	Label string
	Href  string
	On    bool
	Kind  string // dash, users, content, reports, settings, ext
	Badge int    // a count shown next to the label (new notifications)
}

type navApp struct {
	Slug   string
	Name   string
	URL    string
	Tag    string
	Health Health
	Admin  bool // reachable through the admin API (has a dashboard)
	Open   bool // the folder is expanded (current app)
	Href   string
	Items  []navItem
}

type navData struct {
	Pages  []navItem
	Apps   []navApp
	Global bool
}

// extLabel names an extension in the sidebar and page titles.
func extLabel(l, ext string) string {
	if v := tl(l, "ext."+ext); v != "ext."+ext {
		return v
	}
	return plainWords(ext)
}

// plainWords spells a machine key out: api_tokens becomes "Api tokens".
func plainWords(k string) string {
	k = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(k), "_", " "), "-", " ")
	if k == "" {
		return ""
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// keyLabel is the label of a key an app reports (a count, a column, a table name): the
// translated KPI or label when there is one, else the key spelled out. Never raw underscores.
func keyLabel(l, k string) string {
	for _, p := range []string{"kpi.", "lbl."} {
		if v := tl(l, p+k); v != p+k {
			return v
		}
	}
	return plainWords(k)
}

// splitTabs separates the tabs of an app: the main ones, and the extension tabs that go
// under "More".
func splitTabs(items []navItem) (main, more []navItem) {
	n := 0
	for _, it := range items {
		if it.Kind == "ext" {
			n++
		}
	}
	for _, it := range items {
		if it.Kind == "ext" {
			more = append(more, it)
		} else {
			main = append(main, it)
		}
	}
	return main, more
}

// cachedManifest returns an app's manifest when one was fetched recently;
// the sidebar never blocks a page on a slow app.
func (a *App) cachedManifest(slug string) *Manifest {
	a.manifests.Lock()
	defer a.manifests.Unlock()
	if m, ok := a.manifests.m[slug]; ok && time.Since(m.FetchedAt) < time.Hour {
		return m
	}
	return nil
}

func (a *App) healthSnapshot() map[string]Health {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]Health, len(a.health))
	for k, v := range a.health {
		out[k] = v
	}
	return out
}

// appItems lists the modules for one app's content navigation bar.
func (a *App) appItems(r *http.Request, slug string, m *Manifest, path, typ string) []navItem {
	l := a.langOf(r)
	base := "/apps/" + slug
	items := []navItem{{Label: a.Tr(r, "dash.title"), Href: base, Kind: "dash", On: path == base}}
	if slug == "hub" {
		items = append(items, navItem{Label: a.Tr(r, "nav.users"), Href: "/users", Kind: "users", On: path == "/users" || strings.HasPrefix(path, "/users/")})
	}
	if m == nil {
		return items
	}
	// One fixed set of tabs for every app: Dashboard, Content, Reports, Settings, More. The kinds of
	// content (Images, Albums, Links, …) are picked inside Content; every extension is under More.
	if len(m.ContentTypes) > 0 {
		items = append(items, navItem{Label: tl(l, "mod.content"), Href: base + "/content", Kind: "content", On: strings.HasPrefix(path, base+"/content")})
	}
	if m.HasReports() {
		items = append(items, navItem{Label: a.Tr(r, "mod.reports"), Href: base + "/reports", Kind: "reports", On: strings.HasPrefix(path, base+"/reports")})
	}
	if m.HasSettings() {
		items = append(items, navItem{Label: a.Tr(r, "mod.settings"), Href: base + "/settings", Kind: "settings", On: path == base+"/settings"})
	}
	for _, ext := range m.Extensions {
		items = append(items, navItem{Label: extLabel(l, ext), Href: base + "/x/" + url.PathEscape(ext), Kind: "ext", On: path == base+"/x/"+ext})
	}
	return items
}

// nav builds the sidebar for the current request.
func (a *App) nav(r *http.Request) navData {
	path := r.URL.Path
	typ := r.URL.Query().Get("type")
	sess := current(r)
	global := globalRole(sess)
	n := navData{Global: global, Pages: []navItem{{Label: a.Tr(r, "nav.home"), Href: "/", Kind: "home", On: path == "/"}}}
	if global {
		n.Pages = append(n.Pages, navItem{Label: a.Tr(r, "nav.users"), Href: "/users", Kind: "users", On: path == "/users" || strings.HasPrefix(path, "/users/")})
	}
	n.Pages = append(n.Pages,
		navItem{Label: a.Tr(r, "nav.apps"), Href: "/apps", Kind: "apps", On: path == "/apps"},
		navItem{Label: a.Tr(r, "nav.notifications"), Href: "/notifications", Kind: "notifications", On: strings.HasPrefix(path, "/notifications"), Badge: a.newErrors(r)})
	if global {
		n.Pages = append(n.Pages, navItem{Label: a.Tr(r, "nav.audit"), Href: "/audit", Kind: "audit", On: path == "/audit"})
	}
	health := a.healthSnapshot()
	current := ""
	if rest, ok := strings.CutPrefix(path, "/apps/"); ok {
		current, _, _ = strings.Cut(rest, "/")
	}
	add := func(ap db.App, tag string) {
		admin := ap.Can("admin_api") && ap.Secret != ""
		na := navApp{Slug: ap.Slug, Name: ap.Name, URL: ap.URL, Tag: tag, Health: health[ap.Slug], Admin: admin, Open: ap.Slug == current}
		if admin {
			na.Href = "/apps/" + ap.Slug
			na.Items = a.appItems(r, ap.Slug, a.cachedManifest(ap.Slug), path, typ)
		} else {
			na.Href = "/apps"
		}
		if ap.Slug == "linkhop" {
			na.Name = "LinkHop"
		}
		n.Apps = append(n.Apps, na)
	}
	if lh, _ := a.source(r.Context(), "hub"); canSee(sess, "hub") {
		add(*lh, "")
	}
	apps, _ := a.db.Apps(r.Context())
	for _, ap := range visibleApps(sess, apps) {
		if ap.Status == "off" || ap.Slug == "hub" {
			continue
		}
		add(ap, "")
	}
	return n
}

// ---- per-app dashboard ----

type kpi struct {
	Key   string
	Label string
	Value string
	Warn  bool
}

type seriesView struct {
	Key    string
	Label  string
	Points []point
	Total  int64
	Bytes  bool
	Max    int64
}

// kpiOrder puts the numbers an admin looks at first at the front; anything
// else the app reports follows alphabetically.
var kpiOrder = map[string][]string{
	"hub":      {"users", "users_new_7d", "users_suspended"},
	"linkhop":  {"users", "users_new_7d", "users_suspended", "links", "links_live", "bio_pages", "clicks_today", "clicks_30d", "open_reports", "open_flags", "guest_links_24h", "guest_links_pending"},
	"biohop":   {"pages", "pages_published", "pages_draft", "pages_adult", "blocks", "views_total", "clicks_total", "reports_open"},
	"pixelhop": {"users", "content", "images", "videos", "pdfs", "albums", "bytes", "views", "uploads_today", "open_reports", "bans", "guest_uploads"},
	"filehop":  {"users", "users_active_30d", "sessions_live", "users_suspended"},
	"payhop":   {"users", "users_active_30d", "sessions_live", "users_suspended"},
	"notehop":  {"users", "users_sso", "notes", "notes_encrypted", "notes_public", "notes_owned", "notes_burn", "files", "files_bytes", "files_budget_bytes", "live_rooms", "slug_holds", "reports_open", "takedowns"},
}

// hiddenKPIs are numbers an app reports that the dashboard leaves out. PayHop counts the saved payment
// methods and bills of its people; those never appear in Imperium, not even as totals.
var hiddenKPIs = map[string][]string{"payhop": {"methods_saved", "bills_saved"}}

// attentionKeys are KPIs that mean work when they are above zero.
var attentionKeys = []string{"open_reports", "open_flags", "reports_open", "takedowns", "users_suspended", "failed"}

// dashExtensions are the extensions whose data belongs on the dashboard itself
// (the old control panels showed the queue worker and the realtime rooms there;
// LinkHop's daily digest of guest links by domain sits next to its queue).
var dashExtensions = map[string][]string{"linkhop": {"queue", "guest_digest"}, "notehop": {"realtime"}}

// extPaths names the extensions an app serves under a path that differs from
// the manifest name (LinkHop lists guest_digest and serves /x/guest-digest).
var extPaths = map[string]string{"guest_digest": "guest-digest"}

// extPath is the path segment of an extension in the app's API.
func extPath(ext string) string {
	if p, ok := extPaths[ext]; ok {
		return p
	}
	return url.PathEscape(ext)
}

// dashRows caps a table on the dashboard; the extension page has it all.
const dashRows = 10

// opsCard is one extension's answer as the dashboard shows it: the scalars
// as key/value rows, the lists as small tables.
type opsCard struct {
	Ext, Label, Err string
	KV              []kvRow
	Tables          []table
}

func (a *App) opsCard(l, ext string, data any, err string) opsCard {
	c := opsCard{Ext: ext, Label: extLabel(l, ext), Err: err, KV: kvRows(data), Tables: tables(data)}
	for i := range c.Tables {
		if len(c.Tables[i].Rows) > dashRows {
			c.Tables[i].Rows = c.Tables[i].Rows[:dashRows]
		}
	}
	return c
}

func isBytesKey(k string) bool { return k == "bytes" || strings.HasSuffix(k, "_bytes") }

// kpiLabel translates a KPI key, falling back to the key spelled out.
func kpiLabel(l, key string) string {
	if key == "" {
		return ""
	}
	if v := tl(l, "kpi."+key); v != "kpi."+key {
		return v
	}
	return strings.ToUpper(key[:1]) + strings.ReplaceAll(key[1:], "_", " ")
}

func seriesLabel(l, key string) string {
	if v := tl(l, "series."+key); v != "series."+key {
		return v
	}
	return strings.ToUpper(key[:1]) + strings.ReplaceAll(key[1:], "_", " ")
}

// kpis orders and formats the KPI map of one app.
func (a *App) kpis(l, slug string, raw map[string]int64) (all, attention []kpi) {
	seen := map[string]bool{}
	var keys []string
	for _, k := range kpiOrder[slug] {
		if _, ok := raw[k]; ok {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	var rest []string
	for k := range raw {
		if contains(hiddenKPIs[slug], k) {
			continue
		}
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	keys = append(keys, rest...)
	for _, k := range keys {
		v := raw[k]
		item := kpi{Key: k, Label: kpiLabel(l, k), Value: humanInt(v)}
		if isBytesKey(k) {
			item.Value = humanBytes(v)
		}
		if contains(attentionKeys, k) && v > 0 {
			item.Warn = true
			attention = append(attention, item)
		}
		all = append(all, item)
	}
	return all, attention
}

func (a *App) seriesViews(l string, raw map[string][][2]any) []seriesView {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []seriesView
	for _, k := range keys {
		pts := series(raw[k])
		if len(pts) == 0 {
			continue
		}
		sv := seriesView{Key: k, Label: seriesLabel(l, k), Points: pts, Bytes: isBytesKey(k)}
		for _, p := range pts {
			sv.Total += p.N
			if p.N > sv.Max {
				sv.Max = p.N
			}
		}
		out = append(out, sv)
	}
	return out
}

// kvRow is one scalar of an extension's answer (the queue worker's heartbeat, …).
type kvRow struct{ Key, Value string }

func kvRows(data any) []kvRow {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	keys := keysOfAny(m)
	sort.Strings(keys)
	var out []kvRow
	for _, k := range keys {
		switch m[k].(type) {
		case map[string]any, []any:
			continue
		}
		out = append(out, kvRow{Key: k, Value: cell(m[k])})
	}
	return out
}

// GET /apps/{app}: the app's dashboard: health, KPIs, series, latest content,
// open reports and the operational extension, all fetched concurrently.
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	ap, m, ok := a.withSource(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	var (
		wg         sync.WaitGroup
		health     Health
		st         stats
		statsErr   string
		latest     []row
		latestErr  string
		reports    []map[string]any
		reportsErr string
	)
	var firstCT *ContentType
	if len(m.ContentTypes) > 0 {
		firstCT = &m.ContentTypes[0]
	}
	run := func(f func()) { wg.Add(1); go func() { defer wg.Done(); f() }() }
	run(func() {
		health = a.probe(ctx, s, *ap)
		a.mu.Lock()
		a.health[ap.Slug] = health
		a.mu.Unlock()
	})
	run(func() {
		if err := a.call(ctx, s, ap, "GET", "/stats", nil, &st); err != nil {
			statsErr = err.Error()
		}
	})
	if firstCT != nil {
		run(func() {
			if err := a.call(ctx, s, ap, "GET", "/content?type="+url.QueryEscape(firstCT.Type)+"&limit=8", nil, &latest); err != nil {
				latestErr = err.Error()
			}
		})
	}
	if m.HasReports() {
		run(func() {
			if err := a.call(ctx, s, ap, "GET", "/reports?status=open&limit=6", nil, &reports); err != nil {
				reportsErr = err.Error()
			}
		})
	}
	var exts []string
	for _, ext := range dashExtensions[ap.Slug] {
		if contains(m.Extensions, ext) {
			exts = append(exts, ext)
		}
	}
	opsData, opsErr := make([]any, len(exts)), make([]string, len(exts))
	for i, ext := range exts {
		run(func() {
			if err := a.call(ctx, s, ap, "GET", "/x/"+extPath(ext), nil, &opsData[i]); err != nil {
				opsErr[i] = err.Error()
			}
		})
	}
	wg.Wait()
	a.aliasRows(latest)
	var ops []opsCard
	for i, ext := range exts {
		ops = append(ops, a.opsCard(a.langOf(r), ext, opsData[i], opsErr[i]))
	}
	kpiMap := st.KPI
	if len(kpiMap) == 0 {
		kpiMap = st.Totals
	}
	all, attention := a.kpis(a.langOf(r), ap.Slug, kpiMap)
	all = topNumbers(all, 5)
	seriesAll := a.seriesViews(a.langOf(r), st.Series)
	if len(seriesAll) > 1 {
		seriesAll = seriesAll[:1]
	}
	a.render(w, r, "dashboard.html", ap.Name, map[string]any{
		"App": ap, "M": m, "Health": health, "Items": a.appItems(r, ap.Slug, m, r.URL.Path, ""),
		"KPIs": all, "Attention": attention, "Series": seriesAll, "StatsErr": statsErr,
		"CT": firstCT, "Latest": latest, "LatestErr": latestErr,
		"Reports": reports, "ReportsErr": reportsErr, "HasReports": m.HasReports(),
		"Ops": ops, "CanAct": canActIn(s, ap.Slug),
	})
}

// statCols picks a column class for a row of n cards so that the last row is never a
// lone card: n cards are spread over as few rows as max columns allow, then evenly.
func statCols(n, max int) string {
	if n < 1 {
		n = 1
	}
	rows := (n + max - 1) / max
	cols := (n + rows - 1) / rows
	return "c" + strconv.Itoa(cols)
}

// topNumbers keeps at most n numbers for the dashboard row: the ones that need attention first, then the
// app's own order.
func topNumbers(all []kpi, n int) []kpi {
	if len(all) <= n {
		return all
	}
	var out []kpi
	for _, k := range all {
		if k.Warn && len(out) < n {
			out = append(out, k)
		}
	}
	for _, k := range all {
		if !k.Warn && len(out) < n {
			out = append(out, k)
		}
	}
	return out
}
