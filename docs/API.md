# API dan kontrak data Ngajio (acuan semua worker)

Sumber: rencana PM 2026-11-03, turunan dari `docs/BRIEF.md` dan `docs/DECISIONS.md`. Bila ada pertentangan, BRIEF menang, lalu DECISIONS, lalu dokumen ini.

## Aturan umum
- Semua error memakai bentuk `{"error":{"code":"...","message":"..."}}`. Kode: `bad_request` 400, `unauthorized` 401, `forbidden` atau `csrf` 403, `too_large` 413, `rate_limited` 429 (dengan `Retry-After`), `not_configured` 503, `internal` 500. Middleware recover mengubah panic menjadi `internal`.
- Semua respons mengirim `Cache-Control: no-store`, kecuali `/api/config` (`max-age=300`). Gzip untuk respons di atas 1 KB.
- CSRF berlaku untuk setiap POST: `Content-Type: application/json`, header `X-Ngajio: 1`, dan `Origin` ada serta host-nya sama dengan `Host` permintaan (atau ada di env `ALLOWED_ORIGINS`). nginx harus mengirim `proxy_set_header Host $host`.
- Pengguna selalu diambil dari sesi. Permintaan tidak pernah membawa id pengguna.
- Log hanya berisi method, path, status, durasi. Tanpa data pribadi, tanpa token.

## Konfigurasi (environment)
`ENV` (default `prod`), `LISTEN` (default `127.0.0.1:8787`), `DB_PATH` (default `/var/lib/ngajio/ngajio.db`), `BACKUP_DIR`, `GOOGLE_CLIENT_ID`, `DEV_LOGIN`, `COOKIE_SECURE` (default true, boleh false hanya bila `ENV=dev`), `ALLOWED_ORIGINS`, `TRUST_PROXY_HEADER` (default `X-Real-IP`, hanya dipercaya dari peer loopback).
Server menolak start (exit non-nol dengan pesan jelas) bila `DEV_LOGIN=1` dan (`ENV!=dev` atau host listen bukan loopback). Host `""`, `0.0.0.0`, `::`, dan IP non-loopback ditolak. `localhost` dan `127.x` diterima.

## Titik akhir
1. `GET /api/health` mengembalikan `{"ok":true}` dan memeriksa DB.
2. `GET /api/config` mengembalikan `{"googleClientId":"<GOOGLE_CLIENT_ID atau kosong>","devLogin":false}`. `devLogin` bernilai true hanya saat login dev aktif.
3. `POST /api/auth/google`. Permintaan `{"credential":"<id token Google>"}`. Respons 200 `{"user":{"id":1,"email":"a@b.c","nama":"...","plan":"gratis"},"baru":true,"cursor":0}` dan `Set-Cookie: ngajio_sid=<43 karakter base64url>; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000`. 401 untuk tanda tangan palsu, kedaluwarsa, `aud`, `iss`, `email_verified=false`, `alg` atau `kid` salah. 503 `not_configured` bila Client ID kosong. `baru` true bila akun baru dibuat.
4. `POST /api/auth/dev`, rute ada hanya saat login dev aktif. Permintaan `{"email":"x@y.z"}`, sub `dev:<email>`, respons sama dengan nomor 3.
5. `POST /api/auth/logout`. Permintaan `{}`. Respons 200 `{"ok":true}`, baris sesi dihapus, cookie dihapus (`Max-Age=0`). Idempoten: tanpa cookie atau cookie tidak valid tetap 200.
6. `GET /api/me`. Respons 200 `{"user":{"id","email","nama","plan"},"cursor":123}`, `cursor` adalah `rev` pengguna saat ini. 401 tanpa sesi valid. Memperpanjang kedaluwarsa bila sisa kurang dari 25 hari (paling banyak satu tulis per hari).
7. `POST /api/sync`. Permintaan `{"cursor":0,"changes":[{"mid":"uuid","t":"setoran","id":"z1","data":{...}},{"mid":"uuid","t":"target","id":"t1","hapus":true}]}`; `changes` boleh kosong (hanya tarik).
   Respons `{"cursor":812,"changes":[{"t":"setoran","id":"z1","rev":810,"data":{...}},{"t":"target","id":"t1","rev":811,"hapus":true}],"more":false,"rejected":[{"mid":"..","code":"validation|quota|table","message":".."}]}`.
   Kesalahan struktural (JSON rusak, lebih dari 500 perubahan, body lebih dari 2 MB, `mid` hilang) menggagalkan seluruh permintaan dengan 4xx. Pelanggaran per perubahan (field salah, record lebih dari 8 KB, kuota) masuk `rejected` tanpa memblokir perubahan lain maupun penarikan.
   `cursor` adalah `rev` perubahan terakhir yang dikembalikan bila `more=true`, selain itu `rev` pengguna saat ini. Klien mengulang dengan `changes` kosong selama `more`. Penarikan maksimal 500 perubahan berurutan `rev`. Perubahan yang baru ditulis oleh permintaan yang sama tidak di-echo (rev milik sendiri berurutan, kecualikan `rev BETWEEN first AND last`). Jalur cepat: bila `cursor` sama dengan `users.rev` dan tanpa perubahan, satu kali baca tanpa transaksi tulis.
