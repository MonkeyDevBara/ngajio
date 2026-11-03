// Package validate checks one synced record against the whitelist of its table
// (docs/API.md, "Validasi per tabel"). Unknown keys are rejected and the output
// is re-marshalled from the known fields only, so what is stored is canonical.
// Error messages never contain client data.
package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxRecord is the size limit of one record, input and output.
const MaxRecord = 8 << 10

// ErrTable is returned for a table name that is not synced.
var ErrTable = errors.New("unknown table")

var (
	idRe   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	waRe   = regexp.MustCompile(`^[0-9+ -]*$`)
	hariRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// Table reports whether t is a synced table.
func Table(t string) bool { _, ok := tables[t]; return ok }

// ValidID reports whether s is an acceptable record or mutation id.
func ValidID(s string) bool { return idRe.MatchString(s) }

type check func(json.RawMessage) (any, error)

type field struct {
	req bool
	fn  check
}

type schema map[string]field

func req(fn check) field { return field{true, fn} }
func opt(fn check) field { return field{false, fn} }

// Record validates data for table and returns the canonical JSON to store.
func Record(table, id string, data json.RawMessage) ([]byte, error) {
	s, ok := tables[table]
	if !ok {
		return nil, ErrTable
	}
	if !ValidID(id) || (table == "pengajar" && id != "me") {
		return nil, errors.New("invalid id")
	}
	if len(data) > MaxRecord {
		return nil, errors.New("record too large")
	}
	m, err := s.object(data)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, errors.New("cannot encode record")
	}
	out := bytes.TrimRight(buf.Bytes(), "\n")
	if len(out) > MaxRecord {
		return nil, errors.New("record too large")
	}
	return out, nil
}

func (s schema) object(raw json.RawMessage) (map[string]any, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return nil, errors.New("must be an object")
	}
	var in map[string]json.RawMessage
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, errors.New("invalid object")
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		f, ok := s[k]
		if !ok {
			return nil, errors.New("unknown field")
		}
		val, err := f.fn(bytes.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = val
	}
	for k, f := range s {
		if _, ok := out[k]; f.req && !ok {
			return nil, fmt.Errorf("%s: missing", k)
		}
	}
	return out, nil
}

// ---- field checks ----------------------------------------------------------

func text(max int, multiline bool) check {
	return func(raw json.RawMessage) (any, error) {
		s, err := str(raw)
		if err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(s) > max {
			return nil, errors.New("too long")
		}
		for _, r := range s {
			if r == utf8.RuneError || (unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t'))) {
				return nil, errors.New("invalid character")
			}
		}
		return s, nil
	}
}

func str(raw json.RawMessage) (string, error) {
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", errors.New("must be a string")
	}
	return s, nil
}

func pattern(re *regexp.Regexp, max int) check {
	t := text(max, false)
	return func(raw json.RawMessage) (any, error) {
		v, err := t(raw)
		if err != nil {
			return nil, err
		}
		if !re.MatchString(v.(string)) {
			return nil, errors.New("invalid format")
		}
		return v, nil
	}
}

func enum(vals ...string) check {
	return func(raw json.RawMessage) (any, error) {
		s, err := str(raw)
		if err != nil {
			return nil, err
		}
		for _, v := range vals {
			if s == v {
				return s, nil
			}
		}
		return nil, errors.New("not an allowed value")
	}
}

func id(raw json.RawMessage) (any, error) {
	s, err := str(raw)
	if err != nil || !ValidID(s) {
		return nil, errors.New("must be an id")
	}
	return s, nil
}

// ref is an id that may be empty (references are not enforced, see API.md).
func ref(raw json.RawMessage) (any, error) {
	if s, err := str(raw); err == nil && s == "" {
		return s, nil
	}
	return id(raw)
}

func integer(lo, hi int64) check {
	return func(raw json.RawMessage) (any, error) {
		if len(raw) == 0 || !(raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9')) {
			return nil, errors.New("must be an integer")
		}
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil || n < lo || n > hi {
			return nil, errors.New("must be an integer in range")
		}
		return n, nil
	}
}

