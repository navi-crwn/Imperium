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
