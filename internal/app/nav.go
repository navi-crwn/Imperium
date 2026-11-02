package app

import (
	"context"
	"net/http"
	"net/url"
	"sort"
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
	Pages []navItem
	Apps  []navApp
}

// extLabel names an extension in the sidebar and page titles.
func (a *App) extLabel(ext string) string {
	if v := a.T("ext." + ext); v != "ext."+ext {
		return v
	}
	return ext
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
func (a *App) appItems(slug string, m *Manifest, path, typ string) []navItem {
	base := "/apps/" + slug
	items := []navItem{{Label: a.T("dash.title"), Href: base, Kind: "dash", On: path == base}}
	if slug == "hub" {
		items = append(items, navItem{Label: a.T("nav.users"), Href: "/users", Kind: "users", On: path == "/users" || strings.HasPrefix(path, "/users/")})
	}
	if m == nil {
		return items
	}
	for i, ct := range m.ContentTypes {
		on := strings.HasPrefix(path, base+"/content") && (typ == ct.Type || (typ == "" && i == 0))
		items = append(items, navItem{Label: ct.Label, Href: base + "/content?type=" + url.QueryEscape(ct.Type), Kind: "content", On: on})
	}
	if m.HasReports() {
		items = append(items, navItem{Label: a.T("mod.reports"), Href: base + "/reports", Kind: "reports", On: strings.HasPrefix(path, base+"/reports")})
	}
	if m.HasSettings() {
		items = append(items, navItem{Label: a.T("mod.settings"), Href: base + "/settings", Kind: "settings", On: path == base+"/settings"})
	}
	for _, ext := range m.Extensions {
		items = append(items, navItem{Label: a.extLabel(ext), Href: base + "/x/" + url.PathEscape(ext), Kind: "ext", On: path == base+"/x/"+ext})
	}
	return items
}

// nav builds the sidebar for the current request.
func (a *App) nav(r *http.Request) navData {
	path := r.URL.Path
	typ := r.URL.Query().Get("type")
	n := navData{Pages: []navItem{
		{Label: a.T("nav.home"), Href: "/", Kind: "home", On: path == "/"},
		{Label: a.T("nav.users"), Href: "/users", Kind: "users", On: path == "/users" || strings.HasPrefix(path, "/users/")},
		{Label: a.T("nav.apps"), Href: "/apps", Kind: "apps", On: path == "/apps"},
		{Label: a.T("nav.notifications"), Href: "/notifications", Kind: "notifications", On: strings.HasPrefix(path, "/notifications"), Badge: a.newErrors(r)},
		{Label: a.T("nav.audit"), Href: "/audit", Kind: "audit", On: path == "/audit"},
	}}
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
			na.Items = a.appItems(ap.Slug, a.cachedManifest(ap.Slug), path, typ)
		} else {
			na.Href = "/apps"
		}
		if ap.Slug == "linkhop" {
			na.Name = "LinkHop"
		}
		n.Apps = append(n.Apps, na)
	}
	lh, _ := a.source(r.Context(), "hub")
	add(*lh, "")
	apps, _ := a.db.Apps(r.Context())
	for _, ap := range apps {
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
	"linkhop":  {"users", "users_new_7d", "users_suspended", "links", "links_live", "bio_pages", "clicks_today", "clicks_30d", "open_reports", "open_flags"},
	"biohop":   {"pages", "pages_published", "pages_draft", "pages_adult", "blocks", "views_total", "clicks_total", "reports_open"},
	"pixelhop": {"users", "content", "images", "videos", "pdfs", "albums", "bytes", "views", "uploads_today", "open_reports", "bans", "guest_uploads"},
	"notehop":  {"users", "users_sso", "notes", "notes_encrypted", "notes_public", "notes_owned", "notes_burn", "files", "files_bytes", "files_budget_bytes", "live_rooms", "slug_holds", "reports_open", "takedowns"},
}

// attentionKeys are KPIs that mean work when they are above zero.
var attentionKeys = []string{"open_reports", "open_flags", "reports_open", "takedowns", "users_suspended", "failed"}

// dashExtensions are the extensions whose data belongs on the dashboard itself
// (the old control panels showed the queue worker and the realtime rooms there).
var dashExtensions = map[string]string{"linkhop": "queue", "notehop": "realtime"}

func isBytesKey(k string) bool { return k == "bytes" || strings.HasSuffix(k, "_bytes") }

// kpiLabel translates a KPI key, falling back to the key spelled out.
func (a *App) kpiLabel(key string) string {
	if v := a.T("kpi." + key); v != "kpi."+key {
		return v
	}
	return strings.ToUpper(key[:1]) + strings.ReplaceAll(key[1:], "_", " ")
}

func (a *App) seriesLabel(key string) string {
	if v := a.T("series." + key); v != "series."+key {
		return v
	}
	return strings.ToUpper(key[:1]) + strings.ReplaceAll(key[1:], "_", " ")
}

// kpis orders and formats the KPI map of one app.
func (a *App) kpis(slug string, raw map[string]int64) (all, attention []kpi) {
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
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	keys = append(keys, rest...)
	for _, k := range keys {
		v := raw[k]
		item := kpi{Key: k, Label: a.kpiLabel(k), Value: humanInt(v)}
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

func (a *App) seriesViews(raw map[string][][2]any) []seriesView {
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
		sv := seriesView{Key: k, Label: a.seriesLabel(k), Points: pts, Bytes: isBytesKey(k)}
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
		ops        any
		opsErr     string
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
	ext := dashExtensions[ap.Slug]
	if ext != "" && !contains(m.Extensions, ext) {
		ext = ""
	}
	if ext != "" {
		run(func() {
			if err := a.call(ctx, s, ap, "GET", "/x/"+ext, nil, &ops); err != nil {
				opsErr = err.Error()
			}
		})
	}
	wg.Wait()
	kpiMap := st.KPI
	if len(kpiMap) == 0 {
		kpiMap = st.Totals
	}
	all, attention := a.kpis(ap.Slug, kpiMap)
	a.render(w, r, "dashboard.html", ap.Name, map[string]any{
		"App": ap, "M": m, "Health": health, "Items": a.appItems(ap.Slug, m, r.URL.Path, ""),
		"KPIs": all, "Attention": attention, "Series": a.seriesViews(st.Series), "StatsErr": statsErr,
		"CT": firstCT, "Latest": latest, "LatestErr": latestErr,
		"Reports": reports, "ReportsErr": reportsErr, "HasReports": m.HasReports(),
		"Ext": ext, "ExtLabel": a.extLabel(ext), "Ops": kvRows(ops), "OpsErr": opsErr,
		"CanAct": canAct(s),
	})
}
