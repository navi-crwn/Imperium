// Package db is Imperium's own small store: sessions, the local audit trail,
// a cached copy of the HOP app registry, and settings. SQLite, pure Go.
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	sub TEXT NOT NULL,
	username TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	avatar TEXT NOT NULL DEFAULT '',
	roles TEXT NOT NULL DEFAULT '[]',
	mfa INTEGER NOT NULL DEFAULT 0,
	ip TEXT NOT NULL DEFAULT '',
	ua TEXT NOT NULL DEFAULT '',
	created INTEGER NOT NULL,
	expires INTEGER NOT NULL,
	last_seen INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_sub ON sessions(sub);
CREATE TABLE IF NOT EXISTS audit (
	id INTEGER PRIMARY KEY,
	at INTEGER NOT NULL,
	actor_sub TEXT NOT NULL,
	actor_name TEXT NOT NULL,
	ip TEXT NOT NULL DEFAULT '',
	app TEXT NOT NULL DEFAULT '',
	action TEXT NOT NULL,
	target TEXT NOT NULL DEFAULT '',
	detail TEXT NOT NULL DEFAULT '{}',
	ok INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS audit_at ON audit(at);
CREATE TABLE IF NOT EXISTS apps (
	slug TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	prefix TEXT NOT NULL DEFAULT '/hop',
	secret TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	icon TEXT NOT NULL DEFAULT '',
	color TEXT NOT NULL DEFAULT '',
	menu_label TEXT NOT NULL DEFAULT '',
	capabilities TEXT NOT NULL DEFAULT '[]',
	hidden INTEGER NOT NULL DEFAULT 0,
	sort_order INTEGER NOT NULL DEFAULT 100,
	synced_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
`

func Open(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(schema); err != nil {
		return nil, err
	}
	return &DB{d}, nil
}

// ---- sessions ----

type Session struct {
	TokenHash string
	Sub       string
	Username  string
	Name      string
	Email     string
	Avatar    string
	Roles     []string
	MFA       bool
	IP, UA    string
	Created   time.Time
	Expires   time.Time
	LastSeen  time.Time
}

func (d *DB) CreateSession(ctx context.Context, s *Session) error {
	roles, _ := json.Marshal(s.Roles)
	_, err := d.ExecContext(ctx, `INSERT INTO sessions (token_hash,sub,username,name,email,avatar,roles,mfa,ip,ua,created,expires,last_seen)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.TokenHash, s.Sub, s.Username, s.Name, s.Email, s.Avatar, string(roles), s.MFA, s.IP, s.UA,
		s.Created.Unix(), s.Expires.Unix(), s.LastSeen.Unix())
	return err
}

func (d *DB) Session(ctx context.Context, tokenHash string) (*Session, error) {
	var s Session
	var roles string
	var created, expires, last int64
	err := d.QueryRowContext(ctx, `SELECT token_hash,sub,username,name,email,avatar,roles,mfa,ip,ua,created,expires,last_seen FROM sessions WHERE token_hash=? AND expires>?`,
		tokenHash, time.Now().Unix()).Scan(&s.TokenHash, &s.Sub, &s.Username, &s.Name, &s.Email, &s.Avatar, &roles, &s.MFA, &s.IP, &s.UA, &created, &expires, &last)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(roles), &s.Roles)
	s.Created, s.Expires, s.LastSeen = time.Unix(created, 0), time.Unix(expires, 0), time.Unix(last, 0)
	return &s, nil
}

func (d *DB) TouchSession(ctx context.Context, tokenHash string) {
	_, _ = d.ExecContext(ctx, `UPDATE sessions SET last_seen=? WHERE token_hash=?`, time.Now().Unix(), tokenHash)
}

func (d *DB) DeleteSession(ctx context.Context, tokenHash string) {
	_, _ = d.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, tokenHash)
}

func (d *DB) DeleteSessionsOf(ctx context.Context, sub string) {
	_, _ = d.ExecContext(ctx, `DELETE FROM sessions WHERE sub=?`, sub)
}

func (d *DB) Sweep(ctx context.Context) {
	_, _ = d.ExecContext(ctx, `DELETE FROM sessions WHERE expires<?`, time.Now().Unix())
}

