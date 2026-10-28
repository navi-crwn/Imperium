package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func humanSince(t time.Time) string {
	if t.IsZero() || t.Unix() <= 0 {
		return "–"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d mnt", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d jam", int(d.Hours()))
	default:
		return fmt.Sprintf("%d hari", int(d.Hours()/24))
	}
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
func humanBytes(n int64) string {
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

// dayOf keeps the date of an ISO timestamp ("2026-10-28T07:40:00Z" → "2026-10-28").
func dayOf(v any) string {
	s := str(v)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}
