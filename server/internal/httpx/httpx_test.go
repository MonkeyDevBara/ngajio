package httpx

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type errBody struct {
	Error struct{ Code, Message string }
}

func newServer(t *testing.T, allowed ...string) (*httptest.Server, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) { WriteJSON(w, 200, map[string]bool{"ok": true}) })
	mux.HandleFunc("GET /big", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, 200, map[string]string{"x": strings.Repeat("a", 5000)})
	})
	mux.HandleFunc("GET /boundary", func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("a"), gzipMin)) // exactly 1 KB: not compressed
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) { WriteJSON(w, 200, map[string]bool{"ok": true}) })
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) { panic("secret-in-panic-not-for-client") })
	mux.HandleFunc("GET /boom-buffered", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("partial"))
		panic("late")
	})
	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Email string }
		if !ReadJSON(w, r, &in, BodySmall) {
			return
		}
		WriteJSON(w, 200, map[string]string{"email": in.Email})
	})
	srv := httptest.NewServer(Wrap(mux, log.New(&logs, "", 0), allowed))
	t.Cleanup(srv.Close)
	return srv, &logs
}

func do(t *testing.T, req *http.Request) (*http.Response, []byte) {
	t.Helper()
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "identity") // stop the transport adding gzip
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func get(t *testing.T, srv *httptest.Server, path string, hdr ...string) (*http.Response, []byte) {
	req, _ := http.NewRequest("GET", srv.URL+path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	return do(t, req)
}

func post(t *testing.T, srv *httptest.Server, body string, hdr map[string]string) (*http.Response, []byte) {
	req, _ := http.NewRequest("POST", srv.URL+"/echo", strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return do(t, req)
}

func goodHdr(srv *httptest.Server) map[string]string {
	return map[string]string{"Content-Type": "application/json", "X-Ngajio": "1", "Origin": srv.URL}
}

func wantErr(t *testing.T, resp *http.Response, body []byte, status int, code string) {
	t.Helper()
	var e errBody
	if err := json.Unmarshal(body, &e); err != nil || resp.StatusCode != status || e.Error.Code != code || e.Error.Message == "" {
		t.Fatalf("want %d %s, got %d %s", status, code, resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type %q", ct)
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv, _ := newServer(t)
	resp, _ := get(t, srv, "/ok")
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		resp.Header.Get("Referrer-Policy") == "" || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Errorf("headers: %v", resp.Header)
	}
	resp, _ = get(t, srv, "/api/config")
	if resp.Header.Get("Cache-Control") != "max-age=300" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("config headers: %v", resp.Header)
	}
	// errors carry the headers too
	resp, _ = post(t, srv, "{}", nil)
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("error response lacks no-store")
	}
}

func TestPanicBecomesInternalAndServerSurvives(t *testing.T) {
	srv, logs := newServer(t)
	for i := 0; i < 3; i++ {
		resp, body := get(t, srv, "/boom")
		wantErr(t, resp, body, 500, "internal")
		if strings.Contains(string(body), "secret-in-panic") {
			t.Fatal("panic value leaked to client")
		}
	}
	// gzip-enabled client: a panic after a small buffered write still gives a clean JSON error
	resp, body := get(t, srv, "/boom-buffered", "Accept-Encoding", "gzip")
	wantErr(t, resp, body, 500, "internal")
	resp, body = get(t, srv, "/ok")
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("server not serving after panic: %d %s", resp.StatusCode, body)
	}
	if !strings.Contains(logs.String(), "status=500") {
		t.Errorf("500 not logged: %s", logs.String())
	}
}