8. `GET /api/export`. `Content-Disposition: attachment; filename="ngajio-data.json"`. Bentuk sama dengan `eksporJSON()`: `{"aplikasi":"Ngajio","versi":1,"diekspor":ISO,"pengajar":{...},"metode":[],"kelompok":[],"santri":[],"setoran":[],"target":[]}`. Setoran tombstone (`hapusPada`) ikut, record `hapus=1` tidak. `pengajar` mendapat email dan plan dari `users`. Di-stream.

## Skema SQLite
Pragma saat buka: `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`, `foreign_keys=ON`. Migrasi lewat `PRAGMA user_version`. Satu koneksi tulis (`SetMaxOpenConns(1)`), pool baca terpisah (4). File DB mode 0600, direktori 0700 (umask 077).
```sql
CREATE TABLE users(
  id INTEGER PRIMARY KEY,
  sub TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL,
  nama TEXT NOT NULL DEFAULT '',
  plan TEXT NOT NULL DEFAULT 'gratis' CHECK(plan IN('gratis','bayar')),
  rev INTEGER NOT NULL DEFAULT 0,
  nrec INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  last_seen INTEGER NOT NULL);
CREATE TABLE sessions(
  token_hash BLOB PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, last_seen INTEGER NOT NULL
) WITHOUT ROWID;
CREATE INDEX sessions_user ON sessions(user_id);
CREATE INDEX sessions_exp ON sessions(expires_at);
CREATE TABLE rec(
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tabel TEXT NOT NULL CHECK(tabel IN('pengajar','metode','kelompok','santri','setoran','target')),
  id TEXT NOT NULL,
  rev INTEGER NOT NULL,
  hapus INTEGER NOT NULL DEFAULT 0 CHECK(hapus IN(0,1)),
  json TEXT NOT NULL,
  PRIMARY KEY(user_id,tabel,id)) WITHOUT ROWID;
CREATE INDEX rec_rev ON rec(user_id,rev);
CREATE TABLE mut(
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mid TEXT NOT NULL, at INTEGER NOT NULL,
  PRIMARY KEY(user_id,mid)) WITHOUT ROWID;
```
- `token_hash` adalah `sha256(token)`. `json` bernilai `'{}'` bila `hapus=1`.
- `mut` membuat pengiriman ulang idempoten dan mencegah `mid` lama menimpa suntingan baru dari perangkat lain. Baris lebih tua dari 90 hari dibersihkan oleh pekerjaan harian.
- `users.nrec` menegakkan kuota 20.000 record tanpa COUNT. Hanya pembuatan baru non-tombstone yang dihitung.
- Transaksi sinkron: `BEGIN IMMEDIATE`, baca `users.rev`, untuk tiap perubahan lewati bila `mid` sudah ada di `mut`, selain itu upsert `rec` dengan `rev=++n` dan masukkan `mid`, lalu `UPDATE users SET rev, nrec`, commit.
- `users.nama` hanya cache tampilan, tidak pernah masuk log.

