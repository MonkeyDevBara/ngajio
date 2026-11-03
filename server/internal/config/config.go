// Package config reads the server settings from environment variables and
// refuses unsafe combinations (see docs/API.md, section Konfigurasi).
package config

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Env              string
	Listen           string
	DBPath           string
	BackupDir        string
	GoogleClientID   string
	DevLogin         bool
	CookieSecure     bool
	AllowedOrigins   []string
	TrustProxyHeader string
}

// Load builds a Config from getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	get := func(k, def string) string {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
		return def
	}
	c := Config{
		Env:              get("ENV", "prod"),
		Listen:           get("LISTEN", "127.0.0.1:8787"),
		DBPath:           get("DB_PATH", "/var/lib/ngajio/ngajio.db"),
		GoogleClientID:   get("GOOGLE_CLIENT_ID", ""),
		TrustProxyHeader: get("TRUST_PROXY_HEADER", "X-Real-IP"),
		CookieSecure:     true,
	}
	c.BackupDir = get("BACKUP_DIR", filepath.Join(filepath.Dir(c.DBPath), "backup"))
	for _, o := range strings.Split(getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.ToLower(o))
		}
	}

	switch v := get("DEV_LOGIN", ""); v {
	case "", "0", "false":
	case "1", "true":
		c.DevLogin = true
	default:
		return c, fmt.Errorf("DEV_LOGIN=%q is invalid (use 1 or 0)", v)
	}
	switch v := get("COOKIE_SECURE", ""); v {
	case "", "1", "true":
	case "0", "false":
		c.CookieSecure = false
	default:
		return c, fmt.Errorf("COOKIE_SECURE=%q is invalid (use true or false)", v)
	}

	if err := validListen(c.Listen); err != nil {
		return c, err
	}
	if !c.CookieSecure && c.Env != "dev" {
		return c, errors.New("COOKIE_SECURE=false is only allowed when ENV=dev")
	}
	if c.DevLogin {
		if c.Env != "dev" {
			return c, fmt.Errorf("DEV_LOGIN=1 is only allowed when ENV=dev (ENV=%q)", c.Env)
		}
		if !loopbackListen(c.Listen) {
			return c, fmt.Errorf("DEV_LOGIN=1 requires a loopback LISTEN host such as 127.0.0.1 (LISTEN=%q)", c.Listen)
		}
	}
	return c, nil
}

// validListen checks that addr is host:port with a numeric port 1..65535, so a
// typo fails at startup before anything is created.
func validListen(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("LISTEN=%q is not host:port", addr)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("LISTEN=%q has an invalid port (use 1 to 65535)", addr)
	}
	return nil
}

// loopbackListen reports whether the host part of addr is localhost or a
// loopback IP. An empty host, 0.0.0.0 and :: listen on every interface.
func loopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
