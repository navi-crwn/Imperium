package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func pageURL(path string, values url.Values, without ...string) string {
	q := url.Values{}
	for key, list := range values {
		if !contains(without, key) {
			q[key] = append([]string(nil), list...)
		}
	}
	if encoded := q.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func cursorURL(path string, values url.Values, cursor string) string {
	q := url.Values{}
	for key, list := range values {
		if key != "cursor" {
			q[key] = append([]string(nil), list...)
		}
	}
	q.Set("cursor", cursor)
	return path + "?" + q.Encode()
}

func versionLabel(l, version string) string {
	if version == "dev" {
		return tl(l, "version.dev")
	}
	return version
}

// since renders a relative time in the configured catalogue language.
func since(l string, t time.Time) string {
	if t.IsZero() || t.Unix() <= 0 {
		return "\u2013" // en dash; escaped so the source carries no byte-order surprises
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return tl(l, "since.now")
	case d < time.Hour:
		return fmt.Sprintf(tl(l, "since.min"), int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf(tl(l, "since.hour"), int(d.Hours()))
	default:
		return fmt.Sprintf(tl(l, "since.day"), int(d.Hours()/24))
	}
}

// dateWithZone renders an instant in UTC with the zone spelled out, and the raw
// instant in a datetime attribute so the browser can show it in the reader's own
// zone. Times used to appear in the server's zone (CET) with no label at all.
func dateWithZone(l string, t time.Time) template.HTML {
	if t.IsZero() || t.Unix() <= 0 {
		return template.HTML("<span class=\"muted\">–</span>")
	}
	utc := t.UTC()
	return template.HTML(fmt.Sprintf(
		`<time datetime="%s" data-utc-label="%s">%s</time> <span class="time-zone">%s</span>`,
		utc.Format(time.RFC3339), template.HTMLEscapeString(tl(l, "time.utc")),
		utc.Format("2006-01-02 15:04"), template.HTMLEscapeString(tl(l, "time.utc"))))
}

// datesz is dateWithZone for a timestamp that arrives as text from another app,
// which may be RFC 3339 or already formatted. Unparseable text is shown as it
// came, so nothing disappears.
func datesz(l, v string) template.HTML {
	v = strings.TrimSpace(v)
	if v == "" {
		return template.HTML("<span class=\"muted\">–</span>")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return dateWithZone(l, t)
		}
	}
	return template.HTML(template.HTMLEscapeString(v))
}

func humanInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var out []string
	for len(s) > 3 {
		out = append([]string{s[len(s)-3:]}, out...)
		s = s[:len(s)-3]
	}
	return s + "." + strings.Join(out, ".")
}

// humanAny formats any integer-ish value with thousands separators.
func humanAny(v any) string {
	switch x := v.(type) {
	case int:
		return humanInt(int64(x))
	case int64:
		return humanInt(x)
	case float64:
		return humanInt(int64(x))
	case string:
		return x
	default:
		return fmt.Sprint(v)
	}
}

func jsonPretty(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func parseTime(s string) time.Time {
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04:05.000000Z", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// humanBytes formats a byte count: 0 B, 12,3 KB, 4,5 GB.
func humanBytes(v any) string {
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int64:
		n = x
	case float64:
		n = int64(x)
	case int32:
		n = int64(x)
	case uint64:
		n = int64(x)
	default:
		return fmt.Sprint(v)
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// dayOf keeps the date of an ISO timestamp ("2026-10-28T07:40:00Z" ÔåÆ "2026-10-28").
func dayOf(v any) string {
	s := str(v)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

// returnTo is where an action sends the browser afterwards: the page it was started from when the form
// names one (a row action on a list goes back to the list), else def. Only local paths are followed.
func returnTo(r *http.Request, def string) string {
	if b := r.FormValue("back"); b != "" {
		return safeNext(b)
	}
	return def
}