## Validasi per tabel
Whitelist di server. Kunci tak dikenal ditolak, keluaran di-marshal ulang dari field yang dikenal. Umum: id cocok `^[A-Za-z0-9_-]{1,64}$`, record maksimal 8 KB, string UTF-8 valid tanpa karakter kontrol, batas panjang dihitung dalam rune.

| Tabel | Aturan |
|---|---|
| pengajar | id harus `me`. nama ≤80, lembaga ≤120, skala salah satu huruf, bintang, predikat, angka, metodeId berupa id, arah juz30 atau mushaf, ambangLulus int, tema otomatis, terang, atau gelap, batasHari cocok `^([01]\d\|2[0-3]):[0-5]\d$`. email dan plan tidak diterima. |
| metode | nama ≤60, template bool, level[] maksimal 30 masing-masing `{id, nama ≤80, jumlahHalaman int 1..2000}` |
| kelompok | nama ≤80, jadwal ≤120, metodeId, arsip bool, urutan? maksimal 500 id |
| santri | nama ≤100, kelompokId, metodeId, arah enum, waWali? ≤20 cocok `^[0-9+ -]*$`, arsip bool, hafalAwal[] maksimal 114 int 1..114, targetZiyadahPekan? int 0..1000 |
| setoran | santriId, kelompokSaatItu (boleh kosong), jenis ziyadah, murojaah, atau tahsin, suratAwal dan suratAkhir? 1..114, ayatAwal dan ayatAkhir? 1..286, levelId?, halamanAwal dan halamanAkhir? 1..2000, nilai int (rentang diambil dari `NilaiPicker` dan `nilaiLabel`, jangan menebak), catatan? ≤1000, tag[] maksimal 10 masing-masing ≤40, dicatatPada wajib RFC3339 ≤40 karakter, diubahPada? dan hapusPada? RFC3339 |
| target | santriId?, kelompokId?, jenis hafalan, ziyadah-pekan, atau tahsin, nilaiTarget angka hingga 0..100000, satuan ≤20, mulai dan tenggat RFC3339 |

Integritas referensi (santriId ada, dan seterusnya) tidak ditegakkan di server karena push offline datang tidak berurutan. Klien menoleransi referensi menggantung seperti sekarang.

## Perilaku data
- Hapus setoran adalah pembaruan biasa dengan `data.hapusPada` terisi dan `hapus=0`. Pulihkan (`pulihkanSetoran`) juga pembaruan biasa.
- Hapus target atau metode memakai `hapus:true`, server mengisi `hapus=1` dan `json='{}'`. Membuat ulang id yang sama dengan `data` menghapus tombstone. Tombstone tidak pernah dibuang.
- Last-write-wins menurut urutan terima server (`rev`). Jam klien tidak pernah dipakai. Seluruh record diganti, tidak digabung per field.
- Klien mengirim id di `id` dan membuang `id` dari `data`. Server tidak menyimpan `id` di dalam JSON. Saat menarik, klien menyuntikkan kembali `id`.

## Pemetaan klien ke tabel sinkron
- `pengajar` menjadi satu record dengan id `me`, data adalah `pengajar` tanpa `email` dan `plan`.
- `metode[]`, `kelompok[]`, `santri[]`, `setoran[]`, `target[]` menjadi `t` dengan nama yang sama, id adalah `id` record. Template metode bawaan (`ummi`, `wafa`, dan lainnya) disinkron seperti record biasa.
- Tetap lokal: `ekspor`, `sesi`, `onboarded`, `introSelesai`, `online`.
- `ubah()` di store tidak membawa tabel atau id (20+ mutator), jadi mesin sinkron (`src/lib/sinkron.ts`) membandingkan snapshot store dengan bayangan (shadow) hash per record yang disimpan. Record target dan metode yang hilang dari snapshot dikirim sebagai `hapus:true`.

