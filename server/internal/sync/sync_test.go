package sync_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ngajio/server/internal/auth"
	"ngajio/server/internal/httpx"
	"ngajio/server/internal/limit"
	"ngajio/server/internal/store"
	syncapi "ngajio/server/internal/sync"
)

type env struct {
	srv *httptest.Server
	st  *store.Store
}

func setup(t *testing.T, quota int, lim *limit.Limiter) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	a := &auth.Auth{Store: st, Limiter: limit.New(1000, 1000)}
	mux := http.NewServeMux()
	a.Register(mux, true)
	(&syncapi.Handler{Store: st, Limiter: lim, Quota: quota}).Register(mux, a.Require)
	srv := httptest.NewServer(httpx.Wrap(mux, log.New(io.Discard, "", 0), nil))
	t.Cleanup(srv.Close)
	return &env{srv, st}
}

type device struct {
	t *testing.T
	e *env
	c *http.Client
}

// device logs in with email; the same email on two devices is one account.
func (e *env) device(t *testing.T, email string) *device {
	jar, _ := cookiejar.New(nil)
	d := &device{t, e, &http.Client{Jar: jar}}
	if code, b := d.do("POST", "/api/auth/dev", fmt.Sprintf(`{"email":%q}`, email)); code != 200 {
		t.Fatalf("login: %d %s", code, b)
	}
	return d
}

func (d *device) do(method, path, body string) (int, []byte) {
	return d.doH(method, path, body, nil)
}

