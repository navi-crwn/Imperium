package app

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/navi-crwn/imperium/internal/db"
)

// The admin side of the notification inbox. Who may do what:
//   - owner, admin, moderator: see and change every app's reports;
//   - support: see every app's reports, change none;
//   - admin:<app>: see and change that app's reports only.

// notifScope is what the session may see.
func notifScope(s *db.Session) db.NotifScope {
	if s == nil {
		return db.NotifScope{}
	}
	if isAdmin(s) || contains(s.Roles, "moderator") || contains(s.Roles, "support") {
		return db.NotifScope{All: true}
	}
	var apps []string
	for _, r := range s.Roles {
		if slug, ok := strings.CutPrefix(r, "admin:"); ok && slug != "" {
			apps = append(apps, slug)
		}
	}
	return db.NotifScope{Apps: apps}
}

func canSeeNotif(s *db.Session, app string) bool {
	sc := notifScope(s)
	return sc.All || contains(sc.Apps, app)
}

func canActNotif(s *db.Session, app string) bool {
	return s != nil && (isAdmin(s) || contains(s.Roles, "moderator") || contains(s.Roles, "admin:"+app))
}

// newErrors counts the "new" groups the session may see (nav badge, home page).
func (a *App) newErrors(r *http.Request) int {
	s := current(r)
	if s == nil {
		return 0
	}
	n, err := a.db.CountNew(r.Context(), notifScope(s))
	if err != nil {
		log.Printf("notifications: count: %v", err)
	}
	return n
}

// fail renders an error page with a real status code.
func (a *App) fail(w http.ResponseWriter, r *http.Request, status int, msg, back string) {
	a.renderStatus(w, r, status, "error.html", a.T("notif.title"), map[string]any{"Err": msg, "Back": back})
}

const notifPageSize = 50

var notifStatuses = []string{db.NotifNew, db.NotifSeen, db.NotifResolved, db.NotifMuted}

// GET /notifications?app=&status=&p=
func (a *App) notifications(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	scope := notifScope(s)
	q := r.URL.Query()
	status := q.Get("status")
	if status != "all" && !contains(notifStatuses, status) {
		status = db.NotifNew
	}
	apps, err := a.db.NotificationApps(r.Context(), scope)
	if err != nil {
		log.Printf("notifications: apps: %v", err)
	}
	app := q.Get("app")
	if !contains(apps, app) { // only an app that has reports the viewer may see
		app = ""
	}
	page, _ := strconv.Atoi(q.Get("p"))
	if page < 1 {
		page = 1
	}
	rows, total, err := a.db.ListNotifications(r.Context(), scope, db.NotifFilter{App: app, Status: status, Limit: notifPageSize, Offset: (page - 1) * notifPageSize})
	if err != nil {
		log.Printf("notifications: list: %v", err)
		a.fail(w, r, http.StatusInternalServerError, a.T("notif.err.load"), "/")
		return
	}
	counts, err := a.db.NotificationStatusCounts(r.Context(), scope, app)
	if err != nil {
		log.Printf("notifications: counts: %v", err)
	}
	all := 0
	for _, n := range counts {
		all += n
	}
	href := func(p int) string {
		v := url.Values{"status": {status}}
		if app != "" {
			v.Set("app", app)
		}
		if p > 1 {
			v.Set("p", strconv.Itoa(p))
		}
		return "/notifications?" + v.Encode()
	}
	chipHref := func(st string) string {
		v := url.Values{"status": {st}}
		if app != "" {
			v.Set("app", app)
		}
		return "/notifications?" + v.Encode()
	}
	type chip struct {
		Status, Href string
		Count        int
		On           bool
	}
	chips := []chip{}
	for _, st := range notifStatuses {
		chips = append(chips, chip{st, chipHref(st), counts[st], st == status})
	}
	chips = append(chips, chip{"all", chipHref("all"), all, status == "all"})
	d := map[string]any{"Rows": rows, "Total": total, "Status": status, "App": app, "Apps": apps, "Chips": chips, "Page": page}
	if page > 1 {
		d["PrevHref"] = href(page - 1)
	}
	if page*notifPageSize < total {
		d["NextHref"] = href(page + 1)
	}
	a.render(w, r, "notifications.html", a.T("notif.title"), d)
}

