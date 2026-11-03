package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ngajio/server/internal/auth"
	"ngajio/server/internal/config"
	"ngajio/server/internal/limit"
	"ngajio/server/internal/store"
)

func setup(t *testing.T, cfg config.Config) (*httptest.Server, *store.Store, *bytes.Buffer) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ngajio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	var logs bytes.Buffer
	srv := httptest.NewServer(newHandler(cfg, st, log.New(&logs, "", 0)))
	t.Cleanup(srv.Close)
	return srv, st, &logs
}

func devCfg() config.Config {
	return config.Config{Env: "dev", DevLogin: true, CookieSecure: false}
}

func req(t *testing.T, srv *httptest.Server, method, path, body string, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	r, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	r.Header.Set("Accept-Encoding", "identity")
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func good(srv *httptest.Server) map[string]string {
	return map[string]string{"Content-Type": "application/json", "X-Ngajio": "1", "Origin": srv.URL}
}

func TestHealthAndConfig(t *testing.T) {
	cfg := devCfg()
	cfg.GoogleClientID = "client-123"
	srv, st, _ := setup(t, cfg)
	resp, b := req(t, srv, "GET", "/api/health", "", nil)
	if resp.StatusCode != 200 || strings.TrimSpace(string(b)) != `{"ok":true}` {
		t.Fatalf("health: %d %s", resp.StatusCode, b)
	}
	resp, b = req(t, srv, "GET", "/api/config", "", nil)
	if resp.StatusCode != 200 || strings.TrimSpace(string(b)) != `{"devLogin":true,"googleClientId":"client-123"}` ||
		resp.Header.Get("Cache-Control") != "max-age=300" {
		t.Fatalf("config: %d %s %v", resp.StatusCode, b, resp.Header)
	}
	st.Close()
	resp, b = req(t, srv, "GET", "/api/health", "", nil)
	if resp.StatusCode != 500 || !strings.Contains(string(b), `"internal"`) {
		t.Fatalf("health with dead db: %d %s", resp.StatusCode, b)
	}
}

func TestConfigDevLoginOffAndNoRoute(t *testing.T) {
	srv, _, _ := setup(t, config.Config{Env: "prod", CookieSecure: true, GoogleClientID: ""})
	_, b := req(t, srv, "GET", "/api/config", "", nil)
	if strings.TrimSpace(string(b)) != `{"devLogin":false,"googleClientId":""}` {
		t.Fatalf("config: %s", b)
	}
	resp, b := req(t, srv, "POST", "/api/auth/dev", `{"email":"a@b.co"}`, good(srv))
	if resp.StatusCode != 404 || !strings.Contains(string(b), `"not_found"`) {
		t.Fatalf("dev route must not exist: %d %s", resp.StatusCode, b)
	}
}

type loginResp struct {
	User struct {
		ID    int64
		Email string
		Nama  string
		Plan  string
	}
	Baru   bool
	Cursor int64
}

func TestDevLogin(t *testing.T) {
	srv, st, _ := setup(t, devCfg())
	resp, b := req(t, srv, "POST", "/api/auth/dev", `{"email":"  Ust@Example.ID "}`, good(srv))
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	var lr loginResp
	json.Unmarshal(b, &lr)
	if !lr.Baru || lr.Cursor != 0 || lr.User.ID == 0 || lr.User.Email != "ust@example.id" || lr.User.Plan != "gratis" {
		t.Fatalf("response: %s", b)
	}
	// cookie flags
	cs := resp.Cookies()
	if len(cs) != 1 {
		t.Fatalf("cookies: %v", cs)
	}
	c := cs[0]
	if c.Name != "ngajio_sid" || len(c.Value) != 43 || !c.HttpOnly || c.Secure || c.SameSite != http.SameSiteLaxMode ||
		c.Path != "/" || c.MaxAge != 2592000 {
		t.Fatalf("cookie: %+v", c)
	}
	// session is stored by hash and is valid for 30 days
	u, ss, err := st.LookupSession(ctx(), store.HashToken(c.Value), time.Now())
	if err != nil || u.ID != lr.User.ID || ss.ExpiresAt-ss.CreatedAt != 2592000 {
		t.Fatalf("session: %+v %+v %v", u, ss, err)
	}
	// same email again: same user, not new, a distinct session
	resp2, b2 := req(t, srv, "POST", "/api/auth/dev", `{"email":"ust@example.id"}`, good(srv))
	var lr2 loginResp
	json.Unmarshal(b2, &lr2)
	if lr2.Baru || lr2.User.ID != lr.User.ID || resp2.Cookies()[0].Value == c.Value {
		t.Fatalf("second login: %s", b2)
	}
	// other email: other user
	_, b3 := req(t, srv, "POST", "/api/auth/dev", `{"email":"lain@example.id"}`, good(srv))
	var lr3 loginResp
	json.Unmarshal(b3, &lr3)
	if lr3.User.ID == lr.User.ID || !lr3.Baru {
		t.Fatalf("third login: %s", b3)
	}
}

func TestDevLoginSecureCookie(t *testing.T) {
	cfg := devCfg()
	cfg.CookieSecure = true
	srv, _, _ := setup(t, cfg)
	resp, _ := req(t, srv, "POST", "/api/auth/dev", `{"email":"a@b.co"}`, good(srv))
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "Secure") {
		t.Fatalf("Set-Cookie: %s", resp.Header.Get("Set-Cookie"))
	}
}

