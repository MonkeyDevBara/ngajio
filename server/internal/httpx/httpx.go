// Package httpx holds the small HTTP toolkit: JSON helpers, the error shape
// and the middleware chain (log, recover, security headers, gzip, CSRF).
package httpx

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Request body caps. Every handler passes its own cap to ReadJSON.
const (
	BodySmall = 4 << 10 // login and logout
	BodySync  = 2 << 20 // /api/sync
)

// Error codes from docs/API.md. not_found is the only addition (unknown routes).
const (
	CodeBadRequest    = "bad_request"
	CodeUnauthorized  = "unauthorized"
	CodeForbidden     = "forbidden"
	CodeCSRF          = "csrf"
	CodeNotFound      = "not_found"
	CodeTooLarge      = "too_large"
	CodeRateLimited   = "rate_limited"
	CodeNotConfigured = "not_configured"
	CodeInternal      = "internal"
)

var status = map[string]int{
	CodeBadRequest:    http.StatusBadRequest,
	CodeUnauthorized:  http.StatusUnauthorized,
	CodeForbidden:     http.StatusForbidden,
	CodeCSRF:          http.StatusForbidden,
	CodeNotFound:      http.StatusNotFound,
	CodeTooLarge:      http.StatusRequestEntityTooLarge,
	CodeRateLimited:   http.StatusTooManyRequests,
	CodeNotConfigured: http.StatusServiceUnavailable,
	CodeInternal:      http.StatusInternalServerError,
}

// WriteJSON sends v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		WriteError(w, CodeInternal, "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(append(b, '\n'))
}

// WriteError sends {"error":{"code","message"}} with the status of code.
func WriteError(w http.ResponseWriter, code, message string) {
	st, ok := status[code]
	if !ok {
		st, code = http.StatusInternalServerError, CodeInternal
	}
	WriteJSON(w, st, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

// ReadJSON decodes exactly one JSON value from the body into dst, rejecting
// unknown fields, trailing data and bodies over max bytes. A declared
// Content-Length over max is rejected before any byte is read; chunked bodies
// stop at max. The oversized response closes the connection so the server does
// not drain the rest. On failure it writes the error and returns false.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any, max int64) bool {
	if r.ContentLength > max {
		tooLarge(w)
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, max))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil {
		if _, e := dec.Token(); e != io.EOF {
			err = errors.New("unexpected data after JSON body")
			if e != nil {
				err = e
			}
		}
	}
	if err == nil {
		return true
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		tooLarge(w)
	} else {
		WriteError(w, CodeBadRequest, "invalid JSON body")
	}
	return false
}

func tooLarge(w http.ResponseWriter) {
	w.Header().Set("Connection", "close")
	WriteError(w, CodeTooLarge, "request body too large")
}

// Wrap applies the standard middleware chain. Order, outermost first: log,
// recover, security headers, gzip, CSRF.
func Wrap(h http.Handler, logger *log.Logger, allowedOrigins []string) http.Handler {
	return logRequests(logger, recoverPanic(logger, secureHeaders(gzipResponses(csrf(allowedOrigins, h)))))
}

// ---- request log -----------------------------------------------------------

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (s *statusWriter) WriteHeader(c int) {
	if s.code == 0 {
		s.code = c
	}
	s.ResponseWriter.WriteHeader(c)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.code == 0 {
		s.code = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// logRequests logs method, path, status and duration. Nothing else: no query
// string, headers, cookies, bodies or client address.
func logRequests(l *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.code == 0 {
			sw.code = http.StatusOK
		}
		l.Printf("method=%s path=%q status=%d ms=%.1f", r.Method, r.URL.Path, sw.code, float64(time.Since(start).Microseconds())/1000)
	})
}

// ---- recover ---------------------------------------------------------------

func recoverPanic(l *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			l.Printf("panic method=%s path=%q: %s\n%s", r.Method, r.URL.Path, panicText(v), debug.Stack())
			WriteError(w, CodeInternal, "internal error")
		}()
		next.ServeHTTP(w, r)
	})
}

// panicText describes a panic value without risking personal data: runtime
// errors carry no user input, anything else (for example a *strconv.NumError
// that embeds the parsed text) is reduced to its type.
func panicText(v any) string {
	if e, ok := v.(runtime.Error); ok {
		return e.Error()
	}
	return fmt.Sprintf("%T", v)
}

// ---- security headers ------------------------------------------------------

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if r.URL.Path == "/api/config" {
			h.Set("Cache-Control", "max-age=300")
		} else {
			h.Set("Cache-Control", "no-store")
		}
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// ---- gzip ------------------------------------------------------------------

const gzipMin = 1024 // compress only responses larger than 1 KB

var gzPool = sync.Pool{New: func() any { gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed); return gz }}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(name), "gzip") {
			return strings.ReplaceAll(strings.TrimSpace(params), " ", "") != "q=0"
		}
	}
	return false
}

// gzipWriter buffers up to gzipMin bytes. Small responses go out as is; once
// the body exceeds the threshold it switches to gzip.
type gzipWriter struct {
	http.ResponseWriter
	buf  []byte
	code int
	gz   *gzip.Writer
	sent bool // headers sent to the client
}

func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !acceptsGzip(r) || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		g := &gzipWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(g, r)
		// Not deferred on purpose: after a panic the buffered bytes are dropped
		// and the recover middleware writes a clean error response.
		g.finish()
	})
}

func (g *gzipWriter) WriteHeader(c int) {
	if !g.sent {
		g.code = c
	}
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if g.sent {
		if g.gz != nil {
			return g.gz.Write(b)
		}
		return g.ResponseWriter.Write(b)
	}
	if len(g.buf)+len(b) <= gzipMin {
		g.buf = append(g.buf, b...)
		return len(b), nil
	}
	g.start(true)
	return g.gz.Write(b)
}

// start sends the headers; useGzip decides whether the body is compressed.
func (g *gzipWriter) start(useGzip bool) {
	h := g.ResponseWriter.Header()
	bodyless := g.code == http.StatusNoContent || g.code == http.StatusNotModified
	if useGzip && !bodyless && h.Get("Content-Encoding") == "" {
		h.Set("Content-Encoding", "gzip")
		h.Del("Content-Length")
		g.gz = gzPool.Get().(*gzip.Writer)
		g.gz.Reset(g.ResponseWriter)
	}
	g.sent = true
	g.ResponseWriter.WriteHeader(g.code)
	if len(g.buf) > 0 {
		if g.gz != nil {
			g.gz.Write(g.buf)
		} else {
			g.ResponseWriter.Write(g.buf)
		}
		g.buf = nil
	}
}

func (g *gzipWriter) finish() {
	if !g.sent {
		g.start(false)
	}
	if g.gz != nil {
		g.gz.Close()
		gzPool.Put(g.gz)
		g.gz = nil
	}
}

// Flush supports streaming handlers: it commits to gzip and pushes data out.
func (g *gzipWriter) Flush() {
	if !g.sent {
		g.start(true)
	}
	if g.gz != nil {
		g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

// ---- CSRF ------------------------------------------------------------------

// csrf guards every state-changing request (anything but GET, HEAD, OPTIONS):
// Content-Type application/json, X-Ngajio: 1 and an Origin whose host equals
// the request Host, or that is listed in allowed.
func csrf(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" ||
			r.Header.Get("X-Ngajio") != "1" || !originOK(r, allowed) {
			WriteError(w, CodeCSRF, "request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originOK(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "#") {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	norm := strings.ToLower(strings.TrimRight(origin, "/"))
	for _, a := range allowed {
		if a == norm {
			return true
		}
	}
	return false
}