func notifID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// GET /notifications/{id}
func (a *App) notification(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	id, ok := notifID(r)
	if !ok {
		a.fail(w, r, http.StatusNotFound, a.T("notif.err.notfound"), "/notifications")
		return
	}
	n, err := a.db.Notification(r.Context(), id)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		log.Printf("notifications: load %d: %v", id, err)
		a.fail(w, r, http.StatusInternalServerError, a.T("notif.err.load"), "/notifications")
		return
	}
	if err != nil || !canSeeNotif(s, n.App) { // a report outside the role's apps looks like a missing one
		a.fail(w, r, http.StatusNotFound, a.T("notif.err.notfound"), "/notifications")
		return
	}
	events, err := a.db.NotificationEvents(r.Context(), id)
	if err != nil {
		log.Printf("notifications: events %d: %v", id, err)
		a.fail(w, r, http.StatusInternalServerError, a.T("notif.err.load"), "/notifications")
		return
	}
	var actions []string
	if canActNotif(s, n.App) {
		for _, act := range db.NotificationActions() {
			if db.CanTransition(n.Status, act) {
				actions = append(actions, act)
			}
		}
	}
	d := map[string]any{"N": n, "Events": events, "Actions": actions, "CanAct": canActNotif(s, n.App)}
	title := n.Title
	if dg := n.Sample.Digest; dg != nil {
		title = a.T("notif.digest.title")
		d["Digest"] = dg
		d["DigestRows"], d["OtherSpikes"] = digestRows(dg)
	}
	a.render(w, r, "notification.html", title, d)
}

// digestRow is one line of the digest's domain table.
type digestRow struct {
	Domain string
	Count  int64
	Spike  bool
}

// digestRows marks the spiking domains in the top list and returns the spikes
// that are not in it (a domain can spike without being among the top ten).
func digestRows(dg *db.Digest) (rows []digestRow, other []string) {
	for _, dc := range dg.TopDomains {
		rows = append(rows, digestRow{Domain: dc.Domain, Count: dc.Count, Spike: contains(dg.Spikes, dc.Domain)})
	}
	for _, s := range dg.Spikes {
		if !contains(rows2domains(rows), s) {
			other = append(other, s)
		}
	}
	return rows, other
}

func rows2domains(rows []digestRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Domain
	}
	return out
}

// POST /notifications/{id}/actions  (form: action = ack|resolve|mute|reopen, reason)
func (a *App) notificationAction(w http.ResponseWriter, r *http.Request) {
	s := current(r)
	id, ok := notifID(r)
	if !ok {
		a.fail(w, r, http.StatusNotFound, a.T("notif.err.notfound"), "/notifications")
		return
	}
	n, err := a.db.Notification(r.Context(), id)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		log.Printf("notifications: load %d: %v", id, err)
		a.fail(w, r, http.StatusInternalServerError, a.T("notif.err.load"), "/notifications")
		return
	}
	if err != nil || !canSeeNotif(s, n.App) {
		a.fail(w, r, http.StatusNotFound, a.T("notif.err.notfound"), "/notifications")
		return
	}
	back := "/notifications/" + strconv.FormatInt(id, 10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	action := r.FormValue("action")
	reason := strings.TrimSpace(r.FormValue("reason"))
	switch {
	case !contains(db.NotificationActions(), action):
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	case !canActNotif(s, n.App):
		a.flashError(w, a.T("notif.err.forbidden"))
	case utf8.RuneCountInString(reason) > 200 || (action == "mute" && reason == ""):
		a.flashError(w, a.T("notif.err.reason"))
	default:
		_, err := a.db.ChangeNotification(r.Context(), id, action, reason, db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r)})
		switch {
		case errors.Is(err, db.ErrBadTransition):
			a.flashError(w, a.T("notif.err.transition"))
		case err != nil:
			log.Printf("notifications: %s %d: %v", action, id, err)
			a.flashError(w, a.T("notif.err.save"))
		default:
			a.flash(w, a.T("notif.done"))
		}
	}
	http.Redirect(w, r, back, http.StatusFound)
}
