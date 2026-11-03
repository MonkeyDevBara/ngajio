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
	"syscall"
	"time"

	"ngajio/server/internal/auth"
	"ngajio/server/internal/config"
	"ngajio/server/internal/httpx"
	"ngajio/server/internal/store"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ngajio: refusing to start:", err)
		os.Exit(1)
	}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := run(cfg, logger); err != nil {
		logger.Println("fatal:", err)
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

	logger.Printf("listening on %s (env=%s dev_login=%v)", ln.Addr(), cfg.Env, cfg.DevLogin)
	if err := serve(ctx, newServer(cfg, st, logger), ln, 10*time.Second); err != nil {
		return err
	}
	logger.Println("stopped")
	return nil
}

func newServer(cfg config.Config, st *store.Store, logger *log.Logger) *http.Server {
	return &http.Server{
		Handler:           newHandler(cfg, st, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

// serve runs srv until ctx is done, then drains in-flight requests for up to
// timeout. It returns only after the drain finished, so the caller can close
// the database safely.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	drained := make(chan struct{})
	serveDone := make(chan struct{})
	go func() {
		defer close(drained)
		select {
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			srv.Shutdown(sctx)
		case <-serveDone:
		}
	}()
	err := srv.Serve(ln)
	close(serveDone)
	<-drained
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newHandler(cfg config.Config, st *store.Store, logger *log.Logger) http.Handler {
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
	if cfg.DevLogin {
		a := &auth.Auth{Store: st, CookieSecure: cfg.CookieSecure}
		mux.HandleFunc("POST /api/auth/dev", a.Dev)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, httpx.CodeNotFound, "not found")
	})
	return httpx.Wrap(mux, logger, cfg.AllowedOrigins)
}
