# PM_STATUS: backend Ngajio

Cabang kerja: `pm/ngajio-backend`. Acuan: `CLAUDE.md`, `docs/BRIEF.md`, `docs/DECISIONS.md` (D1 sampai D11), `docs/API.md`. Kartu Beads: `Development-sij`.

| # | Tugas | Worker | Status |
|---|---|---|---|
| 1 | Kerangka server, SQLite, store, config, login dev, health | feature-builder | done. QA ronde 1 NOT READY (6 temuan), ronde 2 READY. RSS puncak 17 MB, drain SIGTERM, log panic tanpa nilai |
| 2 | Auth: ID token Google, sesi, cookie, CSRF, logout, /api/me | feature-builder | done (tes unit lulus; QA adversarial dihentikan pemilik) |
| 3 | Sinkron, validasi, idempotensi, batas, ekspor, backup harian | feature-builder | done (tes unit lulus, smoke dua perangkat OK) |
| 4 | Uji otomatis, fuzz, uji beban, gerbang performa | feature-builder | ditunda (mode MVP) |
| 5 | Frontend: mesin sinkron, login, SyncChip, logout, aturan login pertama | feature-builder | done (check 0 error, build OK, e2e dua profil OK) |
| 6 | Berkas persiapan deploy (tidak diterapkan ke server) | devops | todo |
| R | Release check: tester-qa penuh + security-reviewer | tester-qa, security-reviewer | todo |
| G | Gerbang persetujuan pemilik sebelum menyentuh server | PM | todo |

Menunggu dari pemilik untuk tahap deploy dan uji login nyata: subdomain `ngajio.transbara.my.id` (A ke 43.173.9.126) dan Google OAuth Client ID.

## Log
- 2026-11-03: PRD disetujui, Go 1.27.3 dipasang, `git init` di ngajio (commit `0a0d81b`), rencana dari Plan diterima.
- 2026-11-03: T1 selesai dan di-commit. Catatan kecil dari QA untuk T2: log 'shutdown timed out, forcing' bila drain melewati 10 detik, dan parse port memakai pengecekan ketat (menolak `+80`). Belum diuji di Linux (target deploy).

- 2026-11-03: PEMILIK MEMUTUSKAN MODE MVP CEPAT: tidak ada QA mendalam, fuzz, uji beban, atau review keamanan tingkat tinggi sekarang. Fokus: backend berjalan lokal agar bisa dicoba. T3 (sinkron) dan T5 (frontend) dikerjakan paralel dan ringkas. T4 dan T6 ditunda. Penyempurnaan dilakukan bersama sambil berjalan. Risiko diterima pemilik.
- 2026-11-03: MVP jalan lokal: `npm run dev:all` (backend 127.0.0.1:8787 + UI 127.0.0.1:5173, login dev). Belum diuji: login Google nyata, 401/sesi berakhir, offline+backoff di browser, Linux. Kartu tindak lanjut di Beads: review keamanan, uji beban, deploy, penghapusan akun.
- 2026-11-03: cabang pm/ngajio-backend di-merge ke main (1b95faf). Atas perintah pemilik, langkah review keamanan DILEWATI dan lanjut ke deploy. Berkas deploy ditulis di server/deploy/ (deploy.sh dry-run default, --apply, --first-time) dan direview PM. Belum menyentuh server. Menunggu dari pemilik: record DNS A ngajio.transbara.my.id ke 43.173.9.126 dan Google OAuth Client ID. Nginx belum pernah dites (nginx -t pertama di server; gagal membuang symlink dan tidak reload). CSP belum dipasang.
- 2026-11-03 (malam): DEPLOY KE SERVER SELESAI (HTTP). `deploy.sh --apply --first-time` dijalankan atas persetujuan pemilik. Terpasang: /var/www/html/ngajio (37 file frontend), /opt/ngajio/ngajio, /etc/ngajio/env (mode 600, GOOGLE_CLIENT_ID terisi, tanpa DEV_LOGIN), ngajio.service (aktif, ENV=prod, dev_login=false, google=true, RSS sekitar 11 MB), site nginx ngajio.transbara.my.id + snippet header. nginx -t lulus sebelum reload. Diverifikasi lewat IP langsung: health 200, /api/config devLogin=false, /api/auth/dev 404, CSRF 403, /api/me 401, kredensial Google palsu 401, jalur tak dikenal hanya mengembalikan cangkang SPA.
- BELUM: DNS A ngajio.transbara.my.id masih NXDOMAIN di ns1/ns2.sumopod.com (record belum tersimpan di Sumopod). Akibatnya certbot (HTTPS) dan tes login Google nyata belum bisa. Langkah berikutnya setelah DNS aktif: `sudo certbot --nginx -d ngajio.transbara.my.id --redirect`, lalu uji login Google dari browser.
- Temuan di luar Ngajio: money-me.transbara.my.id juga NXDOMAIN di Sumopod (server masih melayaninya, 200 lewat IP langsung). Data IndexedDB lama terikat origin itu.
- Client secret OAuth sempat ditempel di chat oleh pemilik. Tidak dipakai, tidak disimpan di file. Disarankan reset di Google Console.