func TestDevLoginValidation(t *testing.T) {
	srv, _, _ := setup(t, devCfg())
	for _, body := range []string{
		`{}`, `{"email":""}`, `{"email":"nope"}`, `{"email":"a@b"}`, `{"email":"a b@c.de"}`,
		`{"email":"` + strings.Repeat("a", 250) + `@b.co"}`, `{"email":5}`, `{"email":"a@b.co","x":1}`, `not json`,
	} {
		resp, b := req(t, srv, "POST", "/api/auth/dev", body, good(srv))
		if resp.StatusCode != 400 || !strings.Contains(string(b), `"bad_request"`) || len(resp.Cookies()) != 0 {
			t.Errorf("%.40s: %d %s", body, resp.StatusCode, b)
		}
	}
}

func TestDevLoginCSRF(t *testing.T) {
	srv, _, _ := setup(t, devCfg())
	for name, mutate := range map[string]func(map[string]string){
		"no X-Ngajio":  func(h map[string]string) { delete(h, "X-Ngajio") },
		"wrong origin": func(h map[string]string) { h["Origin"] = "https://evil.example" },
		"no origin":    func(h map[string]string) { delete(h, "Origin") },
		"wrong type":   func(h map[string]string) { h["Content-Type"] = "text/plain" },
	} {
		h := good(srv)
		mutate(h)
		resp, b := req(t, srv, "POST", "/api/auth/dev", `{"email":"a@b.co"}`, h)
		if resp.StatusCode != 403 || !strings.Contains(string(b), `"csrf"`) || len(resp.Cookies()) != 0 {
			t.Errorf("%s: %d %s", name, resp.StatusCode, b)
		}
	}
}

func TestNotFoundIsJSON(t *testing.T) {
	srv, _, _ := setup(t, devCfg())
	for _, c := range [][2]string{{"GET", "/nope"}, {"GET", "/"}, {"GET", "/api/nope"}} {
		resp, b := req(t, srv, c[0], c[1], "", nil)
		if resp.StatusCode != 404 || !strings.Contains(string(b), `"not_found"`) {
			t.Errorf("%v: %d %s", c, resp.StatusCode, b)
		}
	}
}

func TestLoginEmailNotLogged(t *testing.T) {
	srv, _, logs := setup(t, devCfg())
	req(t, srv, "POST", "/api/auth/dev", `{"email":"rahasia@example.id"}`, good(srv))
	if strings.Contains(logs.String(), "rahasia") || !strings.Contains(logs.String(), `path="/api/auth/dev" status=200`) {
		t.Fatalf("log: %s", logs.String())
	}
}

func TestDBFileMode(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "sub", "ngajio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	fi, _ := os.Stat(filepath.Join(dir, "sub", "ngajio.db"))
	var mode string
	st.Reader().QueryRow("PRAGMA journal_mode").Scan(&mode)
	if fi.Mode().Perm() != 0o600 || mode != "wal" {
		t.Fatalf("mode %v journal %s", fi.Mode().Perm(), mode)
	}
}

func ctx() context.Context { return context.Background() }

