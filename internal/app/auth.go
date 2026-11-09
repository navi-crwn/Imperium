package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

const ssoCookie = "imp_sso"

func (a *App) loginPage(w http.ResponseWriter, r *http.Request) {
	if current(r) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	// Which door the visitor came through: the Tailscale host or the Cloudflare Access one.
	kind := "Tailscale"
	if a.cfg.AltHost != "" && strings.EqualFold(r.Host, a.cfg.AltHost) {
		kind = "Cloudflare Access"
	}
	a.render(w, r, "login.html", a.T("login.title"), map[string]any{
		"Enabled": a.cfg.SSOEnabled(), "Next": safeNext(r.URL.Query().Get("next")),
		"Door": r.Host, "DoorKind": kind,
	})
}

// ssoStart sends the visitor to HOP ID to sign in.
func (a *App) ssoStart(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.SSOEnabled() {
		a.flashError(w, a.T("login.off"))
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	client, _, base := a.cfg.Door(r.Host)
	state := randomToken(16)
	http.SetCookie(w, &http.Cookie{Name: ssoCookie, Value: state + "|" + url.QueryEscape(safeNext(r.URL.Query().Get("next"))),
		Path: "/auth/", HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode, MaxAge: 900})
	q := url.Values{"state": {state}, "return_to": {base + "/auth/sso/callback"}}
	http.Redirect(w, r, a.cfg.HopURL+"/sso/"+url.PathEscape(client)+"?"+q.Encode(), http.StatusFound)
}

// ssoCallback checks the ticket and the person's roles, then opens a session.
func (a *App) ssoCallback(w http.ResponseWriter, r *http.Request) {
	fail := func(msg string) {
		a.flashError(w, msg)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
	c, err := r.Cookie(ssoCookie)
	if err != nil {
		fail(a.T("login.bad_ticket"))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: ssoCookie, Path: "/auth/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
	state, nextEsc, _ := strings.Cut(c.Value, "|")
	next, _ := url.QueryUnescape(nextEsc)
	client, secret, _ := a.cfg.Door(r.Host)
	t, err := hopproto.ParseTicket(r.URL.Query().Get("ticket"), secret, time.Now())
	if err != nil || t.State != state || (t.Aud != "" && t.Aud != client) || !a.jti.First(t.Jti, time.Unix(t.Exp, 0)) {
		fail(a.T("login.bad_ticket"))
		return
	}
	if alias := a.cfg.Alias(t.Username, t.Email); alias != "" {
		// The owner's real name and email never reach a page, the audit log or Telegram.
		t.Username, t.Name, t.Email = alias, alias, ""
	}
	if t.Status != "" && t.Status != "active" {
		fail(fmt.Sprintf(a.T("login.denied"), "@"+t.Username))
		return
	}
	if !allowedIn(t.Roles) {
		a.db.AddAudit(r.Context(), &db.Audit{ActorSub: t.Sub, ActorName: t.Username, IP: clientIP(r), Action: "login.denied", Detail: map[string]any{"roles": t.Roles}})
		a.notify(fmt.Sprintf("⛔ Login ditolak: @%s (%s) tanpa peran admin, dari %s", t.Username, t.Email, clientIP(r)))
		fail(fmt.Sprintf(a.T("login.denied"), "@"+t.Username))
		return
	}
	if a.cfg.RequireMFA && !t.MFA {
		a.db.AddAudit(r.Context(), &db.Audit{ActorSub: t.Sub, ActorName: t.Username, IP: clientIP(r), Action: "login.no_mfa"})
		a.notify(fmt.Sprintf("⚠️ Login admin tanpa 2FA ditolak: @%s dari %s", t.Username, clientIP(r)))
		fail(fmt.Sprintf(a.T("login.mfa"), "@"+t.Username))
		return
	}
	tok := randomToken(32)
	now := time.Now()
	s := &db.Session{TokenHash: hashToken(tok), Sub: t.Sub, Username: t.Username, Name: t.Name, Email: t.Email, Avatar: t.Avatar,
		Roles: t.Roles, MFA: t.MFA, IP: clientIP(r), UA: trim(r.UserAgent(), 200), Created: now, Expires: now.Add(a.cfg.SessionTTL), LastSeen: now}
	if err := a.db.CreateSession(r.Context(), s); err != nil {
		fail("session: " + err.Error())
		return
	}
	a.checkMu.Lock()
	a.checked[s.TokenHash] = now.Add(sessionRecheck) // HOP ID has just vouched for this person
	a.checkMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true, Secure: a.cfg.Secure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(a.cfg.SessionTTL.Seconds())})
	a.db.AddAudit(r.Context(), &db.Audit{ActorSub: t.Sub, ActorName: t.Username, IP: clientIP(r), Action: "login"})
	a.notify(fmt.Sprintf("✅ @%s masuk ke Imperium dari %s", t.Username, clientIP(r)))
	// A fresh sign-in is a good moment to refresh the registry.
	go a.syncApps(s)
	http.Redirect(w, r, safeNext(next), http.StatusFound)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if s := current(r); s != nil {
		a.db.DeleteSession(r.Context(), s.TokenHash)
		a.db.AddAudit(r.Context(), &db.Audit{ActorSub: s.Sub, ActorName: s.Username, IP: clientIP(r), Action: "logout"})
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.Secure, SameSite: http.SameSiteLaxMode})
	if current(r) != nil {
		a.flash(w, a.T("logout.done"))
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "me.html", "Me", current(r))
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
