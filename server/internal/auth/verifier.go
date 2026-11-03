package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Claims are the verified identity claims of a Google ID token.
type Claims struct {
	Sub           string
	Email         string
	Name          string
	EmailVerified bool
}

// Verifier checks an ID token. Every failure is a *VerifyError.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (Claims, error)
}

// VerifyError carries only a short reason code, safe to log. It never holds
// token content. Clients see one generic 401 whatever the code.
type VerifyError struct{ Code string }

func (e *VerifyError) Error() string { return "verify: " + e.Code }

func fail(code string) error { return &VerifyError{Code: code} }

// GoogleCertsURL is where Google publishes its ID token signing keys.
const GoogleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

const (
	maxTokenBytes   = 8 << 10
	maxHeaderBytes  = 2 << 10
	maxSigBytes     = 1 << 10
	maxJWKSBytes    = 100 << 10
	maxKeys         = 20
	minKeyBits      = 2048
	skew            = 60 * time.Second
	minCacheTTL     = 60 * time.Second
	maxCacheTTL     = 24 * time.Hour
	unknownKidEvery = time.Minute // refetch on an unknown kid at most this often
	failBackoff     = 5 * time.Second
	fetchTimeout    = 5 * time.Second
	maxSub          = 255
	maxEmail        = 254
)

var issuers = map[string]bool{"https://accounts.google.com": true, "accounts.google.com": true}

// GoogleVerifier verifies RS256 Google ID tokens with the standard library.
type GoogleVerifier struct {
	clientID string
	url      string
	hc       *http.Client
	now      func() time.Time
	timeout  time.Duration

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expires   time.Time // the cached key set is valid until here
	lastFetch time.Time // start of the latest fetch, success or failure
	lastFail  bool
	inflight  *fetchCall
}

type fetchCall struct {
	done chan struct{}
	keys map[string]*rsa.PublicKey // set before done is closed
	err  error
}

// NewGoogleVerifier builds a verifier for clientID. jwksURL is normally
// GoogleCertsURL; hc may be nil; now may be nil (time.Now).
func NewGoogleVerifier(clientID, jwksURL string, hc *http.Client, now func() time.Time) *GoogleVerifier {
	if hc == nil {
		hc = &http.Client{}
	}
	if now == nil {
		now = time.Now
	}
	return &GoogleVerifier{clientID: clientID, url: jwksURL, hc: hc, now: now, timeout: fetchTimeout}
}

// Verify checks structure, algorithm, signature and claims. The returned error
// is always a *VerifyError.
func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (Claims, error) {
	var zero Claims
	if idToken == "" {
		return zero, fail("empty")
	}
	if len(idToken) > maxTokenBytes {
		return zero, fail("too_large")
	}
	if strings.Count(idToken, ".") != 2 {
		return zero, fail("segments")
	}
	i := strings.IndexByte(idToken, '.')
	j := i + 1 + strings.IndexByte(idToken[i+1:], '.')
	h64, p64, s64 := idToken[:i], idToken[i+1:j], idToken[j+1:]
	if h64 == "" || p64 == "" || s64 == "" {
		return zero, fail("segments")
	}
	if len(h64) > maxHeaderBytes || len(s64) > maxSigBytes {
		return zero, fail("too_large")
	}
	hb, err1 := base64.RawURLEncoding.DecodeString(h64)
	pb, err2 := base64.RawURLEncoding.DecodeString(p64)
	sig, err3 := base64.RawURLEncoding.DecodeString(s64)
	if err1 != nil || err2 != nil || err3 != nil {
		return zero, fail("base64")
	}

	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(hb, &hdr) != nil {
		return zero, fail("header_json")
	}
	if hdr.Alg != "RS256" {
		return zero, fail("alg")
	}
	if hdr.Kid == "" || len(hdr.Kid) > 128 {
		return zero, fail("kid_missing")
	}

	pub, err := v.key(ctx, hdr.Kid)
	if err != nil {
		return zero, err
	}
	sum := sha256.Sum256([]byte(h64 + "." + p64))
	if rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig) != nil {
		return zero, fail("signature")
	}
	return v.checkClaims(pb)
}

type rawClaims struct {
	Iss           string          `json:"iss"`
	Aud           json.RawMessage `json:"aud"`
	Sub           string          `json:"sub"`
	Email         string          `json:"email"`
	EmailVerified json.RawMessage `json:"email_verified"`
	Name          json.RawMessage `json:"name"`
	Exp           *float64        `json:"exp"`
	Iat           *float64        `json:"iat"`
	Nbf           *float64        `json:"nbf"`
}

func (v *GoogleVerifier) checkClaims(payload []byte) (Claims, error) {
	var zero Claims
	var c rawClaims
	if json.Unmarshal(payload, &c) != nil {
		return zero, fail("payload_json")
	}
	if !issuers[c.Iss] {
		return zero, fail("iss")
	}
	if !audOK(c.Aud, v.clientID) {
		return zero, fail("aud")
	}
	now := v.now()
	if c.Exp == nil || !finite(*c.Exp) || !now.Add(-skew).Before(unix(*c.Exp)) {
		return zero, fail("exp")
	}
	if c.Iat != nil && (!finite(*c.Iat) || unix(*c.Iat).After(now.Add(skew))) {
		return zero, fail("iat")
	}
	if c.Nbf != nil && (!finite(*c.Nbf) || unix(*c.Nbf).After(now.Add(skew))) {
		return zero, fail("nbf")
	}
	if !emailVerified(c.EmailVerified) {
		return zero, fail("email_unverified")
	}
	if c.Sub == "" || len(c.Sub) > maxSub || !cleanText(c.Sub) {
		return zero, fail("sub")
	}
	if c.Email == "" || len(c.Email) > maxEmail || !cleanText(c.Email) {
		return zero, fail("email")
	}
	var name string
	if json.Unmarshal(c.Name, &name) != nil {
		name = ""
	}
	return Claims{Sub: c.Sub, Email: c.Email, Name: name, EmailVerified: true}, nil
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) && math.Abs(f) < 1e15 }

