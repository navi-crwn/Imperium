package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/navi-crwn/imperium/internal/db"
)

func txt(s string) template.HTML { return template.HTML(esc(s)) }

func ownerCell(l string, r row) template.HTML {
	if r.OwnerSub() != "" {
		return hLink("/users/"+url.PathEscape(r.OwnerSub()), r.OwnerName())
	}
	return template.HTML(`<span class="muted">` + esc(tl(l, "table.guest")) + `</span>`)
}

func metricCell(r row) string {
	switch {
	case r.Clicks > 0:
		return humanInt(r.Clicks) + " clicks"
	case r.Views > 0:
		return humanInt(r.Views) + " views"
	case r.Size > 0:
		return humanBytes(r.Size)
	}
	return ""
}

// ---- Users (list) ----

func (a *App) usersPage(r *http.Request, rows []userRow, errMsg, next string, apps []db.App) listPage {
	l, s, q := a.langOf(r), current(r), r.URL.Query()
	admin := isAdmin(s)
	p := listPage{Title: tl(l, "users.title"), Path: "/users", Err: errMsg, Empty: tl(l, "users.none"), HasMenu: admin}
	if errMsg != "" {
		p.Err = tl(l, "err.hop") + ": " + errMsg
	}
	p.Filters = []uiFilter{
		{Kind: "search", Name: "q", Label: tl(l, "f.q"), Value: q.Get("q"), Class: "grow"},
		{Kind: "select", Name: "status", Label: tl(l, "f.status"), Value: q.Get("status"), Options: []string{"active", "suspended"}},
		{Kind: "select", Name: "role", Label: tl(l, "user.roles"), Value: q.Get("role"), Options: roleOptions(apps, nil)},
	}
	p.Cols = []uiCol{{Label: tl(l, "col.account")}, {Label: tl(l, "col.name"), Class: "w22"}, {Label: tl(l, "col.email"), Class: "w22"}, {Label: tl(l, "col.status")}, {Label: tl(l, "col.links")}, {Label: tl(l, "col.joined")}, {Label: tl(l, "col.last_login")}}
	for _, u := range rows {
		locked := u.Sub == s.Sub || contains(u.Roles, "owner")
		status := hChip(u.Status, u.Status)
		for _, role := range u.Roles {
			status += hChip(role, role)
		}
		email := esc(u.Email)
		if !u.Verified {
			email += " " + hChip("", tl(l, "chip.unverified"))
		}
		row := uiRow{PickLabel: tl(l, "bulk.pick_user") + " @" + u.Username, Cells: []uiCell{
			{template.HTML(`<a href="/users/` + esc(url.PathEscape(u.Sub)) + `">@` + esc(u.Username) + `</a> <span class="muted mono">#` + esc(u.Sub) + `</span>`), "nw"},
			{txt(u.Name), "cut"}, {template.HTML(email), "cut"}, {template.HTML(status), "nw"},
			{txt(fmt.Sprint(u.Links)), ""}, {datesz(l, u.CreatedAt), "small muted nw"}, {datesz(l, u.LastLoginAt), "small muted nw"}}}
		if admin && !locked {
			row.Pick = u.Sub
			uid := url.PathEscape(u.Sub)
			n, who := len(p.Rows), "@"+u.Username
			act := func(name string) uiAction {
				return uiAction{Label: actionLabel(l, name), Action: name, URL: "/users/" + uid + "/actions", ID: dlgID(n, "u", u.Sub, name), Target: who}
			}
			toggle := act("suspend")
			if u.Status == "suspended" {
				toggle = act("activate")
			}
			row.Actions = []uiAction{toggle, act("revoke_sessions"), act("revoke_tokens")}
		}
		p.Rows = append(p.Rows, row)
	}
	if admin && len(rows) > 0 {
		p.BulkURL = "/users/actions"
		p.BulkActions = a.bulkChoices(l, userBulkActions)
		p.BulkHidden = [][2]string{{"q", q.Get("q")}, {"status", q.Get("status")}, {"role", q.Get("role")}}
	}
	if q.Get("cursor") != "" {
		p.First = pageURL("/users", q, "cursor")
	}
	if next != "" {
		p.Next = cursorURL("/users", q, next)
	}
	return p
}

