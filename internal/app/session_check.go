package app

import (
	"context"
	"errors"
	"log"
	"net/url"
	"slices"
	"time"

	"github.com/navi-crwn/imperium/internal/db"
	"github.com/navi-crwn/imperium/internal/hopproto"
)

// A session is only as good as the account behind it. Every sessionRecheck, and on the first request
// after a restart, the next request asks HOP ID about the person: suspended, banned, deleted or without an
// admin role means the session is deleted. When HOP ID cannot give a verdict (down, slow, rejecting our
// signature, an answer we do not understand) the session stays until its lifetime (IMPERIUM_SESSION_HOURS,
// 2 h by default) runs out, so an outage at HOP ID never locks every admin out; the next try comes
// after sessionRetry.
const (
	sessionRecheck = 5 * time.Minute
	sessionRetry   = time.Minute
)

var recheckTimeout = 4 * time.Second // a var so a test can make a hung hub quick

// hubStillTrusts reports whether the session may go on. It also copies the roles HOP ID holds today
// into the session, so a demotion applies at once and not at the next sign-in.
func (a *App) hubStillTrusts(ctx context.Context, s *db.Session) bool {
	now := time.Now()
	a.checkMu.Lock()
	if now.Before(a.checked[s.TokenHash]) {
		a.checkMu.Unlock()
		return true
	}
	a.checked[s.TokenHash] = now.Add(sessionRetry) // claimed: parallel requests do not all call the hub
	a.checkMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, recheckTimeout)
	defer cancel()
	var u struct {
		Profile struct {
			Status string `json:"status"`
		} `json:"profile"`
		Roles []string `json:"roles"`
	}
	_, err := a.callHop(ctx, s, "GET", "/users/"+url.PathEscape(s.Sub), nil, &u)
	var apiErr *hopproto.APIError
	switch {
	case errors.As(err, &apiErr) && apiErr.Status == 404:
		return false
	case err != nil:
		log.Printf("session check for %s: %v (session kept)", s.Sub, err)
		return true
	case u.Profile.Status == "" || u.Roles == nil:
		log.Printf("session check for %s: unreadable answer (session kept)", s.Sub)
		return true
	case u.Profile.Status != "active" || !allowedIn(u.Roles):
		return false
	}
	if !slices.Equal(u.Roles, s.Roles) {
		a.db.SetSessionRoles(ctx, s.TokenHash, u.Roles)
		s.Roles = u.Roles
	}
	a.checkMu.Lock()
	a.checked[s.TokenHash] = now.Add(sessionRecheck)
	a.checkMu.Unlock()
	return true
}

// forgetChecks drops the entries that are due anyway, so the map does not grow with dead sessions.
func (a *App) forgetChecks() {
	now := time.Now()
	a.checkMu.Lock()
	for k, due := range a.checked {
		if now.After(due) {
			delete(a.checked, k)
		}
	}
	a.checkMu.Unlock()
}
