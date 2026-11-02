// Package hopproto implements the HOP App Protocol v1: signed server-to-server
// messages between HOP ID (LinkHop), Imperium and the HOP apps, and the SSO
// ticket HOP ID hands to an app after sign-in.
package hopproto

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Version = "hop1"

// B64 is base64url without padding, as the protocol uses everywhere.
func B64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// UnB64 decodes base64url without padding.
func UnB64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// StringToSign is "hop1|app|ts|nonce|METHOD|PATH|sha256hex(body)". PATH is the
// request target with its query string.
func StringToSign(app, ts, nonce, method, path string, body []byte) string {
	sum := sha256.Sum256(body)
	return strings.Join([]string{Version, app, ts, nonce, strings.ToUpper(method), path, hex.EncodeToString(sum[:])}, "|")
}

func sign(secret, base string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(base))
	return B64(m.Sum(nil))
}

// Headers signs an outgoing request as `app` with `secret`.
func Headers(app, secret, actor, method, path string, body []byte) http.Header {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	n := make([]byte, 16)
	_, _ = rand.Read(n)
	nonce := B64(n)
	h := http.Header{}
	h.Set("X-HOP-App", app)
	h.Set("X-HOP-Timestamp", ts)
	h.Set("X-HOP-Nonce", nonce)
	h.Set("X-HOP-Actor", actor)
	h.Set("X-HOP-Signature", sign(secret, StringToSign(app, ts, nonce, method, path, body)))
	return h
}

// Nonces remembers nonces until they expire so each request works once.
type Nonces struct {
	mu sync.Mutex
	m  map[string]time.Time
}

// First reports whether key is new, remembering it until exp.
func (n *Nonces) First(key string, exp time.Time) bool { return n.first(key, time.Until(exp)) }

func (n *Nonces) first(key string, ttl time.Duration) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.m == nil {
		n.m = map[string]time.Time{}
	}
	now := time.Now()
	for k, t := range n.m {
		if t.Before(now) {
			delete(n.m, k)
		}
	}
	if _, seen := n.m[key]; seen {
		return false
	}
	n.m[key] = now.Add(ttl)
	return true
}

// Verify checks an incoming request. secrets are the possible secrets of the
// calling app (current and, during a rotation, the previous one). The returned
// error code is one of stale_request, bad_signature, replay.
func (n *Nonces) Verify(r *http.Request, body []byte, app string, secrets []string, maxAge time.Duration) error {
	if r.Header.Get("X-HOP-App") != app {
		return errors.New("bad_signature")
	}
	ts := r.Header.Get("X-HOP-Timestamp")
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || abs(time.Now().Unix()-t) > int64(maxAge.Seconds()) {
		return errors.New("stale_request")
	}
	nonce := r.Header.Get("X-HOP-Nonce")
	if len(nonce) < 16 || len(nonce) > 128 {
		return errors.New("bad_signature")
	}
	base := StringToSign(app, ts, nonce, r.Method, r.URL.RequestURI(), body)
	got := r.Header.Get("X-HOP-Signature")
	ok := false
	for _, s := range secrets {
		if hmac.Equal([]byte(sign(s, base)), []byte(got)) {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New("bad_signature")
	}
	if !n.first(app+"|"+nonce, 2*maxAge) {
		return errors.New("replay")
	}
	return nil
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// ---- SSO ticket ------------------------------------------------------------

// Ticket is what HOP ID sends to an app after sign-in (v2 fields included).
type Ticket struct {
	Sub            string   `json:"sub"`
	Aud            string   `json:"aud"`
	Email          string   `json:"email"`
	Name           string   `json:"name"`
	Username       string   `json:"username"`
	Avatar         string   `json:"avatar"`
	Admin          bool     `json:"admin"`
	Roles          []string `json:"roles"`
	Status         string   `json:"status"`
	Verified       bool     `json:"verified"`
	MFA            bool     `json:"mfa"`
	ProfileVersion int      `json:"profile_version"`
	State          string   `json:"state"`
	Iat            int64    `json:"iat"`
	Exp            int64    `json:"exp"`
	Jti            string   `json:"jti"`
	V              int      `json:"v"`
}

var ErrBadTicket = errors.New("this sign-in link is not valid any more")

// ParseTicket checks the signature and lifetime of a ticket: b64u(json).b64u(hmac).
func ParseTicket(raw, secret string, now time.Time) (*Ticket, error) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok || body == "" || sig == "" {
		return nil, ErrBadTicket
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	got, err := UnB64(sig)
	if err != nil || !hmac.Equal(got, m.Sum(nil)) {
		return nil, ErrBadTicket
	}
	js, err := UnB64(body)
	if err != nil {
		return nil, ErrBadTicket
	}
	var t Ticket
	if err := json.Unmarshal(js, &t); err != nil || t.Sub == "" || t.Jti == "" || t.State == "" {
		return nil, ErrBadTicket
	}
	n := now.Unix()
	if t.Exp < n || t.Iat > n+60 || t.Exp-t.Iat > 600 {
		return nil, ErrBadTicket
	}
	return &t, nil
}

// HasRole reports a global role ("owner") or an app-scoped one ("admin:pixelhop").
func (t *Ticket) HasRole(role string) bool {
	for _, r := range t.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// ---- client ----------------------------------------------------------------

// App is a target of signed calls: its base URL, the path prefix of its HOP
// endpoints (/hop for the apps, /api/hop for HOP ID itself) and its secret.
type App struct {
	Slug   string
	URL    string
	Prefix string
	Secret string
}

// Client makes signed calls on behalf of `As` (e.g. "imperium").
type Client struct {
	As   string
	HTTP *http.Client
}

// Envelope is the standard reply shape.
type Envelope struct {
	OK      bool            `json:"ok"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Message string          `json:"message"`
	Page    *struct {
		Next  string `json:"next"`
		Total int    `json:"total"`
	} `json:"page"`
}

// Call does one signed request and decodes the envelope. A non-2xx answer or
// ok=false becomes an error carrying the protocol error code.
func (c *Client) Call(ctx context.Context, app App, actor, method, pathAndQuery string, body any) (*Envelope, error) {
	var buf []byte
	if body != nil {
		var err error
		if buf, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	full := strings.TrimRight(app.URL, "/") + app.Prefix + pathAndQuery
	req, err := http.NewRequestWithContext(ctx, method, full, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	for k, v := range Headers(c.As, app.Secret, actor, method, app.Prefix+pathAndQuery, buf) {
		req.Header[k] = v
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if res.StatusCode >= 300 {
			return nil, fmt.Errorf("http_%d", res.StatusCode)
		}
		return nil, fmt.Errorf("bad_json: %w", err)
	}
	if !env.OK && env.Error == "" {
		// Older endpoints answer {"error": "..."} without ok.
		var alt struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &alt)
		env.Error = alt.Error
	}
	if res.StatusCode >= 300 || (!env.OK && env.Error != "") {
		if env.Error == "" {
			env.Error = fmt.Sprintf("http_%d", res.StatusCode)
		}
		return &env, &APIError{Status: res.StatusCode, Code: env.Error, Message: cleanMessage(env.Message)}
	}
	return &env, nil
}

// APIError is an app's refusal. Error() is the protocol code, so callers can
// compare it ("not_found"); for a 422 (the app rejected a value) it carries the
// app's own message too, so the admin sees which field or range failed.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Status == http.StatusUnprocessableEntity && e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

// cleanMessage makes an app's message safe to show and to keep in a cookie:
// no control characters, at most 300 characters.
func cleanMessage(s string) string {
	s = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)), " ")
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300])
	}
	return s
}
