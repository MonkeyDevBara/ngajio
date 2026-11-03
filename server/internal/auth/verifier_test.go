package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func newVerifier(t *testing.T) (*GoogleVerifier, *fakeJWKS, *fakeClock) {
	t.Helper()
	f := newFakeJWKS(t)
	c := newFakeClock()
	return NewGoogleVerifier(testClientID, f.URL(), f.srv.Client(), c.Now), f, c
}

func code(err error) string {
	var ve *VerifyError
	if errors.As(err, &ve) {
		return ve.Code
	}
	if err == nil {
		return ""
	}
	return "NOT_A_VERIFYERROR:" + err.Error()
}

func TestVerifyValid(t *testing.T) {
	v, _, c := newVerifier(t)
	cl, err := v.Verify(context.Background(), mint(t, c.Now(), tok{}))
	if err != nil {
		t.Fatal(err)
	}
	if cl != (Claims{Sub: "sub-1", Email: "ust@example.com", Name: "Ust A", EmailVerified: true}) {
		t.Fatalf("claims: %+v", cl)
	}
}

func TestVerifyAccepts(t *testing.T) {
	v, _, c := newVerifier(t)
	now := c.Now()
	for name, m := range map[string]func(h, c map[string]any){
		"iss without scheme":         func(h, c map[string]any) { c["iss"] = "accounts.google.com" },
		"email_verified string true": func(h, c map[string]any) { c["email_verified"] = "true" },
		"aud array of only the id":   func(h, c map[string]any) { c["aud"] = []string{testClientID} },
		"exp inside skew":            func(h, c map[string]any) { c["exp"] = now.Add(-59 * time.Second).Unix() },
		"nbf inside skew":            func(h, c map[string]any) { c["nbf"] = now.Add(59 * time.Second).Unix() },
		"nbf in the past":            func(h, c map[string]any) { c["nbf"] = now.Add(-time.Hour).Unix() },
		"iat inside skew":            func(h, c map[string]any) { c["iat"] = now.Add(59 * time.Second).Unix() },
		"no name":                    func(h, c map[string]any) { c["name"] = nil },
		"name not a string":          func(h, c map[string]any) { c["name"] = 5 },
		"sub of 255 bytes":           func(h, c map[string]any) { c["sub"] = strings.Repeat("1", 255) },
		"email of 254 bytes":         func(h, c map[string]any) { c["email"] = strings.Repeat("a", 254) },
		"extra claims and header":    func(h, c map[string]any) { c["hd"] = "x.id"; h["x5t"] = "abc" },
	} {
		if _, err := v.Verify(context.Background(), mint(t, now, tok{Mutate: m})); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestVerifyRejects(t *testing.T) {
	v, _, c := newVerifier(t)
	now := c.Now()
	rejects := []struct {
		name, want string
		m          func(h, c map[string]any)
		key        *rsa.PrivateKey
	}{
		{"forged signature (other key)", "signature", nil, testKey(t, 1)},
		{"exp past by more than skew", "exp", func(h, c map[string]any) { c["exp"] = now.Add(-61 * time.Second).Unix() }, nil},
		{"exp exactly at the skew edge", "exp", func(h, c map[string]any) { c["exp"] = now.Add(-60 * time.Second).Unix() }, nil},
		{"exp long ago", "exp", func(h, c map[string]any) { c["exp"] = 1 }, nil},
		{"exp missing", "exp", func(h, c map[string]any) { c["exp"] = nil }, nil},
		{"exp is a string", "payload_json", func(h, c map[string]any) { c["exp"] = "9999999999" }, nil},
		{"wrong aud", "aud", func(h, c map[string]any) { c["aud"] = "other-client" }, nil},
		{"aud with prefix of the id", "aud", func(h, c map[string]any) { c["aud"] = testClientID + "x" }, nil},
		{"aud empty", "aud", func(h, c map[string]any) { c["aud"] = "" }, nil},
		{"aud missing", "aud", func(h, c map[string]any) { c["aud"] = nil }, nil},
		{"aud array with another audience", "aud", func(h, c map[string]any) { c["aud"] = []string{testClientID, "evil"} }, nil},
		{"aud array other first", "aud", func(h, c map[string]any) { c["aud"] = []string{"evil", testClientID} }, nil},
		{"aud number", "aud", func(h, c map[string]any) { c["aud"] = 5 }, nil},
		{"wrong iss", "iss", func(h, c map[string]any) { c["iss"] = "https://evil.example" }, nil},
		{"iss with trailing slash", "iss", func(h, c map[string]any) { c["iss"] = "https://accounts.google.com/" }, nil},
		{"iss http", "iss", func(h, c map[string]any) { c["iss"] = "http://accounts.google.com" }, nil},
		{"iss missing", "iss", func(h, c map[string]any) { c["iss"] = nil }, nil},
		{"email_verified false", "email_unverified", func(h, c map[string]any) { c["email_verified"] = false }, nil},
		{"email_verified string false", "email_unverified", func(h, c map[string]any) { c["email_verified"] = "false" }, nil},
		{"email_verified missing", "email_unverified", func(h, c map[string]any) { c["email_verified"] = nil }, nil},
		{"email_verified True", "email_unverified", func(h, c map[string]any) { c["email_verified"] = "True" }, nil},
		{"email_verified 1", "email_unverified", func(h, c map[string]any) { c["email_verified"] = 1 }, nil},
		{"email_verified null", "email_unverified", func(h, c map[string]any) { c["email_verified"] = json.RawMessage("null") }, nil},
		{"alg none", "alg", func(h, c map[string]any) { h["alg"] = "none" }, nil},
		{"alg None", "alg", func(h, c map[string]any) { h["alg"] = "None" }, nil},
		{"alg lowercase rs256", "alg", func(h, c map[string]any) { h["alg"] = "rs256" }, nil},
		{"alg RS384", "alg", func(h, c map[string]any) { h["alg"] = "RS384" }, nil},
		{"alg missing", "alg", func(h, c map[string]any) { h["alg"] = nil }, nil},
		{"unknown kid", "kid_unknown", func(h, c map[string]any) { h["kid"] = "nope" }, nil},
		{"missing kid", "kid_missing", func(h, c map[string]any) { h["kid"] = nil }, nil},
		{"empty kid", "kid_missing", func(h, c map[string]any) { h["kid"] = "" }, nil},
		{"huge kid", "kid_missing", func(h, c map[string]any) { h["kid"] = strings.Repeat("k", 200) }, nil},
		{"nbf in the future beyond skew", "nbf", func(h, c map[string]any) { c["nbf"] = now.Add(61 * time.Second).Unix() }, nil},
		{"nbf far in the future", "nbf", func(h, c map[string]any) { c["nbf"] = now.Add(24 * time.Hour).Unix() }, nil},
		{"iat beyond skew", "iat", func(h, c map[string]any) { c["iat"] = now.Add(61 * time.Second).Unix() }, nil},
		{"iat far in the future", "iat", func(h, c map[string]any) { c["iat"] = now.Add(10 * 365 * 24 * time.Hour).Unix() }, nil},
		{"sub empty", "sub", func(h, c map[string]any) { c["sub"] = "" }, nil},
		{"sub missing", "sub", func(h, c map[string]any) { c["sub"] = nil }, nil},
		{"sub 256 bytes", "sub", func(h, c map[string]any) { c["sub"] = strings.Repeat("1", 256) }, nil},
		{"sub with control char", "sub", func(h, c map[string]any) { c["sub"] = "a\nb" }, nil},
		{"sub not a string", "payload_json", func(h, c map[string]any) { c["sub"] = 12345 }, nil},
		{"email empty", "email", func(h, c map[string]any) { c["email"] = "" }, nil},
		{"email missing", "email", func(h, c map[string]any) { c["email"] = nil }, nil},
		{"email 255 bytes", "email", func(h, c map[string]any) { c["email"] = strings.Repeat("a", 255) }, nil},
		{"email with control char", "email", func(h, c map[string]any) { c["email"] = "a@b.co\r\nBcc: x" }, nil},
	}
	for _, r := range rejects {
		tk := mint(t, now, tok{Key: r.key, Mutate: r.m})
		_, err := v.Verify(context.Background(), tk)
		if got := code(err); got != r.want {
			t.Errorf("%s: code %q, want %q", r.name, got, r.want)
		}
		if err != nil && strings.Contains(err.Error(), tk[:20]) {
			t.Errorf("%s: error leaks token content", r.name)
		}
	}
}

func TestVerifyRejectsHMACWithPublicKeyAsSecret(t *testing.T) {
	v, f, c := newVerifier(t)
	pub := testKey(t, 0).PublicKey
	secrets := [][]byte{pub.N.Bytes(), []byte(string(jwksDoc(f.keys))), []byte("secret")}
	for _, alg := range []string{"HS256", "HS384", "HS512"} {
		for _, s := range secrets {
			tk := mintHS256(t, c.Now(), s, alg)
			if _, err := v.Verify(context.Background(), tk); code(err) != "alg" {
				t.Errorf("%s: %v", alg, err)
			}
		}
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	v, _, c := newVerifier(t)
	good := mint(t, c.Now(), tok{})
	parts := strings.Split(good, ".")
	cases := []struct{ name, tok, want string }{
		{"empty", "", "empty"},
		{"one segment", parts[0], "segments"},
		{"two segments", parts[0] + "." + parts[1], "segments"},
		{"four segments", good + "." + parts[2], "segments"},
		{"five dots", "a.b.c.d.e.f", "segments"},
		{"only dots", "..", "segments"},
		{"empty signature", parts[0] + "." + parts[1] + ".", "segments"},
		{"empty header", "." + parts[1] + "." + parts[2], "segments"},
		{"oversized", strings.Repeat("a", 8193), "too_large"},
		{"oversized with dots", strings.Repeat("a", 4000) + "." + strings.Repeat("b", 4000) + "." + strings.Repeat("c", 400), "too_large"},
		{"oversized signature", parts[0] + "." + parts[1] + "." + strings.Repeat("A", 1500), "too_large"},
		{"bad base64 header", "!!!." + parts[1] + "." + parts[2], "base64"},
		{"bad base64 payload", parts[0] + ".@@@." + parts[2], "base64"},
		{"bad base64 signature", parts[0] + "." + parts[1] + ".%%%", "base64"},
		{"padded base64", parts[0] + "=." + parts[1] + "." + parts[2], "base64"},
		{"std alphabet", parts[0] + "." + parts[1] + "." + strings.ReplaceAll(parts[2], "-", "+") + "/+", "base64"},
		{"header not json", b64([]byte("nope")) + "." + parts[1] + "." + parts[2], "header_json"},
		{"header is an array", b64([]byte("[]")) + "." + parts[1] + "." + parts[2], "header_json"},
		{"payload not json", parts[0] + "." + b64([]byte("nope")) + "." + parts[2], "signature"},
		{"whitespace token", " ", "segments"},
		{"nul bytes", "\x00.\x00.\x00", "base64"},
	}
	for _, tc := range cases {
		_, err := v.Verify(context.Background(), tc.tok)
		if got := code(err); got != tc.want {
			t.Errorf("%s: code %q, want %q", tc.name, got, tc.want)
		}
	}
	// a correctly signed payload that is not JSON
	h := b64(mustJSON(map[string]any{"alg": "RS256", "kid": "k1"}))
	if _, err := v.Verify(context.Background(), signRaw(t, h, b64([]byte("nope")))); code(err) != "payload_json" {
		t.Errorf("signed non-json payload: %v", err)
	}
}

func signRaw(t *testing.T, h64, p64 string) string {
	t.Helper()
	// reuse mint's signing by minting then swapping segments is not possible, so sign here
	return mintRaw(t, testKey(t, 0), h64, p64)
}

func TestVerifierClientIDEmptyNeverMatches(t *testing.T) {
	f := newFakeJWKS(t)
	c := newFakeClock()
	v := NewGoogleVerifier("", f.URL(), f.srv.Client(), c.Now)
	tk := mint(t, c.Now(), tok{Mutate: func(h, cl map[string]any) { cl["aud"] = "" }})
	if _, err := v.Verify(context.Background(), tk); code(err) != "aud" {
		t.Fatalf("empty client id must not match an empty aud: %v", err)
	}
}

// ---- JWKS cache, rotation, failure ------------------------------------------

func verifyKid(t *testing.T, v *GoogleVerifier, c *fakeClock, kid string, key *rsa.PrivateKey) error {
	t.Helper()
	tk := mint(t, c.Now(), tok{Key: key, Mutate: func(h, cl map[string]any) { h["kid"] = kid }})
	_, err := v.Verify(context.Background(), tk)
	return err
}

func TestJWKSCacheHonoured(t *testing.T) {
	v, f, c := newVerifier(t)
	for i := 0; i < 5; i++ {
		if err := verifyKid(t, v, c, "k1", nil); err != nil {
			t.Fatal(err)
		}
	}
	if f.Hits() != 1 {
		t.Fatalf("hits %d, want 1", f.Hits())
	}
	c.Add(3599 * time.Second)
	if err := verifyKid(t, v, c, "k1", nil); err != nil || f.Hits() != 1 {
		t.Fatalf("still fresh: err=%v hits=%d", err, f.Hits())
	}
	c.Add(2 * time.Second) // max-age=3600 passed
	if err := verifyKid(t, v, c, "k1", nil); err != nil || f.Hits() != 2 {
		t.Fatalf("expired cache must refetch: err=%v hits=%d", err, f.Hits())
	}
}

func TestCacheTTL(t *testing.T) {
	h := func(cc, age string) http.Header {
		hd := http.Header{}
		if cc != "" {
			hd.Set("Cache-Control", cc)
		}
		if age != "" {
			hd.Set("Age", age)
		}
		return hd
	}
	for _, tc := range []struct {
		cc, age string
		want    time.Duration
	}{
		{"", "", time.Minute},
		{"public, max-age=20000, must-revalidate", "", 20000 * time.Second},
		{"max-age=5", "", time.Minute},
		{"max-age=0", "", time.Minute},
		{"max-age=999999999", "", 24 * time.Hour},
		{"no-store", "", time.Minute},
		{"max-age=abc", "", time.Minute},
		{"max-age=-5", "", time.Minute},
		{"MAX-AGE=600", "", 600 * time.Second},
		{"max-age=3600", "1000", 2600 * time.Second},
		{"max-age=3600", "99999", time.Minute},
		{"max-age=3600", "junk", 3600 * time.Second},
	} {
		if got := cacheTTL(h(tc.cc, tc.age)); got != tc.want {
			t.Errorf("cc=%q age=%q: %v, want %v", tc.cc, tc.age, got, tc.want)
		}
	}
}

func TestJWKSShortMaxAgeClampedToMinute(t *testing.T) {
	v, f, c := newVerifier(t)
	f.set(func() { f.cacheControl = "max-age=1" })
	verifyKid(t, v, c, "k1", nil)
	c.Add(30 * time.Second)
	verifyKid(t, v, c, "k1", nil)
	if f.Hits() != 1 {
		t.Fatalf("min TTL is 60 s, hits %d", f.Hits())
	}
	c.Add(31 * time.Second)
	verifyKid(t, v, c, "k1", nil)
	if f.Hits() != 2 {
		t.Fatalf("hits %d", f.Hits())
	}
}

func TestJWKSRotationRefetchAtMostOncePerMinute(t *testing.T) {
	v, f, c := newVerifier(t)
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatal(err)
	}
	// Google rotates: k2 appears. Within a minute of the last fetch, no refetch.
	f.set(func() { f.keys["k2"] = testKey(t, 1) })
	c.Add(30 * time.Second)
	if err := verifyKid(t, v, c, "k2", testKey(t, 1)); code(err) != "kid_unknown" || f.Hits() != 1 {
		t.Fatalf("throttled: %v hits=%d", err, f.Hits())
	}
	c.Add(31 * time.Second)
	if err := verifyKid(t, v, c, "k2", testKey(t, 1)); err != nil || f.Hits() != 2 {
		t.Fatalf("after a minute: %v hits=%d", err, f.Hits())
	}
	// an unknown kid right after costs no fetch, a minute later it costs one
	for i := 0; i < 20; i++ {
		if err := verifyKid(t, v, c, "bogus", nil); code(err) != "kid_unknown" {
			t.Fatal(err)
		}
	}
	if f.Hits() != 2 {
		t.Fatalf("bogus kids caused fetches: %d", f.Hits())
	}
	c.Add(61 * time.Second)
	verifyKid(t, v, c, "bogus", nil)
	verifyKid(t, v, c, "bogus", nil)
	if f.Hits() != 3 {
		t.Fatalf("hits %d, want 3", f.Hits())
	}
	// known keys keep working throughout
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatal(err)
	}
}

func TestJWKSConcurrentColdStartOneFetch(t *testing.T) {
	v, f, c := newVerifier(t)
	f.set(func() { f.delay = 150 * time.Millisecond })
	tk := mint(t, c.Now(), tok{})
	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := v.Verify(context.Background(), tk)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if f.Hits() != 1 {
		t.Fatalf("hits %d, want 1", f.Hits())
	}
}

func TestJWKSConcurrentUnknownKidOneFetch(t *testing.T) {
	v, f, c := newVerifier(t)
	verifyKid(t, v, c, "k1", nil)
	f.set(func() { f.keys["k2"] = testKey(t, 1); f.delay = 150 * time.Millisecond })
	c.Add(2 * time.Minute)
	tk := mint(t, c.Now(), tok{Key: testKey(t, 1), Mutate: func(h, cl map[string]any) { h["kid"] = "k2" }})
	bogus := mint(t, c.Now(), tok{Mutate: func(h, cl map[string]any) { h["kid"] = "zzz" }})
	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := 0; i < 30; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, err := v.Verify(context.Background(), tk); errs <- err }()
		go func() {
			defer wg.Done()
			if _, err := v.Verify(context.Background(), bogus); code(err) != "kid_unknown" {
				errs <- fmt.Errorf("bogus: %v", err)
				return
			}
			errs <- nil
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if f.Hits() != 2 {
		t.Fatalf("hits %d, want 2 (cold start + one refetch)", f.Hits())
	}
}

func TestJWKSDownWithFreshCacheStillVerifies(t *testing.T) {
	v, f, c := newVerifier(t)
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatal(err)
	}
	f.srv.Close() // server gone
	c.Add(10 * time.Minute)
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatalf("fresh cache must keep working: %v", err)
	}
}