// SIGTERM (ctx cancel) must let an in-flight request finish before serve
// returns, so the caller can close the store afterwards.
func TestShutdownDrainsInFlightRequest(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "ngajio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := newServer(devCfg(), st, log.New(io.Discard, "", 0))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(stop, srv, ln, 5*time.Second) }()

	host := ln.Addr().String()
	body := `{"email":"slow@example.id"}`
	conn, err := net.Dial("tcp", host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "POST /api/auth/dev HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nX-Ngajio: 1\r\nOrigin: http://%s\r\nContent-Length: %d\r\n\r\n%s",
		host, host, len(body), body[:10])

	time.Sleep(200 * time.Millisecond)
	cancel() // shutdown starts while the request body is half sent
	select {
	case err := <-served:
		t.Fatalf("serve returned (%v) while a request was in flight", err)
	case <-time.After(300 * time.Millisecond):
	}
	fmt.Fprint(conn, body[10:])
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("in-flight request got no response: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"baru":true`) {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the request finished")
	}
	// the session was written before the store could be closed
	var n int
	st.Reader().QueryRow("SELECT count(*) FROM sessions").Scan(&n)
	if n != 1 {
		t.Errorf("sessions %d", n)
	}
}

func TestServeReturnsListenerError(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ln.Close()
	done := make(chan error, 1)
	go func() { done <- serve(context.Background(), &http.Server{}, ln, time.Second) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve hung on listener error")
	}
}

// ---- task 2 wiring -----------------------------------------------------------------

// stubVerifier maps a credential to claims, like a fake Google.
type stubVerifier struct{ claims map[string]auth.Claims }

func (v stubVerifier) Verify(_ context.Context, tok string) (auth.Claims, error) {
	if c, ok := v.claims[tok]; ok {
		return c, nil
	}
	return auth.Claims{}, &auth.VerifyError{Code: "signature"}
}

func setupWith(t *testing.T, cfg config.Config, o options) (*httptest.Server, *store.Store, *bytes.Buffer) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ngajio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	var logs bytes.Buffer
	h, _ := buildHandler(cfg, st, log.New(&logs, "", 0), o)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, st, &logs
}

func cookieHdr(h map[string]string, c *http.Cookie) map[string]string {
	h["Cookie"] = c.Name + "=" + c.Value
	return h
}

