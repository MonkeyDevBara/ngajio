// Package auth issues and checks sessions: the Google and dev logins, logout,
// /api/me and the middleware that puts the user into the request context.
package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ngajio/server/internal/httpx"
	"ngajio/server/internal/limit"
	"ngajio/server/internal/store"
)

const CookieName = "ngajio_sid"

// bodyGoogle is the body cap for /api/auth/google: an ID token can be ~2 KB, so
// the 4 KB httpx.BodySmall is too tight. Tokens over 8 KB are refused anyway.
const bodyGoogle = 16 << 10

const maxNamaRunes = 80

type Auth struct {
	Store        *store.Store
	CookieSecure bool

	// Verifier checks Google ID tokens; nil means GOOGLE_CLIENT_ID is not set
	// and /api/auth/google answers 503 not_configured.
	Verifier Verifier
	// Limiter, when set, rate limits the auth endpoints per client IP.
	Limiter *limit.Limiter
	// TrustProxyHeader names the header with the client IP (trusted only from
	// a loopback peer, see httpx.ClientIP).
	TrustProxyHeader string
	// Now is the clock (nil means time.Now); tests inject their own.
	Now func() time.Time
	// Logger receives short operational lines. Never personal data.
	Logger *log.Logger
}

type loginResponse struct {
	User   store.User `json:"user"`
	Baru   bool       `json:"baru"`
	Cursor int64      `json:"cursor"`
}

// Register adds the auth routes to mux. The dev login is added only when
// devLogin is true (main.go passes the guarded config value).
func (a *Auth) Register(mux *http.ServeMux, devLogin bool) {
	mux.HandleFunc("POST /api/auth/google", a.RateLimit(a.Google))
	mux.HandleFunc("POST /api/auth/logout", a.RateLimit(a.Logout))
	mux.Handle("GET /api/me", a.Require(http.HandlerFunc(a.Me)))
	if devLogin {
		mux.HandleFunc("POST /api/auth/dev", a.RateLimit(a.Dev))
	}
}

// RateLimit applies the per-IP bucket to h: 429 rate_limited with a
// Retry-After in whole seconds (at least 1).
func (a *Auth) RateLimit(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Limiter != nil {
			if ok, wait := a.Limiter.Allow(httpx.ClientIP(r, a.TrustProxyHeader)); !ok {
				secs := int64((wait + time.Second - 1) / time.Second)
				w.Header().Set("Retry-After", strconv.FormatInt(max(secs, 1), 10))
				httpx.WriteError(w, httpx.CodeRateLimited, "too many requests")
				return
			}
		}
		h(w, r)
	}
}

// StartSession creates a session for u, sets the cookie and writes the login
// response shared by /api/auth/google and /api/auth/dev.
func (a *Auth) StartSession(w http.ResponseWriter, r *http.Request, u store.User, baru bool) {
	token, hash, err := store.NewToken()
	if err == nil {
		err = a.Store.CreateSession(r.Context(), u.ID, hash, a.now())
	}
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "could not create session")
		return
	}
	a.setCookie(w, token)
	httpx.WriteJSON(w, http.StatusOK, loginResponse{User: u, Baru: baru, Cursor: 0})
}

// Google handles POST /api/auth/google. Every verification failure is the same
// generic 401; the reason is logged as a short code only.
func (a *Auth) Google(w http.ResponseWriter, r *http.Request) {
	if a.Verifier == nil {
		httpx.WriteError(w, httpx.CodeNotConfigured, "google login is not configured")
		return
	}
	var in struct {
		Credential string `json:"credential"`
	}
	if !httpx.ReadJSON(w, r, &in, bodyGoogle) {
		return
	}
	cl, err := a.Verifier.Verify(r.Context(), in.Credential)
	if err != nil {
		if a.Logger != nil {
			a.Logger.Printf("auth: google login rejected code=%s", verifyCode(err))
		}
		httpx.WriteError(w, httpx.CodeUnauthorized, "login failed")
		return
	}
	// The account is keyed by sub, never by email.
	u, baru, err := a.Store.UpsertLoginUser(r.Context(), cl.Sub, cl.Email, cleanNama(cl.Name), a.now())
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "could not load account")
		return
	}
	a.StartSession(w, r, u, baru)
}

