package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// The base schema in db.go is CREATE TABLE IF NOT EXISTS only. Everything that
// changes an existing table, or adds a table after the first release, is a
// numbered migration here. Each one runs once, in its own transaction, and is
// recorded in schema_version. All migrations are additive, so an older binary
// keeps working on a migrated database.

type migration struct {
	version int
	name    string
	up      func(ctx context.Context, tx *sql.Tx) error
}

var migrations = []migration{
	{1, "notifications", migrateNotifications},
}

// SchemaVersion is the newest migration this binary knows.
func SchemaVersion() int { return migrations[len(migrations)-1].version }

func migrate(ctx context.Context, d *sql.DB) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	var cur sql.NullInt64
	if err := d.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_version`).Scan(&cur); err != nil {
		return err
	}
	for _, m := range migrations {
		if int64(m.version) <= cur.Int64 {
			continue
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := m.up(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%d %s: %w", m.version, m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version,name,applied_at) VALUES (?,?,?)`, m.version, m.name, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, col string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return false, err
		}
		if n == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

func migrateNotifications(ctx context.Context, tx *sql.Tx) error {
	has, err := hasColumn(ctx, tx, "apps", "own_secret")
	if err != nil {
		return err
	}
	if !has {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE apps ADD COLUMN own_secret TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS notifications (
	id INTEGER PRIMARY KEY,
	kind TEXT NOT NULL,
	app TEXT NOT NULL,
	env TEXT NOT NULL DEFAULT '',
	fingerprint TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	first_seen INTEGER NOT NULL,
	last_seen INTEGER NOT NULL,
	count INTEGER NOT NULL DEFAULT 1,
	suppressed INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','seen','resolved','muted')),
	regression INTEGER NOT NULL DEFAULT 0,
	resolved_at INTEGER NOT NULL DEFAULT 0,
	notified_at INTEGER NOT NULL DEFAULT 0,
	sample_json TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS notifications_status ON notifications(status, last_seen);
CREATE INDEX IF NOT EXISTS notifications_app ON notifications(app, last_seen);
CREATE INDEX IF NOT EXISTS notifications_last ON notifications(last_seen);
CREATE TABLE IF NOT EXISTS notification_events (
	id INTEGER PRIMARY KEY,
	group_id INTEGER NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
	report_id TEXT NOT NULL UNIQUE,
	at INTEGER NOT NULL,
	ref TEXT NOT NULL DEFAULT '',
	version TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS notification_events_group ON notification_events(group_id, at);
CREATE INDEX IF NOT EXISTS notification_events_at ON notification_events(at);
`)
	return err
}