// ---- Content (list) ----

func (a *App) contentPage(r *http.Request, ap *db.App, m *Manifest, typ string, ct *ContentType, rows []row, errMsg, next string, total int, status string, statuses []string, can bool) listPage {
	l, q := a.langOf(r), r.URL.Query()
	base := "/apps/" + ap.Slug + "/content"
	title := typ
	if ct != nil {
		title = ct.Label
	}
	p := listPage{Title: title, Path: base, Err: errMsg, Empty: tl(l, "content.none"), Hidden: [][2]string{{"type", typ}}}
	if total > 0 {
		p.Count = humanInt(int64(total))
	}
	if len(m.ContentTypes) > 1 {
		for _, t := range m.ContentTypes {
			p.Subnav = append(p.Subnav, navItem{Label: t.Label, Href: base + "?type=" + url.QueryEscape(t.Type), On: t.Type == typ})
		}
	}
	p.Filters = []uiFilter{{Kind: "search", Name: "q", Label: tl(l, "f.q"), Value: q.Get("q"), Class: "grow"}}
	if ap.Slug != "pixelhop" || typ == "image" {
		f := uiFilter{Kind: "text", Name: "status", Label: tl(l, "f.status"), Value: status}
		if ap.Slug == "pixelhop" {
			f.Kind, f.Options = "select", statuses
		}
		p.Filters = append(p.Filters, f)
	}
	p.Filters = append(p.Filters, uiFilter{Kind: "text", Name: "owner", Label: tl(l, "table.owner"), Value: q.Get("owner")})
	p.Cols = []uiCol{{Label: tl(l, "table.item")}, {Label: tl(l, "table.owner")}, {Label: tl(l, "table.status")}, {Label: ""}, {Label: tl(l, "chart.date")}}
	bulk := bulkContentActions(ct)
	for _, it := range rows {
		name := it.Title
		if name == "" {
			name = it.ID
		}
		item := ""
		if it.Preview != "" {
			item = `<img class="thumb sm" src="` + esc(it.Preview) + `" alt=""> `
		}
		item += `<a href="` + esc(base+"/"+url.PathEscape(it.Type)+"/"+url.PathEscape(it.ID)) + `" title="` + esc(name) + `">` + esc(name) + `</a> <span class="muted small mono">` + esc(it.ID) + `</span>`
		status := hChip(it.Status, it.Status)
		for _, f := range it.Flags {
			status += hChip("", f)
		}
		row := uiRow{Pick: it.ID, PickLabel: tl(l, "bulk.pick_item") + " " + name, Cells: []uiCell{{template.HTML(item), "cut"}, {ownerCell(l, it), "nw"}, {template.HTML(status), "nw"}, {txt(metricCell(it)), "small nw"}, {txt(dayOf(it.Created)), "small muted nw"}}}
		if can && ct != nil {
			row.Actions = a.itemActions(l, ap.Slug, it.Type, it.ID, name, len(p.Rows), ct.Actions, it.Status, it.Flags, false)
		}
		p.Rows = append(p.Rows, row)
	}
	p.HasMenu = can && ct != nil
	if can && len(bulk) > 0 && len(rows) > 0 {
		p.BulkURL = base + "/actions"
		p.BulkActions = a.bulkChoices(l, bulk)
		p.BulkHidden = [][2]string{{"type", typ}, {"q", q.Get("q")}, {"status", status}, {"owner", q.Get("owner")}}
	} else {
		for i := range p.Rows {
			p.Rows[i].Pick = ""
		}
	}
	if q.Get("cursor") != "" {
		p.First = pageURL(base, q, "cursor")
	}
	if next != "" {
		p.Next = cursorURL(base, q, next)
	}
	return p
}

// ---- Reports (list) ----

