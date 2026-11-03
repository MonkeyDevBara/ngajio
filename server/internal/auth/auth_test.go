package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ngajio/server/internal/httpx"
	"ngajio/server/internal/limit"
	"ngajio/server/internal/store"
)

func TestValidEmail(t *testing.T) {
	ok := []string{"a@b.co", "ust.az+tag@sub.example.id", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 60) + ".com"}
	bad := []string{"", "a", "a@b", "@b.co", "a@.co", "a@b.", "a b@c.d", "a@b.c d", "Nama <a@b.co>", "a@b.co\n", "a@@b.co",
		"é@b.co", "a@b.co,c@d.co", "\x00@b.co", strings.Repeat("a", 65) + "@b.co", "a@" + strings.Repeat("b", 250) + ".co"}
	for _, s := range ok {
		if !validEmail(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range bad {
		if validEmail(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestEmailLengthLimit(t *testing.T) {
	long := "a@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63) + "." + strings.Repeat("e", 56) + ".com"
	if len(long) != 254 || !validEmail(long) {
		t.Fatalf("254 bytes must pass (len %d)", len(long))
	}
	if validEmail("x" + long) {
		t.Fatal("255 bytes must fail")
	}
}

// ---- handler harness ---------------------------------------------------------

type harness struct {
	t      *testing.T
	st     *store.Store
	a      *Auth
	h      http.Handler
	clock  *fakeClock
	jwks   *fakeJWKS
	ver    *GoogleVerifier
	logs   *bytes.Buffer
	dbPath string
}

type harnessOpts struct {
	noVerifier   bool
	noDev        bool
	cookieSecure bool
	limiter      bool
}

func newHarness(t *testing.T, o harnessOpts) *harness {
	t.Helper()
	h := &harness{t: t, clock: newFakeClock(), logs: &bytes.Buffer{}}
	h.dbPath = filepath.Join(t.TempDir(), "db", "ngajio.db")
	st, err := store.Open(h.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h.st = st
	h.jwks = newFakeJWKS(t)
	logger := log.New(h.logs, "", 0)
	h.a = &Auth{Store: st, CookieSecure: o.cookieSecure, TrustProxyHeader: "X-Real-IP", Now: h.clock.Now, Logger: logger}
	if !o.noVerifier {
		h.ver = NewGoogleVerifier(testClientID, h.jwks.URL(), h.jwks.srv.Client(), h.clock.Now)
		h.a.Verifier = h.ver
	}
	if o.limiter {
		h.a.Limiter = limit.New(10.0/60, 10)
		h.a.Limiter.Now = h.clock.Now
	}
	mux := http.NewServeMux()
	h.a.Register(mux, !o.noDev)
	// a stand-in for /api/sync: shows what the middleware puts in the context
	mux.Handle("POST /api/whoami", h.a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserID(r.Context())
		httpx.WriteJSON(w, 200, map[string]any{"id": id, "ok": ok})
	})))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, httpx.CodeNotFound, "not found") })
	h.h = httpx.Wrap(mux, logger, nil)
	return h
}

func withCookie(v string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: CookieName, Value: v}) }
}
func withRemote(addr string) func(*http.Request) {
	return func(r *http.Request) { r.RemoteAddr = addr }
}
func withHeader(k, v string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set(k, v) }
}
func noCSRF() func(*http.Request) {
	return func(r *http.Request) { r.Header.Del("X-Ngajio"); r.Header.Del("Origin") }
}

// do sends a request straight to the handler. POSTs carry the CSRF headers
// unless noCSRF is passed. The default peer is 127.0.0.1.
func (h *harness) do(method, path, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	h.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:5555"
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Ngajio", "1")
		r.Header.Set("Origin", "http://example.com")
	}
	for _, o := range opts {
		o(r)
	}
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	return w
}

func (h *harness) google(cred string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	h.t.Helper()
	b, _ := json.Marshal(map[string]string{"credential": cred})
	return h.do("POST", "/api/auth/google", string(b), opts...)
}

// token mints a valid token for sub, email and name at the current fake time.
func (h *harness) token(sub, email, name string) string {
	return mint(h.t, h.clock.Now(), tok{Mutate: func(hd, c map[string]any) {
		c["sub"], c["email"], c["name"] = sub, email, name
	}})
}

type login struct {
	User struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Nama  string `json:"nama"`
		Plan  string `json:"plan"`
	} `json:"user"`
	Baru   bool  `json:"baru"`
	Cursor int64 `json:"cursor"`
}

func (h *harness) loginOK(sub, email, name string) (login, string) {
	h.t.Helper()
	w := h.google(h.token(sub, email, name))
	if w.Code != 200 {
		h.t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	var l login
	if err := json.Unmarshal(w.Body.Bytes(), &l); err != nil {
		h.t.Fatal(err)
	}
	return l, cookieOf(w)
}

func cookieOf(w *httptest.ResponseRecorder) string {
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName && c.MaxAge > 0 {
			return c.Value
		}
	}
	return ""
}

