// Package fakehub is a stand-in for the parts of HOP ID's admin API that do not exist in the hub yet, so
// Imperium can be built, tested and photographed without it. It implements the contract "Hub admin API for
// announcements" (docs/announcements-contract.md); the real hub must answer exactly like this.
package fakehub

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Handler is a signed admin call: the caller verified the signature and read the body already.
type Handler func(w http.ResponseWriter, r *http.Request, body []byte)

// Wrap turns a Handler into an http.HandlerFunc (verifying the HOP signature, reading the body).
type Wrap func(Handler) http.HandlerFunc

// Levels are the levels the hub's Announcement model knows.
var Levels = []string{"info", "warning", "critical"}

// Row is one announcement as the hub stores it.
type Row struct {
	ID          int
	Title, Body string
	Level       string
	AppSlug     string
	PublishedAt *time.Time
	ExpiresAt   *time.Time
	Author      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Call is one request the fake received, for tests to look at.
type Call struct {
	Method, Path string
	Body         map[string]any
	Actor        string
}

// ErrSpec makes every announcement call fail the way the hub would.
type ErrSpec struct {
	Status        int
	Code, Message string
}

// Store holds the announcements and the calls made.
type Store struct {
	mu    sync.Mutex
	rows  []*Row
	next  int
	Now   func() time.Time
	Apps  []string // slugs app_slug may name
	calls []Call
	fail  *ErrSpec
}

// New returns an empty store.
func New(apps ...string) *Store {
	return &Store{next: 1, Now: func() time.Time { return time.Now().UTC() }, Apps: apps}
}

// Seed adds a row and returns its id.
func (s *Store) Seed(r Row) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = s.next
	s.next++
	now := s.Now()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = r.CreatedAt
	}
	s.rows = append(s.rows, &r)
	return r.ID
}

// Calls returns a copy of the calls made so far.
func (s *Store) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// Reset forgets the recorded calls.
func (s *Store) ResetCalls() {
	s.mu.Lock()
	s.calls = nil
	s.mu.Unlock()
}

// FailWith makes every announcement call answer with e until FailWith(nil).
func (s *Store) FailWith(e *ErrSpec) {
	s.mu.Lock()
	s.fail = e
	s.mu.Unlock()
}

// Len is the number of stored announcements.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}

// Get returns a copy of one row.
func (s *Store) Get(id int) (Row, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.ID == id {
			return *r, true
		}
	}
	return Row{}, false
}

func (r *Row) state(now time.Time) string {
	switch {
	case r.PublishedAt == nil:
		return "draft"
	case r.PublishedAt.After(now):
		return "scheduled"
	case r.ExpiresAt != nil && !r.ExpiresAt.After(now):
		return "expired"
	}
	return "live"
}

