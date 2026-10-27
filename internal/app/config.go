package app

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config comes from the environment (/etc/imperium/imperium.env in production).
type Config struct {
	Addr       string // listen address
	DBPath     string
	BaseURL    string // https://imperium.hel.ink
	HopURL     string // https://hel.ink (HOP ID)
	HopSecret  string // this app's secret in HOP ID's registry
	HopClient  string // this app's slug there: imperium
	HopAPIAddr string // optional host:port to reach HOP ID on this machine (127.0.0.1:443)
	// A second door (Cloudflare Access at citadel.hel.ink) is a second app in
	// HOP ID's registry, because a sign-in ticket may only return to one URL.
	AltHost   string // host of the second door, e.g. citadel.hel.ink
	AltClient string // its slug in the registry, e.g. citadel
	AltSecret string
	// Telegram alerts (optional).
	TelegramToken  string
	TelegramChat   string
	TelegramThread string // forum topic id, optional
	TelegramAPI    string
	WatchEvery     time.Duration
	SessionTTL     time.Duration
	RequireMFA     bool
	TrustProxy     bool
	Secure         bool
	Lang           string // id or en
	AllowedCIDR    []string
	Version        string
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := strings.ToLower(env(k, ""))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func LoadConfig(version string) Config {
	watchMin, _ := strconv.Atoi(env("IMPERIUM_WATCH_MINUTES", "2"))
	if watchMin <= 0 {
		watchMin = 2
	}
	hours, _ := strconv.Atoi(env("IMPERIUM_SESSION_HOURS", "8"))
	if hours <= 0 {
		hours = 8
	}
	c := Config{
		Addr:           env("IMPERIUM_ADDR", "127.0.0.1:8100"),
		DBPath:         env("IMPERIUM_DB", "imperium.db"),
		BaseURL:        strings.TrimRight(env("IMPERIUM_BASE_URL", "http://127.0.0.1:8100"), "/"),
		HopURL:         strings.TrimRight(env("HOP_URL", ""), "/"),
		HopSecret:      env("HOP_SSO_SECRET", ""),
		HopClient:      env("HOP_SSO_CLIENT", "imperium"),
		HopAPIAddr:     env("HOP_API_ADDR", ""),
		AltHost:        env("IMPERIUM_ALT_HOST", ""),
		AltClient:      env("IMPERIUM_ALT_CLIENT", "citadel"),
		AltSecret:      env("IMPERIUM_ALT_SECRET", ""),
		TelegramToken:  env("IMPERIUM_TELEGRAM_TOKEN", ""),
		TelegramChat:   env("IMPERIUM_TELEGRAM_CHAT", ""),
		TelegramThread: env("IMPERIUM_TELEGRAM_THREAD", ""),
		TelegramAPI:    env("IMPERIUM_TELEGRAM_API", "https://api.telegram.org"),
		WatchEvery:     time.Duration(watchMin) * time.Minute,
		SessionTTL:     time.Duration(hours) * time.Hour,
		RequireMFA:     envBool("IMPERIUM_REQUIRE_MFA", true),
		TrustProxy:     envBool("IMPERIUM_TRUST_PROXY", false),
		Lang:           env("IMPERIUM_LANG", "id"),
		Version:        version,
	}
	c.Secure = envBool("IMPERIUM_SECURE_COOKIES", strings.HasPrefix(c.BaseURL, "https://"))
	if c.HopURL == "" || len(c.HopSecret) < 32 {
		log.Printf("HOP ID sign-in is off: set HOP_URL and HOP_SSO_SECRET (32+ characters)")
	}
	return c
}

// Door is the SSO identity to use for a request: the second door's when the
// request came in through it, else the main one.
func (c Config) Door(host string) (client, secret, baseURL string) {
	if c.AltHost != "" && strings.EqualFold(host, c.AltHost) && len(c.AltSecret) >= 32 {
		return c.AltClient, c.AltSecret, "https://" + c.AltHost
	}
	return c.HopClient, c.HopSecret, c.BaseURL
}

// SSOEnabled reports whether sign-in through HOP ID is configured.
func (c Config) SSOEnabled() bool { return c.HopURL != "" && len(c.HopSecret) >= 32 }