func TestCSRF(t *testing.T) {
	srv, _ := newServer(t, "https://app.example")
	ok := func(h map[string]string) bool {
		resp, body := post(t, srv, `{"email":"a@b.c"}`, h)
		if resp.StatusCode == 200 {
			return true
		}
		wantErr(t, resp, body, 403, "csrf")
		return false
	}
	if !ok(goodHdr(srv)) {
		t.Fatal("valid request rejected")
	}
	h := goodHdr(srv)
	h["Content-Type"] = "application/json; charset=utf-8"
	if !ok(h) {
		t.Error("charset param must be accepted")
	}
	h = goodHdr(srv)
	h["Origin"] = "https://app.example"
	if !ok(h) {
		t.Error("allowed origin rejected")
	}
	h = goodHdr(srv)
	h["Origin"] = srv.URL + "/" // a bare trailing slash is still an origin
	if !ok(h) {
		t.Error("origin with trailing slash rejected")
	}
	bad := map[string]func(map[string]string){
		"no X-Ngajio":   func(h map[string]string) { delete(h, "X-Ngajio") },
		"X-Ngajio 0":    func(h map[string]string) { h["X-Ngajio"] = "0" },
		"no origin":     func(h map[string]string) { delete(h, "Origin") },
		"wrong origin":  func(h map[string]string) { h["Origin"] = "https://evil.example" },
		"null origin":   func(h map[string]string) { h["Origin"] = "null" },
		"origin prefix": func(h map[string]string) { h["Origin"] = srv.URL + ".evil.example" },
		"userinfo":      func(h map[string]string) { h["Origin"] = "http://evil.com@" + strings.TrimPrefix(srv.URL, "http://") },
		"userinfo only": func(h map[string]string) {
			h["Origin"] = "http://" + strings.TrimPrefix(srv.URL, "http://") + "@evil.com"
		},
		"origin path":     func(h map[string]string) { h["Origin"] = srv.URL + "/x" },
		"origin query":    func(h map[string]string) { h["Origin"] = srv.URL + "?a=1" },
		"origin fragment": func(h map[string]string) { h["Origin"] = srv.URL + "#x" },
		"empty host":      func(h map[string]string) { h["Origin"] = "http://" },
		"ftp scheme":      func(h map[string]string) { h["Origin"] = "ftp://" + strings.TrimPrefix(srv.URL, "http://") },
		"origin garbage":  func(h map[string]string) { h["Origin"] = "::::" },
		"no content-type": func(h map[string]string) { delete(h, "Content-Type") },
		"text/plain":      func(h map[string]string) { h["Content-Type"] = "text/plain" },
		"form":            func(h map[string]string) { h["Content-Type"] = "application/x-www-form-urlencoded" },
	}
	for name, mutate := range bad {
		h := goodHdr(srv)
		mutate(h)
		if ok(h) {
			t.Errorf("%s: accepted", name)
		}
	}
	// other unsafe methods are guarded too
	req, _ := http.NewRequest("DELETE", srv.URL+"/echo", nil)
	resp, body := do(t, req)
	wantErr(t, resp, body, 403, "csrf")
	// GET is not subject to the check
	if resp, _ := get(t, srv, "/ok"); resp.StatusCode != 200 {
		t.Errorf("GET blocked: %d", resp.StatusCode)
	}
}

func TestReadJSON(t *testing.T) {
	srv, _ := newServer(t)
	cases := []struct {
		body   string
		status int
		code   string
	}{
		{`{"email":"a@b.c"}`, 200, ""},
		{`{"email":"a@b.c","extra":1}`, 400, "bad_request"}, // unknown field
		{`{"email":"a@b.c"} {"x":1}`, 400, "bad_request"},   // trailing data
		{`{"email":`, 400, "bad_request"},
		{``, 400, "bad_request"},
		{`[]`, 400, "bad_request"},
		{`{"email":"` + strings.Repeat("a", BodySmall) + `"}`, 413, "too_large"},
	}
	for _, c := range cases {
		resp, body := post(t, srv, c.body, goodHdr(srv))
		if c.status == 200 {
			if resp.StatusCode != 200 {
				t.Errorf("%.30q: %d %s", c.body, resp.StatusCode, body)
			}
			continue
		}
		wantErr(t, resp, body, c.status, c.code)
	}
}

func TestGzip(t *testing.T) {
	srv, _ := newServer(t)
	gz := func(path, ae string) (*http.Response, []byte) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("Accept-Encoding", ae)
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, b
	}
	resp, b := gz("/big", "gzip")
	if resp.Header.Get("Content-Encoding") != "gzip" || resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Fatalf("big not gzipped: %v", resp.Header)
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(zr)
	if !strings.Contains(string(plain), strings.Repeat("a", 5000)) || len(b) > 200 {
		t.Errorf("bad gzip body (%d bytes compressed)", len(b))
	}
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("content-type lost: %q", resp.Header.Get("Content-Type"))
	}
	for _, c := range []struct{ path, ae string }{
		{"/ok", "gzip"},       // small
		{"/boundary", "gzip"}, // exactly 1 KB
		{"/big", "identity"},  // not accepted
		{"/big", "gzip;q=0"},  // refused
	} {
		resp, b := gz(c.path, c.ae)
		if resp.Header.Get("Content-Encoding") != "" {
			t.Errorf("%s with %q must not be compressed", c.path, c.ae)
		}
		if resp.Header.Get("Vary") != "Accept-Encoding" {
			t.Errorf("%s: Vary missing", c.path)
		}
		if c.path == "/boundary" && len(b) != gzipMin {
			t.Errorf("boundary body %d", len(b))
		}
	}
}

