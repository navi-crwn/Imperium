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
	// NotifyCooldown is the least time between two Telegram messages about the
	// same error group (IMPERIUM_NOTIFY_COOLDOWN_MINUTES, default 60).
	NotifyCooldown time.Duration
	SessionTTL     time.Duration
	RequireMFA     bool
	TrustProxy     bool
	Secure         bool
	Lang           string // UI language: id or en (default en)
	AllowedCIDR    []string
	// Aliases hide the owner's real identity on every screen, audit row and
	// alert: "your_username=superadmin,you@example.com=superadmin" (key = username or email).
	Aliases map[string]string
	Version string
}

// Alias returns the display name for a HOP ID, or "" when it has none.
func (c Config) Alias(username, email string) string {
	if v := c.Aliases[strings.ToLower(username)]; v != "" {
		return v
	}
	return c.Aliases[strings.ToLower(email)]
}

func envAliases(k string) map[string]string {
	m := map[string]string{}
	for _, pair := range strings.Split(env(k, ""), ",") {
		key, alias, ok := strings.Cut(pair, "=")
		key, alias = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(alias)
		if ok && key != "" && alias != "" {
			m[key] = alias
		}
	}
	return m
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
	coolMin, _ := strconv.Atoi(env("IMPERIUM_NOTIFY_COOLDOWN_MINUTES", "60"))
	if coolMin <= 0 {
		coolMin = 60
	}
	hours, _ := strconv.Atoi(env("IMPERIUM_SESSION_HOURS", "2"))
	if hours <= 0 {
		hours = 2
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
		NotifyCooldown: time.Duration(coolMin) * time.Minute,
		SessionTTL:     time.Duration(hours) * time.Hour,
		RequireMFA:     envBool("IMPERIUM_REQUIRE_MFA", true),
		TrustProxy:     envBool("IMPERIUM_TRUST_PROXY", false),
		Lang:           normLang(env("IMPERIUM_LANG", "en")),
		Aliases:        envAliases("IMPERIUM_ALIASES"),
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

// normLang maps a language setting to the catalog code ("zh-cn" and "zh" to
// "zh-CN", "pt" and "pt-br" to "pt-BR"); unknown values stay as they are and
// fall back to English later.
func normLang(v string) string {
	switch strings.ToLower(strings.ReplaceAll(v, "_", "-")) {
	case "zh", "zh-cn":
		return "zh-CN"
	case "pt", "pt-br":
		return "pt-BR"
	}
	for _, l := range []string{"en", "id", "de", "ja", "es", "fr", "ru"} {
		if strings.EqualFold(v, l) {
			return l
		}
	}
	return v
}
