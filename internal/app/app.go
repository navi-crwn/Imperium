// Package app is the Imperium web application: sign-in through HOP ID, the
// combined home page, users, the app registry and the audit trail.
package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

type App struct {
	cfg    Config
	db     *db.DB
	tpls   map[string]*template.Template // one parsed set per catalog language
	static fs.FS
	hop    *hopproto.Client
	jti    hopproto.Nonces
	mu     sync.Mutex
	health map[string]Health // last health per app slug
	start  time.Time

	manifests manifestCache
	inbox     *notifyInbox

	checkMu sync.Mutex
	checked map[string]time.Time // session token hash -> when HOP ID is asked about it next
}

// New wires the app. templates and static come from the embedded web/ folder.
func New(cfg Config, d *db.DB, web embed.FS) (*App, error) {
	a := &App{cfg: cfg, db: d, health: map[string]Health{}, start: time.Now(), manifests: manifestCache{m: map[string]*Manifest{}}, inbox: newNotifyInbox(), checked: map[string]time.Time{}}
	a.hop = &hopproto.Client{As: cfg.HopClient, HTTP: a.httpClient()}
	a.tpls = map[string]*template.Template{}
	for _, l := range langOrder {
		tpl, err := template.New("").Funcs(a.funcs(l)).ParseFS(web, "web/templates/*.html")
		if err != nil {
			return nil, err
		}
		a.tpls[l] = tpl
	}
	a.static, _ = fs.Sub(web, "web/static")
	return a, nil
}

// appMark is the embedded mark of a family app; Imperium's own for anything else.
func appMark(slug string) string {
	switch slug {
	case "hub":
		return "/static/hop/apps/hub/rabbit-mark.png"
	case "linkhop", "biohop", "pixelhop", "notehop", "filehop":
		return "/static/hop/apps/" + slug + "/mark.svg"
	}
	return "/static/imperium.svg"
}

// funcs are the template helpers bound to one language; New parses one template set per language.
func (a *App) funcs(l string) template.FuncMap {
	return template.FuncMap{
		"t":             func(k string) string { return tl(l, k) },
		"partOf":        partOf,
		"lang":          func() string { return l },
		"langOptions":   langOptions,
		"since":         func(t time.Time) string { return since(l, t) },
		"date":          func(t time.Time) string { return t.Local().Format("2006-01-02 15:04") },
		"datez":         func(t time.Time) template.HTML { return dateWithZone(l, t) },
		"datesz":        func(v string) template.HTML { return datesz(l, v) },
		"versionLabel":  func(v string) string { return versionLabel(l, v) },
		"pageURL":       pageURL,
		"cursorURL":     cursorURL,
		"utcz":          func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
		"n":             humanAny,
		"has":           func(list []string, x string) bool { return contains(list, x) },
		"json":          jsonPretty,
		"lower":         strings.ToLower,
		"list":          func(items ...string) []string { return items },
		"bytes":         humanBytes,
		"day":           dayOf,
		"utc":           func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") },
		"ext":           func(e string) string { return extLabel(l, e) },
		"actionLabel":   func(x string) string { return actionLabel(l, x) },
		"confirmAction": confirmAction,
		"appMark":       appMark,
	}
}

func (a *App) httpClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if addr := a.cfg.HopAPIAddr; addr != "" {
		// Reach HOP ID on this machine without the CDN round trip; the TLS name stays HOP ID's.
		d := &net.Dialer{Timeout: 5 * time.Second}
		base := t.DialContext
		hopHost := strings.TrimPrefix(strings.TrimPrefix(a.cfg.HopURL, "https://"), "http://")
		t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			if host, _, _ := net.SplitHostPort(address); host == hopHost {
				return d.DialContext(ctx, network, addr)
			}
			return base(ctx, network, address)
		}
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: t}
}

