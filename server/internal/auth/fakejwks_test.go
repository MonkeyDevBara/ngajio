package auth

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testClientID = "client-123.apps.googleusercontent.com"

// Test RSA keys are generated at test time, once per process.
var (
	keyOnce sync.Once
	keys    []*rsa.PrivateKey
)

func testKey(t testing.TB, i int) *rsa.PrivateKey {
	t.Helper()
	keyOnce.Do(func() {
		for n := 0; n < 3; n++ {
			k, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				panic(err)
			}
			keys = append(keys, k)
		}
	})
	return keys[i]
}

// fakeClock is a settable clock shared by the verifier and the handlers.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_800_000_000, 0)} }

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// fakeJWKS serves a JWKS from httptest and mints tokens signed by its keys.
type fakeJWKS struct {
	srv  *httptest.Server
	hits atomic.Int32

	mu           sync.Mutex
	keys         map[string]*rsa.PrivateKey // kid -> key currently published
	cacheControl string
	status       int           // 0 means 200
	body         []byte        // when set, served instead of the key set
	delay        time.Duration // response delay
}

// newFakeJWKS publishes key 0 under kid "k1" with Cache-Control max-age=3600.
func newFakeJWKS(t testing.TB) *fakeJWKS {
	f := &fakeJWKS{keys: map[string]*rsa.PrivateKey{"k1": testKey(t, 0)}, cacheControl: "public, max-age=3600"}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeJWKS) URL() string { return f.srv.URL }
func (f *fakeJWKS) Hits() int   { return int(f.hits.Load()) }

func (f *fakeJWKS) set(fn func()) { f.mu.Lock(); fn(); f.mu.Unlock() }

func (f *fakeJWKS) serve(w http.ResponseWriter, r *http.Request) {
	f.hits.Add(1)
	f.mu.Lock()
	cc, status, body, delay := f.cacheControl, f.status, f.body, f.delay
	doc := jwksDoc(f.keys)
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	if cc != "" {
		w.Header().Set("Cache-Control", cc)
	}
	if status != 0 {
		w.WriteHeader(status)
	}
	if body != nil {
		w.Write(body)
		return
	}
	w.Write(doc)
}

func jwksDoc(keys map[string]*rsa.PrivateKey) []byte {
	type jwk struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		Alg string `json:"alg"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	var out struct {
		Keys []jwk `json:"keys"`
	}
	for kid, k := range keys {
		out.Keys = append(out.Keys, jwk{kid, "RSA", "RS256", "sig",
			b64(k.N.Bytes()), b64(big.NewInt(int64(k.E)).Bytes())})
	}
	b, _ := json.Marshal(out)
	return b
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// claimsAt returns a valid claim set for now.
func claimsAt(now time.Time) map[string]any {
	return map[string]any{
		"iss": "https://accounts.google.com", "aud": testClientID, "sub": "sub-1",
		"email": "ust@example.com", "email_verified": true, "name": "Ust A",
		"iat": now.Add(-10 * time.Second).Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

// tok describes a token to mint. Mutate returns the header and claims to
// change before signing.
type tok struct {
	Key    *rsa.PrivateKey // nil: key 0 (the published "k1")
	Mutate func(h, c map[string]any)
}

// mint builds an RS256 token valid at now, after applying m.Mutate. A nil
// value in the maps deletes the field.
func mint(t testing.TB, now time.Time, m tok) string {
	t.Helper()
	h := map[string]any{"alg": "RS256", "kid": "k1", "typ": "JWT"}
	c := claimsAt(now)
	if m.Mutate != nil {
		m.Mutate(h, c)
	}
	for _, mp := range []map[string]any{h, c} {
		for k, v := range mp {
			if v == nil {
				delete(mp, k)
			}
		}
	}
	key := m.Key
	if key == nil {
		key = testKey(t, 0)
	}
	signing := b64(mustJSON(h)) + "." + b64(mustJSON(c))
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64(sig)
}

// mintHS256 signs with HMAC-SHA256 using secret (an attacker might use the
// public key bytes), with alg HS256.
func mintHS256(t testing.TB, now time.Time, secret []byte, alg string) string {
	h := map[string]any{"alg": alg, "kid": "k1", "typ": "JWT"}
	signing := b64(mustJSON(h)) + "." + b64(mustJSON(claimsAt(now)))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signing))
	return signing + "." + b64(mac.Sum(nil))
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// mintRaw signs already-encoded header and payload segments with key.
func mintRaw(t testing.TB, key *rsa.PrivateKey, h64, p64 string) string {
	t.Helper()
	signing := h64 + "." + p64
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64(sig)
}
