package app

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// BioHop rejects a bad value with 422 and says which key and range failed;
// the admin must see that text instead of a bare "bad_params".
func TestSettingsSaveShowsTheAppsOwn422Message(t *testing.T) {
	hop := newFakeHop(t)
	defer hop.srv.Close()
	_, srv := newApp(t, hop)
	defer srv.Close()
	mux := hop.srv.Config.Handler.(*http.ServeMux)
	api := func(h func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			if err := hop.nonces.Verify(r, b, "imperium", []string{secret}, time.Minute); err != nil {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"ok":false,"error":"` + err.Error() + `"}`))
				return
			}
			h(w, r, b)
		}
	}
	mux.HandleFunc("GET /api/hop/manifest", api(func(w http.ResponseWriter, r *http.Request, _ []byte) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"app":"linkhop","name":"LinkHop","capabilities":["admin_api","settings"],"settings_schema":[{"key":"bio.slug_min_length","type":"int"}]}}`))
	}))
	mux.HandleFunc("PUT /api/hop/settings", api(func(w http.ResponseWriter, r *http.Request, b []byte) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"ok":false,"error":"bad_params","message":"bio.slug_min_length must be a whole number from 3 to 20"}`))
	}))
	hop.ticket = map[string]any{"sub": "1", "username": "boss", "name": "Boss", "email": "b@x", "roles": []string{"owner"}, "mfa": true, "status": "active"}
	c := client(t)
	body(login(t, c, srv))
	res, err := c.PostForm(srv.URL+"/apps/linkhop/settings", url.Values{"t_bio.slug_min_length": {"int"}, "s_bio.slug_min_length": {"99"}})
	if err != nil {
		t.Fatal(err)
	}
	b := body(res)
	if !strings.Contains(b, "bio.slug_min_length must be a whole number from 3 to 20") {
		t.Fatalf("the admin does not see the app's message: %q", b)
	}
	// and the audit trail keeps it
	res, _ = c.Get(srv.URL + "/audit")
	if a := body(res); !strings.Contains(a, "settings.update") || !strings.Contains(a, "from 3 to 20") {
		t.Error("audit row lacks the failure text")
	}
}