func TestJWKSDownWithExpiredCacheNeverFailsOpen(t *testing.T) {
	v, f, c := newVerifier(t)
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatal(err)
	}
	f.set(func() { f.status = 500 })
	c.Add(2 * time.Hour) // cache expired
	if err := verifyKid(t, v, c, "k1", nil); code(err) != "jwks_unavailable" {
		t.Fatalf("expired cache + failed fetch must be an error, got %v", err)
	}
	hits := f.Hits()
	// failures are not retried at once by every request
	for i := 0; i < 10; i++ {
		if err := verifyKid(t, v, c, "k1", nil); code(err) != "jwks_unavailable" {
			t.Fatal(err)
		}
	}
	if f.Hits() != hits {
		t.Fatalf("failed fetch retried within the backoff: %d -> %d", hits, f.Hits())
	}
	// recovery
	f.set(func() { f.status = 0 })
	c.Add(6 * time.Second)
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatalf("after recovery: %v", err)
	}
}

func TestJWKSServerClosedAndNeverReached(t *testing.T) {
	f := newFakeJWKS(t)
	c := newFakeClock()
	url := f.URL()
	f.srv.Close()
	v := NewGoogleVerifier(testClientID, url, nil, c.Now)
	if err := verifyKid(t, v, c, "k1", nil); code(err) != "jwks_unavailable" {
		t.Fatalf("cold start with no JWKS server must fail: %v", err)
	}
}

