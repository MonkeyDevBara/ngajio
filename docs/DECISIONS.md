# DECISIONS: Backend Ngajio

## D1. Backend Go, satu binary, tanpa framework
Alasan: prioritas pemilik adalah ringan, cepat, tanpa bug. Go memberi memori kecil (sekitar 15 hingga 20 MB), binary statis tanpa runtime di server (tidak bergantung versi Node server), dan `net/http` standar sudah cukup. Driver SQLite `modernc.org/sqlite` (Go murni, tanpa CGO) membuat cross-compile dari Mac ke Linux x86_64 sederhana. Dependency yang diizinkan: driver SQLite itu saja, selebihnya pustaka standar.
Ditolak: Node dengan `node:sqlite` (tanpa instal apa pun, tetapi API-nya masih eksperimental di Node 22 dan 24, memori sekitar 50 MB, dan bergantung pada Node di server); Node dengan `better-sqlite3` (modul native, butuh build di server); Postgres atau Docker (berlebihan untuk beban ini dan menambah beban ke server bersama).
Prasyarat: Go belum terpasang di laptop pemilik (`brew install go`), perlu persetujuan.

## D2. SQLite mode WAL, satu file
Alasan: satu penulis per transaksi cukup untuk ratusan pengajar, tanpa proses database terpisah. `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`, `foreign_keys=ON`, satu koneksi tulis dan beberapa koneksi baca. File di `/var/lib/ngajio/ngajio.db`.
Backup: `VACUUM INTO` oleh aplikasi sendiri sekali sehari ke `/var/lib/ngajio/backup/`, simpan 7 salinan terakhir. Tidak butuh CLI `sqlite3` (server tidak memilikinya) maupun cron.

## D3. Sinkronisasi berbasis revisi per pengguna
Tabel `rec(user_id, tabel, id, rev, hapus, json)` dengan kunci utama `(user_id, tabel, id)` dan indeks `(user_id, rev)`. Setiap pengguna punya penghitung `rev` yang naik satu per perubahan yang diterima, di dalam transaksi yang sama. Server menetapkan `rev`, bukan jam klien, sehingga urutan tidak bergantung pada jam perangkat.
Satu titik akhir `POST /api/sync`: klien mengirim `{cursor, changes:[{t, id, data|hapus, mid}]}`, server menerapkan perubahan dalam satu transaksi, lalu mengembalikan `{cursor, changes (rev > cursor), more}`. `mid` (mutation id) membuat pengiriman ulang idempoten. Konflik diselesaikan last-write-wins berdasarkan urutan diterima server. Tombstone disimpan (`hapus=1`), tidak pernah dibuang.
Tabel yang diizinkan: `pengajar`, `metode`, `kelompok`, `santri`, `setoran`, `target` (daftar putih). Isi record disimpan sebagai JSON tervalidasi, bukan kolom per field, supaya perubahan model UI tidak butuh migrasi server. Validasi di server per tabel: tipe, enum (`jenis`, `arah`, dan lainnya), panjang string, rentang angka, dan panjang id.
Ditolak: sinkronisasi per record (terlalu banyak permintaan), CRDT (berlebihan), jam klien sebagai pengurut (rawan jam salah).

## D4. Login Google lewat ID token, sesi cookie buram
Klien memakai Google Identity Services, mengirim ID token ke `POST /api/auth/google`. Server memverifikasi tanda tangan RS256 dengan kunci JWKS Google (disimpan di memori mengikuti `Cache-Control`), memeriksa `iss`, `aud` (sama dengan Client ID), `exp`, dan `email_verified`. Akun diidentifikasi dengan `sub`, bukan email. Server memberi token sesi acak 256 bit, menyimpan hash SHA-256-nya, dan mengirim cookie `HttpOnly; Secure; SameSite=Lax`. Sesi 30 hari, diperpanjang bertahap. Verifikasi JWT ditulis dengan pustaka standar (`crypto/rsa`), tanpa dependency JWT.
Ditolak: JWT sebagai sesi (tidak bisa dicabut), localStorage untuk token (terbaca skrip bila ada XSS), tautan email (butuh layanan kirim email, di luar lingkup versi pertama).
Konsekuensi: skrip Google (`accounts.google.com/gsi/client`) dimuat di halaman masuk, jadi CSP perlu mengizinkan host itu hanya untuk script dan frame.