// The log must contain method, path, status and duration and nothing from the
// query string, headers, cookies or body.
func TestLogOnlyMethodPathStatusMs(t *testing.T) {
	srv, logs := newServer(t)
	h := goodHdr(srv)
	h["Cookie"] = "ngajio_sid=TOPSECRETTOKEN"
	h["Authorization"] = "Bearer TOPSECRETBEARER"
	post(t, srv, `{"email":"santri-privat@example.com"}`, h)
	get(t, srv, "/ok?email=query-secret@example.com&token=abc")
	out := logs.String()
	for _, s := range []string{"santri-privat", "TOPSECRET", "query-secret", "token=abc", "127.0.0.1", "Mozilla", "Go-http"} {
		if strings.Contains(out, s) {
			t.Errorf("log leaks %q:\n%s", s, out)
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], `method=POST path="/echo" status=200 ms=`) ||
		!strings.HasPrefix(lines[1], `method=GET path="/ok" status=200 ms=`) {
		t.Errorf("unexpected log:\n%s", out)
	}
}

func TestWriteErrorUnknownCode(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, "nonsense", "x")
	wantErr(t, rec.Result(), rec.Body.Bytes(), 500, "internal")
}

func TestErrorCodeStatuses(t *testing.T) {
	want := map[string]int{"bad_request": 400, "unauthorized": 401, "forbidden": 403, "csrf": 403,
		"too_large": 413, "rate_limited": 429, "not_configured": 503, "internal": 500}
	for code, st := range want {
		rec := httptest.NewRecorder()
		WriteError(rec, code, "m")
		wantErr(t, rec.Result(), rec.Body.Bytes(), st, code)
	}
}

// A declared Content-Length over the cap is rejected before the body is read:
// the client sends only the headers and still gets the 413 at once, with the
// connection marked to close.
func TestOversizedContentLengthRejectedWithoutReading(t *testing.T) {
	srv, _ := newServer(t)
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprintf(conn, "POST /echo HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nX-Ngajio: 1\r\nOrigin: %s\r\nContent-Length: 10485760\r\n\r\n",
		strings.TrimPrefix(srv.URL, "http://"), srv.URL)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	wantErr(t, resp, b, 413, "too_large")
	if !resp.Close {
		t.Error("response must close the connection")
	}
}

func TestChunkedBodyStopsAtCap(t *testing.T) {
	srv, _ := newServer(t)
	body := io.MultiReader(strings.NewReader(`{"email":"`), neverEnding{})
	req, _ := http.NewRequest("POST", srv.URL+"/echo", body) // unknown length: chunked
	for k, v := range goodHdr(srv) {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Skipf("server closed before the client finished writing (acceptable): %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	wantErr(t, resp, b, 413, "too_large")
}

type neverEnding struct{}

func (neverEnding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// A panic value may embed user input (strconv, time.Parse errors). The log
// gets the type for ordinary values and the message only for runtime errors.
func TestPanicLogHasNoPersonalData(t *testing.T) {
	var logs bytes.Buffer
	mux := http.NewServeMux()
	mux.HandleFunc("GET /str", func(w http.ResponseWriter, r *http.Request) {
		panic("budi@example.com 0812-3456-7890")
	})
	mux.HandleFunc("GET /num", func(w http.ResponseWriter, r *http.Request) {
		_, err := strconv.Atoi("0812-3456-7890 Budi bin Ahmad budi@example.com")
		panic(err)
	})
	mux.HandleFunc("GET /rt", func(w http.ResponseWriter, r *http.Request) {
		var m map[string]int
		m["x"] = 1
	})
	srv := httptest.NewServer(Wrap(mux, log.New(&logs, "", 0), nil))
	defer srv.Close()
	for _, p := range []string{"/str", "/num", "/rt"} {
		resp, body := get(t, srv, p)
		wantErr(t, resp, body, 500, "internal")
	}
	out := logs.String()
	for _, s := range []string{"budi", "0812", "3456", "Ahmad", "example.com"} {
		if strings.Contains(out, s) {
			t.Errorf("log leaks %q:\n%s", s, out)
		}
	}
	for _, s := range []string{"panic method=GET path=\"/str\": string", "*strconv.NumError", "assignment to entry in nil map", "goroutine "} {
		if !strings.Contains(out, s) {
			t.Errorf("log lacks %q:\n%s", s, out)
		}
	}
}