func (h *harness) sessionRow(token string) (exp, seen int64, found bool) {
	h.t.Helper()
	err := h.st.Reader().QueryRow(`SELECT expires_at,last_seen FROM sessions WHERE token_hash=?`, store.HashToken(token)).Scan(&exp, &seen)
	return exp, seen, err == nil
}

func (h *harness) count(q string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.st.Reader().QueryRow(q, args...).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

const genericUnauthorized = `{"error":{"code":"unauthorized","message":"login failed"}}` + "\n"

// ---- acceptance 1: accounts --------------------------------------------------

func TestGoogleLoginCreatesAndMatchesBySub(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	l1, c1 := h.loginOK("sub-A", "a@example.com", "Ust A")
	if !l1.Baru || l1.User.ID == 0 || l1.User.Email != "a@example.com" || l1.User.Nama != "Ust A" || l1.User.Plan != "gratis" || l1.Cursor != 0 {
		t.Fatalf("first login: %+v", l1)
	}
	if !ValidToken(c1) {
		t.Fatalf("cookie value %q", c1)
	}
	l2, c2 := h.loginOK("sub-A", "a@example.com", "Ust A")
	if l2.Baru || l2.User.ID != l1.User.ID || c2 == c1 {
		t.Fatalf("second login: %+v", l2)
	}
	// same email, other sub: another user (never matched by email)
	l3, _ := h.loginOK("sub-B", "a@example.com", "Ust B")
	if !l3.Baru || l3.User.ID == l1.User.ID {
		t.Fatalf("other sub: %+v", l3)
	}
	// same sub, new email and name: both updated
	l4, _ := h.loginOK("sub-A", "renamed@example.com", "Ust Baru")
	if l4.Baru || l4.User.ID != l1.User.ID || l4.User.Email != "renamed@example.com" || l4.User.Nama != "Ust Baru" {
		t.Fatalf("update: %+v", l4)
	}
	var sub, email, nama string
	h.st.Reader().QueryRow(`SELECT sub,email,nama FROM users WHERE id=?`, l1.User.ID).Scan(&sub, &email, &nama)
	if sub != "sub-A" || email != "renamed@example.com" || nama != "Ust Baru" {
		t.Fatalf("row: %q %q %q", sub, email, nama)
	}
	if n := h.count(`SELECT count(*) FROM users`); n != 2 {
		t.Fatalf("users %d", n)
	}
	if n := h.count(`SELECT count(*) FROM sessions`); n != 4 {
		t.Fatalf("sessions %d", n)
	}
}

func TestGoogleLoginNamaHandling(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	long := strings.Repeat("ن", 100) // 100 runes, 200 bytes
	l, _ := h.loginOK("s1", "a@example.com", long)
	if n := len([]rune(l.User.Nama)); n != 80 {
		t.Fatalf("nama runes %d, want 80", n)
	}
	// exactly 80 runes stays whole; control characters are dropped
	l, _ = h.loginOK("s1", "a@example.com", "  Ust\u0000a\n Z  ")
	if l.User.Nama != "Usta Z" {
		t.Fatalf("nama %q", l.User.Nama)
	}
	// a token without name keeps the cached one
	w := h.google(mint(t, h.clock.Now(), tok{Mutate: func(hd, c map[string]any) { c["sub"] = "s1"; c["name"] = nil }}))
	var l2 login
	json.Unmarshal(w.Body.Bytes(), &l2)
	if w.Code != 200 || l2.User.Nama != "Usta Z" || l2.Baru {
		t.Fatalf("no-name login: %d %s", w.Code, w.Body)
	}
}

func TestGoogleResponseShapeAndCookie(t *testing.T) {
	h := newHarness(t, harnessOpts{cookieSecure: true})
	w := h.google(h.token("s", "a@example.com", "N"))
	var generic map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &generic)
	if len(generic) != 3 || generic["user"] == nil || generic["baru"] == nil || generic["cursor"] == nil {
		t.Fatalf("keys: %s", w.Body)
	}
	var user map[string]json.RawMessage
	json.Unmarshal(generic["user"], &user)
	if len(user) != 4 {
		t.Fatalf("user keys: %s", generic["user"])
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type %q", ct)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control %q", w.Header().Get("Cache-Control"))
	}
	sc := w.Header().Values("Set-Cookie")
	if len(sc) != 1 {
		t.Fatalf("Set-Cookie: %v", sc)
	}
	attrs := strings.Split(sc[0], "; ")
	if !strings.HasPrefix(attrs[0], "ngajio_sid=") || len(attrs[0]) != len("ngajio_sid=")+43 {
		t.Fatalf("cookie: %s", sc[0])
	}
	got := map[string]bool{}
	for _, a := range attrs[1:] {
		got[a] = true
	}
	want := []string{"HttpOnly", "Secure", "SameSite=Lax", "Path=/", "Max-Age=2592000"}
	if len(got) != len(want) {
		t.Fatalf("attributes %v, want exactly %v", attrs[1:], want)
	}
	for _, a := range want {
		if !got[a] {
			t.Fatalf("missing %q in %s", a, sc[0])
		}
	}
}