func number(lo, hi float64) check {
	return func(raw json.RawMessage) (any, error) {
		if len(raw) == 0 || !(raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9')) {
			return nil, errors.New("must be a number")
		}
		n, err := strconv.ParseFloat(string(raw), 64)
		if err != nil || n < lo || n > hi {
			return nil, errors.New("must be a number in range")
		}
		return n, nil
	}
}

func boolean(raw json.RawMessage) (any, error) {
	switch string(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return nil, errors.New("must be true or false")
}

func rfc3339(raw json.RawMessage) (any, error) {
	s, err := str(raw)
	if err != nil || len(s) > 40 {
		return nil, errors.New("must be an RFC3339 date")
	}
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return nil, errors.New("must be an RFC3339 date")
	}
	return s, nil
}

func list(max int, elem check) check {
	return func(raw json.RawMessage) (any, error) {
		var in []json.RawMessage
		if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &in) != nil {
			return nil, errors.New("must be a list")
		}
		if len(in) > max {
			return nil, errors.New("too many items")
		}
		out := make([]any, len(in))
		for i, v := range in {
			e, err := elem(bytes.TrimSpace(v))
			if err != nil {
				return nil, err
			}
			out[i] = e
		}
		return out, nil
	}
}

func nested(s schema) check {
	return func(raw json.RawMessage) (any, error) {
		m, err := s.object(raw)
		if err != nil {
			return nil, err
		}
		return m, nil
	}
}

// ---- tables ----------------------------------------------------------------

var (
	arah  = enum("juz30", "mushaf")
	surat = integer(1, 114)
	ayat  = integer(1, 286)
	hal   = integer(1, 2000)
)

var tables = map[string]schema{
	"pengajar": {
		"nama": req(text(80, false)), "lembaga": req(text(120, false)),
		"skala":    req(enum("huruf", "bintang", "predikat", "angka")),
		"metodeId": req(ref), "arah": req(arah),
		"ambangLulus": req(integer(1, 5)), // nilai is stored 1..5
		"tema":        req(enum("otomatis", "terang", "gelap")),
		"batasHari":   req(pattern(hariRe, 5)),
	},
	"metode": {
		"nama": req(text(60, false)), "template": req(boolean),
		"level": req(list(30, nested(schema{
			"id": req(id), "nama": req(text(80, false)), "jumlahHalaman": req(integer(1, 2000)),
		}))),
	},
	"kelompok": {
		"nama": req(text(80, false)), "jadwal": req(text(120, false)),
		"metodeId": req(ref), "arsip": req(boolean), "urutan": opt(list(500, id)),
	},
	"santri": {
		"nama": req(text(100, false)), "kelompokId": req(ref), "metodeId": req(ref),
		"arah": req(arah), "waWali": opt(pattern(waRe, 20)), "arsip": req(boolean),
		"hafalAwal": req(list(114, surat)), "targetZiyadahPekan": opt(integer(0, 1000)),
	},
	"setoran": {
		"santriId": req(id), "kelompokSaatItu": req(ref),
		"jenis":     req(enum("ziyadah", "murojaah", "tahsin")),
		"suratAwal": opt(surat), "suratAkhir": opt(surat),
		"ayatAwal": opt(ayat), "ayatAkhir": opt(ayat),
		"levelId":     opt(id),
		"halamanAwal": opt(hal), "halamanAkhir": opt(hal),
		"nilai":       req(integer(1, 5)), // NilaiPicker and nilaiLabel: stored 1..5
		"catatan":     opt(text(1000, true)),
		"tag":         req(list(10, text(40, false))),
		"dicatatPada": req(rfc3339), "diubahPada": opt(rfc3339), "hapusPada": opt(rfc3339),
	},
	"target": {
		"santriId": opt(id), "kelompokId": opt(id),
		"jenis":       req(enum("hafalan", "ziyadah-pekan", "tahsin")),
		"nilaiTarget": req(number(0, 100000)), "satuan": req(text(20, false)),
		"mulai": req(rfc3339), "tenggat": req(rfc3339),
	},
}
