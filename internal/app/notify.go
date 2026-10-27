package app

import (
	"context"
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
		form := url.Values{"chat_id": {a.cfg.TelegramChat}, "text": {"🏛 Imperium\n" + msg}, "disable_web_page_preview": {"1"}}
		req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.cfg.TelegramAPI, "/")+"/bot"+a.cfg.TelegramToken+"/sendMessage", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := a.hop.HTTP.Do(req)
		if err != nil {
			log.Printf("telegram: %v", err)
			return
		}
		res.Body.Close()
		if res.StatusCode >= 300 {
			log.Printf("telegram: %s", res.Status)
		}
	}()
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
