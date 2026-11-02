// notifysend sends a sample error report to Imperium's POST /hop/notify, signed
// as an app, to try the notification inbox on a development machine.
//
//	NOTIFY_SECRET=<the app's secret> go run ./scripts/notifysend -to http://127.0.0.1:9100 -app pixelhop -n 3
//
// The secret is read from the environment, never from a flag, so it stays out of the shell history.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/navi-crwn/imperium/internal/hopproto"
)

func main() {
	to := flag.String("to", "http://127.0.0.1:8100", "Imperium base URL")
	app := flag.String("app", "pixelhop", "sending app (the X-HOP-App slug)")
	class := flag.String("class", "panic", "error class")
	msg := flag.String("message", "runtime error: index out of range", "scrubbed message, at most 200 characters")
	origin := flag.String("origin", "internal/app/handlers.go:212", "relative file:line")
	route := flag.String("route", "GET /i/{id}", "route template")
	method := flag.String("method", "GET", "HTTP method")
	env := flag.String("env", "production", "environment")
	n := flag.Int("n", 1, "how many reports to send")
	flag.Parse()
	secret := os.Getenv("NOTIFY_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "set NOTIFY_SECRET to the sending app's secret")
		os.Exit(2)
	}
	for i := 0; i < *n; i++ {
		id, ref := make([]byte, 10), make([]byte, 6)
		_, _ = rand.Read(id)
		_, _ = rand.Read(ref)
		body, _ := json.Marshal(map[string]any{
			"id": hex.EncodeToString(id), "type": "error.500", "at": time.Now().UTC().Format("2006-01-02T15:04:05Z"), "app": *app, "env": *env,
			"data": map[string]any{"class": *class, "message": *msg, "origin": *origin, "route": *route, "method": *method, "ref": hex.EncodeToString(ref), "version": "dev"},
		})
		req, _ := http.NewRequest("POST", *to+"/hop/notify", bytes.NewReader(body))
		for k, v := range hopproto.Headers(*app, secret, "system", "POST", "/hop/notify", body) {
			req.Header[k] = v
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, "send:", err)
			os.Exit(1)
		}
		out, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		res.Body.Close()
		fmt.Printf("%d %s\n", res.StatusCode, bytes.TrimSpace(out))
	}
}
