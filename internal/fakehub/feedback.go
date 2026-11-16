package fakehub

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The feedback API of HOP ID (/api/hop/feedback), as the real hub has to answer it: the list, one report, its
// screenshot and the four actions. Reports come from the form on hel.ink; Imperium only reads and changes them.

// FeedbackKinds, FeedbackStatuses and FeedbackApps are the values the hub's table knows.
var (
	FeedbackKinds    = []string{"problem", "idea", "other"}
	FeedbackStatuses = []string{"new", "read", "done"}
	FeedbackApps     = []string{"hub", "linkhop", "biohop", "pixelhop", "notehop", "filehop", "payhop"}
)

// Report is one feedback report as the hub stores it.
type Report struct {
	ID                        int
	Kind, Status              string
	App, Page, AppVersion     string
	Message                   string
	UserID                    int // 0: a guest
	Username                  string
	IP                        string // empty: removed after 30 days
	UserAgent, Viewport, Lang string
	Screenshot                []byte // nil: none
	CreatedAt                 time.Time
	ReadAt, DoneAt            *time.Time
}

// Feedback holds the reports and the calls made.
type Feedback struct {
	mu    sync.Mutex
	rows  []*Report
	next  int
	Now   func() time.Time
	calls []Call
	fail  *ErrSpec
	// Missing makes every call answer 404 with no JSON, like a hub that predates the routes.
	Missing bool
	// IgnoreApp makes the list ignore the app filter, like a careless hub (Imperium filters again itself).
	IgnoreApp bool
	// RawBody, when set, is what every call answers with (status RawStatus): a broken or foreign answer.
	RawStatus int
	RawBody   string
	// Delay makes every call wait that long (or until the caller gives up) before it answers.
	Delay time.Duration
}

// NewFeedback returns an empty store.
func NewFeedback() *Feedback {
	return &Feedback{next: 1, Now: func() time.Time { return time.Now().UTC() }}
}

// Seed adds a report and returns its id. An empty status is "new", an empty kind "problem".
func (f *Feedback) Seed(r Report) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.ID = f.next
	f.next++
	if r.Status == "" {
		r.Status = "new"
	}
	if r.Kind == "" {
		r.Kind = "problem"
	}
	if r.App == "" {
		r.App = "hub"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = f.Now()
	}
	f.rows = append(f.rows, &r)
	return r.ID
}

// Calls returns a copy of the calls made so far.
func (f *Feedback) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// ResetCalls forgets the recorded calls.
func (f *Feedback) ResetCalls() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

// FailWith makes every call answer with e until FailWith(nil).
func (f *Feedback) FailWith(e *ErrSpec) {
	f.mu.Lock()
	f.fail = e
	f.mu.Unlock()
}

// SetMissing turns the "hub without the feedback routes" behaviour on or off.
func (f *Feedback) SetMissing(v bool) {
	f.mu.Lock()
	f.Missing = v
	f.mu.Unlock()
}

// Get returns a copy of one report.
func (f *Feedback) Get(id int) (Report, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			return *r, true
		}
	}
	return Report{}, false
}

// Len is the number of stored reports.
func (f *Feedback) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 160 {
		s = string([]rune(s)[:159]) + "…"
	}
	return s
}

func (f *Feedback) detail(r *Report) map[string]any {
	var user any
	if r.UserID != 0 {
		user = map[string]any{"id": r.UserID, "username": r.Username}
	}
	return map[string]any{
		"id": strconv.Itoa(r.ID), "kind": r.Kind, "status": r.Status, "app": r.App, "page": nilIfEmpty(r.Page), "app_version": nilIfEmpty(r.AppVersion),
		"message": r.Message, "user": user, "ip": nilIfEmpty(r.IP), "user_agent": nilIfEmpty(r.UserAgent), "viewport": nilIfEmpty(r.Viewport), "locale": nilIfEmpty(r.Lang),
		"has_screenshot": r.Screenshot != nil, "created_at": r.CreatedAt.UTC().Format(time.RFC3339), "read_at": iso(r.ReadAt), "done_at": iso(r.DoneAt),
	}
}

func (f *Feedback) row(r *Report) map[string]any {
	var user any
	if r.UserID != 0 {
		user = r.Username
	}
	return map[string]any{
		"id": strconv.Itoa(r.ID), "kind": r.Kind, "status": r.Status, "app": r.App, "page": nilIfEmpty(r.Page), "excerpt": excerpt(r.Message),
		"has_screenshot": r.Screenshot != nil, "username": user, "created_at": r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// Register mounts the four calls under prefix (for example "/api/hop").
func (f *Feedback) Register(m *http.ServeMux, prefix string, wrap Wrap) {
	h := func(fn func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return wrap(func(w http.ResponseWriter, r *http.Request, body []byte) {
			var parsed map[string]any
			_ = json.Unmarshal(body, &parsed)
			f.mu.Lock()
			f.calls = append(f.calls, Call{Method: r.Method, Path: r.URL.RequestURI(), Body: parsed, Actor: r.Header.Get("X-HOP-Actor")})
			e, missing, delay := f.fail, f.Missing, f.Delay
			rawStatus, rawBody := f.RawStatus, f.RawBody
			f.mu.Unlock()
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-r.Context().Done():
					return
				}
			}
			if rawBody != "" {
				w.WriteHeader(rawStatus)
				_, _ = w.Write([]byte(rawBody))
				return
			}
			if missing {
				http.NotFound(w, r)
				return
			}
			if e != nil {
				fail(w, e.Status, e.Code, e.Message)
				return
			}
			fn(w, r, body)
		})
	}
	m.HandleFunc("GET "+prefix+"/feedback", h(f.list))
	m.HandleFunc("GET "+prefix+"/feedback/{id}", h(f.show))
	m.HandleFunc("GET "+prefix+"/feedback/{id}/screenshot", h(f.shot))
	m.HandleFunc("POST "+prefix+"/feedback/{id}/actions", h(f.act))
}

