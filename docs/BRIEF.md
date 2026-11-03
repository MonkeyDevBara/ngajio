# BRIEF: Backend Ngajio, versi pertama

## Masalah dan tujuan
UI Ngajio sekarang hanya menyimpan data contoh di `localStorage`. Pengajar tidak bisa memakai di lebih dari satu perangkat, dan data hilang kalau data browser terhapus. Tujuannya: akun pengajar (login Google) dan penyimpanan data di server yang sinkron antar perangkat, tanpa mengorbankan sifat offline-first. Backend harus ringan dan cepat, dan bebas bug.

## Pengguna
Pengajar (ustaz/ustazah) yang mencatat setoran santri dari HP, sering tanpa sinyal. Satu pengajar punya satu akun. Data satu pengajar tidak pernah terlihat oleh pengajar lain.

## Fitur versi pertama
1. **Masuk dengan Google.** Klien mendapat ID token Google, server memverifikasi dan membuat sesi.
2. **Sesi.** Cookie HttpOnly, keluar dan cek sesi aktif.
3. **Sinkronisasi dua arah.** Klien mengirim antrean perubahan dan menarik perubahan baru dari server. Berlaku untuk pengaturan pengajar, metode, kelompok, santri, setoran, dan target. Setoran yang dihapus tetap tersinkron sebagai tombstone (`hapusPada`).
4. **Offline-first tetap utuh.** Semua aksi langsung berhasil di perangkat. Sinkronisasi berjalan di belakang layar dan indikator Tersinkron / Menunggu / Gagal di UI memakai status nyata.
5. **Ekspor data pengajar** (JSON) dari server, supaya pengajar bisa mengambil datanya kapan saja.
6. **Backup server otomatis** harian untuk database.

## Kriteria penerimaan
Fungsi:
- Login Google valid membuat akun baru pada kali pertama dan masuk ke akun yang sama pada kali berikutnya. Token palsu, kedaluwarsa, `aud` salah, `iss` salah, atau email belum terverifikasi ditolak (401).
- Data dibuat di perangkat A muncul di perangkat B setelah sinkronisasi, dan sebaliknya. Perubahan yang dibuat offline tersinkron setelah online, tanpa duplikat meski permintaan dikirim ulang (idempoten).
- Dua perangkat mengubah record yang sama: yang terakhir diterima server menang, dan keduanya akhirnya konvergen ke data yang sama.
- Hapus setoran di satu perangkat menghilang di perangkat lain, laporan lama tetap konsisten.
- Pengajar A tidak bisa membaca atau mengubah data pengajar B lewat cara apa pun (uji dengan dua akun).
- Logout menghapus sesi di server. Cookie lama tidak lagi berlaku.
- Frontend: tombol Masuk dengan Google bekerja, indikator sinkron menampilkan status nyata, antrean offline bertahan setelah muat ulang halaman, mode contoh (`masukContoh`) tetap lokal tanpa sinkron.
- Data lokal lama: bila server kosong dan perangkat berisi data nyata (bukan contoh), data lokal diunggah. Bila server sudah berisi data, data server yang dipakai.

Performa (diukur oleh tester-qa dengan skrip beban, mesin setara server 2 vCPU):
- p95 satu sinkronisasi berisi 100 perubahan di bawah 50 ms di sisi server. p95 tarik perubahan inkremental (tanpa perubahan) di bawah 10 ms.
- Sinkronisasi awal 5.000 setoran selesai dalam hitungan detik, dibagi halaman maksimal 500 perubahan per permintaan.
- RSS memori proses di bawah 50 MB saat 50 pengguna aktif. Binary di bawah 20 MB. Start di bawah 1 detik.
- Respons dikompresi (gzip) dan payload hanya berisi yang berubah.

Kualitas ("tanpa bug"):
- `go vet` bersih, `go test -race ./...` lulus, ada uji untuk validasi input, idempotensi, konvergensi dua perangkat, isolasi antar pengguna, dan kedaluwarsa sesi.
- Fuzz singkat pada validator payload tanpa panic.
- Tidak ada panic yang bocor ke klien. Semua error mengembalikan JSON yang konsisten.
- Frontend: tanpa error di console, tanpa regresi di alur setoran, laporan, dan pengaturan.

Keamanan (diperiksa security-reviewer):
- Token sesi acak 256 bit, hanya hash SHA-256-nya disimpan. Cookie `HttpOnly; Secure; SameSite=Lax; Path=/`.
- Proteksi CSRF: permintaan yang mengubah data wajib `Content-Type: application/json`, header `X-Ngajio: 1`, dan `Origin` sama dengan host.
- Batas ukuran body (2 MB), batas jumlah perubahan per permintaan (500), batas ukuran record (8 KB), kuota record per pengguna, dan pembatasan laju per IP dan per pengguna.
- Tidak ada data pribadi di log. File database dan env hanya bisa dibaca pemilik proses.
- Login contoh dev tidak bisa aktif di produksi.

## Di luar lingkup
Login tautan email, pembayaran dan penagihan, penegakan batas paket gratis 50 santri di server (status paket diubah manual), banyak pengajar dalam satu lembaga (berbagi kelompok), peran admin, notifikasi WhatsApp ke wali, push notification, antarmuka admin, enkripsi end-to-end.

## Batasan
- Server bersama dengan site produksi lain (2 vCPU, 7,4 GB RAM): layanan baru harus kecil dan tidak mengganggu site lain.
- Data pribadi anak dan orang tua (nama, nomor WhatsApp wali): HTTPS wajib, akses ketat per pengguna, backup terenkripsi tidak diwajibkan tetapi izin file ketat.
- Biaya tambahan: nol. Google OAuth Client ID gratis.
- Dibutuhkan dari pemilik: subdomain DNS (A ke 43.173.9.126) dan Google OAuth Client ID (tipe Web application, Authorized JavaScript origins `https://ngajio.transbara.my.id` dan `http://localhost:5173`; tidak butuh redirect URI maupun client secret).