## Outbox klien
Kunci baru `ngajio:sync:v1`. `ngajio:v1` dan `ngajio:intro` tidak diubah.
```
{ v:1, sub, cursor, shadow:{"setoran/id":"<hash>",...}, outbox:[{mid,t,id,data?|hapus?}], tglSinkron }
```
- Shadow menyimpan hash pendek (FNV-1a 64 atas JSON kanonis), bukan salinan penuh.
- `flush()`: bandingkan snapshot dengan shadow, tambahkan entri outbox yang digabung (satu per record, `mid` baru), simpan, lalu POST per 500 dengan cursor.
- 200: buang `mid` terkirim, terapkan perubahan tarikan, perbarui cursor, simpan, ulangi selama `more`.
- Galat jaringan atau 5xx: outbox dipertahankan, coba ulang dengan jeda 2, 4, 8, 16, 32 detik. Setelah 5 kegagalan `gagal=true`. `cobaLagiSinkron()` mengulang dari awal.
- `mid` dibuat saat entri dibuat dan dipakai ulang di setiap percobaan ulang untuk entri yang sama (inilah yang membuat percobaan ulang idempoten).
- Pemicu: debounce 1,2 detik setelah `ubah()`, event `online`, saat aplikasi dibuka, dan tiap 60 detik saat halaman tampil.
- 401 menghentikan sinkron, menyalakan `sesiBerakhir`, dan TIDAK menghapus data. Banner masuk lagi muncul.
- `app.antrian` adalah panjang outbox ditambah jumlah selisih yang belum masuk. `statusSinkron` diturunkan dari keadaan nyata: `gagal`, `menunggu` (offline, antrian lebih dari 0, atau sedang sinkron), atau `tersinkron`.
- Mode contoh: sinkron tidak pernah berjalan. Data contoh tidak pernah diunggah.
- Aturan terapkan tarikan: lewati record tarikan yang masih punya entri outbox lokal (entri itu dikirim sesudahnya dan menang). Setelah menerapkan perubahan tarikan, set hash shadow ke hash data tarikan agar tidak dikirim balik. `mid` yang ditolak dibuang dari outbox, shadow diset ke hash lokal agar tidak dikirim lagi, dan satu toast per batch. `mid` yang ditolak tidak pernah memblokir antrian.

## Aturan login pertama (`masukAkun`)
1. Bila state lokal sebelumnya `mode:'contoh'`, buang. Data contoh tidak pernah diunggah.
2. Tarik dengan cursor 0.
3. Bila server mengembalikan record apa pun, pakai data server, set `onboarded = pengajar.nama !== ''`, dan buang data lokal.
4. Bila server kosong dan perangkat berisi data nyata (`mode:'akun'`, bukan contoh, minimal satu santri atau setoran, dari akun lokal lama), unggah semuanya per 500.
5. Bila keduanya kosong, `isiKosong(email)` lalu ke `/mulai`.
- Bila `akunSub` berbeda dari `sub` baru dan outbox tidak kosong: blokir dan minta pengguna mengekspor atau membuang dulu. Tidak pernah menghapus diam-diam.
- `Simpanan` mendapat `mode: 'contoh'|'akun'` dan `akunSub` (aditif, `v:1` dan kunci `ngajio:v1` tetap). Tanpa `mode` berarti lama: contoh bila email sama dengan email mock (`fulan@contoh.id`), selain itu `akun`.
- Pengaturan waktu: `sekarang()` memalsukan "hari ini" (OFFSET). Jangan dipakai untuk apa pun yang terlihat server selain field yang sudah ada. `tglSinkron` memakai `Date.now()` asli.