func (a *App) reportsPage(r *http.Request, ap *db.App, m *Manifest, rows []map[string]any, errMsg, status string, can bool, choices []string) listPage {
	l := a.langOf(r)
	base := "/apps/" + ap.Slug + "/reports"
	p := listPage{Title: tl(l, "mod.reports"), Path: base, Err: errMsg, Empty: tl(l, "content.none"), HasMenu: can}
	for _, st := range [][2]string{{"open", "reports.open"}, {"closed", "reports.closed"}} {
		p.Subnav = append(p.Subnav, navItem{Label: tl(l, st[1]), Href: base + "?status=" + st[0], On: status == st[0]})
	}
	p.Cols = []uiCol{{Label: tl(l, "col.when")}, {Label: tl(l, "col.kind")}, {Label: tl(l, "col.subject")}, {Label: tl(l, "col.summary")}, {Label: tl(l, "col.status")}}
	for _, row := range rows {
		id, kind := str(row["id"]), row["kind"]
		if id == "" {
			id = fmt.Sprint(row["id"])
		}
		ks := str(kind)
		when := datesz(l, str(row["created_at"])+str(row["created"]))
		kindCell := hChip("", ks)
		if sc := row["score"]; sc != nil {
			kindCell += " " + hChip("locked", fmt.Sprint(sc))
		}
		subject := ""
		if sub, ok := row["subject"].(map[string]any); ok {
			subject = strings.TrimSpace(str(sub["type"]) + " " + fmt.Sprint(sub["id"]))
		}
		for _, k := range []string{"content", "image"} {
			if c, ok := row[k].(map[string]any); ok && str(c["url"]) != "" {
				subject = str(c["url"])
			}
		}
		summary := strings.TrimSpace(str(row["reason"]) + " " + str(row["summary"]) + " " + str(row["note"]))
		ur := uiRow{Pick: id + "|" + ks, PickLabel: tl(l, "bulk.pick") + " " + ks + " " + id, Cells: []uiCell{{when, "small nw"}, {template.HTML(kindCell), "nw"}, {txt(subject), "small cut"}, {txt(summary), "small cut"}, {template.HTML(hChip(str(row["status"]), str(row["status"]))), "nw"}}}
		if can {
			for _, act := range m.ReportActions(kind) {
				ur.Actions = append(ur.Actions, uiAction{Label: actionLabel(l, act), Action: act, URL: base + "/" + url.PathEscape(id) + "/actions", Fields: [][2]string{{"kind", ks}},
					ID: dlgID(len(p.Rows), "r", ks, id, act), Target: ks + " " + id, Danger: destructiveAction(act)})
			}
		}
		p.Rows = append(p.Rows, ur)
	}
	if can && len(rows) > 0 {
		p.BulkURL = base + "/actions"
		p.BulkActions = a.bulkChoices(l, choices)
		p.BulkHidden = [][2]string{{"status", status}}
	} else {
		for i := range p.Rows {
			p.Rows[i].Pick = ""
		}
	}
	return p
}

// ---- User (detail) ----

func (a *App) rolesForm(l, id string, options, held []string, admin bool) template.HTML {
	var b strings.Builder
	if admin {
		b.WriteString(`<form method="post" action="/users/` + esc(url.PathEscape(id)) + `/actions"><input type="hidden" name="action" value="set_roles">`)
	}
	b.WriteString(`<div class="role-list" role="group" aria-label="` + esc(tl(l, "user.roles")) + `">`)
	for _, o := range options {
		desc := tl(l, "role.app")
		if _, ok := map[string]bool{"owner": true, "admin": true, "moderator": true, "support": true}[o]; ok {
			desc = tl(l, "role."+o)
		}
		dis := ""
		if !admin {
			dis = " disabled"
		}
		chk := ""
		if contains(held, o) {
			chk = " checked"
		}
		b.WriteString(`<label class="check role"><input type="checkbox" name="roles" value="` + esc(o) + `"` + chk + dis + `><span><b>` + esc(o) + `</b><small class="muted">` + esc(desc) + `</small></span></label>`)
	}
	b.WriteString(`</div>`)
	if admin {
		b.WriteString(`<p class="muted small">` + esc(tl(l, "user.roles_hint")) + `</p><button class="btn" type="submit">` + esc(tl(l, "action.set_roles")) + `</button></form>`)
	}
	return template.HTML(b.String())
}

func kvLines(l string, rows [][2]string) []uiKV {
	var out []uiKV
	for _, r := range rows {
		if r[1] != "" {
			out = append(out, uiKV{r[0], txt(r[1])})
		}
	}
	return out
}

