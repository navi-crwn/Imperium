package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// The notification inbox: apps report failures (HTTP 500) to Imperium, which
// groups identical ones by fingerprint. A group keeps counts and the newest
// occurrences; the admin acknowledges, resolves or mutes it.

// Statuses of a group.
const (
	NotifNew      = "new"
	NotifSeen     = "seen"
	NotifResolved = "resolved"
	NotifMuted    = "muted"
)

// Retention: occurrences live 30 days, groups 90 days (resolved groups 30).
const (
	EventRetention         = 30 * 24 * time.Hour
	GroupRetention         = 90 * 24 * time.Hour
	ResolvedGroupRetention = 30 * 24 * time.Hour
)

// NotifLimits bounds what the inbox may hold, so a flood cannot grow the database.
type NotifLimits struct {
	KeepEvents      int // newest occurrences kept per group
	MaxGroups       int
	MaxGroupsPerApp int
}

var DefaultNotifLimits = NotifLimits{KeepEvents: 20, MaxGroups: 5000, MaxGroupsPerApp: 1000}

var (
	ErrNotFound      = errors.New("not_found")
	ErrBadTransition = errors.New("invalid_transition")
	ErrInboxFull     = errors.New("inbox_full")
)

// Report is one validated occurrence, as the inbound endpoint hands it over.
type Report struct {
	ID      string // unique per app; the idempotency key
	Kind    string
	App     string
	Env     string
	At      time.Time
	Class   string
	Message string
	Origin  string
	Route   string
	Method  string
	Ref     string
	Version string
}

// Fingerprint groups identical failures: sha256(app | env | class | origin | route).
func Fingerprint(app, env, class, origin, route string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{app, env, class, origin, route}, "|")))
	return hex.EncodeToString(sum[:])
}

func (r Report) fingerprint() string { return Fingerprint(r.App, r.Env, r.Class, r.Origin, r.Route) }

