package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func rep(id string, mod func(*Report)) Report {
	r := Report{ID: id, Kind: "error.500", App: "linkhop", Env: "production", At: time.Now().UTC().Truncate(time.Second),
		Class: `Illuminate\Database\QueryException`, Message: "SQLSTATE[HY000]", Origin: "app/Http/Controllers/LinkController.php:88",
		Route: "GET /l/{code}", Method: "GET", Ref: "7f3a9c1e2b4d", Version: "abc1234"}
	if mod != nil {
		mod(&r)
	}
	return r
}

func record(t *testing.T, d *DB, r Report) RecordResult {
	t.Helper()
	res, err := d.RecordNotification(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestMigrationFromTheOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	// A database as production has it today: the base tables, no schema_version, no own_secret.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO apps (slug,name,url,secret) VALUES ('pixelhop','PixelHop','https://p','old-secret')`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var v int
	if err := d.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil || v != SchemaVersion() {
		t.Fatalf("schema version %d err %v", v, err)
	}
	a, err := d.App(ctx, "pixelhop")
	if err != nil || a.Secret != "old-secret" || a.OwnSecret != "" {
		t.Fatalf("existing app row must survive: %+v %v", a, err)
	}
	record(t, d, rep("aaaaaaaa", nil))
	d.Close()

	// Opening again must not run anything twice or lose data.
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var rows int
	_ = d.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&rows)
	if rows != len(migrations) {
		t.Fatalf("migrations recorded %d times", rows)
	}
	if n, _ := d.CountNew(ctx, NotifScope{All: true}); n != 1 {
		t.Fatalf("group lost on reopen: %d", n)
	}
}