func appSummarySection(l, name string, u map[string]any, errText string, kind string) uiSection {
	sec := uiSection{Title: name}
	switch {
	case len(u) > 0:
		chip := func(v any) template.HTML { return template.HTML(hChip(str(v), str(v))) }
		kv := []uiKV{{tl(l, "col.status"), chip(u["status"])}}
		switch kind {
		case "pixel":
			kv = append(kv, uiKV{tl(l, "col.images"), txt(humanAny(u["images"]) + suffix(l, u["albums"], "unit.albums"))}, uiKV{tl(l, "col.storage"), txt(humanBytes(u["storage_bytes"]))})
		case "note":
			kv = append(kv, uiKV{tl(l, "col.notes"), txt(humanAny(u["notes"]) + suffix(l, u["notes_public"], "chip.public"))}, uiKV{tl(l, "user.vault"), txt(map[bool]string{true: tl(l, "chip.set"), false: tl(l, "chip.unset")}[u["vault_set"] == true])})
		default:
			kv = append(kv, uiKV{tl(l, "col.first_sign_in"), datesz(l, fmt.Sprint(u["created_at"]))}, uiKV{tl(l, "col.last_seen"), datesz(l, fmt.Sprint(u["last_seen_at"]))})
		}
		sec.KV = kv
	case errText != "":
		sec.HTML = template.HTML(`<p class="flash err small">` + esc(tl(l, "user.app_unreachable")+": "+errText) + `</p>`)
	default:
		sec.Empty = tl(l, "user.app_not_linked")
	}
	return sec
}

type userDetailIn struct {
	ID, Raw, Tab                       string
	U                                  user360
	Protected, Admin, Note             bool
	Options                            []string
	Pixel, NoteU, File, Pay            map[string]any
	PixelErr, NoteErr, FileErr, PayErr string
}

