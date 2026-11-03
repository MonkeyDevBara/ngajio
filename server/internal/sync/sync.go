// Package sync implements POST /api/sync (push and pull in one transaction)
// and GET /api/export. See docs/API.md.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"ngajio/server/internal/auth"
	"ngajio/server/internal/httpx"
	"ngajio/server/internal/limit"
	"ngajio/server/internal/store"
	"ngajio/server/internal/validate"
)

const (
	MaxChanges   = 500    // changes per request, and records per pull page
	DefaultQuota = 20_000 // live (non-tombstone) records per user
)

// Per-user rate limit for /api/sync: about 5 requests per second, burst 20.
const (
	Rate  = 5.0
	Burst = 20
)

type Handler struct {
	Store   *store.Store
	Limiter *limit.Limiter // per user; nil disables the limit
	Quota   int            // 0 means DefaultQuota (tests lower it)
}

// Register adds the routes; require is auth.Require.
func (h *Handler) Register(mux *http.ServeMux, require func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sync", require(http.HandlerFunc(h.Sync)))
	mux.Handle("GET /api/export", require(http.HandlerFunc(h.Export)))
}

type change struct {
	MID   string          `json:"mid"`
	T     string          `json:"t"`
	ID    string          `json:"id"`
	Data  json.RawMessage `json:"data"`
	Hapus bool            `json:"hapus"`
}

type request struct {
	Cursor  int64    `json:"cursor"`
	Changes []change `json:"changes"`
}

type pulled struct {
	T     string          `json:"t"`
	ID    string          `json:"id"`
	Rev   int64           `json:"rev"`
	Data  json.RawMessage `json:"data,omitempty"`
	Hapus bool            `json:"hapus,omitempty"`
}

