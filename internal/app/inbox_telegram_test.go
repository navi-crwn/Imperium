package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

const tgToken = "123456789:AAH-test-token-that-must-never-be-logged"

// fakeTelegram records every sendMessage call.
type fakeTelegram struct {
	srv  *httptest.Server
	msgs chan url.Values
	path chan string
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	f := &fakeTelegram{msgs: make(chan url.Values, 32), path: make(chan string, 32)}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		v, _ := url.ParseQuery(string(b))
		f.path <- r.URL.Path
		f.msgs <- v
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTelegram) expect(t *testing.T) string {
	t.Helper()
	select {
	case v := <-f.msgs:
		return v.Get("text")
	case <-time.After(3 * time.Second):
		t.Fatal("expected a Telegram message, none came")
		return ""
	}
}

func (f *fakeTelegram) expectNone(t *testing.T, why string) {
	t.Helper()
	select {
	case v := <-f.msgs:
		t.Fatalf("unexpected Telegram message (%s): %q", why, v.Get("text"))
	case <-time.After(300 * time.Millisecond):
	}
}

func telegramApp(t *testing.T) (*App, *httptest.Server, *fakeTelegram) {
	a, srv, _ := newInbox(t)
	tg := newFakeTelegram(t)
	a.cfg.TelegramAPI, a.cfg.TelegramToken, a.cfg.TelegramChat = tg.srv.URL, tgToken, "42"
	a.cfg.NotifyCooldown = time.Hour
	return a, srv, tg
}

func sendN(t *testing.T, srv *httptest.Server, n int, mod func(m, d map[string]any)) {
	t.Helper()
	for i := 0; i < n; i++ {
		i := i
		mustAccept(t, srv, "linkhop", lhOwnSecret, report("linkhop", func(m, d map[string]any) {
			m["id"] = fmt.Sprintf("tg-%d-%d", time.Now().UnixNano(), i)
			if mod != nil {
				mod(m, d)
			}
		}))
	}
}

func setCount(t *testing.T, a *App, n int) {
	t.Helper()
	if _, err := a.db.Exec(`UPDATE notifications SET count=?`, n); err != nil {
		t.Fatal(err)
	}
}