func TestJWKSBadBodies(t *testing.T) {
	good := string(jwksDoc(map[string]*rsa.PrivateKey{"k1": testKey(t, 0)}))
	weak, _ := rsa.GenerateKey(rand.Reader, 1024)
	for name, f := range map[string]struct {
		status int
		body   string
	}{
		"oversized body":      {0, good + strings.Repeat(" ", 100<<10)},
		"not json":            {0, "<html>nope</html>"},
		"empty body":          {0, ""},
		"no keys":             {0, `{"keys":[]}`},
		"keys is a string":    {0, `{"keys":"x"}`},
		"only EC key":         {0, `{"keys":[{"kid":"k1","kty":"EC","crv":"P-256","x":"a","y":"b"}]}`},
		"bad n":               {0, `{"keys":[{"kid":"k1","kty":"RSA","alg":"RS256","n":"!!","e":"AQAB"}]}`},
		"tiny n":              {0, `{"keys":[{"kid":"k1","kty":"RSA","alg":"RS256","n":"AQAB","e":"AQAB"}]}`},
		"1024 bit key":        {0, string(jwksDoc(map[string]*rsa.PrivateKey{"k1": weak}))},
		"wrong alg":           {0, strings.Replace(good, `"RS256"`, `"RS384"`, 1)},
		"500":                 {500, good},
		"404":                 {404, good},
		"redirect to nowhere": {302, good},
	} {
		v, fj, c := newVerifier(t)
		fj.set(func() { fj.status = f.status; fj.body = []byte(f.body) })
		if err := verifyKid(t, v, c, "k1", nil); code(err) != "jwks_unavailable" {
			t.Errorf("%s: want jwks_unavailable, got %v", name, err)
		}
	}
	// just under the cap is fine
	v, fj, c := newVerifier(t)
	fj.set(func() { fj.body = []byte(good + strings.Repeat(" ", 100<<10-len(good))) })
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Errorf("body of exactly 100 KB: %v", err)
	}
}