func iso(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func (s *Store) api(r *Row) map[string]any {
	return map[string]any{
		"id": strconv.Itoa(r.ID), "title": r.Title, "body": r.Body, "level": r.Level, "app_slug": nilIfEmpty(r.AppSlug),
		"state": r.state(s.Now()), "published_at": iso(r.PublishedAt), "expires_at": iso(r.ExpiresAt), "author": nilIfEmpty(r.Author),
		"created_at": r.CreatedAt.UTC().Format(time.RFC3339), "updated_at": r.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func out(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	out(w, status, map[string]any{"ok": false, "error": code, "message": msg})
}

// Register mounts the five calls under prefix (for example "/api/hop").
func (s *Store) Register(m *http.ServeMux, prefix string, wrap Wrap) {
	h := func(f func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return wrap(func(w http.ResponseWriter, r *http.Request, body []byte) {
			var parsed map[string]any
			_ = json.Unmarshal(body, &parsed)
			s.mu.Lock()
			s.calls = append(s.calls, Call{Method: r.Method, Path: r.URL.RequestURI(), Body: parsed, Actor: r.Header.Get("X-HOP-Actor")})
			e := s.fail
			s.mu.Unlock()
			if e != nil {
				fail(w, e.Status, e.Code, e.Message)
				return
			}
			f(w, r, body)
		})
	}
	m.HandleFunc("GET "+prefix+"/announcements", h(s.list))
	m.HandleFunc("GET "+prefix+"/announcements/{id}", h(s.show))
	m.HandleFunc("POST "+prefix+"/announcements", h(s.create))
	m.HandleFunc("PUT "+prefix+"/announcements/{id}", h(s.update))
	m.HandleFunc("POST "+prefix+"/announcements/{id}/actions", h(s.act))
}

func (s *Store) find(w http.ResponseWriter, r *http.Request) *Row {
	id, _ := strconv.Atoi(r.PathValue("id"))
	for _, row := range s.rows {
		if row.ID == id {
			return row
		}
	}
	fail(w, 404, "not_found", "No such announcement.")
	return nil
}

func (s *Store) list(w http.ResponseWriter, r *http.Request, _ []byte) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit == 0 {
		limit = 50
	}
	limit = max(1, min(200, limit))
	cursor, _ := strconv.Atoi(q.Get("cursor"))
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	var match []*Row
	for _, row := range s.rows {
		if st := q.Get("state"); st != "" && row.state(now) != st {
			continue
		}
		if lv := q.Get("level"); lv != "" && row.Level != lv {
			continue
		}
		match = append(match, row)
	}
	sort.Slice(match, func(i, j int) bool { return match[i].ID > match[j].ID })
	total := len(match)
	var rows []*Row
	for _, row := range match {
		if cursor == 0 || row.ID < cursor {
			rows = append(rows, row)
		}
	}
	var next any
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if len(rows) == limit { // like the users list: a full page may be followed by an empty one
		next = strconv.Itoa(rows[len(rows)-1].ID)
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, s.api(row))
	}
	out(w, 200, map[string]any{"ok": true, "data": data, "page": map[string]any{"next": next, "total": total}})
}

func (s *Store) show(w http.ResponseWriter, r *http.Request, _ []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row := s.find(w, r); row != nil {
		out(w, 200, map[string]any{"ok": true, "data": s.api(row)})
	}
}

// fields is the editable part of an announcement as a request carries it.
type fields struct {
	Title, Body, Level string
	AppSlug            *string
	PublishedAt        *string
	ExpiresAt          *string
	PublishNow         bool
	Reason             any
}

func (s *Store) apply(w http.ResponseWriter, body []byte, row *Row) bool {
	var f fields
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		fail(w, 422, "bad_params", "the body must be a JSON object")
		return false
	}
	f.Title, _ = raw["title"].(string)
	f.Body, _ = raw["body"].(string)
	f.Level, _ = raw["level"].(string)
	f.AppSlug = optString(raw["app_slug"])
	f.PublishedAt = optString(raw["published_at"])
	f.ExpiresAt = optString(raw["expires_at"])
	f.PublishNow, _ = raw["publish_now"].(bool)
	bad := func(msg string) bool { fail(w, 422, "bad_params", msg); return false }
	switch {
	case strings.TrimSpace(f.Title) == "":
		return bad("title is required")
	case utf8.RuneCountInString(f.Title) > 160:
		return bad("title may not be longer than 160 characters")
	case strings.TrimSpace(f.Body) == "":
		return bad("body is required")
	case utf8.RuneCountInString(f.Body) > 5000:
		return bad("body may not be longer than 5000 characters")
	}
	okLevel := false
	for _, l := range Levels {
		okLevel = okLevel || l == f.Level
	}
	if !okLevel {
		return bad("level must be one of " + strings.Join(Levels, ", "))
	}
	app := ""
	if f.AppSlug != nil && *f.AppSlug != "" {
		app = *f.AppSlug
		known := false
		for _, a := range s.Apps {
			known = known || a == app
		}
		if !known {
			return bad("app_slug must be empty or the slug of a registered app")
		}
	}
	pub, err := parseTime(f.PublishedAt)
	if err != nil {
		return bad("published_at must be an ISO 8601 date and time")
	}
	exp, err := parseTime(f.ExpiresAt)
	if err != nil {
		return bad("expires_at must be an ISO 8601 date and time")
	}
	now := s.Now()
	// Checked against the submitted published_at, before publish_now replaces it (as HOP ID does).
	if exp != nil && (pub == nil || !exp.After(*pub)) {
		return bad("expires_at must be after published_at")
	}
	if f.PublishNow {
		pub = &now
	}
	row.Title, row.Body, row.Level, row.AppSlug, row.PublishedAt, row.ExpiresAt = f.Title, f.Body, f.Level, app, pub, exp
	row.UpdatedAt = now
	return true
}

func optString(v any) *string {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return &s
}

func parseTime(p *string) (*time.Time, error) {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil, nil
	}
	// YYYY-MM-DDTHH:MM[:SS[.fff]] with an optional Z or +-HH:MM; no offset means UTC. Output is whole seconds.
	v := strings.TrimSpace(*p)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.Parse(layout, v); err == nil {
			t = t.UTC().Truncate(time.Second)
			return &t, nil
		}
	}
	return nil, errors.New("bad time")
}

func (s *Store) create(w http.ResponseWriter, r *http.Request, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := &Row{ID: s.next, CreatedAt: s.Now(), Author: r.Header.Get("X-HOP-Actor")}
	if !s.apply(w, body, row) {
		return
	}
	s.next++
	s.rows = append(s.rows, row)
	out(w, 201, map[string]any{"ok": true, "data": s.api(row)})
}

func (s *Store) update(w http.ResponseWriter, r *http.Request, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.find(w, r)
	if row == nil {
		return
	}
	cp := *row
	if !s.apply(w, body, &cp) {
		return
	}
	*row = cp
	out(w, 200, map[string]any{"ok": true, "data": s.api(row)})
}

func (s *Store) act(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(body, &in)
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Action != "publish" && in.Action != "unpublish" && in.Action != "delete" {
		fail(w, 422, "unknown_action", "action must be one of publish, unpublish, delete")
		return
	}
	row := s.find(w, r)
	if row == nil {
		return
	}
	now := s.Now()
	switch in.Action {
	case "publish":
		switch row.state(now) {
		case "expired":
			fail(w, 409, "expired", "This announcement has expired; change its expiry date first.")
			return
		case "draft", "scheduled":
			if row.ExpiresAt != nil && !row.ExpiresAt.After(now) { // publishing would expire it at once
				fail(w, 409, "expired", "This announcement has expired; change its expiry date first.")
				return
			}
			row.PublishedAt = &now
			row.UpdatedAt = now
		}
	case "unpublish":
		row.PublishedAt = nil
		row.UpdatedAt = now
	case "delete":
		data := s.api(row)
		data["deleted"] = true
		for i, x := range s.rows {
			if x == row {
				s.rows = append(s.rows[:i], s.rows[i+1:]...)
				break
			}
		}
		out(w, 200, map[string]any{"ok": true, "data": data})
		return
	}
	out(w, 200, map[string]any{"ok": true, "data": s.api(row)})
}