// Sample is the latest occurrence's detail kept on the group.
type Sample struct {
	Message string `json:"message,omitempty"`
	Origin  string `json:"origin,omitempty"`
	Route   string `json:"route,omitempty"`
	Method  string `json:"method,omitempty"`
	Version string `json:"version,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

// Notification is one group.
type Notification struct {
	ID          int64
	Kind        string
	App         string
	Env         string
	Fingerprint string
	Title       string // the error class
	FirstSeen   time.Time
	LastSeen    time.Time
	Count       int64
	Suppressed  int64
	Status      string
	Regression  bool
	ResolvedAt  time.Time
	Sample      Sample
}

// NotificationEvent is one stored occurrence.
type NotificationEvent struct {
	ID      int64
	At      time.Time
	Ref     string
	Version string
}

// RecordResult says what a stored report did to its group.
type RecordResult struct {
	GroupID    int64
	Duplicate  bool // the report id was already stored: nothing changed
	NewGroup   bool
	Regression bool // a resolved group was re-opened
	OldCount   int64
	Count      int64
	Status     string
}

// RecordNotification stores one occurrence in a single transaction: it is
// idempotent on the report id, creates or updates the group, re-opens a
// resolved one, and keeps only the newest occurrences.
func (d *DB) RecordNotification(ctx context.Context, rep Report) (RecordResult, error) {
	var res RecordResult
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()
	rid := rep.App + ":" + rep.ID
	err = tx.QueryRowContext(ctx, `SELECT e.group_id, g.count, g.status FROM notification_events e JOIN notifications g ON g.id=e.group_id WHERE e.report_id=?`, rid).
		Scan(&res.GroupID, &res.Count, &res.Status)
	switch {
	case err == nil:
		res.Duplicate, res.OldCount = true, res.Count
		return res, nil
	case !errors.Is(err, sql.ErrNoRows):
		return res, err
	}
	sample, _ := json.Marshal(Sample{Message: rep.Message, Origin: rep.Origin, Route: rep.Route, Method: rep.Method, Version: rep.Version, Ref: rep.Ref})
	at := rep.At.Unix()
	fp := rep.fingerprint()
	var status string
	var count int64
	var regression bool
	err = tx.QueryRowContext(ctx, `SELECT id,status,count,regression FROM notifications WHERE fingerprint=?`, fp).Scan(&res.GroupID, &status, &count, &regression)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := d.makeRoom(ctx, tx, rep.App); err != nil {
			return res, err
		}
		r, err := tx.ExecContext(ctx, `INSERT INTO notifications (kind,app,env,fingerprint,title,first_seen,last_seen,count,status,sample_json)
			VALUES (?,?,?,?,?,?,?,1,'new',?)`, rep.Kind, rep.App, rep.Env, fp, rep.Class, at, at, string(sample))
		if err != nil {
			return res, err
		}
		if res.GroupID, err = r.LastInsertId(); err != nil {
			return res, err
		}
		res.NewGroup, res.OldCount, res.Count, res.Status = true, 0, 1, NotifNew
	case err != nil:
		return res, err
	default:
		res.OldCount, res.Count, res.Status = count, count+1, status
		if status == NotifResolved {
			res.Status, res.Regression, regression = NotifNew, true, true
		}
		if _, err := tx.ExecContext(ctx, `UPDATE notifications SET count=count+1,
			first_seen=MIN(first_seen,?),
			sample_json=CASE WHEN ?>=last_seen THEN ? ELSE sample_json END,
			last_seen=MAX(last_seen,?), status=?, regression=?, resolved_at=CASE WHEN ?='new' THEN 0 ELSE resolved_at END
			WHERE id=?`, at, at, string(sample), at, res.Status, regression, res.Status, res.GroupID); err != nil {
			return res, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notification_events (group_id,report_id,at,ref,version) VALUES (?,?,?,?,?)`,
		res.GroupID, rid, at, rep.Ref, rep.Version); err != nil {
		return res, err
	}
	keep := d.Limits.KeepEvents
	if keep <= 0 {
		keep = DefaultNotifLimits.KeepEvents
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_events WHERE group_id=? AND id NOT IN
		(SELECT id FROM notification_events WHERE group_id=? ORDER BY at DESC, id DESC LIMIT ?)`, res.GroupID, res.GroupID, keep); err != nil {
		return res, err
	}
	return res, tx.Commit()
}

// makeRoom keeps the group tables within their caps by dropping the least
// recently seen group that is not "new"; with none to drop, the report is refused.
func (d *DB) makeRoom(ctx context.Context, tx *sql.Tx, app string) error {
	lim := d.Limits
	if lim.MaxGroups <= 0 {
		lim.MaxGroups = DefaultNotifLimits.MaxGroups
	}
	if lim.MaxGroupsPerApp <= 0 {
		lim.MaxGroupsPerApp = DefaultNotifLimits.MaxGroupsPerApp
	}
	for _, scope := range []struct {
		where string
		args  []any
		max   int
	}{{"app=?", []any{app}, lim.MaxGroupsPerApp}, {"1=1", nil, lim.MaxGroups}} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE `+scope.where, scope.args...).Scan(&n); err != nil {
			return err
		}
		if n < scope.max {
			continue
		}
		args := append(append([]any{}, scope.args...), NotifNew)
		r, err := tx.ExecContext(ctx, `DELETE FROM notifications WHERE id=(SELECT id FROM notifications WHERE `+scope.where+` AND status<>? ORDER BY last_seen ASC, id ASC LIMIT 1)`, args...)
		if err != nil {
			return err
		}
		if gone, _ := r.RowsAffected(); gone == 0 {
			return ErrInboxFull
		}
		log.Printf("notifications: inbox at its cap, dropped the least recent non-new group (%s)", app)
	}
	return nil
}

