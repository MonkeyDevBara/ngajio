# PM_STATUS: backend Ngajio

Cabang kerja: `pm/ngajio-backend`. Acuan: `CLAUDE.md`, `docs/BRIEF.md`, `docs/DECISIONS.md` (D1 sampai D11), `docs/API.md`. Kartu Beads: `Development-sij`.

| # | Tugas | Worker | Status |
|---|---|---|---|
| 1 | Kerangka server, SQLite, store, config, login dev, health | feature-builder | done. QA ronde 1 NOT READY (6 temuan), ronde 2 READY. RSS puncak 17 MB, drain SIGTERM, log panic tanpa nilai |
| 2 | Auth: ID token Google, sesi, cookie, CSRF, logout, /api/me | feature-builder | todo |
| 3 | Sinkron, validasi, idempotensi, batas, ekspor, backup harian | feature-builder | todo |
| 4 | Uji otomatis, fuzz, uji beban, gerbang performa | feature-builder | todo |
| 5 | Frontend: mesin sinkron, login, SyncChip, logout, aturan login pertama | feature-builder | todo |
| 6 | Berkas persiapan deploy (tidak diterapkan ke server) | devops | todo |
| R | Release check: tester-qa penuh + security-reviewer | tester-qa, security-reviewer | todo |
| G | Gerbang persetujuan pemilik sebelum menyentuh server | PM | todo |

Menunggu dari pemilik untuk tahap deploy dan uji login nyata: subdomain `ngajio.transbara.my.id` (A ke 43.173.9.126) dan Google OAuth Client ID.

## Log
- 2026-11-03: PRD disetujui, Go 1.27.3 dipasang, `git init` di ngajio (commit `0a0d81b`), rencana dari Plan diterima.
- 2026-11-03: T1 selesai dan di-commit. Catatan kecil dari QA untuk T2: log 'shutdown timed out, forcing' bila drain melewati 10 detik, dan parse port memakai pengecekan ketat (menolak `+80`). Belum diuji di Linux (target deploy).