func TestOwnSecretIsKeptWhenTheRegistryOmitsIt(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	if err := d.UpsertApp(ctx, &App{Slug: "biohop", Name: "BioHop", URL: "https://b", Secret: "imperium-side", OwnSecret: "bio-own", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := d.UpsertApp(ctx, &App{Slug: "biohop", Name: "BioHop", URL: "https://b", Secret: "", OwnSecret: "", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	s, err := d.OwnSecret(ctx, "biohop")
	if err != nil || s != "bio-own" {
		t.Fatalf("own secret %q %v", s, err)
	}
	if s, err := d.OwnSecret(ctx, "nobody"); err != nil || s != "" {
		t.Fatalf("unknown app: %q %v", s, err)
	}
}

func TestGroupingByFingerprint(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	r1 := record(t, d, rep("id000001", func(r *Report) { r.At = t0 }))
	if !r1.NewGroup || r1.Count != 1 || r1.Status != NotifNew {
		t.Fatalf("first: %+v", r1)
	}
	r2 := record(t, d, rep("id000002", func(r *Report) { r.At = t0.Add(10 * time.Minute); r.Ref = "aaaaaaaaaaaa"; r.Message = "newer" }))
	if r2.NewGroup || r2.GroupID != r1.GroupID || r2.Count != 2 || r2.OldCount != 1 {
		t.Fatalf("second: %+v", r2)
	}
	// An older report arriving late widens first_seen but not last_seen or the sample.
	record(t, d, rep("id000003", func(r *Report) { r.At = t0.Add(-5 * time.Minute); r.Message = "late" }))
	n, err := d.Notification(ctx, r1.GroupID)
	if err != nil {
		t.Fatal(err)
	}
	if n.Count != 3 || !n.FirstSeen.Equal(t0.Add(-5*time.Minute)) || !n.LastSeen.Equal(t0.Add(10*time.Minute)) || n.Sample.Message != "newer" {
		t.Fatalf("group: %+v", n)
	}
	// Each of app, env, class, origin and route makes a different group.
	for i, mod := range []func(*Report){
		func(r *Report) { r.App = "biohop" },
		func(r *Report) { r.Env = "staging" },
		func(r *Report) { r.Class = "panic" },
		func(r *Report) { r.Origin = "app/Other.php:1" },
		func(r *Report) { r.Route = "GET /b/{slug}" },
	} {
		res := record(t, d, rep(fmt.Sprintf("diff%04d", i), mod))
		if !res.NewGroup || res.GroupID == r1.GroupID {
			t.Fatalf("variant %d should be a new group: %+v", i, res)
		}
	}
	// Version, ref, method and message are not part of the fingerprint.
	res := record(t, d, rep("same0001", func(r *Report) { r.Version = "zzz"; r.Ref = "bbbbbbbbbbbb"; r.Method = "POST"; r.Message = "x" }))
	if res.NewGroup || res.GroupID != r1.GroupID {
		t.Fatalf("same fingerprint expected: %+v", res)
	}
}

func TestReportIDIsIdempotentPerApp(t *testing.T) {
	d := open(t)
	a := record(t, d, rep("sameid01", nil))
	b := record(t, d, rep("sameid01", nil))
	if !b.Duplicate || b.GroupID != a.GroupID || b.Count != 1 {
		t.Fatalf("duplicate: %+v", b)
	}
	n, _ := d.Notification(context.Background(), a.GroupID)
	if n.Count != 1 {
		t.Fatalf("count grew on a duplicate: %d", n.Count)
	}
	// The same id from another app is another report.
	c := record(t, d, rep("sameid01", func(r *Report) { r.App = "biohop" }))
	if c.Duplicate {
		t.Fatal("ids are scoped to the sending app")
	}
}

func TestOnlyTheNewestOccurrencesAreKept(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	var gid int64
	for i := 0; i < 27; i++ {
		gid = record(t, d, rep(fmt.Sprintf("occ%05d", i), func(r *Report) { r.At = base.Add(time.Duration(i) * time.Second); r.Ref = fmt.Sprintf("%012x", i) })).GroupID
	}
	ev, err := d.NotificationEvents(ctx, gid)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 20 || ev[0].Ref != fmt.Sprintf("%012x", 26) || ev[19].Ref != fmt.Sprintf("%012x", 7) {
		t.Fatalf("kept %d, newest %s oldest %s", len(ev), ev[0].Ref, ev[len(ev)-1].Ref)
	}
	n, _ := d.Notification(ctx, gid)
	if n.Count != 27 {
		t.Fatalf("count must keep counting past the cap: %d", n.Count)
	}
}

func TestResolvedGroupReopensAsRegressionAndOthersKeepTheirStatus(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	au := Audit{ActorSub: "1", ActorName: "boss"}
	res := record(t, d, rep("reg00001", nil))
	if _, err := d.ChangeNotification(ctx, res.GroupID, "resolve", "fixed in abc", au); err != nil {
		t.Fatal(err)
	}
	back := record(t, d, rep("reg00002", nil))
	if !back.Regression || back.Status != NotifNew || back.NewGroup {
		t.Fatalf("regression: %+v", back)
	}
	n, _ := d.Notification(ctx, res.GroupID)
	if n.Status != NotifNew || !n.Regression || !n.ResolvedAt.IsZero() {
		t.Fatalf("group after regression: %+v", n)
	}
	// ack clears the regression mark; further occurrences keep "seen".
	if _, err := d.ChangeNotification(ctx, res.GroupID, "ack", "", au); err != nil {
		t.Fatal(err)
	}
	again := record(t, d, rep("reg00003", nil))
	if again.Status != NotifSeen || again.Regression {
		t.Fatalf("seen group: %+v", again)
	}
	// Muted groups still count.
	if _, err := d.ChangeNotification(ctx, res.GroupID, "mute", "noisy bot", au); err != nil {
		t.Fatal(err)
	}
	m := record(t, d, rep("reg00004", nil))
	n, _ = d.Notification(ctx, res.GroupID)
	if m.Status != NotifMuted || n.Status != NotifMuted || n.Count != 4 {
		t.Fatalf("muted group: %+v %+v", m, n)
	}
}

func TestStatusTransitionsAndAuditInTheSameTransaction(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	gid := record(t, d, rep("tr000001", nil)).GroupID
	au := Audit{ActorSub: "1", ActorName: "boss", IP: "100.64.0.1"}
	for _, bad := range []string{"reopen", "nonsense"} { // a new group cannot be reopened
		if _, err := d.ChangeNotification(ctx, gid, bad, "", au); !errors.Is(err, ErrBadTransition) {
			t.Fatalf("%s: %v", bad, err)
		}
	}
	if _, err := d.ChangeNotification(ctx, 9999, "ack", "", au); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing group: %v", err)
	}
	if _, err := d.ChangeNotification(ctx, gid, "ack", "", au); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ChangeNotification(ctx, gid, "ack", "", au); !errors.Is(err, ErrBadTransition) {
		t.Fatalf("ack twice: %v", err)
	}
	audits, _ := d.Audits(ctx, 0, 10)
	if len(audits) != 1 || audits[0].Action != "notification.ack" || audits[0].App != "linkhop" || audits[0].Target != fmt.Sprint(gid) || audits[0].ActorName != "boss" || audits[0].IP != "100.64.0.1" || !audits[0].OK {
		t.Fatalf("audit: %+v", audits)
	}
	if audits[0].Detail["from"] != "new" || audits[0].Detail["to"] != "seen" {
		t.Fatalf("audit detail: %+v", audits[0].Detail)
	}

	// When the audit row cannot be written, the status must not change and the error must come back.
	if _, err := d.Exec(`CREATE TRIGGER audit_fail BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT, 'disk full'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := d.ChangeNotification(ctx, gid, "resolve", "x", au)
	if err == nil || errors.Is(err, ErrBadTransition) {
		t.Fatalf("a failing audit insert must surface: %v", err)
	}
	n, _ := d.Notification(ctx, gid)
	if n.Status != NotifSeen {
		t.Fatalf("status changed without an audit row: %s", n.Status)
	}
}

func TestRetention(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	day := 24 * time.Hour
	mk := func(id, route string, at time.Time) int64 {
		return record(t, d, rep(id, func(r *Report) { r.Route = route; r.At = at })).GroupID
	}
	fresh := mk("ret00001", "GET /fresh", now.Add(-time.Hour))
	oldEvents := mk("ret00002", "GET /mixed", now.Add(-40*day))
	record(t, d, rep("ret00003", func(r *Report) { r.Route = "GET /mixed"; r.At = now.Add(-2 * day) })) // keeps the group alive
	gone := mk("ret00004", "GET /ancient", now.Add(-100*day))
	resolvedOld := mk("ret00005", "GET /resolved", now.Add(-35*day))
	resolvedFresh := mk("ret00006", "GET /resolved2", now.Add(-5*day))
	mutedMid := mk("ret00007", "GET /muted", now.Add(-60*day))
	au := Audit{ActorSub: "1", ActorName: "boss"}
	for _, id := range []int64{resolvedOld, resolvedFresh} {
		if _, err := d.ChangeNotification(ctx, id, "resolve", "", au); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.ChangeNotification(ctx, mutedMid, "mute", "bot", au); err != nil {
		t.Fatal(err)
	}
	ev, gr, err := d.PruneNotifications(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if gr != 2 {
		t.Fatalf("groups removed: %d (want the 100-day group and the resolved 35-day group)", gr)
	}
	if ev < 1 {
		t.Fatalf("events removed: %d", ev)
	}
	for id, want := range map[int64]bool{fresh: true, oldEvents: true, gone: false, resolvedOld: false, resolvedFresh: true, mutedMid: true} {
		_, err := d.Notification(ctx, id)
		if (err == nil) != want {
			t.Fatalf("group %d present=%v want %v", id, err == nil, want)
		}
	}
	// The mixed group keeps only its recent occurrence; the deleted groups left no orphans.
	if e, _ := d.NotificationEvents(ctx, oldEvents); len(e) != 1 {
		t.Fatalf("mixed group events: %d", len(e))
	}
	var orphans int
	_ = d.QueryRow(`SELECT COUNT(*) FROM notification_events WHERE group_id NOT IN (SELECT id FROM notifications)`).Scan(&orphans)
	if orphans != 0 {
		t.Fatalf("orphan events: %d", orphans)
	}
}

func TestInboxCaps(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	d.Limits = NotifLimits{KeepEvents: 20, MaxGroups: 3, MaxGroupsPerApp: 2}
	au := Audit{ActorSub: "1", ActorName: "boss"}
	now := time.Now().UTC().Truncate(time.Second)
	g := func(id, route string, off time.Duration) (RecordResult, error) {
		return d.RecordNotification(ctx, rep(id, func(r *Report) { r.Route = route; r.At = now.Add(off) }))
	}
	a, _ := g("cap00001", "GET /a", -3*time.Minute)
	if _, err := g("cap00002", "GET /b", -2*time.Minute); err != nil {
		t.Fatal(err)
	}
	// The app is at its cap of 2 and both groups are "new": nothing may be dropped.
	if _, err := g("cap00003", "GET /c", -time.Minute); !errors.Is(err, ErrInboxFull) {
		t.Fatalf("want ErrInboxFull, got %v", err)
	}
	// Acknowledged groups can make room: the least recent non-new group goes.
	if _, err := d.ChangeNotification(ctx, a.GroupID, "ack", "", au); err != nil {
		t.Fatal(err)
	}
	if _, err := g("cap00004", "GET /d", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Notification(ctx, a.GroupID); !errors.Is(err, ErrNotFound) {
		t.Fatal("the acknowledged group should have been dropped")
	}
	// The global cap counts all apps.
	if _, err := d.RecordNotification(ctx, rep("cap00005", func(r *Report) { r.App = "biohop" })); err != nil {
		t.Fatal(err)
	}
	if _, err := d.RecordNotification(ctx, rep("cap00006", func(r *Report) { r.App = "pixelhop" })); !errors.Is(err, ErrInboxFull) {
		t.Fatalf("global cap: %v", err)
	}
}

func TestSuppressedAndAlertCooldown(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	r := rep("sup00001", nil)
	gid := record(t, d, r).GroupID
	if err := d.BumpSuppressed(ctx, Fingerprint(r.App, r.Env, r.Class, r.Origin, r.Route)); err != nil {
		t.Fatal(err)
	}
	if n, _ := d.Notification(ctx, gid); n.Suppressed != 1 || n.Count != 1 {
		t.Fatalf("suppressed counter: %+v", n)
	}
	now := time.Now()
	if ok, _ := d.ClaimAlert(ctx, gid, now, time.Hour); !ok {
		t.Fatal("first claim should succeed")
	}
	if ok, _ := d.ClaimAlert(ctx, gid, now.Add(59*time.Minute), time.Hour); ok {
		t.Fatal("second claim inside the cooldown must fail")
	}
	if ok, _ := d.ClaimAlert(ctx, gid, now.Add(61*time.Minute), time.Hour); !ok {
		t.Fatal("claim after the cooldown should succeed")
	}
	au := Audit{ActorSub: "1", ActorName: "boss"}
	if _, err := d.ChangeNotification(ctx, gid, "mute", "bot", au); err != nil {
		t.Fatal(err)
	}
	if ok, _ := d.ClaimAlert(ctx, gid, now.Add(5*time.Hour), time.Hour); ok {
		t.Fatal("a muted group never alerts")
	}
}

func TestScopeAndListing(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	for i, app := range []string{"linkhop", "linkhop", "pixelhop", "biohop"} {
		record(t, d, rep(fmt.Sprintf("scp0000%d", i), func(r *Report) { r.App = app; r.Route = fmt.Sprintf("GET /r%d", i) }))
	}
	all := NotifScope{All: true}
	px := NotifScope{Apps: []string{"pixelhop"}}
	none := NotifScope{}
	if n, _ := d.CountNew(ctx, all); n != 4 {
		t.Fatalf("all: %d", n)
	}
	if n, _ := d.CountNew(ctx, px); n != 1 {
		t.Fatalf("pixelhop: %d", n)
	}
	if n, _ := d.CountNew(ctx, none); n != 0 {
		t.Fatalf("none: %d", n)
	}
	rows, total, err := d.ListNotifications(ctx, px, NotifFilter{App: "linkhop"})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("a scoped viewer must not reach another app by filter: %d %v", total, err)
	}
	rows, total, _ = d.ListNotifications(ctx, all, NotifFilter{App: "linkhop", Status: "new", Limit: 1})
	if total != 2 || len(rows) != 1 {
		t.Fatalf("list: %d/%d", len(rows), total)
	}
	apps, _ := d.NotificationApps(ctx, px)
	if len(apps) != 1 || apps[0] != "pixelhop" {
		t.Fatalf("apps: %v", apps)
	}
	counts, _ := d.NotificationStatusCounts(ctx, all, "linkhop")
	if counts["new"] != 2 {
		t.Fatalf("counts: %v", counts)
	}
}
