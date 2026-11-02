package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

// POST /hop/notify: an app reports a failure (an HTTP 500 it served). It is a
// normal signed HOP request made by the sending app with its own registry
// secret, and it only works for direct callers on this machine.

// Per-app and global limits on accepted reports.
const (
	notifyAppRate         = 0.5 // per second (30 a minute)
	notifyAppBurst        = 60
	notifyGlobalRate      = 2.0 // per second (120 a minute)
	notifyGlobalBurst     = 120
	notifyMaxSkew         = 120 * time.Second
	defaultNotifyCooldown = time.Hour
)

// A request that went through any proxy or door is refused: apps call
// 127.0.0.1 directly, so these headers mean someone else is calling.
var notifyForbiddenHeaders = []string{
	"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-Ip", "Forwarded", "Via",
	"Cf-Connecting-Ip", "Cf-Ray", "Cf-Visitor", "True-Client-Ip", "X-Hop-Door",
	"Cookie", "Authorization",
}

type bucket struct {
	tokens float64
	at     time.Time
}

// rateLimiter is a token bucket per key.
type rateLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
	now     func() time.Time
}

func newRateLimiter(rate, burst float64) *rateLimiter {
	return &rateLimiter{rate: rate, burst: burst, buckets: map[string]*bucket{}, now: time.Now}
}

// allow takes a token for key, or says how long to wait.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.at).Seconds()*l.rate)
	b.at = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
}

// notifyInbox is the state of the inbound endpoint.
type notifyInbox struct {
	nonces  hopproto.Nonces
	perApp  *rateLimiter
	global  *rateLimiter
	logMu   sync.Mutex
	lastLog map[string]time.Time
}

func newNotifyInbox() *notifyInbox {
	return &notifyInbox{perApp: newRateLimiter(notifyAppRate, notifyAppBurst), global: newRateLimiter(notifyGlobalRate, notifyGlobalBurst), lastLog: map[string]time.Time{}}
}

// logReject writes one line per sender and reason at most every ten seconds.
// It never prints a secret, a signature or any part of the body.
func (n *notifyInbox) logReject(app, code string) {
	key := app + "|" + code
	n.logMu.Lock()
	defer n.logMu.Unlock()
	if time.Since(n.lastLog[key]) < 10*time.Second {
		return
	}
	if len(n.lastLog) > 256 {
		n.lastLog = map[string]time.Time{}
	}
	n.lastLog[key] = time.Now()
	log.Printf("notify: refused app=%s reason=%s", app, code)
}