// Handler builds the router with its middleware.
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	m.Handle("GET /static/", http.StripPrefix("/static/", cacheStatic(http.FileServerFS(a.static))))
	m.HandleFunc("GET /healthz", a.healthz)
	m.HandleFunc("GET /login", a.loginPage)
	m.HandleFunc("GET /auth/sso", a.ssoStart)
	m.HandleFunc("GET /auth/sso/callback", a.ssoCallback)
	m.HandleFunc("POST /logout", a.logout)
	m.HandleFunc("POST /prefs", a.prefs)
	m.HandleFunc("GET /{$}", a.member(a.home))
	m.HandleFunc("GET /search", a.member(a.search))
	m.HandleFunc("GET /users", a.member(a.users))
	m.HandleFunc("GET /users/{id}", a.member(a.user))
	m.HandleFunc("POST /users/{id}/actions", a.member(a.userAction))
	m.HandleFunc("POST /users/actions", a.member(a.usersBulkAction))
	m.HandleFunc("GET /apps", a.member(a.apps))
	m.HandleFunc("POST /apps/sync", a.member(a.appsSync))
	m.HandleFunc("GET /apps/{app}", a.member(a.dashboard))
	m.HandleFunc("GET /apps/{app}/content", a.member(a.content))
	m.HandleFunc("POST /apps/{app}/content/actions", a.member(a.contentBulkAction))
	m.HandleFunc("GET /apps/{app}/content/{type}/{id}", a.member(a.item))
	m.HandleFunc("POST /apps/{app}/content/{type}/{id}/actions", a.member(a.itemAction))
	m.HandleFunc("GET /apps/{app}/reports", a.member(a.reports))
	m.HandleFunc("POST /apps/{app}/reports/{id}/actions", a.member(a.reportAction))
	m.HandleFunc("POST /apps/{app}/reports/actions", a.member(a.reportsBulkAction))
	m.HandleFunc("GET /apps/{app}/settings", a.member(a.settings))
	m.HandleFunc("POST /apps/{app}/settings", a.member(a.saveSettings))
	m.HandleFunc("GET /apps/{app}/x/{ext}", a.member(a.extension))
	m.HandleFunc("POST /apps/{app}/x/{ext}/actions", a.member(a.extensionAction))
	m.HandleFunc("POST /hop/notify", a.hopNotify) // signed by the sending app, not a browser session
	m.HandleFunc("GET /notifications", a.member(a.notifications))
	m.HandleFunc("GET /notifications/{id}", a.member(a.notification))
	m.HandleFunc("POST /notifications/{id}/actions", a.member(a.notificationAction))
	m.HandleFunc("GET /audit", a.member(a.audit))
	m.HandleFunc("GET /me", a.member(a.me))

	cop := http.NewCrossOriginProtection()
	cop.AddInsecureBypassPattern("POST /hop/notify") // server to server; the HMAC is the authentication
	var h http.Handler = m
	h = cop.Handler(h)
	h = a.withSession(h)
	h = a.securityHeaders(h)
	h = a.withIP(h)
	h = a.recoverer(h)
	return h
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		next.ServeHTTP(w, r)
	})
}

// ---- middleware ----

type ctxKey int

const (
	ctxSession ctxKey = iota
	ctxIP
)

func (a *App) withIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if a.cfg.TrustProxy {
			// The trusted door (nginx) overwrites X-Forwarded-For with the
			// address it saw, so the last entry is the connecting one; any
			// earlier entry was typed by the client.
			if xf := strings.Join(r.Header.Values("X-Forwarded-For"), ","); xf != "" {
				parts := strings.Split(xf, ",")
				if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
					ip = last
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxIP, ip)))
	})
}

func clientIP(r *http.Request) string { ip, _ := r.Context().Value(ctxIP).(string); return ip }

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store") // static files override this in cacheStatic
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'; form-action 'self' "+a.cfg.HopURL+"; base-uri 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Robots-Tag", "noindex, nofollow")
		if a.cfg.Secure {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("panic %s %s: %v", r.Method, r.URL.Path, e)
				http.Error(w, "internal error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

const sessionCookie = "imp_session"

func hashToken(t string) string { s := sha256.Sum256([]byte(t)); return hex.EncodeToString(s[:]) }

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *App) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if s, err := a.db.Session(r.Context(), hashToken(c.Value)); err == nil {
				if !strings.HasPrefix(r.URL.Path, "/static/") && !a.hubStillTrusts(r.Context(), s) {
					// Suspended, deleted or demoted at HOP ID since sign-in: the cookie is worth nothing now.
					a.db.DeleteSession(r.Context(), s.TokenHash)
					a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), Action: "session.revoked", Detail: map[string]any{"reason": "hub"}})
					http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
				} else {
					if time.Since(s.LastSeen) > time.Minute {
						a.db.TouchSession(r.Context(), s.TokenHash)
					}
					r = r.WithContext(context.WithValue(r.Context(), ctxSession, s))
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func current(r *http.Request) *db.Session {
	s, _ := r.Context().Value(ctxSession).(*db.Session)
	return s
}

// member requires a signed-in admin. Role checks per action live in can().
func (a *App) member(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if current(r) == nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(safeNext(r.URL.RequestURI())), http.StatusFound)
			return
		}
		h(w, r)
	}
}

func safeNext(next string) string {
	// Browsers drop tab, CR and LF inside a URL, so "/<TAB>/host" would be read as "//host".
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsFunc(next, func(r rune) bool {
		return r < ' ' || r == 0x7f || r == '\\'
	}) {
		return "/"
	}
	return next
}

// Roles on Imperium: owner and admin may do everything, moderator may act on
// content and read users, support may only read.
func isAdmin(s *db.Session) bool {
	return contains(s.Roles, "owner") || contains(s.Roles, "admin")
}