func verifyCode(err error) string {
	var ve *VerifyError
	if errors.As(err, &ve) {
		return ve.Code
	}
	return "other"
}

// cleanNama drops control characters and invalid UTF-8, trims, and cuts the
// display name to maxNamaRunes runes.
func cleanNama(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxNamaRunes {
		s = string([]rune(s)[:maxNamaRunes])
	}
	return s
}

// Dev handles POST /api/auth/dev. It is only registered when DEV_LOGIN is on.
func (a *Auth) Dev(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !httpx.ReadJSON(w, r, &in, httpx.BodySmall) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(email) {
		httpx.WriteError(w, httpx.CodeBadRequest, "invalid email")
		return
	}
	u, baru, err := a.Store.UpsertUser(r.Context(), "dev:"+email, email, "", a.now())
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "could not load account")
		return
	}
	a.StartSession(w, r, u, baru)
}

// Logout handles POST /api/auth/logout. It deletes the session of the cookie
// and clears the cookie. It is idempotent: a missing, malformed or unknown
// cookie still answers 200 {"ok":true}. The body is not read (the client sends
// {}); the size cap only bounds what the server drains.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, httpx.BodySmall)
	if token, ok := sessionToken(r); ok {
		if err := a.Store.DeleteSession(r.Context(), store.HashToken(token)); err != nil {
			httpx.WriteError(w, httpx.CodeInternal, "could not end session")
			return
		}
	}
	a.clearCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Me handles GET /api/me behind Require.
func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, httpx.CodeUnauthorized, "not signed in")
		return
	}
	rev, err := a.Store.UserRev(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "could not load account")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": u, "cursor": rev})
}

// ---- middleware ------------------------------------------------------------

type ctxKey struct{}

// UserFrom returns the signed-in user stored by Require.
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(store.User)
	return u, ok
}

// UserID returns the signed-in user id stored by Require. Handlers must take
// the user from here and never from the request.
func UserID(ctx context.Context) (int64, bool) {
	u, ok := UserFrom(ctx)
	return u.ID, ok
}

// Require wraps next so it only runs with a valid session. It reads the
// ngajio_sid cookie, checks its shape before touching the database, looks the
// session up by SHA-256 hash, answers 401 for a missing, malformed, unknown or
// expired session, slides the expiry when due (re-sending the cookie so the
// browser's copy lives on), and puts the user into the request context.
func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := sessionToken(r)
		if !ok {
			httpx.WriteError(w, httpx.CodeUnauthorized, "not signed in")
			return
		}
		hash := store.HashToken(token)
		now := a.now()
		u, ss, err := a.Store.LookupSession(r.Context(), hash, now)
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteError(w, httpx.CodeUnauthorized, "not signed in")
			return
		}
		if err != nil {
			httpx.WriteError(w, httpx.CodeInternal, "could not check session")
			return
		}
		if wrote, err := a.slide(r.Context(), hash, ss, now); err != nil {
			if a.Logger != nil {
				a.Logger.Printf("auth: session extend failed: %v", err)
			}
		} else if wrote {
			a.setCookie(w, token)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

// validEmail accepts a bare address (no display name, no spaces or control
// characters) of at most 254 bytes whose domain contains a dot.
func validEmail(s string) bool {
	if s == "" || len(s) > 254 {
		return false
	}
	for _, c := range s {
		if c > unicode.MaxASCII || c <= ' ' || c == 0x7f {
			return false
		}
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return false
	}
	at := strings.LastIndexByte(s, '@')
	dom := s[at+1:]
	return at > 0 && at <= 64 && strings.Contains(dom, ".") && !strings.HasPrefix(dom, ".") && !strings.HasSuffix(dom, ".")
}