func (d *device) doH(method, path, body string, hdr http.Header) (int, []byte) {
	d.t.Helper()
	r, _ := http.NewRequest(method, d.e.srv.URL+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Ngajio", "1")
	r.Header.Set("Origin", d.e.srv.URL)
	r.Header.Set("Accept-Encoding", "identity")
	resp, err := d.c.Do(r)
	if err != nil {
		d.t.Fatal(err)
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		if hdr != nil {
			hdr[k] = v
		}
	}
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

type chg = map[string]any

type syncResp struct {
	Cursor  int64
	Changes []struct {
		T     string
		ID    string
		Rev   int64
		Data  map[string]any
		Hapus bool
	}
	More     bool
	Rejected []struct{ MID, Code, Message string }
}

func (d *device) sync(cursor int64, changes ...chg) syncResp {
	d.t.Helper()
	if changes == nil {
		changes = []chg{}
	}
	body, _ := json.Marshal(map[string]any{"cursor": cursor, "changes": changes})
	code, b := d.do("POST", "/api/sync", string(body))
	if code != 200 {
		d.t.Fatalf("sync: %d %s", code, b)
	}
	var r syncResp
	if err := json.Unmarshal(b, &r); err != nil {
		d.t.Fatal(err)
	}
	return r
}

func setoran(mid, id string, nilai int) chg {
	return chg{"mid": mid, "t": "setoran", "id": id, "data": chg{"santriId": "s1", "kelompokSaatItu": "k1",
		"jenis": "ziyadah", "nilai": nilai, "tag": []string{}, "dicatatPada": "2026-11-03T10:00:00.000Z"}}
}

func kelompok(mid, id, nama string) chg {
	return chg{"mid": mid, "t": "kelompok", "id": id, "data": chg{"nama": nama, "jadwal": "", "metodeId": "", "arsip": false}}
}

func (d *device) userRev() int64 {
	d.t.Helper()
	_, b := d.do("GET", "/api/me", "")
	var m struct{ Cursor int64 }
	json.Unmarshal(b, &m)
	return m.Cursor
}

func TestReplayIsIdempotent(t *testing.T) {
	d := setup(t, 0, nil).device(t, "a@x.id")
	push := []chg{kelompok("m1", "k1", "Satu"), setoran("m2", "z1", 4)}
	r := d.sync(0, push...)
	if len(r.Changes) != 0 || len(r.Rejected) != 0 || r.Cursor != 2 {
		t.Fatalf("first push: %+v", r)
	}
	r = d.sync(0, push...) // exact replay: no new rev, own... old records are pulled
	if d.userRev() != 2 || len(r.Changes) != 2 || r.Cursor != 2 {
		t.Fatalf("replay: rev=%d %+v", d.userRev(), r)
	}
	var n int
	d.e.st.Reader().QueryRow(`SELECT count(*) FROM rec`).Scan(&n)
	if n != 2 {
		t.Fatalf("records: %d", n)
	}
}

func TestTwoDevicesConverge(t *testing.T) {
	e := setup(t, 0, nil)
	a, b := e.device(t, "a@x.id"), e.device(t, "a@x.id")
	a.sync(0, kelompok("a1", "k1", "dari A"))
	rb := b.sync(0, kelompok("b1", "k1", "dari B")) // B has not pulled yet: last arrival wins
	if len(rb.Changes) != 0 {
		t.Fatalf("own write echoed: %+v", rb.Changes)
	}
	ra := a.sync(1)
	if len(ra.Changes) != 1 || ra.Changes[0].Data["nama"] != "dari B" || ra.Changes[0].Rev != 2 {
		t.Fatalf("A after pull: %+v", ra)
	}
	// an old mid replayed later must not overwrite the newer edit
	a.sync(ra.Cursor, kelompok("a1", "k1", "dari A"))
	if r := b.sync(0); len(r.Changes) != 1 || r.Changes[0].Data["nama"] != "dari B" || r.Cursor != 2 {
		t.Fatalf("old mid overwrote: %+v", r)
	}
}

func TestTombstoneAndRecreate(t *testing.T) {
	e := setup(t, 0, nil)
	a, b := e.device(t, "a@x.id"), e.device(t, "a@x.id")
	a.sync(0, chg{"mid": "m1", "t": "target", "id": "t1", "data": chg{"jenis": "hafalan", "nilaiTarget": 5, "satuan": "juz",
		"mulai": "2026-11-01T00:00:00Z", "tenggat": "2026-12-01T00:00:00Z"}})
	r := a.sync(1, chg{"mid": "m2", "t": "target", "id": "t1", "hapus": true})
	if r.Cursor != 2 {
		t.Fatal(r)
	}
	rb := b.sync(0)
	if len(rb.Changes) != 1 || !rb.Changes[0].Hapus || rb.Changes[0].Data != nil {
		t.Fatalf("tombstone: %+v", rb.Changes)
	}
	a.sync(2, chg{"mid": "m3", "t": "target", "id": "t1", "data": chg{"jenis": "tahsin", "nilaiTarget": 2.5, "satuan": "jilid",
		"mulai": "2026-11-01T00:00:00Z", "tenggat": "2026-12-01T00:00:00Z"}})
	rb = b.sync(rb.Cursor)
	if len(rb.Changes) != 1 || rb.Changes[0].Hapus || rb.Changes[0].Data["jenis"] != "tahsin" {
		t.Fatalf("recreate: %+v", rb.Changes)
	}
	var nrec int
	e.st.Reader().QueryRow(`SELECT nrec FROM users`).Scan(&nrec)
	if nrec != 1 {
		t.Fatalf("nrec %d", nrec)
	}
}

func TestSetoranHapusPadaIsNormalUpdate(t *testing.T) {
	e := setup(t, 0, nil)
	a, b := e.device(t, "a@x.id"), e.device(t, "a@x.id")
	a.sync(0, setoran("m1", "z1", 3))
	z := setoran("m2", "z1", 3)
	z["data"].(chg)["hapusPada"] = "2026-11-03T11:00:00.000Z"
	a.sync(1, z)
	r := b.sync(0)
	if len(r.Changes) != 1 || r.Changes[0].Hapus || r.Changes[0].Data["hapusPada"] != "2026-11-03T11:00:00.000Z" {
		t.Fatalf("%+v", r.Changes)
	}
}

func TestStructuralLimits(t *testing.T) {
	d := setup(t, 0, nil).device(t, "a@x.id")
	many := make([]chg, 501)
	for i := range many {
		many[i] = chg{"mid": fmt.Sprint("m", i), "t": "kelompok", "id": "k"}
	}
	body, _ := json.Marshal(map[string]any{"cursor": 0, "changes": many})
	if code, _ := d.do("POST", "/api/sync", string(body)); code != 400 {
		t.Fatalf("501 changes: %d", code)
	}
	big := `{"cursor":0,"changes":[{"mid":"m","t":"kelompok","id":"k","data":{"nama":"` + strings.Repeat("a", 2<<20) + `"}}]}`
	if code, _ := d.do("POST", "/api/sync", big); code != 413 {
		t.Fatalf("2MB: %d", code)
	}
	if code, _ := d.do("POST", "/api/sync", `{"cursor":0,"changes":[{"t":"kelompok","id":"k","data":{}}]}`); code != 400 {
		t.Fatalf("missing mid: %d", code)
	}
}

func TestRejectedDoNotBlock(t *testing.T) {
	d := setup(t, 0, nil).device(t, "a@x.id")
	bigNote := setoran("big", "z2", 3)
	bigNote["data"].(chg)["catatan"] = strings.Repeat("x", 900) // 8 x 900 > 8 KB via tag below
	bigNote["data"].(chg)["tag"] = []string{}
	huge := kelompok("huge", "k9", strings.Repeat("x", 9000))
	r := d.sync(0,
		kelompok("ok1", "k1", "A"),
		huge,
		chg{"mid": "tbl", "t": "bogus", "id": "x", "data": chg{}},
		setoran("enum", "z1", 9),
		chg{"mid": "unk", "t": "kelompok", "id": "k2", "data": chg{"nama": "x", "jadwal": "", "metodeId": "", "arsip": false, "extra": 1}},
		kelompok("ok2", "k3", "B"),
	)
	codes := map[string]string{}
	for _, x := range r.Rejected {
		codes[x.MID] = x.Code
	}
	want := map[string]string{"huge": "validation", "tbl": "table", "enum": "validation", "unk": "validation"}
	if len(codes) != 4 {
		t.Fatalf("rejected: %+v", r.Rejected)
	}
	for k, v := range want {
		if codes[k] != v {
			t.Fatalf("%s: %q", k, codes[k])
		}
	}
	if r.Cursor != 2 {
		t.Fatalf("cursor %d", r.Cursor)
	}
}

func TestQuota(t *testing.T) {
	d := setup(t, 3, nil).device(t, "a@x.id")
	r := d.sync(0, kelompok("1", "k1", "a"), kelompok("2", "k2", "a"), kelompok("3", "k3", "a"), kelompok("4", "k4", "a"))
	if len(r.Rejected) != 1 || r.Rejected[0].Code != "quota" || r.Rejected[0].MID != "4" || r.Cursor != 3 {
		t.Fatalf("%+v", r)
	}
	// updates of existing records still work at the limit; a delete frees a slot
	r = d.sync(3, kelompok("5", "k1", "b"), chg{"mid": "6", "t": "kelompok", "id": "k2", "hapus": true}, kelompok("7", "k4", "a"))
	if len(r.Rejected) != 0 || r.Cursor != 6 {
		t.Fatalf("%+v", r)
	}
	// the rejected mid was not recorded: it can be retried
	r = d.sync(6, kelompok("4", "k5", "a"))
	if len(r.Rejected) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestUsersAreIsolated(t *testing.T) {
	e := setup(t, 0, nil)
	a, b := e.device(t, "a@x.id"), e.device(t, "b@x.id")
	a.sync(0, kelompok("same", "k1", "milik A"))
	b.sync(0, kelompok("same", "k1", "milik B")) // same id and mid, other account
	if r := a.sync(0); len(r.Changes) != 1 || r.Changes[0].Data["nama"] != "milik A" {
		t.Fatalf("A sees %+v", r.Changes)
	}
	if r := b.sync(0); len(r.Changes) != 1 || r.Changes[0].Data["nama"] != "milik B" {
		t.Fatalf("B sees %+v", r.Changes)
	}
	_, ex := a.do("GET", "/api/export", "")
	if strings.Contains(string(ex), "milik B") || !strings.Contains(string(ex), "milik A") {
		t.Fatalf("export of A: %s", ex)
	}
}

func TestUnauthenticated(t *testing.T) {
	e := setup(t, 0, nil)
	d := &device{t, e, &http.Client{}}
	if code, _ := d.do("POST", "/api/sync", `{"cursor":0,"changes":[]}`); code != 401 {
		t.Fatalf("sync: %d", code)
	}
	if code, _ := d.do("GET", "/api/export", ""); code != 401 {
		t.Fatalf("export: %d", code)
	}
}

func TestPagingAndFastPath(t *testing.T) {
	e := setup(t, 0, nil)
	a, b := e.device(t, "a@x.id"), e.device(t, "a@x.id")
	for p := 0; p < 2; p++ {
		var cs []chg
		for i := 0; i < 500; i++ {
			cs = append(cs, setoran(fmt.Sprint("m", p, "-", i), fmt.Sprint("z", p, "-", i), 3))
		}
		if r := a.sync(int64(p*500), cs...); len(r.Changes) != 0 || r.Cursor != int64(p*500+500) {
			t.Fatalf("push %d: cursor %d changes %d", p, r.Cursor, len(r.Changes))
		}
	}
	r := b.sync(0)
	if len(r.Changes) != 500 || !r.More || r.Cursor != 500 {
		t.Fatalf("page 1: %d %v %d", len(r.Changes), r.More, r.Cursor)
	}
	r = b.sync(r.Cursor)
	if len(r.Changes) != 500 || r.More || r.Cursor != 1000 {
		t.Fatalf("page 2: %d %v %d", len(r.Changes), r.More, r.Cursor)
	}
	r = b.sync(r.Cursor)
	if len(r.Changes) != 0 || r.More || r.Cursor != 1000 {
		t.Fatalf("fast path: %+v", r)
	}
}

func TestExportShape(t *testing.T) {
	d := setup(t, 0, nil).device(t, "a@x.id")
	d.sync(0,
		chg{"mid": "p", "t": "pengajar", "id": "me", "data": chg{"nama": "Ust", "lembaga": "", "skala": "huruf", "metodeId": "",
			"arah": "juz30", "ambangLulus": 3, "tema": "otomatis", "batasHari": "00:00"}},
		kelompok("k", "k1", "A"), setoran("z", "z1", 5),
		chg{"mid": "d", "t": "target", "id": "t1", "hapus": true})
	hdr := http.Header{}
	code, b := d.doH("GET", "/api/export", "", hdr)
	if code != 200 || hdr.Get("Content-Disposition") != `attachment; filename="ngajio-data.json"` {
		t.Fatalf("%d %v", code, hdr)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	for _, k := range []string{"aplikasi", "versi", "diekspor", "pengajar", "metode", "kelompok", "santri", "setoran", "target"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing %s in %s", k, b)
		}
	}
	var peng map[string]any
	json.Unmarshal(m["pengajar"], &peng)
	var kel, tgt []map[string]any
	json.Unmarshal(m["kelompok"], &kel)
	json.Unmarshal(m["target"], &tgt)
	if peng["email"] != "a@x.id" || peng["plan"] != "gratis" || peng["nama"] != "Ust" || len(kel) != 1 || kel[0]["id"] != "k1" || len(tgt) != 0 ||
		!bytes.Contains(m["setoran"], []byte(`"id":"z1"`)) || string(m["santri"]) != "[]" {
		t.Fatalf("%s", b)
	}
}

func TestRateLimit(t *testing.T) {
	d := setup(t, 0, limit.New(0.001, 2)).device(t, "a@x.id")
	d.sync(0)
	d.sync(0)
	hdr := http.Header{}
	code, _ := d.doH("POST", "/api/sync", `{"cursor":0,"changes":[]}`, hdr)
	if code != 429 || hdr.Get("Retry-After") == "" {
		t.Fatalf("%d %v", code, hdr)
	}
}