// ---- audit ----

type Audit struct {
	ID        int64
	At        time.Time
	ActorSub  string
	ActorName string
	IP        string
	App       string
	Action    string
	Target    string
	Detail    map[string]any
	OK        bool
}

func (d *DB) AddAudit(ctx context.Context, a *Audit) {
	detail, _ := json.Marshal(a.Detail)
	if a.Detail == nil {
		detail = []byte("{}")
	}
	_, _ = d.ExecContext(ctx, `INSERT INTO audit (at,actor_sub,actor_name,ip,app,action,target,detail,ok) VALUES (?,?,?,?,?,?,?,?,?)`,
		time.Now().Unix(), a.ActorSub, a.ActorName, a.IP, a.App, a.Action, a.Target, string(detail), a.OK)
}

func (d *DB) Audits(ctx context.Context, before int64, limit int) ([]Audit, error) {
	if before <= 0 {
		before = 1 << 62
	}
	rows, err := d.QueryContext(ctx, `SELECT id,at,actor_sub,actor_name,ip,app,action,target,detail,ok FROM audit WHERE id<? ORDER BY id DESC LIMIT ?`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Audit
	for rows.Next() {
		var a Audit
		var at int64
		var detail string
		if err := rows.Scan(&a.ID, &at, &a.ActorSub, &a.ActorName, &a.IP, &a.App, &a.Action, &a.Target, &detail, &a.OK); err != nil {
			return nil, err
		}
		a.At = time.Unix(at, 0)
		_ = json.Unmarshal([]byte(detail), &a.Detail)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---- apps (cached registry) ----

type App struct {
	Slug, Name, URL, Prefix, Secret, Status, Icon, Color, MenuLabel string
	Capabilities                                                    []string
	Hidden                                                          bool
	SortOrder                                                       int
	SyncedAt                                                        time.Time
}

func (a App) Can(c string) bool {
	for _, x := range a.Capabilities {
		if x == c {
			return true
		}
	}
	return false
}

func (d *DB) UpsertApp(ctx context.Context, a *App) error {
	caps, _ := json.Marshal(a.Capabilities)
	_, err := d.ExecContext(ctx, `INSERT INTO apps (slug,name,url,prefix,secret,status,icon,color,menu_label,capabilities,hidden,sort_order,synced_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(slug) DO UPDATE SET name=excluded.name,url=excluded.url,prefix=excluded.prefix,
		  secret=CASE WHEN excluded.secret='' THEN apps.secret ELSE excluded.secret END,
		  status=excluded.status,icon=excluded.icon,color=excluded.color,menu_label=excluded.menu_label,
		  capabilities=excluded.capabilities,hidden=excluded.hidden,sort_order=excluded.sort_order,synced_at=excluded.synced_at`,
		a.Slug, a.Name, a.URL, a.Prefix, a.Secret, a.Status, a.Icon, a.Color, a.MenuLabel, string(caps), a.Hidden, a.SortOrder, time.Now().Unix())
	return err
}

func (d *DB) Apps(ctx context.Context) ([]App, error) {
	rows, err := d.QueryContext(ctx, `SELECT slug,name,url,prefix,secret,status,icon,color,menu_label,capabilities,hidden,sort_order,synced_at FROM apps ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var caps string
		var synced int64
		if err := rows.Scan(&a.Slug, &a.Name, &a.URL, &a.Prefix, &a.Secret, &a.Status, &a.Icon, &a.Color, &a.MenuLabel, &caps, &a.Hidden, &a.SortOrder, &synced); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(caps), &a.Capabilities)
		a.SyncedAt = time.Unix(synced, 0)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (d *DB) App(ctx context.Context, slug string) (*App, error) {
	apps, err := d.Apps(ctx)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		if apps[i].Slug == slug {
			return &apps[i], nil
		}
	}
	return nil, sql.ErrNoRows
}

// ---- settings ----

func (d *DB) Setting(ctx context.Context, key, def string) string {
	var v string
	if err := d.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

func (d *DB) SetSetting(ctx context.Context, key, value string) {
	_, _ = d.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
}
