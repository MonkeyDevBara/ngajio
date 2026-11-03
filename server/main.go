// Command ngajio is the Ngajio backend: one binary, SQLite in WAL mode.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ngajio/server/internal/auth"
	"ngajio/server/internal/backup"
	"ngajio/server/internal/config"
	"ngajio/server/internal/httpx"
	"ngajio/server/internal/limit"
	"ngajio/server/internal/store"
	syncapi "ngajio/server/internal/sync"
)

// errShutdownTimeout is returned by serve when the drain ran out of time.
var errShutdownTimeout = errors.New("shutdown timed out")

// Auth endpoints allow 10 requests per minute per IP, with a burst of 10.
const (
	authRate  = 10.0 / 60
	authBurst = 10
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ngajio: refusing to start:", err)
		os.Exit(1)
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := run(cfg, logger); err != nil {
		if !errors.Is(err, errShutdownTimeout) { // run already logged that one
			logger.Println("fatal:", err)
		}
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *log.Logger) error {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background jobs stop with bg and are awaited before the store closes.
	bg, stopBG := context.WithCancel(ctx)
	var jobs sync.WaitGroup
	defer func() { stopBG(); jobs.Wait() }()
	lim := limit.New(authRate, authBurst)
	syncLim := limit.New(syncapi.Rate, syncapi.Burst)
	h, a := buildHandler(cfg, st, logger, options{Limiter: lim, SyncLimiter: syncLim})
	bk := &backup.Job{Store: st, Dir: cfg.BackupDir, Logger: logger}
	jobs.Add(4)
	go func() { defer jobs.Done(); syncLim.Run(bg, time.Minute) }()
	go func() { defer jobs.Done(); bk.Run(bg) }()
	go func() { defer jobs.Done(); a.RunCleanup(bg, auth.CleanupInterval) }()
	go func() { defer jobs.Done(); lim.Run(bg, time.Minute) }()

	logger.Printf("listening on %s (env=%s dev_login=%v google=%v)", ln.Addr(), cfg.Env, cfg.DevLogin, cfg.GoogleClientID != "")
	if err := serve(ctx, serverFor(h), ln, 10*time.Second); err != nil {
		if errors.Is(err, errShutdownTimeout) {
			logger.Println("shutdown timed out, forcing close")
		}
		return err
	}
	logger.Println("stopped")
	return nil
}

func newServer(cfg config.Config, st *store.Store, logger *log.Logger) *http.Server {
	return serverFor(newHandler(cfg, st, logger))
}

func serverFor(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

// serve runs srv until ctx is done, then drains in-flight requests for up to
// timeout. It returns only after the drain finished, so the caller can close
// the database safely. When the drain runs out of time it closes the remaining
// connections and returns errShutdownTimeout.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	drained := make(chan struct{})
	serveDone := make(chan struct{})
	var drainErr error // written before drained is closed
	go func() {
		defer close(drained)
		select {
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := srv.Shutdown(sctx); err != nil {
				srv.Close()
				drainErr = errShutdownTimeout
			}
		case <-serveDone:
		}
	}()
	err := srv.Serve(ln)
	close(serveDone)
	<-drained
	if errors.Is(err, http.ErrServerClosed) {
		return drainErr
	}
	return err
}

// options lets tests replace the pieces that talk to the outside world.
type options struct {
	Verifier auth.Verifier  // nil: build the Google verifier when GOOGLE_CLIENT_ID is set
	Limiter  *limit.Limiter // nil: a default limiter without the eviction goroutine
	Now      func() time.Time

	SyncLimiter *limit.Limiter // nil: a default per-user limiter without the eviction goroutine
	SyncQuota   int            // 0: the default record quota (tests lower it)
}

func newHandler(cfg config.Config, st *store.Store, logger *log.Logger) http.Handler {
	h, _ := buildHandler(cfg, st, logger, options{})
	return h
}

// buildHandler returns the full handler and the Auth it uses (for the cleanup job).
func buildHandler(cfg config.Config, st *store.Store, logger *log.Logger, o options) (http.Handler, *auth.Auth) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Ping(r.Context()); err != nil {
			httpx.WriteError(w, httpx.CodeInternal, "database unavailable")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"googleClientId": cfg.GoogleClientID, "devLogin": cfg.DevLogin})
	})
	a := &auth.Auth{Store: st, CookieSecure: cfg.CookieSecure, Verifier: o.Verifier, Limiter: o.Limiter,
		TrustProxyHeader: cfg.TrustProxyHeader, Now: o.Now, Logger: logger}
	if a.Verifier == nil && cfg.GoogleClientID != "" { // keep a nil interface when unset
		a.Verifier = auth.NewGoogleVerifier(cfg.GoogleClientID, auth.GoogleCertsURL, &http.Client{Timeout: 5 * time.Second}, o.Now)
	}
	if a.Limiter == nil {
		a.Limiter = limit.New(authRate, authBurst)
	}
	a.Register(mux, cfg.DevLogin)
	sl := o.SyncLimiter
	if sl == nil {
		sl = limit.New(syncapi.Rate, syncapi.Burst)
	}
	(&syncapi.Handler{Store: st, Limiter: sl, Quota: o.SyncQuota}).Register(mux, a.Require)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, httpx.CodeNotFound, "not found")
	})
	return httpx.Wrap(mux, logger, cfg.AllowedOrigins), a
}
