package hopproto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const secret = "notehop-secret-that-is-long-enough-12345"

func TestSignAndVerify(t *testing.T) {
	body := []byte(`{"a":1}`)
	h := Headers("imperium", secret, "1:owner", "POST", "/hop/users/7/actions?x=1", body)
	req := httptest.NewRequest("POST", "http://app.example/hop/users/7/actions?x=1", strings.NewReader(string(body)))
	req.Header = h
	var n Nonces
	if err := n.Verify(req, body, "imperium", []string{"other-secret-that-is-long-enough-12345", secret}, 2*time.Minute); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// replay
	if err := n.Verify(req, body, "imperium", []string{secret}, 2*time.Minute); err == nil || err.Error() != "replay" {
		t.Fatalf("want replay, got %v", err)
	}
	// wrong app, wrong secret, stale
	req2 := httptest.NewRequest("POST", "http://app.example/hop/users/7/actions?x=1", strings.NewReader(string(body)))
	req2.Header = Headers("pixelhop", secret, "x", "POST", "/hop/users/7/actions?x=1", body)
	if err := n.Verify(req2, body, "imperium", []string{secret}, 2*time.Minute); err == nil {
		t.Fatal("want bad_signature for other app")
	}
	req3 := httptest.NewRequest("POST", "http://app.example/hop/users/7/actions?x=1", strings.NewReader(string(body)))
	req3.Header = Headers("imperium", "wrong-secret-that-is-long-enough-123456", "x", "POST", "/hop/users/7/actions?x=1", body)
	if err := n.Verify(req3, body, "imperium", []string{secret}, 2*time.Minute); err == nil || err.Error() != "bad_signature" {
		t.Fatalf("want bad_signature, got %v", err)
	}
	req4 := httptest.NewRequest("POST", "http://app.example/hop/users/7/actions?x=1", strings.NewReader(string(body)))
	req4.Header = Headers("imperium", secret, "x", "POST", "/hop/users/7/actions?x=1", body)
	req4.Header.Set("X-HOP-Timestamp", "1000")
	if err := n.Verify(req4, body, "imperium", []string{secret}, 2*time.Minute); err == nil || err.Error() != "stale_request" {
		t.Fatalf("want stale_request, got %v", err)
	}
}

// The string to sign must match LinkHop's HopSignature::stringToSign exactly.
func TestStringToSignMatchesPHP(t *testing.T) {
	got := StringToSign("notehop", "1700000000", "nonce-abc", "get", "/api/hop/profiles?ids=1", nil)
	want := "hop1|notehop|1700000000|nonce-abc|GET|/api/hop/profiles?ids=1|e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func makeTicket(t *testing.T, payload map[string]any, sec string) string {
	js, _ := json.Marshal(payload)
	body := B64(js)
	m := hmac.New(sha256.New, []byte(sec))
	m.Write([]byte(body))
	return body + "." + B64(m.Sum(nil))
}

func TestParseTicket(t *testing.T) {
	now := time.Now().Unix()
	p := map[string]any{"sub": "26", "aud": "imperium", "email": "a@b", "name": "A", "username": "a", "roles": []string{"owner"}, "mfa": true,
		"state": "s1", "iat": now, "exp": now + 120, "jti": "j1", "v": 2}
	tk, err := ParseTicket(makeTicket(t, p, secret), secret, time.Now())
	if err != nil || tk.Sub != "26" || !tk.HasRole("owner") || !tk.MFA {
		t.Fatalf("parse: %v %+v", err, tk)
	}
	if _, err := ParseTicket(makeTicket(t, p, "x"+secret), secret, time.Now()); err == nil {
		t.Fatal("wrong secret accepted")
	}
	p["exp"] = now - 1
	if _, err := ParseTicket(makeTicket(t, p, secret), secret, time.Now()); err == nil {
		t.Fatal("expired accepted")
	}
}

func TestClientCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var n Nonces
		if err := n.Verify(r, nil, "imperium", []string{secret}, time.Minute); err != nil {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
			return
		}
		if r.URL.RequestURI() != "/api/hop/stats?since=x" {
			t.Errorf("path %s", r.URL.RequestURI())
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"kpi":{"users":74}}}`))
	}))
	defer srv.Close()
	c := &Client{As: "imperium"}
	env, err := c.Call(t.Context(), App{Slug: "linkhop", URL: srv.URL, Prefix: "/api/hop", Secret: secret}, "sys", "GET", "/stats?since=x", nil)
	if err != nil || !strings.Contains(string(env.Data), "74") {
		t.Fatalf("call: %v %s", err, env.Data)
	}
	_, err = c.Call(t.Context(), App{Slug: "linkhop", URL: srv.URL, Prefix: "/api/hop", Secret: "bad-secret-that-is-long-enough-1234567"}, "sys", "GET", "/stats?since=x", nil)
	if err == nil || err.Error() != "bad_signature" {
		t.Fatalf("want bad_signature, got %v", err)
	}
}

func TestClientKeepsTheAppsMessageOn422Only(t *testing.T) {
	replies := map[string]struct {
		status int
		body   string
	}{
		"/api/hop/settings": {422, `{"ok":false,"error":"bad_params","message":"bio.slug_min_length must be a whole number from 3 to 20\n"}`},
		"/api/hop/gone":     {404, `{"ok":false,"error":"not_found","message":"no such thing"}`},
		"/api/hop/long":     {422, `{"ok":false,"error":"bad_params","message":"` + strings.Repeat("é", 400) + `"}`},
		"/api/hop/nomsg":    {422, `{"ok":false,"error":"bad_params"}`},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rep := replies[r.URL.Path]
		w.WriteHeader(rep.status)
		_, _ = w.Write([]byte(rep.body))
	}))
	defer srv.Close()
	c := &Client{As: "imperium"}
	app := App{Slug: "biohop", URL: srv.URL, Prefix: "/api/hop", Secret: secret}

	_, err := c.Call(t.Context(), app, "sys", "PUT", "/settings", map[string]any{"bio.slug_min_length": "99"})
	if err == nil || err.Error() != "bad_params: bio.slug_min_length must be a whole number from 3 to 20" {
		t.Fatalf("422: %v", err)
	}
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != "bad_params" || ae.Status != 422 {
		t.Fatalf("typed error: %#v", err)
	}
	// Other statuses still read as the bare protocol code, which callers compare ("not_found").
	if _, err := c.Call(t.Context(), app, "sys", "GET", "/gone", nil); err == nil || err.Error() != "not_found" {
		t.Fatalf("404: %v", err)
	}
	if _, err := c.Call(t.Context(), app, "sys", "GET", "/nomsg", nil); err == nil || err.Error() != "bad_params" {
		t.Fatalf("422 without a message: %v", err)
	}
	// A long message is cut so it still fits a cookie.
	_, err = c.Call(t.Context(), app, "sys", "GET", "/long", nil)
	if err == nil || len([]rune(err.Error())) > len("bad_params: ")+300 {
		t.Fatalf("long message: %d runes", len([]rune(err.Error())))
	}
}