func TestGoogleLoginMeLogoutThroughMain(t *testing.T) {
	cfg := devCfg()
	cfg.GoogleClientID = "client-123"
	stub := stubVerifier{claims: map[string]auth.Claims{
		"good": {Sub: "g-1", Email: "rahasia@example.com", Name: "Nama Rahasia", EmailVerified: true},
	}}
	srv, _, logs := setupWith(t, cfg, options{Verifier: stub})

	resp, b := req(t, srv, "POST", "/api/auth/google", `{"credential":"good"}`, good(srv))
	var lr loginResp
	json.Unmarshal(b, &lr)
	if resp.StatusCode != 200 || !lr.Baru || lr.User.Email != "rahasia@example.com" || lr.User.Nama != "Nama Rahasia" || len(resp.Cookies()) != 1 {
		t.Fatalf("login: %d %s", resp.StatusCode, b)
	}
	c := resp.Cookies()[0]
	resp, b = req(t, srv, "GET", "/api/me", "", cookieHdr(map[string]string{}, c))
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"cursor":0`) || !strings.Contains(string(b), `"id":`) {
		t.Fatalf("me: %d %s", resp.StatusCode, b)
	}
	if resp, _ = req(t, srv, "GET", "/api/me", "", nil); resp.StatusCode != 401 {
		t.Fatalf("me without cookie: %d", resp.StatusCode)
	}
	resp, b = req(t, srv, "POST", "/api/auth/logout", `{}`, cookieHdr(good(srv), c))
	if resp.StatusCode != 200 || strings.TrimSpace(string(b)) != `{"ok":true}` || !strings.Contains(resp.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout: %d %s %s", resp.StatusCode, b, resp.Header.Get("Set-Cookie"))
	}
	if resp, _ = req(t, srv, "GET", "/api/me", "", cookieHdr(map[string]string{}, c)); resp.StatusCode != 401 {
		t.Fatalf("me after logout: %d", resp.StatusCode)
	}
	resp, b = req(t, srv, "POST", "/api/auth/google", `{"credential":"forged"}`, good(srv))
	if resp.StatusCode != 401 || !strings.Contains(string(b), `"login failed"`) {
		t.Fatalf("forged: %d %s", resp.StatusCode, b)
	}
	// dev login (DEV_LOGIN is on in devCfg) works next to Google
	if resp, _ = req(t, srv, "POST", "/api/auth/dev", `{"email":"devrahasia@example.com"}`, good(srv)); resp.StatusCode != 200 {
		t.Fatalf("dev: %d", resp.StatusCode)
	}
	for _, secret := range []string{"rahasia", "Rahasia", "good", "forged", c.Value} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("log contains %q:\n%s", secret, logs)
		}
	}
	if !strings.Contains(logs.String(), "auth: google login rejected code=signature") {
		t.Errorf("no reject code in log:\n%s", logs)
	}
}

func TestGoogleNotConfiguredWithoutClientID(t *testing.T) {
	srv, _, _ := setup(t, devCfg()) // GoogleClientID empty
	resp, b := req(t, srv, "POST", "/api/auth/google", `{"credential":"x"}`, good(srv))
	if resp.StatusCode != 503 || !strings.Contains(string(b), `"not_configured"`) {
		t.Fatalf("%d %s", resp.StatusCode, b)
	}
}

// With a client id and no injected verifier, the real Google verifier is built.
// Malformed tokens fail before any network access.
func TestRealVerifierBuiltFromClientID(t *testing.T) {
	cfg := config.Config{Env: "prod", CookieSecure: true, GoogleClientID: "client-123"}
	srv, _, _ := setup(t, cfg)
	for _, body := range []string{`{"credential":"x"}`, `{"credential":"a.b.c"}`, `{"credential":""}`} {
		resp, b := req(t, srv, "POST", "/api/auth/google", body, good(srv))
		if resp.StatusCode != 401 {
			t.Errorf("%s: %d %s", body, resp.StatusCode, b)
		}
	}
}

func TestAuthRateLimitThroughMain(t *testing.T) {
	cfg := devCfg()
	cfg.TrustProxyHeader = "X-Real-IP" // config.Load always sets this default
	srv, _, _ := setupWith(t, cfg, options{Limiter: limit.New(10.0/60, 10)})
	for i := 1; i <= 10; i++ {
		if resp, b := req(t, srv, "POST", "/api/auth/logout", `{}`, good(srv)); resp.StatusCode != 200 {
			t.Fatalf("request %d: %d %s", i, resp.StatusCode, b)
		}
	}
	resp, b := req(t, srv, "POST", "/api/auth/logout", `{}`, good(srv))
	if resp.StatusCode != 429 || !strings.Contains(string(b), `"rate_limited"`) || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("11th: %d %s %v", resp.StatusCode, b, resp.Header)
	}
	// another client IP behind the (loopback) proxy is unaffected
	h := good(srv)
	h["X-Real-IP"] = "198.51.100.7"
	if resp, _ = req(t, srv, "POST", "/api/auth/logout", `{}`, h); resp.StatusCode != 200 {
		t.Fatalf("other IP: %d", resp.StatusCode)
	}
}

func TestHealthIsNotRateLimited(t *testing.T) {
	srv, _, _ := setupWith(t, devCfg(), options{Limiter: limit.New(10.0/60, 1)})
	for i := 0; i < 5; i++ {
		if resp, _ := req(t, srv, "GET", "/api/health", "", nil); resp.StatusCode != 200 {
			t.Fatalf("health limited: %d", resp.StatusCode)
		}
	}
}

// When the drain runs out of time, serve closes the connections and reports it.
func TestServeShutdownTimeout(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block })}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	stop, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(stop, srv, ln, 200*time.Millisecond) }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET /x HTTP/1.1\r\nHost: x\r\n\r\n")
	time.Sleep(100 * time.Millisecond) // the request is in flight and never ends
	start := time.Now()
	cancel()
	select {
	case err := <-served:
		if !errors.Is(err, errShutdownTimeout) || time.Since(start) > 3*time.Second {
			t.Fatalf("err=%v after %v", err, time.Since(start))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the drain timeout")
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Error("connection should have been closed")
	}
}

func TestServeCleanShutdownReturnsNil(t *testing.T) {
	srv := &http.Server{Handler: http.NotFoundHandler()}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	stop, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(stop, srv, ln, time.Second) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-served; err != nil {
		t.Fatalf("idle shutdown must be clean: %v", err)
	}
}