// BumpSuppressed counts a report that was refused by the rate limit.
func (d *DB) BumpSuppressed(ctx context.Context, fingerprint string) error {
	_, err := d.ExecContext(ctx, `UPDATE notifications SET suppressed=suppressed+1 WHERE fingerprint=?`, fingerprint)
	return err
}

// ClaimAlert returns true at most once per cooldown for a group that is still
// "new": the caller then sends the one Telegram message.
func (d *DB) ClaimAlert(ctx context.Context, id int64, now time.Time, cooldown time.Duration) (bool, error) {
	r, err := d.ExecContext(ctx, `UPDATE notifications SET notified_at=? WHERE id=? AND status='new' AND notified_at<=?`, now.Unix(), id, now.Add(-cooldown).Unix())
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}

// NotifScope says which apps' groups a viewer may see.
type NotifScope struct {
	All  bool
	Apps []string
}

func (s NotifScope) clause() (string, []any) {
	if s.All {
		return "1=1", nil
	}
	if len(s.Apps) == 0 {
		return "1=0", nil
	}
	args := make([]any, len(s.Apps))
	for i, a := range s.Apps {
		args[i] = a
	}
	return "app IN (?" + strings.Repeat(",?", len(s.Apps)-1) + ")", args
}

const notifCols = `id,kind,app,env,fingerprint,title,first_seen,last_seen,count,suppressed,status,regression,resolved_at,sample_json`

type rowScanner interface{ Scan(dest ...any) error }

func scanNotification(r rowScanner) (*Notification, error) {
	var n Notification
	var first, last, resolved int64
	var sample string
	if err := r.Scan(&n.ID, &n.Kind, &n.App, &n.Env, &n.Fingerprint, &n.Title, &first, &last, &n.Count, &n.Suppressed, &n.Status, &n.Regression, &resolved, &sample); err != nil {
		return nil, err
	}
	n.FirstSeen, n.LastSeen = time.Unix(first, 0), time.Unix(last, 0)
	if resolved > 0 {
		n.ResolvedAt = time.Unix(resolved, 0)
	}
	_ = json.Unmarshal([]byte(sample), &n.Sample)
	return &n, nil
}

// NotifFilter narrows a listing. Status "" or "all" means every status.
type NotifFilter struct {
	App    string
	Status string
	Limit  int
	Offset int
}

func (f NotifFilter) where(scope NotifScope) (string, []any) {
	w, args := scope.clause()
	if f.App != "" {
		w += " AND app=?"
		args = append(args, f.App)
	}
	if f.Status != "" && f.Status != "all" {
		w += " AND status=?"
		args = append(args, f.Status)
	}
	return w, args
}

// ListNotifications returns the newest groups first, and the total matching.
func (d *DB) ListNotifications(ctx context.Context, scope NotifScope, f NotifFilter) ([]Notification, int, error) {
	w, args := f.where(scope)
	var total int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	rows, err := d.QueryContext(ctx, `SELECT `+notifCols+` FROM notifications WHERE `+w+` ORDER BY last_seen DESC, id DESC LIMIT ? OFFSET ?`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *n)
	}
	return out, total, rows.Err()
}