func TestJWKSSkipsUnusableKeysKeepsGood(t *testing.T) {
	v, f, c := newVerifier(t)
	good := string(jwksDoc(map[string]*rsa.PrivateKey{"k1": testKey(t, 0)}))
	body := strings.Replace(good, `{"keys":[`, `{"keys":[{"kid":"ec","kty":"EC"},{"kid":"","kty":"RSA","n":"AA","e":"AQAB"},`, 1)
	f.set(func() { f.body = []byte(body) })
	if err := verifyKid(t, v, c, "k1", nil); err != nil {
		t.Fatal(err)
	}
}

func TestJWKSFetchTimeout(t *testing.T) {
	v, f, c := newVerifier(t)
	v.timeout = 100 * time.Millisecond
	f.set(func() { f.delay = 3 * time.Second })
	start := time.Now()
	err := verifyKid(t, v, c, "k1", nil)
	if code(err) != "jwks_unavailable" || time.Since(start) > 2*time.Second {
		t.Fatalf("timeout not respected: %v after %v", err, time.Since(start))
	}
}

func TestDefaultFetchTimeoutIsFiveSeconds(t *testing.T) {
	v, _, _ := newVerifier(t)
	if v.timeout != 5*time.Second {
		t.Fatalf("timeout %v", v.timeout)
	}
}

func TestVerifyCallerCancel(t *testing.T) {
	v, f, c := newVerifier(t)
	f.set(func() { f.delay = 2 * time.Second })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	tk := mint(t, c.Now(), tok{})
	start := time.Now()
	_, err := v.Verify(ctx, tk)
	if code(err) != "canceled" || time.Since(start) > time.Second {
		t.Fatalf("%v after %v", err, time.Since(start))
	}
}

func TestVerifyErrorIsAlwaysVerifyError(t *testing.T) {
	v, f, c := newVerifier(t)
	f.set(func() { f.status = 500 })
	_, err := v.Verify(context.Background(), mint(t, c.Now(), tok{}))
	var ve *VerifyError
	if !errors.As(err, &ve) || strings.Contains(err.Error(), f.URL()) {
		t.Fatalf("%v", err)
	}
}
