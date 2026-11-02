package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
)

type dbAudit = db.Audit

// Telegram alerts: an app going down or coming back, and sign-ins Imperium
// refused. Off unless IMPERIUM_TELEGRAM_TOKEN and IMPERIUM_TELEGRAM_CHAT are set.

func (a *App) notify(msg string) {
	if a.cfg.TelegramToken == "" || a.cfg.TelegramChat == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.sendTelegram(ctx, msg); err != nil {
			log.Printf("telegram: %v", err)
		}
	}()
}

// sendTelegram posts one message. Any error it returns is safe to log: the bot
// token is part of the request URL, and Go's *url.Error prints that URL, so
// errors are reduced to their cause and the token is blanked wherever it appears.
func (a *App) sendTelegram(ctx context.Context, msg string) error {
	form := url.Values{"chat_id": {a.cfg.TelegramChat}, "text": {"🏛 Imperium\n" + msg}, "disable_web_page_preview": {"1"}}
	if a.cfg.TelegramThread != "" {
		form.Set("message_thread_id", a.cfg.TelegramThread)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.cfg.TelegramAPI, "/")+"/bot"+a.cfg.TelegramToken+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return redactSecret(err, a.cfg.TelegramToken)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.hop.HTTP.Do(req)
	if err != nil {
		return redactSecret(err, a.cfg.TelegramToken)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s", res.Status)
	}
	return nil
}

// redactSecret drops the request URL from a *url.Error and blanks secret in
// what is left, so the result can go to the journal.
func redactSecret(err error, secret string) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = fmt.Errorf("%s request failed: %w", ue.Op, ue.Err)
	}
	msg := err.Error()
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "[token]")
		msg = strings.ReplaceAll(msg, url.PathEscape(secret), "[token]")
	}
	return errors.New(msg)
}

// watch probes every app periodically and reports changes of state.
// Watch runs the periodic health probes with alerts.
func (a *App) Watch(ctx context.Context) {
	if !a.cfg.SSOEnabled() {
		return
	}
	t := time.NewTicker(a.cfg.WatchEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.probeAndAlert(ctx)
		}
	}
}

func (a *App) probeAndAlert(ctx context.Context) {
	apps, _ := a.db.Apps(ctx)
	a.mu.Lock()
	before := map[string]Health{}
	for k, v := range a.health {
		before[k] = v
	}
	a.mu.Unlock()
	now := a.probeAll(ctx, nil, apps)
	for _, h := range now {
		prev, known := before[h.Slug]
		if h.Status == "maint" {
			continue
		}
		switch {
		case known && prev.Known && prev.OK && !h.OK:
			a.notify(fmt.Sprintf("🔴 %s bermasalah: %s\n%s", h.Name, h.Detail, h.URL))
			a.db.AddAudit(ctx, &dbAudit{ActorSub: "system", ActorName: "watch", App: h.Slug, Action: "health.down", Detail: map[string]any{"detail": h.Detail}, OK: false})
		case known && prev.Known && !prev.OK && h.OK:
			a.notify(fmt.Sprintf("🟢 %s pulih (%d ms)\n%s", h.Name, h.Latency.Milliseconds(), h.URL))
			a.db.AddAudit(ctx, &dbAudit{ActorSub: "system", ActorName: "watch", App: h.Slug, Action: "health.up", OK: true})
		}
	}
}