// validID is the shape the contract fixes for ids and cursors.
func validID(v string) bool {
	if v == "" || len(v) > 64 {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (f *Feedback) find(w http.ResponseWriter, r *http.Request) *Report {
	id, _ := strconv.Atoi(r.PathValue("id"))
	for _, row := range f.rows {
		if row.ID == id {
			return row
		}
	}
	fail(w, 404, "not_found", "No such report.")
	return nil
}

func (f *Feedback) list(w http.ResponseWriter, r *http.Request, _ []byte) {
	q := r.URL.Query()
	if v := q.Get("app"); v != "" && !has(FeedbackApps, v) {
		fail(w, 422, "bad_params", "app must be one of "+strings.Join(FeedbackApps, ", "))
		return
	}
	if v := q.Get("kind"); v != "" && !has(FeedbackKinds, v) {
		fail(w, 422, "bad_params", "kind must be one of "+strings.Join(FeedbackKinds, ", "))
		return
	}
	if v := q.Get("status"); v != "" && !has(FeedbackStatuses, v) {
		fail(w, 422, "bad_params", "status must be one of "+strings.Join(FeedbackStatuses, ", "))
		return
	}
	limit := 25
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 1 {
			fail(w, 422, "bad_params", "limit must be a number from 1 to 100")
			return
		}
		limit = min(100, n)
	}
	if v := q.Get("cursor"); v != "" && !validID(v) {
		fail(w, 422, "bad_params", "cursor is not valid")
		return
	}
	cursor, _ := strconv.Atoi(q.Get("cursor"))
	f.mu.Lock()
	defer f.mu.Unlock()
	var match []*Report
	for _, row := range f.rows {
		if v := q.Get("app"); v != "" && row.App != v && !f.IgnoreApp {
			continue
		}
		if v := q.Get("kind"); v != "" && row.Kind != v {
			continue
		}
		if v := q.Get("status"); v != "" && row.Status != v {
			continue
		}
		match = append(match, row)
	}
	sort.Slice(match, func(i, j int) bool { return match[i].ID > match[j].ID })
	total := len(match)
	var rows []*Report
	for _, row := range match {
		if cursor == 0 || row.ID < cursor {
			rows = append(rows, row)
		}
	}
	var next any
	if len(rows) > limit { // only when something follows: the last page never points at an empty one
		rows = rows[:limit]
		next = strconv.Itoa(rows[len(rows)-1].ID)
	}
	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, f.row(row))
	}
	out(w, 200, map[string]any{"ok": true, "data": data, "page": map[string]any{"next": next, "total": total}})
}

func (f *Feedback) show(w http.ResponseWriter, r *http.Request, _ []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row := f.find(w, r); row != nil {
		out(w, 200, map[string]any{"ok": true, "data": f.detail(row)})
	}
}

func (f *Feedback) shot(w http.ResponseWriter, r *http.Request, _ []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.find(w, r)
	if row == nil {
		return
	}
	if row.Screenshot == nil {
		fail(w, 404, "not_found", "This report has no image.")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(row.Screenshot)
}

func (f *Feedback) act(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(body, &in)
	f.mu.Lock()
	defer f.mu.Unlock()
	if !has([]string{"read", "done", "reopen", "delete"}, in.Action) {
		fail(w, 400, "bad_action", "action must be one of read, done, reopen, delete")
		return
	}
	row := f.find(w, r)
	if row == nil {
		return
	}
	now := f.Now()
	switch in.Action {
	case "read":
		if row.Status == "new" {
			row.Status, row.ReadAt = "read", &now
		}
	case "done":
		row.Status, row.DoneAt = "done", &now
		if row.ReadAt == nil {
			row.ReadAt = &now
		}
	case "reopen":
		row.Status, row.DoneAt = "read", nil
	case "delete":
		for i, x := range f.rows {
			if x == row {
				f.rows = append(f.rows[:i], f.rows[i+1:]...)
				break
			}
		}
		out(w, 200, map[string]any{"ok": true})
		return
	}
	out(w, 200, map[string]any{"ok": true, "data": f.detail(row)})
}

// SetRaw makes every call answer with this status and body; an empty body turns it off.
func (f *Feedback) SetRaw(status int, body string) {
	f.mu.Lock()
	f.RawStatus, f.RawBody = status, body
	f.mu.Unlock()
}

// SetIgnoreApp turns the careless app filter on or off.
func (f *Feedback) SetIgnoreApp(v bool) {
	f.mu.Lock()
	f.IgnoreApp = v
	f.mu.Unlock()
}

// SetDelay makes every call wait d before it answers; 0 turns it off.
func (f *Feedback) SetDelay(d time.Duration) {
	f.mu.Lock()
	f.Delay = d
	f.mu.Unlock()
}