// Notification loads one group (ErrNotFound when missing).
func (d *DB) Notification(ctx context.Context, id int64) (*Notification, error) {
	n, err := scanNotification(d.QueryRowContext(ctx, `SELECT `+notifCols+` FROM notifications WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

// NotificationEvents lists the stored occurrences of a group, newest first.
func (d *DB) NotificationEvents(ctx context.Context, id int64) ([]NotificationEvent, error) {
	rows, err := d.QueryContext(ctx, `SELECT id,at,ref,version FROM notification_events WHERE group_id=? ORDER BY at DESC, id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotificationEvent
	for rows.Next() {
		var e NotificationEvent
		var at int64
		if err := rows.Scan(&e.ID, &at, &e.Ref, &e.Version); err != nil {
			return nil, err
		}
		e.At = time.Unix(at, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountNew is the number of "new" groups the viewer may see (the nav badge).
func (d *DB) CountNew(ctx context.Context, scope NotifScope) (int, error) {
	w, args := scope.clause()
	var n int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE status='new' AND `+w, args...).Scan(&n)
	return n, err
}

// NotificationStatusCounts counts groups per status, optionally for one app.
func (d *DB) NotificationStatusCounts(ctx context.Context, scope NotifScope, app string) (map[string]int, error) {
	w, args := NotifFilter{App: app}.where(scope)
	rows, err := d.QueryContext(ctx, `SELECT status,COUNT(*) FROM notifications WHERE `+w+` GROUP BY status`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

// NotificationApps lists the apps that have groups the viewer may see.
func (d *DB) NotificationApps(ctx context.Context, scope NotifScope) ([]string, error) {
	w, args := scope.clause()
	rows, err := d.QueryContext(ctx, `SELECT DISTINCT app FROM notifications WHERE `+w+` ORDER BY app`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

var transitions = map[string]struct {
	from []string
	to   string
}{
	"ack":     {[]string{NotifNew}, NotifSeen},
	"resolve": {[]string{NotifNew, NotifSeen, NotifMuted}, NotifResolved},
	"mute":    {[]string{NotifNew, NotifSeen}, NotifMuted},
	"reopen":  {[]string{NotifSeen, NotifResolved, NotifMuted}, NotifNew},
}

// NotificationActions are the actions ChangeNotification accepts.
func NotificationActions() []string { return []string{"ack", "resolve", "mute", "reopen"} }

// ChangeNotification moves a group to a new status and writes the audit row in
// the same transaction: when the audit insert fails, nothing changes.
// au carries the actor and IP; App, Action, Target and Detail are filled here.
func (d *DB) ChangeNotification(ctx context.Context, id int64, action, reason string, au Audit) (*Notification, error) {
	t, ok := transitions[action]
	if !ok {
		return nil, ErrBadTransition
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	n, err := scanNotification(tx.QueryRowContext(ctx, `SELECT `+notifCols+` FROM notifications WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, f := range t.from {
		allowed = allowed || f == n.Status
	}
	if !allowed {
		return n, ErrBadTransition
	}
	resolvedAt := int64(0)
	if t.to == NotifResolved {
		resolvedAt = time.Now().Unix()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE notifications SET status=?, regression=0, resolved_at=?, notified_at=CASE WHEN ?='new' THEN 0 ELSE notified_at END WHERE id=?`, t.to, resolvedAt, t.to, id); err != nil {
		return nil, err
	}
	au.App, au.Action, au.Target, au.OK = n.App, "notification."+action, fmt.Sprint(id), true
	au.Detail = map[string]any{"reason": reason, "from": n.Status, "to": t.to, "class": n.Title, "fingerprint": n.Fingerprint[:12]}
	if err := insertAudit(ctx, tx, &au); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	n.Status = t.to
	return n, nil
}

// PruneNotifications applies the retention policy and returns what it removed.
func (d *DB) PruneNotifications(ctx context.Context, now time.Time) (events, groups int64, err error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `DELETE FROM notification_events WHERE at<?`, now.Add(-EventRetention).Unix())
	if err != nil {
		return 0, 0, err
	}
	events, _ = r.RowsAffected()
	r, err = tx.ExecContext(ctx, `DELETE FROM notifications WHERE last_seen<? OR (status='resolved' AND last_seen<?)`,
		now.Add(-GroupRetention).Unix(), now.Add(-ResolvedGroupRetention).Unix())
	if err != nil {
		return events, 0, err
	}
	groups, _ = r.RowsAffected()
	return events, groups, tx.Commit()
}

// CanTransition says whether a group in `status` accepts `action`.
func CanTransition(status, action string) bool {
	t, ok := transitions[action]
	if !ok {
		return false
	}
	for _, f := range t.from {
		if f == status {
			return true
		}
	}
	return false
}