func notifyReply(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (a *App) notifyFail(w http.ResponseWriter, status int, app, code, msg string) {
	a.inbox.logReject(app, code)
	body := map[string]any{"ok": false, "error": code}
	if msg != "" {
		body["message"] = msg
	}
	notifyReply(w, status, body)
}

// senderSecret is the secret a sending app signs with: its own registry
// secret. HOP ID signs as "hub" with the secret Imperium holds for itself in
// the registry. An empty result means the sender is unknown.
func (a *App) senderSecret(ctx context.Context, slug string) string {
	switch slug {
	case "hub":
		return a.cfg.HopSecret
	case a.cfg.HopClient, a.cfg.AltClient, "imperium":
		return ""
	}
	s, err := a.db.OwnSecret(ctx, slug)
	if err != nil {
		log.Printf("notify: registry lookup failed: %v", err)
		return ""
	}
	return s
}

func isLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *App) hopNotify(w http.ResponseWriter, r *http.Request) {
	// Deadlines for this route only: a slow or stalled caller cannot hold the connection.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(5 * time.Second))
	_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	slug := r.Header.Get("X-HOP-App")
	logApp := slug
	if !slugRe.MatchString(slug) {
		logApp = "?"
	}
	if !isLoopback(r.RemoteAddr) {
		a.notifyFail(w, http.StatusForbidden, logApp, "forbidden_door", "")
		return
	}
	for _, h := range notifyForbiddenHeaders {
		if r.Header.Get(h) != "" {
			a.notifyFail(w, http.StatusForbidden, logApp, "forbidden_door", "this endpoint only accepts direct calls from the same machine")
			return
		}
	}
	if r.URL.RawQuery != "" || r.URL.Fragment != "" {
		a.notifyFail(w, http.StatusBadRequest, logApp, "bad_request", "no query string allowed")
		return
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		a.notifyFail(w, http.StatusUnsupportedMediaType, logApp, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	if r.ContentLength > notifyMaxBody {
		a.notifyFail(w, http.StatusRequestEntityTooLarge, logApp, "too_large", "")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, notifyMaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			a.notifyFail(w, http.StatusRequestEntityTooLarge, logApp, "too_large", "")
		} else {
			a.notifyFail(w, http.StatusBadRequest, logApp, "bad_request", "")
		}
		return
	}

	// Signature, with the sending app's own secret.
	secret := ""
	if slugRe.MatchString(slug) {
		secret = a.senderSecret(ctx, slug)
	}
	if len(secret) < 32 {
		a.notifyFail(w, http.StatusUnauthorized, logApp, "bad_signature", "")
		return
	}
	if err := a.inbox.nonces.Verify(r, raw, slug, []string{secret}, notifyMaxSkew); err != nil {
		a.notifyFail(w, http.StatusUnauthorized, logApp, err.Error(), "")
		return
	}

	// Payload.
	p, err := decodeNotify(raw)
	var rep db.Report
	if err == nil {
		rep, err = validateNotify(p, slug, time.Now())
	}
	if err != nil {
		a.notifyFail(w, http.StatusUnprocessableEntity, logApp, "bad_payload", err.Error())
		return
	}

	// Rate limits: per app, then for all apps together. A refused report still
	// counts on its group, without storing an occurrence.
	for _, l := range []struct {
		lim *rateLimiter
		key string
	}{{a.inbox.perApp, slug}, {a.inbox.global, "*"}} {
		if ok, wait := l.lim.allow(l.key); !ok {
			if err := a.db.BumpSuppressed(ctx, db.Fingerprint(rep.App, rep.Env, rep.Class, rep.Origin, rep.Route)); err != nil {
				log.Printf("notify: suppressed counter: %v", err)
			}
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			a.notifyFail(w, http.StatusTooManyRequests, logApp, "rate_limited", "")
			return
		}
	}

	res, err := a.db.RecordNotification(ctx, rep)
	switch {
	case errors.Is(err, db.ErrInboxFull):
		w.Header().Set("Retry-After", "300")
		a.notifyFail(w, http.StatusTooManyRequests, logApp, "inbox_full", "")
		return
	case err != nil:
		log.Printf("notify: store: %v", err)
		a.notifyFail(w, http.StatusInternalServerError, logApp, "internal", "")
		return
	}
	a.alertGroup(rep, res)
	notifyReply(w, http.StatusAccepted, map[string]any{"ok": true, "data": map[string]any{"group": strconv.FormatInt(res.GroupID, 10), "duplicate": res.Duplicate}})
}

// alertGroup sends the one Telegram message a group may cause per cooldown:
// for a new group, a regression, or a count crossing 10, 100 or 1000. Muted
// and acknowledged groups never alert. The text holds the app, class, route
// template and count, nothing a visitor typed.
func (a *App) alertGroup(rep db.Report, res db.RecordResult) {
	if res.Duplicate || res.Status != db.NotifNew || a.cfg.TelegramToken == "" || a.cfg.TelegramChat == "" {
		return
	}
	crossed := false
	for _, t := range []int64{10, 100, 1000} {
		crossed = crossed || (res.OldCount < t && res.Count >= t)
	}
	if !res.NewGroup && !res.Regression && !crossed {
		return
	}
	cool := a.cfg.NotifyCooldown
	if cool <= 0 {
		cool = defaultNotifyCooldown
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ok, err := a.db.ClaimAlert(ctx, res.GroupID, time.Now(), cool)
	if err != nil {
		log.Printf("notify: alert claim: %v", err)
		return
	}
	if !ok {
		return
	}
	title := a.T("notif.tg.title")
	if res.Regression {
		title = a.T("notif.tg.regression")
	}
	msg := fmt.Sprintf("%s\n%s · %s\n%s\n%s: %d", title, rep.App, rep.Class, rep.Route, a.T("notif.count"), res.Count)
	if a.cfg.BaseURL != "" {
		msg += "\n" + a.cfg.BaseURL + "/notifications/" + strconv.FormatInt(res.GroupID, 10)
	}
	a.notify(msg)
}

// pruneNotifications applies the retention policy; called from Sweep.
func (a *App) pruneNotifications(ctx context.Context) {
	events, groups, err := a.db.PruneNotifications(ctx, time.Now())
	if err != nil {
		log.Printf("notifications: prune: %v", err)
		return
	}
	if events > 0 || groups > 0 {
		log.Printf("notifications: pruned %d occurrences and %d groups", events, groups)
	}
}
