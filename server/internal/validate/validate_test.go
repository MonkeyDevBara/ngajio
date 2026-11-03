package validate

import (
	"strings"
	"testing"
)

func TestRecord(t *testing.T) {
	const ok = `{"santriId":"s1","kelompokSaatItu":"","jenis":"tahsin","nilai":5,"tag":["a"],"dicatatPada":"2026-11-03T10:00:00+07:00"}`
	const peng = `{"nama":"A","lembaga":"","skala":"huruf","metodeId":"","arah":"juz30","ambangLulus":3,"tema":"terang","batasHari":"00:00","foto":"%s"}`
	foto := func(f string) string { return strings.Replace(peng, "%s", f, 1) }
	cases := []struct {
		table, id, data string
		ok              bool
	}{
		{"setoran", "z1", ok, true},
		{"setoran", "z1", `{"santriId":"s1","kelompokSaatItu":"","jenis":"tahsin","nilai":0,"tag":[],"dicatatPada":"2026-11-03T10:00:00Z"}`, false},
		{"setoran", "z1", `{"santriId":"s1","kelompokSaatItu":"","jenis":"tahsin","nilai":6,"tag":[],"dicatatPada":"2026-11-03T10:00:00Z"}`, false},
		{"setoran", "z1", `{"santriId":"s1","kelompokSaatItu":"","jenis":"tahsin","nilai":"4","tag":[],"dicatatPada":"2026-11-03T10:00:00Z"}`, false},
		{"setoran", "z1", `{"santriId":"s1","kelompokSaatItu":"","jenis":"tahsin","nilai":4,"tag":[],"dicatatPada":"kemarin"}`, false},
		{"setoran", "z 1", ok, false},
		{"setoran", "z1", `{"id":"z1","santriId":"s1"}`, false},
		{"pengajar", "x", `{}`, false},
		{"santri", "s1", `{"nama":"A","kelompokId":"k","metodeId":"","arah":"juz30","arsip":false,"hafalAwal":[1,114],"waWali":"+62 812-3"}`, true},
		{"santri", "s1", `{"nama":"A","kelompokId":"k","metodeId":"","arah":"juz30","arsip":false,"hafalAwal":[115]}`, false},
		{"santri", "s1", `{"nama":"A\u0000","kelompokId":"k","metodeId":"","arah":"juz30","arsip":false,"hafalAwal":[]}`, false},
		{"nope", "x", `{}`, false},
		{"pengajar", "me", foto("data:image/jpeg;base64,/9j/4AAQSkZJRg=="), true},
		{"pengajar", "me", foto("data:image/png;base64,iVBORw0KGgo="), false},
		{"pengajar", "me", foto("data:image/jpeg;base64," + strings.Repeat("A", maxFoto)), false},
		{"pengajar", "me", foto("data:image/jpeg;base64,<script>"), false},
	}
	for i, c := range cases {
		_, err := Record(c.table, c.id, []byte(c.data))
		if (err == nil) != c.ok {
			t.Errorf("case %d (%s): err=%v", i, c.table, err)
		}
	}
	out, _ := Record("setoran", "z1", []byte(ok))
	if string(out) != `{"dicatatPada":"2026-11-03T10:00:00+07:00","jenis":"tahsin","kelompokSaatItu":"","nilai":5,"santriId":"s1","tag":["a"]}` {
		t.Errorf("canonical: %s", out)
	}
}