func unix(f float64) time.Time { return time.Unix(int64(f), 0) }

// audOK accepts aud as the exact client id, or as an array holding only it.
func audOK(raw json.RawMessage, clientID string) bool {
	if clientID == "" {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s == clientID
	}
	var a []string
	if json.Unmarshal(raw, &a) == nil {
		return len(a) == 1 && a[0] == clientID
	}
	return false
}

// emailVerified accepts the boolean true and the string "true" (Google has
// sent both) and nothing else.
func emailVerified(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "true" || s == `"true"`
}

// cleanText rejects invalid UTF-8 and control characters.
func cleanText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// ---- key set ---------------------------------------------------------------

// key returns the public key for kid. It serves the cache while it is fresh,
// refetches when the cache expired, and refetches for an unknown kid at most
// once per unknownKidEvery. Concurrent callers share one fetch. It never serves
// an expired cache: if the fetch fails, the verification fails.
func (v *GoogleVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	now := v.now()
	fresh := v.keys != nil && now.Before(v.expires)
	if fresh {
		if k, ok := v.keys[kid]; ok {
			v.mu.Unlock()
			return k, nil
		}
	}
	call := v.inflight
	if call == nil {
		switch {
		case fresh && now.Sub(v.lastFetch) < unknownKidEvery:
			v.mu.Unlock()
			return nil, fail("kid_unknown")
		case !fresh && v.lastFail && now.Sub(v.lastFetch) < failBackoff:
			v.mu.Unlock()
			return nil, fail("jwks_unavailable")
		}
		call = &fetchCall{done: make(chan struct{})}
		v.inflight = call
		v.lastFetch = now
		go v.fetch(call)
	}
	v.mu.Unlock()

	select {
	case <-call.done:
	case <-ctx.Done():
		return nil, fail("canceled")
	}
	if call.err != nil {
		return nil, fail("jwks_unavailable")
	}
	if k, ok := call.keys[kid]; ok {
		return k, nil
	}
	return nil, fail("kid_unknown")
}

// fetch downloads the key set and publishes it. It runs on its own timeout so a
// canceled first caller does not fail the others.
func (v *GoogleVerifier) fetch(call *fetchCall) {
	keys, ttl, err := v.download()
	v.mu.Lock()
	if err == nil {
		v.keys = keys
		v.expires = v.now().Add(ttl)
	}
	v.lastFail = err != nil
	v.inflight = nil
	call.keys, call.err = keys, err
	v.mu.Unlock()
	close(call.done)
}

var errJWKS = errors.New("bad jwks")

func (v *GoogleVerifier) download() (map[string]*rsa.PublicKey, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := v.hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, errJWKS
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes+1))
	if err != nil {
		return nil, 0, err
	}
	if len(body) > maxJWKSBytes {
		return nil, 0, errJWKS
	}
	keys, err := parseJWKS(body)
	if err != nil {
		return nil, 0, err
	}
	return keys, cacheTTL(resp.Header), nil
}

// parseJWKS reads the RSA signing keys. Keys that are not usable (other key
// type, other alg, too short, bad numbers) are skipped; no usable key at all is
// an error.
func parseJWKS(body []byte) (map[string]*rsa.PublicKey, error) {
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, errJWKS
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if len(keys) >= maxKeys {
			break
		}
		if k.Kid == "" || len(k.Kid) > 128 || k.Kty != "RSA" || (k.Alg != "" && k.Alg != "RS256") || (k.Use != "" && k.Use != "sig") {
			continue
		}
		nb, err1 := base64.RawURLEncoding.DecodeString(k.N)
		eb, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil || len(eb) == 0 || len(eb) > 4 {
			continue
		}
		n := new(big.Int).SetBytes(nb)
		e := int(new(big.Int).SetBytes(eb).Int64())
		if n.BitLen() < minKeyBits || e < 3 || e%2 == 0 {
			continue
		}
		if _, dup := keys[k.Kid]; dup {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: n, E: e}
	}
	if len(keys) == 0 {
		return nil, errJWKS
	}
	return keys, nil
}

// cacheTTL reads Cache-Control max-age (minus Age) clamped to 60 s .. 24 h. No
// usable max-age means the minimum.
func cacheTTL(h http.Header) time.Duration {
	ttl := minCacheTTL
	for _, part := range strings.Split(h.Get("Cache-Control"), ",") {
		part = strings.TrimSpace(part)
		if len(part) > 8 && strings.EqualFold(part[:8], "max-age=") {
			if n, err := strconv.ParseInt(strings.Trim(part[8:], `"`), 10, 64); err == nil && n >= 0 {
				if n > int64(maxCacheTTL/time.Second) {
					n = int64(maxCacheTTL / time.Second)
				}
				ttl = time.Duration(n) * time.Second
			}
			break
		}
	}
	if age, err := strconv.ParseInt(strings.TrimSpace(h.Get("Age")), 10, 64); err == nil && age > 0 {
		if age > int64(maxCacheTTL/time.Second) {
			age = int64(maxCacheTTL / time.Second)
		}
		ttl -= time.Duration(age) * time.Second
	}
	return min(max(ttl, minCacheTTL), maxCacheTTL)
}