func TestCookieSecureOffOnlyWhenConfigured(t *testing.T) {
	h := newHarness(t, harnessOpts{cookieSecure: false})
	sc := h.google(h.token("s", "a@example.com", "N")).Header().Get("Set-Cookie")
	if strings.Contains(sc, "Secure") || !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Lax") {
		t.Fatalf("cookie: %s", sc)
	}
	sc = h.do("POST", "/api/auth/logout", "{}").Header().Get("Set-Cookie")
	if strings.Contains(sc, "Secure") {
		t.Fatalf("logout cookie: %s", sc)
	}
}

// ---- acceptance 2: rejections --------------------------------------------------

func TestGoogleRejectionsAreGeneric401(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	now := h.clock.Now()
	m := func(f func(hd, c map[string]any)) string { return mint(t, now, tok{Mutate: f}) }
	good := m(nil)
	parts := strings.Split(good, ".")
	tokens := map[string]string{
		"forged":         mint(t, now, tok{Key: testKey(t, 1)}),
		"expired":        m(func(hd, c map[string]any) { c["exp"] = now.Add(-2 * time.Minute).Unix() }),
		"wrong aud":      m(func(hd, c map[string]any) { c["aud"] = "another-client" }),
		"wrong iss":      m(func(hd, c map[string]any) { c["iss"] = "https://evil.example" }),
		"unverified":     m(func(hd, c map[string]any) { c["email_verified"] = false }),
		"unverified str": m(func(hd, c map[string]any) { c["email_verified"] = "false" }),
		"alg none":       m(func(hd, c map[string]any) { hd["alg"] = "none" }),
		"alg HS256":      mintHS256(t, now, testKey(t, 0).N.Bytes(), "HS256"),
		"unknown kid":    m(func(hd, c map[string]any) { hd["kid"] = "zzz" }),
		"no kid":         m(func(hd, c map[string]any) { hd["kid"] = nil }),
		"2 segments":     parts[0] + "." + parts[1],
		"4 segments":     good + ".x",
		"empty":          "",
		"oversized":      strings.Repeat("a", 9000),
		"bad base64":     "@@@." + parts[1] + "." + parts[2],
		"nbf future":     m(func(hd, c map[string]any) { c["nbf"] = now.Add(time.Hour).Unix() }),
		"iat far future": m(func(hd, c map[string]any) { c["iat"] = now.Add(48 * time.Hour).Unix() }),
	}
	for name, tk := range tokens {
		w := h.google(tk)
		if w.Code != 401 || w.Body.String() != genericUnauthorized || len(w.Result().Cookies()) != 0 {
			t.Errorf("%s: %d %q cookies=%v", name, w.Code, w.Body, w.Result().Cookies())
		}
	}
	if n := h.count(`SELECT count(*) FROM users`) + h.count(`SELECT count(*) FROM sessions`); n != 0 {
		t.Fatalf("rejected logins created %d rows", n)
	}
	// The server-side failure (JWKS down) is the same generic answer.
	h2 := newHarness(t, harnessOpts{})
	h2.jwks.srv.Close()
	if w := h2.google(h2.token("s", "a@example.com", "N")); w.Code != 401 || w.Body.String() != genericUnauthorized {
		t.Errorf("jwks down: %d %s", w.Code, w.Body)
	}

	// Logs: short codes only, never token content, email or name.
	logs := h.logs.String()
	for name, tk := range tokens {
		if len(tk) > 30 && strings.Contains(logs, tk[:30]) {
			t.Errorf("%s: log contains token content", name)
		}
	}
	for _, code := range []string{"code=signature", "code=exp", "code=aud", "code=iss", "code=email_unverified", "code=alg", "code=kid_unknown", "code=kid_missing", "code=segments", "code=empty", "code=too_large", "code=base64", "code=nbf", "code=iat"} {
		if !strings.Contains(logs, code) {
			t.Errorf("log lacks %s", code)
		}
	}
	for _, pii := range []string{"ust@example.com", "Ust A", testClientID, "eyJ"} {
		if strings.Contains(logs, pii) {
			t.Errorf("log contains %q", pii)
		}
	}
}