## D5. Satu origin: nginx menyajikan frontend dan meneruskan `/api`
`https://ngajio.transbara.my.id` menyajikan hasil build statis dan meneruskan `/api/` ke `127.0.0.1:8787`. Tanpa CORS dan cookie sederhana (first-party). nginx mengatur gzip, header keamanan, dan batas `client_max_body_size 2m`.
Ditolak: subdomain API terpisah (butuh CORS dan cookie lintas situs).

## D6. Layanan systemd dengan isolasi
`ngajio.service` memakai `DynamicUser=yes`, `StateDirectory=ngajio`, `ProtectSystem=strict`, `ProtectHome=yes`, `NoNewPrivileges=yes`, `PrivateTmp=yes`, `MemoryMax=150M`, `Restart=on-failure`. Rahasia di `/etc/ngajio/env` (root, mode 600). Alasan: tidak menambah pengguna sistem manual, dan batas memori melindungi site produksi lain.

## D7. Batas dan pembatasan laju
Body maksimal 2 MB, 500 perubahan per permintaan, 8 KB per record, kuota 20.000 record per pengguna, pembatasan laju token-bucket di memori per IP dan per pengguna. Alasan: mencegah penyalahgunaan dan menjaga memori kecil.

## D8. Frontend: sambungan minimal
Tambah `src/lib/sinkron.ts` (mesin sinkron: antrean perubahan di penyimpanan klien, kirim dengan debounce, saat `online`, saat aplikasi dibuka, dan berkala saat tampil). `store.svelte.ts` tetap menjadi sumber data di memori dan cache lokal; `statusSinkron` dan `antrian` diisi oleh mesin sinkron. Halaman `masuk` memakai tombol Google yang nyata, formulir tautan email disembunyikan (di luar lingkup). Mode contoh tetap lokal dan tidak pernah disinkronkan.
Logout membersihkan data lokal supaya pergantian akun tidak mencampur data.

## D9. Struktur kode dan pengujian
`server/` berisi `main.go`, paket `internal/{auth,sync,store,limit,validate}`, dan `*_test.go`. Pengujian: `go vet`, `go test -race`, fuzz validator, uji beban sederhana (skrip Go di `server/loadtest/`). Pelacak tugas: `bd` (Beads) di folder induk.

## D10. Repo
Folder `ngajio` belum punya `.git` sendiri (proyek lain punya). Sebelum pekerjaan, jalankan `git init` di `ngajio` dan buat commit awal dari kode yang ada agar PM bisa bekerja di cabang. `docs/` tidak ikut di-deploy.

## D11. Keputusan PM atas ambiguitas rencana (2026-11-03)
- A1: kunci tak dikenal pada push ditolak (`validation`). Dibuang diam-diam berisiko kehilangan data. Saat UI menambah field, server diperbarui bersamaan.
- A2: pelanggaran per perubahan masuk `rejected`, bukan menggagalkan seluruh permintaan, agar antrian tidak macet.
- A3: rentang `nilai` dibaca dari `NilaiPicker` dan `nilaiLabel`, bukan ditebak.
- A4: `onboarded` di perangkat kedua diturunkan dari `pengajar.nama` server tidak kosong.
- A5: akun Google berbeda di perangkat dengan perubahan belum terkirim diblokir sampai pengguna mengekspor atau membuang.
- A6: `/api/config` juga mengembalikan `devLogin` (true hanya di bawah penjaga dev).
- A7: tombol "Hapus akun" di pengaturan hanya keluar dan tidak menghapus data server. Di versi pertama label diganti "Keluar", tanpa endpoint penghapusan akun. Karena menyangkut data anak, ini jadi pertanyaan privasi untuk pemilik (tindak lanjut), bukan diputuskan diam-diam.
- A8: `COOKIE_SECURE=false` hanya diizinkan bila `ENV=dev`.
- A9: `localStorage` cukup untuk versi pertama (5.000 setoran sekitar 1,5 MB plus shadow). IndexedDB adalah peningkatan nanti, tanpa dependency baru.
- A10: Client ID Google belum ada. Semua dibangun dan diuji tanpanya, login nyata baru bisa diuji setelah pemilik memberi nilai env.
- A11: batas per IP di belakang nginx memercayai `X-Real-IP` hanya dari peer loopback.
- A12: batas laju dan sesi dalam memori, reset saat restart. Diterima.
- A13: batas paket gratis 50 santri tetap di klien (di luar lingkup).
- A14: deteksi data lokal lama: tanpa `mode` dan email sama dengan email mock berarti contoh.
- A15: penanda waktu server hanya `rev`. `tglSinkron` memakai waktu asli.
Kontrak lengkap ada di `docs/API.md`.
