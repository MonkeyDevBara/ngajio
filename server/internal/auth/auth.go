// Package auth issues sessions. Task 1 only has the dev login; the Google
// login, logout and the session middleware reuse StartSession later.
package auth

import (
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"ngajio/server/internal/httpx"
	"ngajio/server/internal/store"
)

const CookieName = "ngajio_sid"

type Auth struct {
	Store        *store.Store
	CookieSecure bool
}

type loginResponse struct {
	User   store.User `json:"user"`
	Baru   bool       `json:"baru"`
	Cursor int64      `json:"cursor"`
}

// StartSession creates a session for u, sets the cookie and writes the login
// response shared by /api/auth/google and /api/auth/dev.
func (a *Auth) StartSession(w http.ResponseWriter, r *http.Request, u store.User, baru bool) {
	token, hash, err := store.NewToken()
	if err == nil {
		err = a.Store.CreateSession(r.Context(), u.ID, hash, time.Now())
	}
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "could not create session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(store.SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	httpx.WriteJSON(w, http.StatusOK, loginResponse{User: u, Baru: baru, Cursor: 0})
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
	u, baru, err := a.Store.UpsertUser(r.Context(), "dev:"+email, email, "", time.Now())
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "could not load account")
		return
	}
	a.StartSession(w, r, u, baru)
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