func (a *App) userDetailPage(r *http.Request, in userDetailIn) detailPage {
	l := a.langOf(r)
	u, p := in.U, in.U.Profile
	uid := url.PathEscape(in.ID)
	d := detailPage{Title: "@" + str(p["username"]), Back: "/users", BackLabel: tl(l, "back")}
	if n := str(p["name"]); n != "" {
		d.Chips = append(d.Chips, uiChip{n, "plain"})
	}
	d.Chips = append(d.Chips, uiChip{str(p["status"]), str(p["status"])})
	for _, role := range u.Roles {
		d.Chips = append(d.Chips, uiChip{role, role})
	}
	base := "/users/" + uid
	for _, t := range [][2]string{{"profile", "tab.profile"}, {"content", "tab.content"}, {"sessions", "tab.sessions"}, {"activity", "tab.activity"}} {
		d.Tabs = append(d.Tabs, navItem{Label: tl(l, t[1]), Href: base + "?tab=" + t[0], On: in.Tab == t[0]})
	}
	if in.Admin && !in.Protected {
		who := "@" + str(p["username"])
		act := func(name string) uiAction {
			return uiAction{Label: actionLabel(l, name), Action: name, URL: base + "/actions", ID: dlgID(0, "u", uid, name), Target: who}
		}
		toggle := act("suspend")
		if str(p["status"]) == "suspended" {
			toggle = act("activate")
		}
		d.Actions = []uiAction{toggle, act("revoke_sessions"), act("revoke_tokens"),
			{Label: tl(l, "action.delete") + " " + "@" + str(p["username"]), Action: "delete", URL: base + "/actions", ID: dlgID(0, "u", uid, "delete"), Danger: true, Typed: "@" + str(p["username"]), Target: "@" + str(p["username"])}}
		d.Actions[3].Label = tl(l, "action.delete")
	}
	switch in.Tab {
	case "content":
		var kv []uiKV
		for _, k := range sortedKeys(u.Counts) {
			kv = append(kv, uiKV{keyLabel(l, k), txt(humanInt(u.Counts[k]))})
		}
		d.Sections = append(d.Sections, uiSection{KV: kv, Empty: map[bool]string{true: tl(l, "none")}[len(kv) == 0]})
		d.Sections = append(d.Sections, appSummarySection(l, "PixelHop", in.Pixel, in.PixelErr, "pixel"), appSummarySection(l, "NoteHop", in.NoteU, in.NoteErr, "note"),
			appSummarySection(l, "FileHop", in.File, in.FileErr, "file"), appSummarySection(l, "PayHop", in.Pay, in.PayErr, "file"))
		t := &uiTable{}
		for _, row := range u.RecentLinks {
			t.Rows = append(t.Rows, []template.HTML{txt(str(row["slug"])), txt(str(row["target_url"])), template.HTML(hChip(str(row["status"]), str(row["status"]))), txt(fmt.Sprint(row["clicks"]))})
		}
		d.Sections = append(d.Sections, uiSection{Title: tl(l, "user.links"), Table: t, Empty: map[bool]string{true: tl(l, "none")}[len(t.Rows) == 0]})
	case "sessions":
		var kv []uiKV
		for _, k := range sortedAnyKeys(u.Sessions) {
			kv = append(kv, uiKV{keyLabel(l, k), txt(fmt.Sprint(u.Sessions[k]))})
		}
		d.Sections = append(d.Sections, uiSection{Title: tl(l, "col.sessions"), KV: kv})
		t := &uiTable{Cols: []string{tl(l, "col.when"), tl(l, "col.ip"), tl(l, "col.detail")}}
		for _, row := range u.Logins {
			dev := strings.Join(nonEmpty(strings.TrimSpace(str(row["country"])+" "+str(row["city"])), str(row["device"]), str(row["browser"]), str(row["provider"])), " · ")
			if row["is_proxy"] == true {
				dev += " · " + tl(l, "chip.proxy")
			}
			t.Rows = append(t.Rows, []template.HTML{datesz(l, fmt.Sprint(row["at"])), txt(str(row["ip"])), txt(dev)})
		}
		d.Sections = append(d.Sections, uiSection{Title: tl(l, "user.logins"), Table: t, Empty: map[bool]string{true: tl(l, "none")}[len(t.Rows) == 0]})
	case "activity":
		t := &uiTable{Cols: []string{tl(l, "col.when"), tl(l, "col.action"), tl(l, "col.detail")}}
		for _, row := range u.Activity {
			t.Rows = append(t.Rows, []template.HTML{datesz(l, fmt.Sprint(row["at"])), txt(strings.TrimSpace(str(row["action"]) + " " + str(row["model"]))), txt(str(row["description"]))})
		}
		d.Sections = append(d.Sections, uiSection{Title: tl(l, "user.activity"), Table: t, Empty: map[bool]string{true: tl(l, "none")}[len(t.Rows) == 0]})
		var nb strings.Builder
		if len(u.Notes) > 0 {
			nb.WriteString(`<ul class="notes small">`)
			for _, n := range u.Notes {
				nb.WriteString(`<li><span class="muted">` + string(datesz(l, fmt.Sprint(n["created_at"]))) + ` · ` + esc(str(n["author"])) + `:</span> ` + esc(str(n["text"])) + `</li>`)
			}
			nb.WriteString(`</ul>`)
		}
		if in.Note && !in.Protected {
			nb.WriteString(`<form class="act-form" method="post" action="` + esc(base) + `/actions"><label class="sr" for="f-user-text-1">` + esc(tl(l, "user.notes")) + `</label><input id="f-user-text-1" type="text" name="text" placeholder="` + esc(tl(l, "user.notes")) + `" required><input type="hidden" name="action" value="note"><button class="btn" type="submit">` + esc(tl(l, "action.note")) + `</button></form>`)
		}
		d.Sections = append(d.Sections, uiSection{Title: tl(l, "user.notes"), HTML: template.HTML(nb.String())})
		if in.Raw != "" {
			d.Sections = append(d.Sections, uiSection{HTML: template.HTML(`<details><summary>` + esc(tl(l, "raw.json")) + `</summary><pre class="mono">` + esc(in.Raw) + `</pre></details>`)})
		}
	default:
		email := esc(str(p["email"]))
		if p["verified"] == true {
			email += " " + hChip("active", tl(l, "chip.verified"))
		} else {
			email += " " + hChip("", tl(l, "chip.unverified"))
		}
		if p["mfa"] == true {
			email += " " + hChip("admin", tl(l, "login.mfa_chip"))
		}
		kv := []uiKV{{tl(l, "login.hop_id"), txt(str(p["sub"]) + fmt.Sprint(map[bool]string{true: in.ID}[str(p["sub"]) == ""]))}, {tl(l, "col.email"), template.HTML(email)}}
		kv = append(kv, kvLines(l, [][2]string{{tl(l, "col.bio"), str(p["bio"])}, {tl(l, "user.website"), str(p["website"])}, {tl(l, "user.location"), str(p["location"])}})...)
		if p["created_at"] != nil {
			kv = append(kv, uiKV{tl(l, "col.joined"), datesz(l, fmt.Sprint(p["created_at"]))})
		}
		d.Sections = append(d.Sections, uiSection{KV: kv})
		if in.Protected {
			d.Sections = append(d.Sections, uiSection{HTML: template.HTML(`<p role="status">` + esc(tl(l, "user.superadmin_locked")) + `</p>`)})
		}
		d.Sections = append(d.Sections, uiSection{Title: tl(l, "user.roles"), HTML: a.rolesForm(l, in.ID, in.Options, u.Roles, in.Admin && !in.Protected)})
	}
	return d
}

