# Ngajio

Buku mutaba'ah digital untuk pengajar Quran: kelompok, santri, setoran hafalan (ziyadah, murojaah, tahsin), target, dan laporan. UI Bahasa Indonesia. Tersedia sebagai SPA offline-first yang tetap jalan tanpa sinyal.

## Stack
- Frontend: SvelteKit 3 + Svelte 5 (runes) + Tailwind 4, `adapter-static` dengan `fallback: 'index.html'`. Berada di folder ini (`src/`, `static/`).
- Backend (baru): satu binary Go + SQLite (mode WAL), di folder `server/`. Lihat `docs/DECISIONS.md`.
- Jalankan UI lokal: `npm run dev -- --host 127.0.0.1 --port 5173`.
- Jalankan backend lokal: lihat `server/README.md` setelah dibuat.

## Aturan untuk semua agent
- Ringan dan cepat adalah prioritas utama. Tidak ada dependency baru tanpa alasan tertulis di `docs/DECISIONS.md`.
- Backend: dependency Go sesedikit mungkin, tanpa framework web, tanpa ORM.
- Semua teks dari klien dianggap tidak tepercaya: divalidasi di server (tipe, panjang, enum, rentang).
- Rahasia (Client ID dan lainnya) hanya lewat environment, jangan masuk git. `.env*` sudah di `.gitignore`.
- Login contoh dev (`DEV_LOGIN`) tidak boleh aktif di produksi. Server menolak start bila `DEV_LOGIN=1` tanpa `ENV=dev` dan alamat listen loopback.
- Data pribadi (nama santri, nomor WhatsApp wali) tidak boleh masuk log.
- Kunci penyimpanan klien `ngajio:v1` dan `ngajio:intro` jangan diganti tanpa migrasi.

## Deploy (rencana, belum berjalan)
- Domain: https://ngajio.transbara.my.id (frontend statis dan `/api` satu origin lewat nginx).
- Server: Ubuntu 24.04 (x86_64, 2 vCPU) di 43.173.9.126, user `ubuntu`, `ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes ubuntu@43.173.9.126`. Dipakai bersama site produksi lain. Hanya sentuh file milik ngajio: `/var/www/html/ngajio`, `/etc/nginx/sites-available/ngajio.transbara.my.id`, `ngajio.service`, `/opt/ngajio`, `/var/lib/ngajio`. Selalu `nginx -t` sebelum reload.
- Backend dijalankan systemd (`ngajio.service`) di `127.0.0.1:8787`. Binary dibuild di laptop (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`), dikirim dengan rsync.
- Certbot dan DNS dijalankan pemilik.
