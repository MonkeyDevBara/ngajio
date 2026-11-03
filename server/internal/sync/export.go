package sync

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"ngajio/server/internal/auth"
	"ngajio/server/internal/httpx"
)

// exportTables lists the arrays of eksporJSON() in the order of the file.
var exportTables = []string{"metode", "kelompok", "santri", "setoran", "target"}

// Export handles GET /api/export: the data of the signed-in user in the shape
// of eksporJSON(), streamed from one read snapshot.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, httpx.CodeUnauthorized, "not signed in")
		return
	}
	ctx := r.Context()
	tx, err := h.Store.Reader().BeginTx(ctx, nil)
	if err != nil {
		httpx.WriteError(w, httpx.CodeInternal, "export failed")
		return
	}
	defer tx.Rollback()

	hd := w.Header()
	hd.Set("Content-Type", "application/json; charset=utf-8")
	hd.Set("Content-Disposition", `attachment; filename="ngajio-data.json"`)
	bw := bufio.NewWriterSize(w, 32<<10)
	if err := writeExport(ctx, tx, bw, u.ID, u.Email, u.Plan, time.Now()); err != nil {
		// Headers are gone; cut the connection so the client sees a broken file.
		panic(http.ErrAbortHandler)
	}
}

func writeExport(ctx context.Context, q queryer, bw *bufio.Writer, uid int64, email, plan string, now time.Time) error {
	ts, _ := json.Marshal(now.UTC().Format("2006-01-02T15:04:05.000Z"))
	bw.WriteString(`{"aplikasi":"Ngajio","versi":1,"diekspor":`)
	bw.Write(ts)

	// pengajar: the stored record plus email and plan from users.
	var js string
	err := q.QueryRowContext(ctx, `SELECT json FROM rec WHERE user_id=? AND tabel='pengajar' AND id='me' AND hapus=0`, uid).Scan(&js)
	if err != nil {
		js = "{}" // not synced yet (also covers ErrNoRows)
	}
	e, _ := json.Marshal(email)
	p, _ := json.Marshal(plan)
	bw.WriteString(`,"pengajar":`)
	bw.WriteString(withFields(`"email":`+string(e)+`,"plan":`+string(p), js))

	for _, t := range exportTables {
		rows, err := q.QueryContext(ctx, `SELECT id,json FROM rec WHERE user_id=? AND tabel=? AND hapus=0 ORDER BY id`, uid, t)
		if err != nil {
			return err
		}
		bw.WriteString(`,"` + t + `":[`)
		first := true
		for rows.Next() {
			var id, js string
			if err := rows.Scan(&id, &js); err != nil {
				rows.Close()
				return err
			}
			if !first {
				bw.WriteByte(',')
			}
			first = false
			idj, _ := json.Marshal(id)
			bw.WriteString(withFields(`"id":`+string(idj), js))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		bw.WriteByte(']')
	}
	bw.WriteString("}\n")
	return bw.Flush()
}

// withFields puts extra (already encoded `"k":v` pairs) in front of the fields of obj.
func withFields(extra, obj string) string {
	if len(obj) <= 2 { // "{}"
		return "{" + extra + "}"
	}
	return "{" + extra + "," + obj[1:]
}