func TestGoogleBadRequests(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	for name, body := range map[string]string{
		"not json":      `nope`,
		"empty body":    ``,
		"unknown field": `{"credential":"x","extra":1}`,
		"wrong type":    `{"credential":5}`,
		"two values":    `{"credential":"x"}{}`,
	} {
		w := h.do("POST", "/api/auth/google", body)
		if w.Code != 400 || !strings.Contains(w.Body.String(), `"bad_request"`) {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
	// {} and "" are well-formed requests with no usable credential: 401
	for _, body := range []string{`{}`, `{"credential":""}`, `{"credential":null}`} {
		if w := h.do("POST", "/api/auth/google", body); w.Code != 401 || w.Body.String() != genericUnauthorized {
			t.Errorf("%s: %d %s", body, w.Code, w.Body)
		}
	}
}

func TestGoogleBodyCaps(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	// a 9 KB token is within the 16 KB body cap and fails as a verification error
	if w := h.google(strings.Repeat("a", 9<<10)); w.Code != 401 {
		t.Errorf("9 KB token: %d %s", w.Code, w.Body)
	}
	// a 2.5 KB real token passes the cap (the 4 KB httpx.BodySmall would too)
	long := mint(t, h.clock.Now(), tok{Mutate: func(hd, c map[string]any) { c["picture"] = strings.Repeat("p", 1800) }})
	if w := h.google(long); w.Code != 200 {
		t.Errorf("long token: %d %s", w.Code, w.Body)
	}
	// 17 KB body is refused before it is read
	w := h.do("POST", "/api/auth/google", `{"credential":"`+strings.Repeat("a", 17<<10)+`"}`)
	if w.Code != 413 || !strings.Contains(w.Body.String(), `"too_large"`) {
		t.Errorf("17 KB body: %d %s", w.Code, w.Body)
	}
}

// ---- acceptance 4: only the hash is stored --------------------------------------

func TestRawTokenNeverInDatabaseFiles(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	var tokens []string
	for i := 0; i < 5; i++ {
		_, c := h.loginOK("s"+string(rune('a'+i)), "u"+string(rune('a'+i))+"@example.com", "N")
		tokens = append(tokens, c)
	}
	hash := store.HashToken(tokens[0])
	check := func(stage string) {
		found := false
		for _, suffix := range []string{"", "-wal", "-shm"} {
			b, err := os.ReadFile(h.dbPath + suffix)
			if err != nil {
				continue
			}
			for _, tk := range tokens {
				if bytes.Contains(b, []byte(tk)) {
					t.Errorf("%s: raw token found in %s", stage, "db"+suffix)
				}
			}
			if bytes.Contains(b, hash) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: sanity check failed, the token hash is not in the files", stage)
		}
	}
	check("live (db + wal)")
	h.st.Close()
	check("after close (checkpointed)")
}

// ---- acceptance 5: /api/me ---------------------------------------------------------

func TestMe(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	l, c := h.loginOK("s", "a@example.com", "Ust")
	h.st.Tx(context.Background(), func(tx *storeTx) error {
		_, err := tx.Exec(`UPDATE users SET rev=123 WHERE id=?`, l.User.ID)
		return err
	})
	w := h.do("GET", "/api/me", "", withCookie(c))
	want := `{"cursor":123,"user":{"id":` + itoa(l.User.ID) + `,"email":"a@example.com","nama":"Ust","plan":"gratis"}}` + "\n"
	if w.Code != 200 || w.Body.String() != want {
		t.Fatalf("me: %d %s want %s", w.Code, w.Body, want)
	}
	// GET works without the CSRF headers
	if w := h.do("GET", "/api/me", "", withCookie(c), noCSRF()); w.Code != 200 {
		t.Fatalf("GET needs no csrf headers: %d", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("me must be no-store")
	}
}

func TestMeUnauthorized(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, good := h.loginOK("s", "a@example.com", "Ust")
	other := strings.Repeat("A", 43)
	cases := map[string][]func(*http.Request){
		"no cookie":        nil,
		"empty cookie":     {withCookie("")},
		"unknown token":    {withCookie(other)},
		"too short":        {withCookie(good[:42])},
		"too long":         {withCookie(good + "A")},
		"std base64 chars": {withCookie(good[:40] + "+/=")},
		"padded":           {withCookie(good[:42] + "=")},
		"spaces":           {withCookie(strings.Repeat(" ", 43))},
		"sql":              {withCookie("' OR 1=1 --" + strings.Repeat("a", 32))},
		"other cookie":     {func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "sid", Value: good}) }},
		"in header":        {withHeader("Authorization", "Bearer "+good)},
	}
	for name, opts := range cases {
		w := h.do("GET", "/api/me", "", opts...)
		if w.Code != 401 || !strings.Contains(w.Body.String(), `"unauthorized"`) {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
}

func TestMalformedCookieDoesNotTouchDatabase(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	h.st.Close() // any query now fails with 500
	for _, v := range []string{"", "short", strings.Repeat("a", 44), strings.Repeat("+", 43)} {
		if w := h.do("GET", "/api/me", "", withCookie(v)); w.Code != 401 {
			t.Errorf("cookie %q: %d (a DB lookup would give 500)", v, w.Code)
		}
	}
	if w := h.do("GET", "/api/me", "", withCookie(strings.Repeat("a", 43))); w.Code != 500 {
		t.Errorf("well-formed token must reach the DB (closed): %d", w.Code)
	}
}

func TestMeExpiredSession(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	h.clock.Add(30*24*time.Hour - time.Second)
	if w := h.do("GET", "/api/me", "", withCookie(c)); w.Code != 200 {
		t.Fatalf("one second before expiry: %d", w.Code)
	}
	// the request above slid the session (left < 25 days), so expire it by hand
	h.st.Tx(context.Background(), func(tx *storeTx) error {
		_, err := tx.Exec(`UPDATE sessions SET expires_at=?`, h.clock.Now().Unix())
		return err
	})
	w := h.do("GET", "/api/me", "", withCookie(c))
	if w.Code != 401 || len(w.Result().Cookies()) != 0 {
		t.Fatalf("expired: %d %s", w.Code, w.Body)
	}
}

func TestExpiredWithoutUse(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	h.clock.Add(31 * 24 * time.Hour)
	if w := h.do("GET", "/api/me", "", withCookie(c)); w.Code != 401 {
		t.Fatalf("expired session: %d", w.Code)
	}
	if w := h.do("POST", "/api/whoami", "{}", withCookie(c)); w.Code != 401 {
		t.Fatalf("expired session via whoami: %d", w.Code)
	}
}

func TestSlidingExpiryThroughMe(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	exp0, seen0, _ := h.sessionRow(c)
	day := 24 * time.Hour

	// day 4: 26 days left, no extension, no new cookie
	h.clock.Add(4 * day)
	w := h.do("GET", "/api/me", "", withCookie(c))
	exp, seen, _ := h.sessionRow(c)
	if w.Code != 200 || exp != exp0 || seen != seen0 || len(w.Result().Cookies()) != 0 {
		t.Fatalf("day 4: exp %d->%d seen %d->%d", exp0, exp, seen0, seen)
	}

	// day 6: under 25 days left, extended to now+30d, cookie re-sent
	h.clock.Add(2 * day)
	w = h.do("GET", "/api/me", "", withCookie(c))
	exp1, seen1, _ := h.sessionRow(c)
	if w.Code != 200 || exp1 != h.clock.Now().Add(30*day).Unix() || seen1 != h.clock.Now().Unix() {
		t.Fatalf("day 6: exp %d (want %d) seen %d", exp1, h.clock.Now().Add(30*day).Unix(), seen1)
	}
	if cookieOf(w) != c {
		t.Fatalf("slide must re-send the same cookie: %v", w.Result().Cookies())
	}

	// repeated calls the same day and the next day: no write at all
	for _, step := range []time.Duration{0, time.Minute, 23 * time.Hour, 2 * time.Hour} {
		h.clock.Add(step)
		w = h.do("GET", "/api/me", "", withCookie(c))
		exp2, seen2, _ := h.sessionRow(c)
		if w.Code != 200 || exp2 != exp1 || seen2 != seen1 || len(w.Result().Cookies()) != 0 {
			t.Fatalf("+%v: expected no write, exp %d->%d seen %d->%d", step, exp1, exp2, seen1, seen2)
		}
	}

	// day 11.2: under 25 days left again, extended again
	h.clock.Add(4 * day)
	w = h.do("GET", "/api/me", "", withCookie(c))
	exp3, _, _ := h.sessionRow(c)
	if exp3 != h.clock.Now().Add(30*day).Unix() || cookieOf(w) != c {
		t.Fatalf("second slide: exp %d", exp3)
	}
}

func TestSlideAtMostOncePer24h(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	day := int64(24 * 3600)
	now := h.clock.Now().Unix()
	set := func(exp, seen int64) {
		h.st.Tx(context.Background(), func(tx *storeTx) error {
			_, err := tx.Exec(`UPDATE sessions SET expires_at=?, last_seen=?`, exp, seen)
			return err
		})
	}
	// 10 days left but last write 1 h ago: no write
	set(now+10*day, now-3600)
	h.do("GET", "/api/me", "", withCookie(c))
	if exp, seen, _ := h.sessionRow(c); exp != now+10*day || seen != now-3600 {
		t.Fatalf("written within 24 h: %d %d", exp, seen)
	}
	// last write 25 h ago: write
	set(now+10*day, now-25*3600)
	h.do("GET", "/api/me", "", withCookie(c))
	if exp, seen, _ := h.sessionRow(c); exp != now+30*day || seen != now {
		t.Fatalf("not written after 24 h: %d %d", exp, seen)
	}
}

func TestSlideConcurrentWritesOnce(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	h.clock.Add(10 * 24 * time.Hour)
	hash := store.HashToken(c)
	_, ss, err := h.st.LookupSession(context.Background(), hash, h.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	wrote := 0
	for i := 0; i < 5; i++ { // five requests that all saw the same stale row
		ok, err := h.a.slide(context.Background(), hash, ss, h.clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			wrote++
		}
	}
	if wrote != 1 {
		t.Fatalf("%d writes, want 1", wrote)
	}
}

// ---- acceptance 6: logout ------------------------------------------------------------

func TestLogout(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	if w := h.do("GET", "/api/me", "", withCookie(c)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	w := h.do("POST", "/api/auth/logout", "{}", withCookie(c))
	if w.Code != 200 || w.Body.String() != `{"ok":true}`+"\n" {
		t.Fatalf("logout: %d %s", w.Code, w.Body)
	}
	sc := w.Header().Get("Set-Cookie")
	for _, want := range []string{"ngajio_sid=;", "Max-Age=0", "Path=/", "HttpOnly", "SameSite=Lax"} {
		if !strings.Contains(sc, want) {
			t.Errorf("clearing cookie lacks %q: %s", want, sc)
		}
	}
	if w := h.do("GET", "/api/me", "", withCookie(c)); w.Code != 401 {
		t.Fatalf("old cookie after logout: %d", w.Code)
	}
	if _, _, found := h.sessionRow(c); found {
		t.Fatal("session row still there")
	}
	// twice in a row
	if w := h.do("POST", "/api/auth/logout", "{}", withCookie(c)); w.Code != 200 || w.Body.String() != `{"ok":true}`+"\n" {
		t.Fatalf("second logout: %d %s", w.Code, w.Body)
	}
}

func TestLogoutIdempotentWithBadCookies(t *testing.T) {
	h := newHarness(t, harnessOpts{cookieSecure: true})
	for name, opts := range map[string][]func(*http.Request){
		"no cookie": nil,
		"garbage":   {withCookie("not a token!!")},
		"malformed": {withCookie(strings.Repeat("!", 43))},
		"unknown":   {withCookie(strings.Repeat("a", 43))},
		"empty":     {withCookie("")},
	} {
		w := h.do("POST", "/api/auth/logout", "{}", opts...)
		sc := w.Header().Get("Set-Cookie")
		if w.Code != 200 || w.Body.String() != `{"ok":true}`+"\n" || !strings.Contains(sc, "Max-Age=0") || !strings.Contains(sc, "Secure") {
			t.Errorf("%s: %d %s %s", name, w.Code, w.Body, sc)
		}
	}
	// a body that is not {} does not matter either
	if w := h.do("POST", "/api/auth/logout", ""); w.Code != 200 {
		t.Errorf("empty body: %d", w.Code)
	}
	if w := h.do("POST", "/api/auth/logout", strings.Repeat("x", 100<<10)); w.Code != 200 {
		t.Errorf("huge body: %d", w.Code)
	}
}

func TestLogoutOnlyEndsOwnSession(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c1 := h.loginOK("s", "a@example.com", "Ust")
	_, c2 := h.loginOK("s", "a@example.com", "Ust") // second device
	h.do("POST", "/api/auth/logout", "{}", withCookie(c1))
	if w := h.do("GET", "/api/me", "", withCookie(c2)); w.Code != 200 {
		t.Fatalf("other session must survive: %d", w.Code)
	}
}

// ---- acceptance 7: CSRF ----------------------------------------------------------------

func TestCSRFOnEveryPost(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	_, c := h.loginOK("s", "a@example.com", "Ust")
	bodies := map[string]string{
		"/api/auth/google": `{"credential":"x"}`,
		"/api/auth/logout": `{}`,
		"/api/auth/dev":    `{"email":"a@b.co"}`,
		"/api/whoami":      `{}`,
	}
	mutations := map[string]func(*http.Request){
		"no X-Ngajio":  func(r *http.Request) { r.Header.Del("X-Ngajio") },
		"no origin":    func(r *http.Request) { r.Header.Del("Origin") },
		"bad origin":   func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"text/plain":   func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		"form encoded": func(r *http.Request) { r.Header.Set("Content-Type", "application/x-www-form-urlencoded") },
	}
	for path, body := range bodies {
		for name, mut := range mutations {
			w := h.do("POST", path, body, withCookie(c), mut)
			if w.Code != 403 || !strings.Contains(w.Body.String(), `"csrf"`) {
				t.Errorf("%s %s: %d %s", path, name, w.Code, w.Body)
			}
		}
	}
	if n := h.count(`SELECT count(*) FROM sessions`); n != 1 {
		t.Fatalf("CSRF-blocked requests changed sessions: %d", n)
	}
	// logout blocked by CSRF must not log the user out
	if w := h.do("GET", "/api/me", "", withCookie(c)); w.Code != 200 {
		t.Fatalf("session lost: %d", w.Code)
	}
}

// ---- acceptance 8: rate limit -------------------------------------------------------------

func TestRateLimitOnAuthEndpoints(t *testing.T) {
	h := newHarness(t, harnessOpts{limiter: true})
	ip := withHeader("X-Real-IP", "198.51.100.1")
	for i := 1; i <= 10; i++ {
		if w := h.do("POST", "/api/auth/logout", "{}", ip); w.Code != 200 {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	w := h.google(h.token("s", "a@example.com", "N"), ip)
	ra := w.Header().Get("Retry-After")
	n, err := parseInt(ra)
	if w.Code != 429 || err != nil || n < 1 || n > 6 || !strings.Contains(w.Body.String(), `"rate_limited"`) {
		t.Fatalf("11th: %d Retry-After=%q %s", w.Code, ra, w.Body)
	}
	// every auth endpoint shares the bucket
	for _, p := range []string{"/api/auth/logout", "/api/auth/dev"} {
		if w := h.do("POST", p, `{"email":"a@b.co"}`, ip); w.Code != 429 || w.Header().Get("Retry-After") == "" {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	// a rejected request creates nothing
	if h.count(`SELECT count(*) FROM users`) != 0 {
		t.Fatal("rate limited request created a user")
	}
	// another IP is unaffected
	if w := h.do("POST", "/api/auth/logout", "{}", withHeader("X-Real-IP", "198.51.100.2")); w.Code != 200 {
		t.Fatalf("other IP: %d", w.Code)
	}
	// tokens come back with time
	h.clock.Add(7 * time.Second)
	if w := h.do("POST", "/api/auth/logout", "{}", ip); w.Code != 200 {
		t.Fatalf("after refill: %d", w.Code)
	}
	// /api/me is not rate limited
	for i := 0; i < 30; i++ {
		if w := h.do("GET", "/api/me", "", ip); w.Code != 401 {
			t.Fatalf("me: %d", w.Code)
		}
	}
}

func TestRateLimitIgnoresSpoofedHeaderFromPublicPeer(t *testing.T) {
	h := newHarness(t, harnessOpts{limiter: true})
	peer := withRemote("203.0.113.9:4444")
	for i := 0; i < 10; i++ {
		// each request claims to be a fresh client
		spoof := withHeader("X-Real-IP", "198.51.100."+itoa(int64(i+1)))
		if w := h.do("POST", "/api/auth/logout", "{}", peer, spoof); w.Code != 200 {
			t.Fatalf("request %d: %d", i+1, w.Code)
		}
	}
	if w := h.do("POST", "/api/auth/logout", "{}", peer, withHeader("X-Real-IP", "198.51.100.77")); w.Code != 429 {
		t.Fatalf("spoofed X-Real-IP from a public peer must not buy a fresh bucket: %d", w.Code)
	}
	// a different public peer is its own bucket
	if w := h.do("POST", "/api/auth/logout", "{}", withRemote("203.0.113.10:1")); w.Code != 200 {
		t.Fatalf("other peer: %d", w.Code)
	}
}

func TestRateLimitTrustedProxyHeaderOnlyFromLoopback(t *testing.T) {
	h := newHarness(t, harnessOpts{limiter: true})
	// behind nginx (loopback peer): the real client IP is in X-Real-IP
	for i := 0; i < 10; i++ {
		h.do("POST", "/api/auth/logout", "{}", withHeader("X-Real-IP", "198.51.100.1"))
	}
	if w := h.do("POST", "/api/auth/logout", "{}", withHeader("X-Real-IP", "198.51.100.1")); w.Code != 429 {
		t.Fatalf("%d", w.Code)
	}
	// junk header from loopback falls back to the peer (127.0.0.1), a separate bucket
	if w := h.do("POST", "/api/auth/logout", "{}", withHeader("X-Real-IP", "junk")); w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
}

func TestRateLimitIdleKeysEvicted(t *testing.T) {
	h := newHarness(t, harnessOpts{limiter: true})
	for i := 0; i < 50; i++ {
		h.do("POST", "/api/auth/logout", "{}", withHeader("X-Real-IP", "198.51.100."+itoa(int64(i))))
	}
	if h.a.Limiter.Len() != 50 {
		t.Fatalf("keys %d", h.a.Limiter.Len())
	}
	h.clock.Add(time.Minute)
	if n := h.a.Limiter.Sweep(); n != 50 || h.a.Limiter.Len() != 0 {
		t.Fatalf("swept %d, left %d", n, h.a.Limiter.Len())
	}
}

// ---- acceptance 9: configuration -----------------------------------------------------------

func TestGoogleNotConfigured(t *testing.T) {
	h := newHarness(t, harnessOpts{noVerifier: true})
	w := h.google("whatever")
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"not_configured"`) || len(w.Result().Cookies()) != 0 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	// the dev login still works in that setup
	if w := h.do("POST", "/api/auth/dev", `{"email":"a@b.co"}`); w.Code != 200 {
		t.Fatalf("dev: %d %s", w.Code, w.Body)
	}
}

func TestDevAndGoogleBothWork(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	w := h.do("POST", "/api/auth/dev", `{"email":"dev@example.com"}`)
	var d login
	json.Unmarshal(w.Body.Bytes(), &d)
	if w.Code != 200 || !d.Baru {
		t.Fatalf("dev: %d %s", w.Code, w.Body)
	}
	g, _ := h.loginOK("sub-G", "dev@example.com", "G") // same email, different account
	if g.User.ID == d.User.ID || !g.Baru {
		t.Fatalf("google and dev accounts must not merge: %+v %+v", g, d)
	}
	var sub string
	h.st.Reader().QueryRow(`SELECT sub FROM users WHERE id=?`, d.User.ID).Scan(&sub)
	if sub != "dev:dev@example.com" {
		t.Fatalf("dev sub %q", sub)
	}
}

func TestDevRouteAbsentWhenOff(t *testing.T) {
	h := newHarness(t, harnessOpts{noDev: true})
	if w := h.do("POST", "/api/auth/dev", `{"email":"a@b.co"}`); w.Code != 404 {
		t.Fatalf("%d", w.Code)
	}
}

func TestWrongMethodsAre404(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	for _, c := range [][2]string{{"GET", "/api/auth/google"}, {"GET", "/api/auth/logout"}, {"POST", "/api/me"}, {"DELETE", "/api/me"}} {
		w := h.do(c[0], c[1], "{}")
		if w.Code != 404 && w.Code != 403 { // POST without csrf headers hits CSRF first
			t.Errorf("%v: %d", c, w.Code)
		}
	}
	if w := h.do("POST", "/api/me", "{}"); w.Code != 404 {
		t.Errorf("POST /api/me: %d", w.Code)
	}
}

// ---- acceptance 10: isolation ----------------------------------------------------------------

func TestSessionOfAUserCannotActAsB(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	a, ca := h.loginOK("sub-A", "a@example.com", "A")
	b, cb := h.loginOK("sub-B", "b@example.com", "B")
	who := func(cookie string, opts ...func(*http.Request)) int64 {
		w := h.do("POST", "/api/whoami", "{}", append([]func(*http.Request){withCookie(cookie)}, opts...)...)
		var out struct{ ID int64 }
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.ID
	}
	if who(ca) != a.User.ID || who(cb) != b.User.ID {
		t.Fatal("whoami returned the wrong user")
	}
	// nothing the request carries can change the user
	if who(ca, withHeader("X-User-Id", itoa(b.User.ID)), withHeader("X-Ngajio-User", itoa(b.User.ID))) != a.User.ID {
		t.Fatal("header changed the user")
	}
	w := h.do("GET", "/api/me?user_id="+itoa(b.User.ID)+"&id="+itoa(b.User.ID), "", withCookie(ca))
	var me struct{ User struct{ ID int64 } }
	json.Unmarshal(w.Body.Bytes(), &me)
	if me.User.ID != a.User.ID {
		t.Fatalf("query param changed the user: %s", w.Body)
	}
	// two cookies: the first one wins and nothing mixes
	w = h.do("GET", "/api/me", "", withCookie(ca), withCookie(cb))
	json.Unmarshal(w.Body.Bytes(), &me)
	if me.User.ID != a.User.ID && me.User.ID != b.User.ID {
		t.Fatalf("%s", w.Body)
	}
}

func TestDeleteUserCascadesSessions(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	a, ca := h.loginOK("sub-A", "a@example.com", "A")
	_, cb := h.loginOK("sub-B", "b@example.com", "B")
	h.loginOK("sub-A", "a@example.com", "A")
	if h.count(`SELECT count(*) FROM sessions WHERE user_id=?`, a.User.ID) != 2 {
		t.Fatal("setup")
	}
	h.st.Tx(context.Background(), func(tx *storeTx) error {
		_, err := tx.Exec(`DELETE FROM users WHERE id=?`, a.User.ID)
		return err
	})
	if n := h.count(`SELECT count(*) FROM sessions WHERE user_id=?`, a.User.ID); n != 0 {
		t.Fatalf("sessions left: %d", n)
	}
	if w := h.do("GET", "/api/me", "", withCookie(ca)); w.Code != 401 {
		t.Fatalf("deleted user's cookie: %d", w.Code)
	}
	if w := h.do("GET", "/api/me", "", withCookie(cb)); w.Code != 200 {
		t.Fatalf("other user: %d", w.Code)
	}
}

// ---- acceptance 12: no personal data in logs -------------------------------------------------------

func TestNoPersonalDataInLogs(t *testing.T) {
	h := newHarness(t, harnessOpts{})
	cred := h.token("sub-secret-77", "rahasia@example.com", "Nama Rahasia")
	w := h.google(cred)
	c := cookieOf(w)
	h.do("GET", "/api/me", "", withCookie(c))
	h.do("POST", "/api/auth/dev", `{"email":"devrahasia@example.com"}`)
	h.do("POST", "/api/auth/logout", "{}", withCookie(c))
	h.google(cred + "x")
	logs := h.logs.String()
	for _, secret := range []string{"rahasia", "Rahasia", "sub-secret", c, cred, cred[:25], "eyJ"} {
		if strings.Contains(logs, secret) {
			t.Errorf("log contains %q:\n%s", secret, logs)
		}
	}
	if !strings.Contains(logs, `path="/api/auth/google" status=200`) {
		t.Errorf("no request line:\n%s", logs)
	}
}
