# Deploy Ngajio (https://ngajio.transbara.my.id)

Server: 43.173.9.126 (dipakai bersama site lain). Skrip hanya menyentuh path milik ngajio:
`/var/www/html/ngajio`, `/opt/ngajio`, `/var/lib/ngajio`, `/etc/ngajio`, site nginx `ngajio.transbara.my.id`,
`/etc/nginx/snippets/ngajio-headers.conf`, `/etc/systemd/system/ngajio.service`, log `/var/log/nginx/ngajio.*.log`.

## Prasyarat
1. DNS (di Sumopod): record `A`, nama `ngajio`, nilai `43.173.9.126`.
2. Google Cloud Console, OAuth Client ID tipe **Web application**:
   - Authorized JavaScript origins: `https://ngajio.transbara.my.id` (dan `http://localhost:5173` untuk lokal).
   - Tanpa redirect URI. Tanpa client secret (tidak dipakai).
3. Di laptop: `node`/`npm`, `go`, `rsync`, kunci `~/.ssh/id_ed25519`. Cabang kerja sudah di-commit (skrip menolak `--apply` bila `server/` atau `src/` kotor).

## Deploy pertama
Jalankan dari folder `ngajio`:
```
server/deploy/deploy.sh                        # dry run: build, tes, lalu hanya mencetak perintah remote
server/deploy/deploy.sh --apply --first-time   # kirim frontend, binary, nginx, unit, buat /etc/ngajio/env
```
Skrip berhenti sebelum service jalan karena `GOOGLE_CLIENT_ID` masih kosong. Lanjutkan:
```
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes ubuntu@43.173.9.126
sudo nano /etc/ngajio/env          # isi GOOGLE_CLIENT_ID=<Client ID dari Google>
sudo systemctl restart ngajio
exit
```
Atau isi env lalu jalankan ulang `server/deploy/deploy.sh --apply`. Setelah itu TLS (dijalankan pemilik, bukan skrip):
```
sudo certbot --nginx -d ngajio.transbara.my.id --redirect
```
Verifikasi:
```
curl -s https://ngajio.transbara.my.id/api/health         # {"ok":true}
curl -s https://ngajio.transbara.my.id/api/config         # googleClientId terisi, devLogin false
curl -sI https://ngajio.transbara.my.id/ | head -1        # 200
```
Lalu coba login Google dari browser.

## Deploy berikutnya
```
server/deploy/deploy.sh --apply
```
Frontend di-rsync (`--delete` hanya ke `/var/www/html/ngajio/`), binary diganti atomik, service di-restart.
File nginx/unit/env yang sudah ada tidak ditimpa (certbot mengubah file nginx). Ubah manual bila perlu, selalu `sudo nginx -t` sebelum `sudo systemctl reload nginx`.

## Log
```
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes ubuntu@43.173.9.126
sudo journalctl -u ngajio -n 100 --no-pager     # backend (tambah -f untuk mengikuti)
sudo systemctl status ngajio
sudo tail -n 50 /var/log/nginx/ngajio.error.log
```

## Backup dan restore
Backend membuat salinan harian di `/var/lib/ngajio/backup/ngajio-YYYYMMDD.db` (7 terakhir). Folder `/var/lib/ngajio` hanya bisa dibaca root.
Restore:
```
sudo systemctl stop ngajio
sudo ls /var/lib/ngajio/backup
sudo cp /var/lib/ngajio/backup/ngajio-YYYYMMDD.db /var/lib/ngajio/ngajio.db
sudo rm -f /var/lib/ngajio/ngajio.db-wal /var/lib/ngajio/ngajio.db-shm
sudo systemctl start ngajio
```
Bila `cp` mengubah pemilik file sehingga service gagal membuka DB, lihat `journalctl -u ngajio`; `DynamicUser` memakai uid dinamis, jadi setelah restore jalankan `sudo systemctl restart ngajio` (systemd memperbaiki kepemilikan `StateDirectory` saat start).

Salin backup keluar server (dari laptop; file bermode 0600 milik uid dinamis, jadi baca lewat sudo):
```
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes ubuntu@43.173.9.126 'sudo tar -C /var/lib/ngajio -cz backup' > ngajio-backup-$(date +%Y%m%d).tgz
```

## Rollback
Setiap deploy menyimpan binary lama sebagai `/opt/ngajio/ngajio.prev`:
```
sudo cp -p /opt/ngajio/ngajio.prev /opt/ngajio/ngajio && sudo systemctl restart ngajio
```
Frontend: checkout commit lama di laptop, `npm run build`, lalu `deploy.sh --apply`.

## TODO (belum dikerjakan)
- Content-Security-Policy: `index.html` statis SvelteKit punya skrip inline (butuh hash) dan Google Sign-In memuat skrip/iframe dari `accounts.google.com`. Perlu diuji sebelum dipasang. Jangan pasang `Cross-Origin-Opener-Policy` (merusak popup login Google).
- Review keamanan menyeluruh.
- Uji beban (ringan, terukur) sebelum dibuka ke banyak pengguna.
- Penghapusan akun (tombol "Hapus akun" saat ini hanya keluar; ada implikasi privasi data anak).
- Backup di luar server (otomatis, bukan manual).
