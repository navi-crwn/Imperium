package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

// hopID is the account hub as a call target; LinkHop is a separate registry app.
func (a *App) hopID() hopproto.App {
	return hopproto.App{Slug: "hub", URL: a.cfg.HopURL, Prefix: "/api/hop", Secret: a.cfg.HopSecret}
}

// target is an app as a call target. BioHop lives in HOP ID's shared
// registry and verifies the caller "imperium" against the imperium row, so
// like HOP ID itself it is signed with Imperium's own secret, under /api/hop.
func (a *App) target(ap db.App) hopproto.App {
	prefix, secret := ap.Prefix, ap.Secret
	if ap.Slug == "biohop" || ap.Slug == "linkhop" || ap.Slug == "hub" {
		secret = a.cfg.HopSecret
		if prefix == "" || prefix == "/hop" {
			prefix = "/api/hop"
		}
	}
	return hopproto.App{Slug: ap.Slug, URL: ap.URL, Prefix: prefix, Secret: secret}
}

func actorOf(s *db.Session) string {
	if s == nil {
		return "system"
	}
	return s.Sub + ":" + s.Username
}

// callHop makes a signed call to HOP ID and decodes data into out.
func (a *App) callHop(ctx context.Context, s *db.Session, method, path string, body, out any) (*hopproto.Envelope, error) {
	env, err := a.hop.Call(ctx, a.hopID(), actorOf(s), method, path, body)
	if err != nil {
		return env, err
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return env, err
		}
	}
	return env, nil
}

// registryApp is what HOP ID's /api/hop/apps returns (secrets only for admin clients).
type registryApp struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Status       string   `json:"status"`
	Icon         string   `json:"icon"`
	Color        string   `json:"color"`
	MenuLabel    string   `json:"menu_label"`
	Capabilities []string `json:"capabilities"`
	Hidden       bool     `json:"hidden"`
	SortOrder    int      `json:"sort_order"`
	Secret       string   `json:"secret"`
}

// syncApps copies the registry from HOP ID into the local cache.
func (a *App) syncApps(s *db.Session) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var apps []registryApp
	if _, err := a.callHop(ctx, s, "GET", "/apps", nil, &apps); err != nil {
		log.Printf("registry sync: %v", err)
		return err
	}
	for _, r := range apps {
		if r.Slug == a.cfg.HopClient || r.Slug == a.cfg.AltClient || r.Slug == "hub" {
			continue // ourselves (both doors)
		}
		prefix := "/hop"
		own := r.Secret // the app's own secret: it verifies what the app sends to us (POST /hop/notify)
		if r.Slug == "biohop" || r.Slug == "linkhop" {
			prefix = "/api/hop"
			r.Secret = a.cfg.HopSecret // outbound calls to these apps are signed with Imperium's own secret
		}
		if err := a.db.UpsertApp(ctx, &db.App{Slug: r.Slug, Name: r.Name, URL: r.URL, Prefix: prefix, Secret: r.Secret, OwnSecret: own, Status: r.Status,
			Icon: r.Icon, Color: r.Color, MenuLabel: r.MenuLabel, Capabilities: r.Capabilities, Hidden: r.Hidden, SortOrder: r.SortOrder}); err != nil {
			log.Printf("registry sync: store %s: %v", r.Slug, err)
		}
	}
	return nil
}

// Health is the last known state of one app.
type Health struct {
	Slug    string
	Name    string
	URL     string
	OK      bool
	Known   bool // false until the first probe answered
	Status  string
	Version string
	Detail  string
	Latency time.Duration
	At      time.Time
}

// probe asks an app how it is: its /hop/health when it implements the
// protocol, else a plain GET of /healthz, else of /.
func (a *App) probe(ctx context.Context, s *db.Session, ap db.App) Health {
	h := Health{Slug: ap.Slug, Name: ap.Name, URL: ap.URL, At: time.Now(), Known: true}
	if ap.Status == "maintenance" {
		h.Status = "maint"
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if ap.Can("admin_api") && ap.Secret != "" {
		var data struct {
			Version string `json:"version"`
			DB      string `json:"db"`
		}
		env, err := a.hop.Call(ctx, a.target(ap), actorOf(s), "GET", "/health", nil)
		if err == nil {
			if env == nil || len(env.Data) == 0 {
				err = fmt.Errorf("empty health response")
			} else if decodeErr := json.Unmarshal(env.Data, &data); decodeErr != nil {
				err = fmt.Errorf("invalid health response")
			} else if data.DB != "" && data.DB != "ok" {
				err = fmt.Errorf("database health: %s", data.DB)
			}
		}
		h.Latency = time.Since(start)
		h.OK, h.Version = err == nil, data.Version
		if err != nil {
			h.Detail = err.Error()
		} else if a.cachedManifest(ap.Slug) == nil {
			_, _ = a.manifest(ctx, s, &ap, false) // warm the sidebar's module list
		}
		return h
	}
	for _, p := range []string{"/healthz", "/up", "/"} {
		req, err := http.NewRequestWithContext(ctx, "GET", ap.URL+p, nil)
		if err != nil {
			h.Detail = "invalid app URL"
			return h
		}
		res, err := a.hop.HTTP.Do(req)
		if err != nil {
			h.Detail = err.Error()
			continue
		}
		res.Body.Close()
		h.Latency = time.Since(start)
		if res.StatusCode < 400 {
			h.OK, h.Detail = true, ""
			return h
		}
		h.Detail = res.Status
	}
	return h
}

// probeAll checks every app and HOP ID concurrently.
func (a *App) probeAll(ctx context.Context, s *db.Session, apps []db.App) []Health {
	hub, _ := a.source(ctx, "hub")
	all := []db.App{*hub}
	for _, ap := range apps {
		if ap.Slug != "hub" {
			all = append(all, ap)
		}
	}
	return a.probeApps(ctx, s, all)
}

// probeApps checks the given apps concurrently.
func (a *App) probeApps(ctx context.Context, s *db.Session, all []db.App) []Health {
	out := make([]Health, len(all))
	var wg sync.WaitGroup
	for i, ap := range all {
		wg.Add(1)
		go func(i int, ap db.App) {
			defer wg.Done()
			out[i] = a.probe(ctx, s, ap)
		}(i, ap)
	}
	wg.Wait()
	a.mu.Lock()
	for _, h := range out {
		a.health[h.Slug] = h
	}
	a.mu.Unlock()
	return out
}
