package config

import (
	"strings"
	"testing"
)

func load(env map[string]string) (Config, error) {
	return Load(func(k string) string { return env[k] })
}

func TestDefaults(t *testing.T) {
	c, err := load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != "prod" || c.Listen != "127.0.0.1:8787" || !c.CookieSecure || c.DevLogin ||
		c.DBPath != "/var/lib/ngajio/ngajio.db" || c.TrustProxyHeader != "X-Real-IP" ||
		c.BackupDir != "/var/lib/ngajio/backup" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestDevLoginRefusedOutsideDev(t *testing.T) {
	for _, env := range []string{"", "prod", "staging"} {
		_, err := load(map[string]string{"ENV": env, "DEV_LOGIN": "1"})
		if err == nil || !strings.Contains(err.Error(), "ENV=dev") {
			t.Errorf("ENV=%q: want ENV=dev error, got %v", env, err)
		}
	}
}

func TestDevLoginListenHost(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8787": true,
		"127.5.6.7:80":   true,
		"localhost:8787": true,
		"LocalHost:8787": true,
		"[::1]:8787":     true,
		":8787":          false,
		"0.0.0.0:8787":   false,
		"[::]:8787":      false,
		"10.0.0.5:8787":  false,
		"8.8.8.8:8787":   false,
		"example.com:80": false,
		"8787":           false,
	}
	for listen, ok := range cases {
		_, err := load(map[string]string{"ENV": "dev", "DEV_LOGIN": "1", "LISTEN": listen})
		if ok && err != nil {
			t.Errorf("LISTEN=%q should be accepted: %v", listen, err)
		}
		if !ok && err == nil {
			t.Errorf("LISTEN=%q should be refused", listen)
		}
	}
}

func TestDevLoginOK(t *testing.T) {
	c, err := load(map[string]string{"ENV": "dev", "DEV_LOGIN": "1", "LISTEN": "127.0.0.1:9000"})
	if err != nil || !c.DevLogin {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestListenNotCheckedWithoutDevLogin(t *testing.T) {
	if _, err := load(map[string]string{"LISTEN": "0.0.0.0:8787"}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidFlags(t *testing.T) {
	for _, env := range []map[string]string{
		{"DEV_LOGIN": "yes"},
		{"ENV": "dev", "COOKIE_SECURE": "maybe"},
	} {
		if _, err := load(env); err == nil {
			t.Errorf("%v should fail", env)
		}
	}
}

func TestCookieSecureFalseOnlyInDev(t *testing.T) {
	if _, err := load(map[string]string{"COOKIE_SECURE": "false"}); err == nil {
		t.Error("prod with COOKIE_SECURE=false must fail")
	}
	c, err := load(map[string]string{"ENV": "dev", "COOKIE_SECURE": "false"})
	if err != nil || c.CookieSecure {
		t.Fatalf("dev: got %+v, %v", c, err)
	}
}

func TestAllowedOrigins(t *testing.T) {
	c, err := load(map[string]string{"ALLOWED_ORIGINS": " http://localhost:5173/ , https://Ngajio.example ,,"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.AllowedOrigins) != 2 || c.AllowedOrigins[0] != "http://localhost:5173" || c.AllowedOrigins[1] != "https://ngajio.example" {
		t.Fatalf("got %q", c.AllowedOrigins)
	}
}

func TestListenValidatedAtLoad(t *testing.T) {
	bad := []string{"127.0.0.1:99999", "127.0.0.1:0", "127.0.0.1:-1", "127.0.0.1:+80", "127.0.0.1: 80", "127.0.0.1:8 0", "127.0.0.1:\t80", "127.0.0.1:0x50", "127.0.0.1:http", "127.0.0.1", "127.0.0.1:", "nonsense", ":65536"}
	for _, l := range bad {
		if _, err := load(map[string]string{"LISTEN": l}); err == nil {
			t.Errorf("LISTEN=%q should fail at load", l)
		}
	}
	for _, l := range []string{"127.0.0.1:1", "127.0.0.1:65535", ":8787", "[::1]:8787", "0.0.0.0:8787"} {
		if _, err := load(map[string]string{"LISTEN": l}); err != nil {
			t.Errorf("LISTEN=%q should load: %v", l, err)
		}
	}
	if _, err := load(map[string]string{"ENV": "dev", "DEV_LOGIN": "1", "LISTEN": "127.0.0.1:99999"}); err == nil {
		t.Error("dev login with bad port should fail")
	}
}

func TestValidListenDigitsOnly(t *testing.T) {
	for _, l := range []string{"127.0.0.1:+80", "127.0.0.1: 80", "127.0.0.1:80 ", "127.0.0.1:-80", "[::1]:+8787"} {
		if validListen(l) == nil {
			t.Errorf("validListen(%q) should fail", l)
		}
	}
	if validListen("127.0.0.1:080") != nil || validListen("127.0.0.1:80") != nil {
		t.Error("plain digits must pass")
	}
}