func expireCooldown(t *testing.T, a *App) {
	t.Helper()
	if _, err := a.db.Exec(`UPDATE notifications SET notified_at=notified_at-7200`); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramOnePerGroupPerCooldownAndNoUserData(t *testing.T) {
	a, srv, tg := telegramApp(t)
	private := func(m, d map[string]any) {
		d["message"] = "failed for visitor42@example.com, query secret-visitor-input"
		d["ref"] = "deadbeef1234"
	}
	sendN(t, srv, 1, private) // a new group: one message
	text := tg.expect(t)
	<-tg.path
	for _, want := range []string{"Server error reported (HTTP 500)", "linkhop", `Illuminate\Database\QueryException`, "GET /l/{code}", "Count: 1", "/notifications/"} {
		if !strings.Contains(text, want) {
			t.Errorf("message lacks %q: %q", want, text)
		}
	}
	for _, leak := range []string{"visitor42", "example.com", "secret-visitor-input", "SQLSTATE", "deadbeef1234", "LinkController.php", lhOwnSecret, secret, tgToken} {
		if strings.Contains(text, leak) {
			t.Errorf("message leaks %q: %q", leak, text)
		}
	}

	// the same group again, and a jump over 10 occurrences, both inside the cooldown: silence
	sendN(t, srv, 1, private)
	tg.expectNone(t, "second report of a known group")
	setCount(t, a, 9)
	sendN(t, srv, 1, private) // count 10 crosses the threshold, but the cooldown holds
	tg.expectNone(t, "threshold inside the cooldown")

	// once the cooldown is over, crossing 100 sends one message
	expireCooldown(t, a)
	setCount(t, a, 99)
	sendN(t, srv, 1, private)
	if text := tg.expect(t); !strings.Contains(text, "Count: 100") {
		t.Errorf("threshold message: %q", text)
	}
	sendN(t, srv, 1, private)
	tg.expectNone(t, "right after a message")

	// after the cooldown a report that crosses nothing stays quiet too
	expireCooldown(t, a)
	sendN(t, srv, 1, private)
	tg.expectNone(t, "no new group, no regression, no threshold")
}

func TestTelegramRegressionMutedAndSeen(t *testing.T) {
	a, srv, tg := telegramApp(t)
	au := db.Audit{ActorSub: "1", ActorName: "boss"}
	ctx := context.Background()
	sendN(t, srv, 1, nil)
	tg.expect(t)
	gid := groups(t, a)[0].ID

	if _, err := a.db.ChangeNotification(ctx, gid, "resolve", "fixed", au); err != nil {
		t.Fatal(err)
	}
	sendN(t, srv, 1, nil)
	tg.expectNone(t, "regression inside the cooldown")
	if _, err := a.db.ChangeNotification(ctx, gid, "resolve", "fixed again", au); err != nil {
		t.Fatal(err)
	}
	expireCooldown(t, a)
	sendN(t, srv, 1, nil)
	if text := tg.expect(t); !strings.Contains(text, "is back after it was resolved") {
		t.Errorf("regression message: %q", text)
	}

	// acknowledged and muted groups never alert, even over a threshold and after the cooldown
	for _, action := range []string{"ack", "mute"} {
		if _, err := a.db.ChangeNotification(ctx, gid, action, "noise", au); err != nil {
			t.Fatal(err)
		}
		expireCooldown(t, a)
		setCount(t, a, 999)
		sendN(t, srv, 1, nil)
		tg.expectNone(t, action+" group")
	}
}

func TestTelegramIsOffWithoutTokenAndHonoursTheConfiguredLanguage(t *testing.T) {
	a, srv, tg := telegramApp(t)
	a.cfg.TelegramToken = ""
	sendN(t, srv, 1, nil)
	tg.expectNone(t, "no token configured")
	n := groups(t, a)[0]
	if n.Count != 1 {
		t.Fatal("the report itself must still be stored")
	}

	a.cfg.TelegramToken, a.cfg.Lang = tgToken, "de"
	sendN(t, srv, 1, func(m, d map[string]any) { d["route"] = "GET /b/{slug}" })
	if text := tg.expect(t); !strings.Contains(text, "Serverfehler gemeldet") {
		t.Errorf("German message: %q", text)
	}
}

// The bot token is in the request URL; the journal must never see it.
func TestTelegramTokenNeverReachesTheLog(t *testing.T) {
	a, _, _ := newInbox(t)
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close() // connection refused
	a.cfg.TelegramAPI, a.cfg.TelegramToken, a.cfg.TelegramChat = dead.URL, tgToken, "42"
	logs := captureLog(t)
	a.notify("hello")
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "telegram:") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	out := logs.String()
	if !strings.Contains(out, "telegram:") {
		t.Fatal("the failure was not logged at all")
	}
	for _, leak := range []string{tgToken, "AAH-test", "/bot", "sendMessage"} {
		if strings.Contains(out, leak) {
			t.Errorf("log contains %q: %q", leak, out)
		}
	}

	// an HTTP error status is logged as the status only
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer bad.Close()
	a.cfg.TelegramAPI = bad.URL
	err := a.sendTelegram(context.Background(), "x")
	if err == nil || strings.Contains(err.Error(), tgToken) || !strings.Contains(err.Error(), "401") {
		t.Errorf("status error: %v", err)
	}
	// a malformed API URL makes http.NewRequest fail with the URL in the error
	a.cfg.TelegramAPI = "http://[::1"
	if err := a.sendTelegram(context.Background(), "x"); err == nil || strings.Contains(err.Error(), tgToken) {
		t.Errorf("bad URL error: %v", err)
	}
}

func TestRedactSecret(t *testing.T) {
	ue := &url.Error{Op: "Post", URL: "https://api.telegram.org/bot" + tgToken + "/sendMessage", Err: errors.New("dial tcp: lookup api.telegram.org: no such host")}
	got := redactSecret(ue, tgToken).Error()
	if strings.Contains(got, tgToken) || strings.Contains(got, "/bot") || !strings.Contains(got, "no such host") {
		t.Errorf("redacted: %q", got)
	}
	// even a cause that repeats the token is blanked
	got = redactSecret(fmt.Errorf("proxy said no to %s", tgToken), tgToken).Error()
	if strings.Contains(got, tgToken) || !strings.Contains(got, "[token]") {
		t.Errorf("redacted: %q", got)
	}
	if redactSecret(errors.New("x"), "").Error() != "x" {
		t.Error("an empty secret must not mangle the message")
	}
}

func TestLoginAlertsStillWork(t *testing.T) {
	// notify() keeps its behaviour for the existing callers.
	a, _, _ := newInbox(t)
	tg := newFakeTelegram(t)
	a.cfg.TelegramAPI, a.cfg.TelegramToken, a.cfg.TelegramChat, a.cfg.TelegramThread = tg.srv.URL, tgToken, "42", "7"
	a.notify("plain alert")
	text := tg.expect(t)
	p := <-tg.path
	if !strings.HasSuffix(p, "/sendMessage") || !strings.Contains(p, tgToken) || !strings.Contains(text, "plain alert") {
		t.Errorf("path %q text %q", p, text)
	}
}
