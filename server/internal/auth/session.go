package auth

import (
	"context"
	"net/http"
	"time"

	"ngajio/server/internal/store"
)

const (
	// A session slides when fewer than slideBelow remain, and is then written
	// at most once per slideEvery, so /api/me does not write on every call.
	slideBelow = 25 * 24 * time.Hour
	slideEvery = 24 * time.Hour

	CleanupInterval = time.Hour
)

// tokenLen is the length of a session token: 32 bytes in unpadded base64url.
const tokenLen = 43

// ValidToken reports whether s has the shape of a session token (43 characters
// of the base64url alphabet). The check runs before any database access.
func ValidToken(s string) bool {
	if len(s) != tokenLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func (a *Auth) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// slideTarget returns the new expiry and true when the session should be
// extended at now: under slideBelow left and not written in the last slideEvery.
func slideTarget(ss store.Session, now time.Time) (time.Time, bool) {
	left := time.Unix(ss.ExpiresAt, 0).Sub(now)
	if left >= slideBelow || now.Sub(time.Unix(ss.LastSeen, 0)) < slideEvery {
		return time.Time{}, false
	}
	return now.Add(store.SessionTTL), true
}

// slide extends the session when due and reports whether it wrote. The store
// update repeats the conditions, so concurrent requests write once.
func (a *Auth) slide(ctx context.Context, hash []byte, ss store.Session, now time.Time) (bool, error) {
	exp, due := slideTarget(ss, now)
	if !due {
		return false, nil
	}
	return a.Store.ExtendSession(ctx, hash, exp, now, now.Add(-slideEvery))
}

// Cleanup deletes expired sessions and returns how many.
func (a *Auth) Cleanup(ctx context.Context) (int64, error) {
	return a.Store.DeleteExpiredSessions(ctx, a.now())
}

// RunCleanup deletes expired sessions every interval (CleanupInterval in
// production) until ctx is done. It blocks; run it in a goroutine and wait for
// it before closing the store.
func (a *Auth) RunCleanup(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := a.Cleanup(ctx); err != nil {
				if ctx.Err() == nil && a.Logger != nil {
					a.Logger.Printf("auth: session cleanup failed: %v", err)
				}
			} else if n > 0 && a.Logger != nil {
				a.Logger.Printf("auth: removed %d expired sessions", n)
			}
		}
	}
}

// ---- cookie ----------------------------------------------------------------

func (a *Auth) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(store.SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearCookie expires the cookie with the same attributes (Max-Age=0).
func (a *Auth) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// sessionToken returns the cookie value when it has the shape of a token.
func sessionToken(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || !ValidToken(c.Value) {
		return "", false
	}
	return c.Value, true
}