func nonEmpty(v ...string) []string {
	var out []string
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func sortedKeys(m map[string]int64) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	sortStrings(k)
	return k
}

func sortedAnyKeys(m map[string]any) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	sortStrings(k)
	return k
}

// ---- Content item (detail) ----

func (a *App) itemDetailPage(r *http.Request, ap *db.App, m *Manifest, typ, id string, it row, raw string, can bool) detailPage {
	l := a.langOf(r)
	name := it.Title
	if name == "" {
		name = typ + " " + id
	}
	d := detailPage{Title: name, Back: "/apps/" + ap.Slug + "/content?type=" + url.QueryEscape(typ), BackLabel: tl(l, "back"), Link: it.URL}
	d.Chips = append(d.Chips, uiChip{it.Status, it.Status})
	for _, f := range it.Flags {
		d.Chips = append(d.Chips, uiChip{f, ""})
	}
	if ct := m.ContentType(typ); can && ct != nil {
		d.Actions = a.itemActions(l, ap.Slug, typ, id, name, 0, ct.Actions, it.Status, it.Flags, true)
	}
	kv := []uiKV{{tl(l, "col.item_id"), template.HTML(`<span class="mono">` + esc(it.ID) + `</span>`)}}
	if it.URL != "" {
		e := uiKV{tl(l, "col.url"), txt(it.URL)}
		// Only an http(s) address becomes a link; any other scheme (javascript:, data:) stays plain escaped text.
		if strings.HasPrefix(it.URL, "http://") || strings.HasPrefix(it.URL, "https://") {
			e.V = template.HTML(`<a href="` + esc(it.URL) + `" target="_blank" rel="noopener">` + esc(it.URL) + `</a>`)
		}
		kv = append(kv, e)
	}
	kv = append(kv, uiKV{tl(l, "col.owner"), ownerCell(l, it)}, uiKV{tl(l, "col.created"), datesz(l, it.Created)})
	for _, k := range sortedAnyKeys(it.Extra) {
		kv = append(kv, uiKV{keyLabel(l, k), txt(jsonCompact(it.Extra[k]))})
	}
	sec := uiSection{KV: kv}
	if it.Preview != "" {
		sec.HTML = template.HTML(`<p><img class="preview" src="` + esc(it.Preview) + `" alt=""></p>`)
	}
	d.Sections = append(d.Sections, sec)
	if raw != "" {
		d.Sections = append(d.Sections, uiSection{HTML: template.HTML(`<details><summary>` + esc(tl(l, "raw.json")) + `</summary><pre class="mono">` + esc(raw) + `</pre></details>`)})
	}
	return d
}

func reQuote(s string) string { return "^" + regexp.QuoteMeta(s) + "$" }

func jsonCompact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.Trim(string(b), `"`)
}

// suffix is " (12 albums)": the count and its unit in parentheses, or nothing when the count is empty.
func suffix(l string, v any, unitKey string) string {
	if s := humanAny(v); s != "" && s != "0" {
		return " (" + s + " " + tl(l, unitKey) + ")"
	}
	return ""
}