// globalRole is one of the hub-wide roles. Only these may see HOP ID's user
// accounts and audit log; admin:<app> is scoped to its own app.
func globalRole(s *db.Session) bool {
	return s != nil && (isAdmin(s) || contains(s.Roles, "moderator") || contains(s.Roles, "support"))
}

// canSee says whether the session may open an app's pages. A hub-wide role sees every app;
// admin:<app> sees that app and no other.
func canSee(s *db.Session, app string) bool {
	return globalRole(s) || (s != nil && contains(s.Roles, "admin:"+app))
}

// visibleApps keeps the apps the session may open.
func visibleApps(s *db.Session, apps []db.App) []db.App {
	var out []db.App
	for _, ap := range apps {
		if canSee(s, ap.Slug) {
			out = append(out, ap)
		}
	}
	return out
}

// forbidden answers 403 with the standard page.
func (a *App) forbidden(w http.ResponseWriter, r *http.Request) {
	a.renderStatus(w, r, http.StatusForbidden, "error.html", a.Tr(r, "err.forbidden"), map[string]any{"Err": a.Tr(r, "err.forbidden"), "Back": "/"})
}

// requireGlobal answers 403 and returns false for sessions without a hub-wide role.
func (a *App) requireGlobal(w http.ResponseWriter, r *http.Request) bool {
	if globalRole(current(r)) {
		return true
	}
	a.forbidden(w, r)
	return false
}

func allowedIn(roles []string) bool {
	for _, r := range roles {
		switch r {
		case "owner", "admin", "moderator", "support":
			return true
		}
		if strings.HasPrefix(r, "admin:") {
			return true
		}
	}
	return false
}

func contains(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// ---- rendering ----

type page struct {
	Title   string
	Session *db.Session
	Flash   string
	Error   string
	Path    string
	Query   string
	Version string
	Nav     navData
	Theme   string // "light" or "dark" when the reader chose one (cookie hop_theme)
	UI      string // "clean" when the reader chose the optional Clean style (cookie hop_ui)
	Data    any
}

func (a *App) render(w http.ResponseWriter, r *http.Request, name, title string, data any) {
	a.renderStatus(w, r, http.StatusOK, name, title, data)
}

// renderStatus renders a page with an explicit HTTP status.
func (a *App) renderStatus(w http.ResponseWriter, r *http.Request, status int, name, title string, data any) {
	if d, ok := data.(map[string]any); ok {
		if ap, ok := d["App"].(*db.App); ok {
			m, _ := d["M"].(*Manifest)
			d["Items"] = a.appItems(r, ap.Slug, m, r.URL.Path, r.URL.Query().Get("type"))
		}
	}
	p := page{Title: title, Session: current(r), Path: r.URL.Path, Query: r.URL.Query().Get("q"), Version: a.cfg.Version, UI: uiStyle(r), Theme: themeOf(r), Data: data}
	if p.Session != nil {
		p.Nav = a.nav(r)
	}
	if c, err := r.Cookie("imp_flash"); err == nil && c.Value != "" {
		if v, err := hex.DecodeString(c.Value); err == nil {
			p.Flash = string(v)
		}
		http.SetCookie(w, &http.Cookie{Name: "imp_flash", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
	}
	if c, err := r.Cookie("imp_error"); err == nil && c.Value != "" {
		if v, err := hex.DecodeString(c.Value); err == nil {
			p.Error = string(v)
		}
		http.SetCookie(w, &http.Cookie{Name: "imp_error", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
	}
	// Render to memory first: a template error halfway must not send half a page with a 200.
	var buf bytes.Buffer
	if err := a.tpls[a.langOf(r)].ExecuteTemplate(&buf, name, p); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	_, _ = buf.WriteTo(w)
}

func (a *App) flash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{Name: "imp_flash", Value: hex.EncodeToString([]byte(msg)), Path: "/", MaxAge: 60, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
}

func (a *App) flashError(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{Name: "imp_error", Value: hex.EncodeToString([]byte(msg)), Path: "/", MaxAge: 60, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
}

func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"app":"imperium","version":"` + a.cfg.Version + `","uptime_s":` + strconv.FormatInt(int64(time.Since(a.start).Seconds()), 10) + `}`))
}

// Sweep runs housekeeping (expired sessions) every few minutes.
func (a *App) Sweep(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.db.Sweep(ctx)
			a.forgetChecks()
			a.pruneNotifications(ctx)
			// Keep the registry fresh so a newly registered app shows up without a sign-in.
			if a.cfg.SSOEnabled() {
				_ = a.syncApps(nil)
			}
		}
	}
}

// uiStyle reads the optional interface style: only "clean" switches it on, anything else is the original HOP look.
func uiStyle(r *http.Request) string {
	if c, err := r.Cookie("hop_ui"); err == nil && c.Value == "clean" {
		return "clean"
	}
	return ""
}