type rejection struct {
	MID     string `json:"mid"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type response struct {
	Cursor   int64       `json:"cursor"`
	Changes  []pulled    `json:"changes"`
	More     bool        `json:"more"`
	Rejected []rejection `json:"rejected"`
}

// op is a change that passed validation.
type op struct {
	mid, t, id string
	hapus      bool
	json       string
}

func (h *Handler) quota() int64 {
	if h.Quota > 0 {
		return int64(h.Quota)
	}
	return DefaultQuota
}

// Sync handles POST /api/sync. The user always comes from the session.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		httpx.WriteError(w, httpx.CodeUnauthorized, "not signed in")
		return
	}
	if h.Limiter != nil {
		if ok, wait := h.Limiter.Allow(strconv.FormatInt(uid, 10)); !ok {
			w.Header().Set("Retry-After", strconv.FormatInt(max(int64((wait+time.Second-1)/time.Second), 1), 10))
			httpx.WriteError(w, httpx.CodeRateLimited, "too many requests")
			return
		}
	}
	var in request
	if !httpx.ReadJSON(w, r, &in, httpx.BodySync) {
		return
	}
	if in.Cursor < 0 || len(in.Changes) > MaxChanges {
		httpx.WriteError(w, httpx.CodeBadRequest, "invalid cursor or too many changes")
		return
	}
	for _, c := range in.Changes {
		if !validate.ValidID(c.MID) {
			httpx.WriteError(w, httpx.CodeBadRequest, "mid missing or invalid")
			return
		}
	}

	ctx := r.Context()
	resp := response{Changes: []pulled{}, Rejected: []rejection{}}
	ops := prepare(in.Changes, &resp)

	if len(ops) == 0 { // read only: fast path or plain pull
		if len(in.Changes) == 0 {
			rev, err := h.Store.UserRev(ctx, uid)
			if err != nil {
				httpx.WriteError(w, httpx.CodeInternal, "sync failed")
				return
			}
			if rev == in.Cursor {
				resp.Cursor = rev
				httpx.WriteJSON(w, http.StatusOK, resp)
				return
			}
		}
		tx, err := h.Store.Reader().BeginTx(ctx, nil)
		if err == nil {
			defer tx.Rollback()
			err = pull(ctx, tx, uid, in.Cursor, 0, -1, &resp)
		}
		if err != nil {
			httpx.WriteError(w, httpx.CodeInternal, "sync failed")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, resp)
		return
	}

	var quotaRej []rejection
	err := h.Store.Tx(ctx, func(tx *sql.Tx) error {
		quotaRej = quotaRej[:0]
		first, last, err := apply(ctx, tx, uid, ops, h.quota(), &quotaRej)
		if err != nil {
			return err
		}
		resp.Changes = resp.Changes[:0]
		return pull(ctx, tx, uid, in.Cursor, first, last, &resp)
	})
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "sync failed")
		return
	}
	resp.Rejected = append(resp.Rejected, quotaRej...)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// prepare validates every change; problems go to resp.Rejected.
func prepare(changes []change, resp *response) []op {
	var ops []op
	for _, c := range changes {
		reject := func(code, msg string) {
			resp.Rejected = append(resp.Rejected, rejection{MID: c.MID, Code: code, Message: msg})
		}
		if !validate.Table(c.T) {
			reject("table", "unknown table")
			continue
		}
		if c.Hapus {
			if c.T == "pengajar" || !validate.ValidID(c.ID) {
				reject("validation", "cannot delete this record")
				continue
			}
			ops = append(ops, op{mid: c.MID, t: c.T, id: c.ID, hapus: true, json: "{}"})
			continue
		}
		out, err := validate.Record(c.T, c.ID, c.Data)
		if err != nil {
			reject("validation", err.Error())
			continue
		}
		ops = append(ops, op{mid: c.MID, t: c.T, id: c.ID, json: string(out)})
	}
	return ops
}

// apply writes ops in order inside tx and returns the range of revs it used
// (first > last when nothing was written).
func apply(ctx context.Context, tx *sql.Tx, uid int64, ops []op, quota int64, rej *[]rejection) (first, last int64, err error) {
	var rev, nrec int64
	if err = tx.QueryRowContext(ctx, `SELECT rev,nrec FROM users WHERE id=?`, uid).Scan(&rev, &nrec); err != nil {
		return
	}
	seen, err := tx.PrepareContext(ctx, `SELECT 1 FROM mut WHERE user_id=? AND mid=?`)
	if err != nil {
		return
	}
	defer seen.Close()
	cur, err := tx.PrepareContext(ctx, `SELECT hapus FROM rec WHERE user_id=? AND tabel=? AND id=?`)
	if err != nil {
		return
	}
	defer cur.Close()
	up, err := tx.PrepareContext(ctx, `INSERT INTO rec(user_id,tabel,id,rev,hapus,json) VALUES(?,?,?,?,?,?)
		ON CONFLICT(user_id,tabel,id) DO UPDATE SET rev=excluded.rev,hapus=excluded.hapus,json=excluded.json`)
	if err != nil {
		return
	}
	defer up.Close()
	ins, err := tx.PrepareContext(ctx, `INSERT INTO mut(user_id,mid,at) VALUES(?,?,?)`)
	if err != nil {
		return
	}
	defer ins.Close()

	now := time.Now().Unix()
	n := rev
	for _, o := range ops {
		var one int
		switch e := seen.QueryRowContext(ctx, uid, o.mid).Scan(&one); e {
		case nil:
			continue // replay: already applied
		case sql.ErrNoRows:
		default:
			return 0, 0, e
		}
		var oldHapus int
		exists := true
		switch e := cur.QueryRowContext(ctx, uid, o.t, o.id).Scan(&oldHapus); e {
		case nil:
		case sql.ErrNoRows:
			exists = false
		default:
			return 0, 0, e
		}
		wasLive := exists && oldHapus == 0
		delta := int64(0)
		if !o.hapus && !wasLive {
			delta = 1
		} else if o.hapus && wasLive {
			delta = -1
		}
		if delta == 1 && nrec >= quota {
			*rej = append(*rej, rejection{MID: o.mid, Code: "quota", Message: "record quota reached"})
			continue
		}
		n++
		hapus := 0
		if o.hapus {
			hapus = 1
		}
		if _, err = up.ExecContext(ctx, uid, o.t, o.id, n, hapus, o.json); err != nil {
			return
		}
		if _, err = ins.ExecContext(ctx, uid, o.mid, now); err != nil {
			return
		}
		nrec += delta
	}
	if n > rev {
		if _, err = tx.ExecContext(ctx, `UPDATE users SET rev=?,nrec=? WHERE id=?`, n, nrec, uid); err != nil {
			return
		}
	}
	return rev + 1, n, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// pull fills resp with the changes after cursor (at most MaxChanges, ordered by
// rev), skipping revs in [lo,hi] (the caller's own writes), and sets cursor and
// more. The user rev and the rows come from one transaction.
func pull(ctx context.Context, q queryer, uid, cursor, lo, hi int64, resp *response) error {
	var rev int64
	if err := q.QueryRowContext(ctx, `SELECT rev FROM users WHERE id=?`, uid).Scan(&rev); err != nil {
		return err
	}
	rows, err := q.QueryContext(ctx, `SELECT tabel,id,rev,hapus,json FROM rec
		WHERE user_id=? AND rev>? AND NOT (rev BETWEEN ? AND ?) ORDER BY rev LIMIT ?`, uid, cursor, lo, hi, MaxChanges+1)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p pulled
		var hapus int
		var js string
		if err := rows.Scan(&p.T, &p.ID, &p.Rev, &hapus, &js); err != nil {
			return err
		}
		if hapus == 1 {
			p.Hapus = true
		} else {
			p.Data = json.RawMessage(js)
		}
		resp.Changes = append(resp.Changes, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resp.Cursor = rev
	if len(resp.Changes) > MaxChanges {
		resp.Changes = resp.Changes[:MaxChanges]
		resp.More = true
		resp.Cursor = resp.Changes[MaxChanges-1].Rev
	}
	return nil
}
