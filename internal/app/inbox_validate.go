package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/navi-crwn/imperium/internal/db"
)

// Validation of what an app reports to POST /hop/notify. The payload is a
// closed list of fields; anything else (bodies, cookies, query strings, IP
// addresses, user ids …) is refused rather than stored.

const (
	notifyMaxBody    = 8 << 10
	notifyMaxMessage = 200
	notifyTypeError  = "error.500"
	notifyMaxAge     = 24 * time.Hour
	notifyMaxFuture  = 5 * time.Minute
)

type notifyData struct {
	Class   string `json:"class"`
	Message string `json:"message"`
	Origin  string `json:"origin"`
	Route   string `json:"route"`
	Method  string `json:"method"`
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

type notifyPayload struct {
	ID   string     `json:"id"`
	Type string     `json:"type"`
	At   string     `json:"at"`
	App  string     `json:"app"`
	Env  string     `json:"env"`
	Data notifyData `json:"data"`
}

// fieldError names the field that failed, never its value.
type fieldError struct{ Field, Reason string }

func (e *fieldError) Error() string { return e.Field + ": " + e.Reason }

func bad(field, reason string) error { return &fieldError{field, reason} }

var (
	slugRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	reportRe  = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
	envRe     = regexp.MustCompile(`^[a-z0-9_-]{1,24}$`)
	classRe   = regexp.MustCompile(`^[A-Za-z0-9_.\\$/-]{1,160}$`)
	originRe  = regexp.MustCompile(`^[A-Za-z0-9_.\\/-]{1,180}:[0-9]{1,6}$`)
	refRe     = regexp.MustCompile(`^[0-9a-f]{8,32}$`)
	versionRe = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,40}$`)
	braceRe   = regexp.MustCompile(`\{[A-Za-z0-9_:?.$-]{1,40}\}`)
	routeRe   = regexp.MustCompile(`^[A-Za-z0-9_.~/$*+-]*$`)
	hexRunRe  = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	ipv4Re    = regexp.MustCompile(`\b[0-9]{1,3}(?:\.[0-9]{1,3}){3}\b`)
	ipv6Re    = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{1,4}:){2,7}[0-9a-f]{1,4}\b`)

	// Scrubbing of free text (the app scrubs first; this is the second net).
	emailRe    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	userinfoRe = regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s/]*@\S*`)
	queryURLRe = regexp.MustCompile(`(\b[a-zA-Z][a-zA-Z0-9+.-]*://[^\s?#]*)[?#]\S*`)
	bearerRe   = regexp.MustCompile(`(?i)\b(bearer|basic)\s+\S+`)
	hexTokenRe = regexp.MustCompile(`[0-9a-fA-F]{20,}`)
	wordRunRe  = regexp.MustCompile(`[A-Za-z0-9_+=-]{24,}`)
)

var notifyMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true, "OPTIONS": true}

const redacted = "[redacted]"

// scrubMessage removes what must not be stored from a free-text message:
// control characters, emails, URL credentials and query strings, bearer
// tokens, IP addresses and long token-like runs; then cuts it to 200 characters.
func scrubMessage(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = userinfoRe.ReplaceAllString(s, redacted)
	s = queryURLRe.ReplaceAllString(s, "${1}?"+redacted)
	s = emailRe.ReplaceAllString(s, redacted)
	s = bearerRe.ReplaceAllString(s, "${1} "+redacted)
	s = ipv4Re.ReplaceAllString(s, redacted)
	s = ipv6Re.ReplaceAllString(s, redacted)
	s = hexTokenRe.ReplaceAllString(s, redacted)
	s = wordRunRe.ReplaceAllStringFunc(s, func(m string) string {
		var digit, letter bool
		for _, r := range m {
			digit = digit || (r >= '0' && r <= '9')
			letter = letter || unicode.IsLetter(r)
		}
		if digit && letter {
			return redacted
		}
		return m
	})
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > notifyMaxMessage {
		s = string([]rune(s)[:notifyMaxMessage])
	}
	return s
}

// decodeNotify reads exactly one JSON object with no unknown fields.
func decodeNotify(body []byte) (*notifyPayload, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var p notifyPayload
	if err := dec.Decode(&p); err != nil {
		// encoding/json errors name fields and types, never values.
		return nil, bad("body", "not accepted: "+trimStr(err.Error(), 120))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, bad("body", "must be a single JSON object")
	}
	return &p, nil
}

func trimStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// validateRoute accepts a route template: an optional "METHOD " prefix, then
// slash-separated segments in which parameters are {names}. A path with a
// query string, an email, an IP address or a token-like segment is a real URL
// and is refused.
func validateRoute(route string) error {
	if route == "unmatched" {
		return nil
	}
	if route == "" || len(route) > 200 {
		return bad("data.route", "required, at most 200 characters")
	}
	tpl := route
	if m, rest, ok := strings.Cut(route, " "); ok {
		if !notifyMethods[m] {
			return bad("data.route", "only an HTTP method may precede the space")
		}
		tpl = rest
	}
	if tpl == "" {
		return bad("data.route", "empty template")
	}
	plain := braceRe.ReplaceAllString(tpl, "")
	if !routeRe.MatchString(plain) {
		return bad("data.route", "must be a route template without query string or other characters")
	}
	if ipv4Re.MatchString(plain) {
		return bad("data.route", "looks like a real path (IP address)")
	}
	for _, seg := range strings.Split(plain, "/") {
		var digit, letter bool
		for _, r := range seg {
			digit = digit || (r >= '0' && r <= '9')
			letter = letter || unicode.IsLetter(r)
		}
		if hexRunRe.MatchString(seg) || (len(seg) >= 20 && digit && letter) {
			return bad("data.route", "looks like a real path, send the template")
		}
	}
	return nil
}

func validateOrigin(origin string) error {
	if origin == "unknown" {
		return nil
	}
	if !originRe.MatchString(origin) || strings.HasPrefix(origin, "/") || strings.HasPrefix(origin, `\`) || strings.Contains(origin, "..") {
		return bad("data.origin", "must be a relative file:line, or unknown")
	}
	if ipv4Re.MatchString(origin) {
		return bad("data.origin", "must be a relative file:line, or unknown")
	}
	return nil
}

// validateNotify checks a decoded payload against the sender and the clock
// and returns the report to store, with the message scrubbed.
func validateNotify(p *notifyPayload, sender string, now time.Time) (db.Report, error) {
	var rep db.Report
	if !reportRe.MatchString(p.ID) {
		return rep, bad("id", "8 to 64 characters of A-Z a-z 0-9 _ -")
	}
	if p.Type != notifyTypeError {
		return rep, bad("type", "only "+notifyTypeError+" is accepted")
	}
	if p.App != sender {
		return rep, bad("app", "must equal X-HOP-App")
	}
	if !envRe.MatchString(p.Env) {
		return rep, bad("env", "1 to 24 characters of a-z 0-9 _ -")
	}
	if !strings.HasSuffix(p.At, "Z") {
		return rep, bad("at", "must be an RFC 3339 UTC time ending in Z")
	}
	at, err := time.Parse(time.RFC3339, p.At)
	if err != nil {
		return rep, bad("at", "must be an RFC 3339 UTC time ending in Z")
	}
	if at.After(now.Add(notifyMaxFuture)) || at.Before(now.Add(-notifyMaxAge)) {
		return rep, bad("at", "must be within the last 24 hours")
	}
	d := p.Data
	if !classRe.MatchString(d.Class) {
		return rep, bad("data.class", "1 to 160 characters of A-Z a-z 0-9 _ . \\ $ / -")
	}
	if err := validateOrigin(d.Origin); err != nil {
		return rep, err
	}
	if err := validateRoute(d.Route); err != nil {
		return rep, err
	}
	if !notifyMethods[d.Method] {
		return rep, bad("data.method", "must be an upper-case HTTP method")
	}
	if !refRe.MatchString(d.Ref) {
		return rep, bad("data.ref", "8 to 32 lower-case hex characters")
	}
	if d.Version != "" && !versionRe.MatchString(d.Version) {
		return rep, bad("data.version", "at most 40 characters of A-Z a-z 0-9 . _ + -")
	}
	if !utf8.ValidString(d.Message) {
		return rep, bad("data.message", "must be valid UTF-8")
	}
	if utf8.RuneCountInString(d.Message) > notifyMaxMessage {
		return rep, bad("data.message", fmt.Sprintf("at most %d characters", notifyMaxMessage))
	}
	return db.Report{ID: p.ID, Kind: p.Type, App: sender, Env: p.Env, At: at.UTC(), Class: d.Class, Message: scrubMessage(d.Message),
		Origin: d.Origin, Route: d.Route, Method: d.Method, Ref: d.Ref, Version: d.Version}, nil
}
