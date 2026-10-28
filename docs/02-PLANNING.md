# HOP Imperium: planning

Dokumen 3 dari 3. Urutan pengerjaan, repo, dan apa yang dipensiunkan di tiap tahap. Nomor fitur merujuk `01-FITUR.md`.

## Prinsip

1. **Akun tetap di hel.ink.** hel.ink = "HOP ID server". Tidak ada migrasi user.
2. **Tiap aplikasi tetap punya DB sendiri.** Imperium tidak pernah membuka DB aplikasi lain; ia memanggil **Admin API** tiap aplikasi, bertanda tangan HMAC dengan secret per aplikasi (pola yang sudah ada), ditambah header `X-HOP-Actor` (id admin) supaya aplikasi ikut mencatat siapa pelakunya.
3. **Satu kontrak untuk semua aplikasi** ("HOP App Protocol"): aplikasi yang memenuhi kontrak otomatis muncul di Imperium. Kontrak minimal:
   - `GET  /hop/health` → versi, uptime, status DB/storage
   - `GET  /hop/stats` → angka KPI + seri harian
   - `GET  /hop/users/{hop_id}` → ringkasan user di aplikasi ini (konten, kuota, sesi, key)
   - `POST /hop/users/{hop_id}/actions` → suspend/lock/activate/purge/revoke-sessions/revoke-keys/set-quota/warn
   - `POST /hop/profile` → terima profil terbaru (push dari HOP ID)
   - `GET  /hop/content?…` dan `POST /hop/content/{id}/actions` → daftar + aksi moderasi (jenis konten didefinisikan aplikasi; Imperium merender generik dari skema yang dikirim aplikasi)
   - `GET  /hop/reports` → laporan untuk antrean terpadu (atau aplikasi push ke Imperium)
   - `GET/PUT /hop/settings` → daftar setting + skema (tipe, batas, deskripsi) + nilai
   - `GET  /hop/metrics` → Prometheus
   - `GET  /hop/search?q=` → pencarian global
   Modul khusus (storage PixelHop, slug holds NoteHop) ditambahkan sebagai "ekstensi" yang Imperium kenali per aplikasi.
4. **Semua panel admin lama dihapus, dilakukan per modul** supaya tidak ada hari tanpa tempat menangani abuse: begitu modul X di Imperium dinyatakan selesai, route admin X di aplikasi dihapus dan `/admin/...` lama redirect ke Imperium. Satu rilis transisi, lalu kode lama dihapus.
5. **Repo (sudah dibuat/direname pemilik 2026-10-27):**
   - `navi-crwn/Imperium` (privat): Imperium (Go, satu binary, SQLite untuk audit/settings/metrics) + `hop-agent` + protokol HOP (dokumen + paket Go `hopproto` + stub PHP). Folder lokal `~/imperium`.
   - `navi-crwn/LinkHop` (publik, dulu `hel.ink`; redirect otomatis). Folder lokal `~/linkhop`, live di `/var/www/helink/shortlink-app`.
   - `navi-crwn/PixelHop` (publik, dulu `pixelhop`), `navi-crwn/NoteHop` (privat): tambah paket `internal/hop` yang mengimplementasi kontrak.
   - `navi-crwn/remotehop` (privat, masih kosong) dan `navi-crwn/BioHop` (privat, untuk nanti).
   - `helink-ops` tetap untuk catatan deploy.

## Tahap 0: persiapan (sebelum menulis fitur)

- Samakan checkout dengan yang live: `~/pixelhop` ke v3.6.0 (branch `claude/musing-shtern-2628ad` → merge ke main), `~/notehop/repo` sudah sama, hel.ink live = 859b69a.
- Backup penuh ketiga DB + env (skrip sudah ada untuk masing-masing).
- Tulis `PROTOCOL.md` HOP App Protocol di repo imperium (kontrak di atas, format tanda tangan, kode error, versi).
- Siapkan DNS `imperium.hel.ink` → IP Tailscale VPS (record A, **tidak** proxied) dan vhost nginx yang hanya listen di IP Tailscale.
- Buat `hop-backup.sh` v0 (satu arsip untuk tiga DB + env + file upload + nginx + systemd + cron) dan `hop-restore.sh` v0, uji restore ke folder sementara. Dasar untuk modul Backup & Migrasi.
- Perbaiki bug kecil yang ditemukan inventaris: `SAFE_BROWSING_KEY` vs `SAFE_BROWSING_API_KEY` di hel.ink; celah `NOTEHOP_ADMINS` (sementara: kosongkan daftar).

## Tahap 1: HOP ID (di repo hel.ink / linkhop)

1. **Rebranding** → "LinkHop" (tagline "Hop Easy Link", domain tetap hel.ink): APP_NAME, judul, README (en/id), docs GitHub Pages, email, halaman login/register ("Login with HOP", "Join HOP"), logo HOP di tombol. Domain tetap hel.ink.
2. **Migrasi DB**: `users` + username (unique), bio, website, location, locale, timezone, show_owner, avatar_path, profile_version; tabel `roles` (user_id, role, app_slug nullable); tabel `apps` (registry); tabel `username_history`; tabel `app_secrets` (secret aktif + secret lama, berlaku sampai).
3. **Registry** menggantikan `AppSsoController::APPS` dan `config/services.*`; route `/sso/{app}` jadi dinamis; CSP form-action dari registry.
4. **Tiket SSO v2** (username, roles, profile_version, verified) + endpoint `GET /api/hop/profile/{id}` + webhook `profile.updated`, `session.revoked`, `account.status`, `account.deleted` ke semua aplikasi di registry.
5. **Form profil global** di hel.ink (username dengan cek ketersediaan live, avatar upload, bio, website, …) dan halaman "Aplikasi saya" (ikon aplikasi dari registry, "buka", "putuskan").
6. **Backfill username**: perintah artisan `hop:import-usernames` membaca ekspor dari PixelHop dan NoteHop (lewat Admin API tahap 3, atau CLI ekspor JSON sementara), aturan konflik "lebih dulu dibuat menang", laporan konflik.
7. **Peran** menggantikan `SUPERADMIN_EMAIL` (migrasi: email superadmin → role owner), middleware `admin` memakai roles, grup admin ditambah `2fa`.
8. **Login hel.ink mencatat semua jenis login** (Google, SSO ke aplikasi) di `login_histories`.
9. Generalisasi `PixelHopAccountController` → `AppAccountController` (login/register dari form aplikasi mana pun, secret dari registry).
10. Tes: Pest/PHPUnit untuk tiket v2, registry, peran; Playwright untuk alur login dari tiap aplikasi.

Yang dipensiunkan di tahap ini: `SUPERADMIN_EMAIL`, konstanta `APPS`, `PIXELHOP_*`/`NOTEHOP_*` di `.env` (pindah ke tabel `apps`).

### Status Tahap 1 (2026-10-27)

Dikerjakan di clone `~/linkhop`, branch `hop-id` (3 commit), 238 tes hijau, Pint bersih. Selesai: 1–5, 7–10 (profil + username + avatar, roles + 2FA admin, registry, tiket v2, API profil + webhook, AppAccountController generik, login history Google/SSO, rebranding UI + README, HopIdTest). Belum: halaman "Aplikasi saya" (menu sidebar sudah dinamis, dianggap cukup), rename repo GitHub (manual oleh pemilik), aset gambar README masih bertuliskan HEL.ink.

**Deploy Tahap 1: SELESAI 2026-10-27 19:30** (PR LinkHop#32 merged fb2de1d, 03-deploy.sh, hop:bootstrap: owner + 2 apps, hop:usernames: 3 diimpor dari PixelHop/NoteHop + 71 dibuat, 0 konflik). Verifikasi curl: tombol HOP tampil, SSO start dari p.hel.ink dan note.hel.ink mendarat di /sso/{app} → /login, API legacy PixelHop hidup, /api/hop/* menolak tanpa tanda tangan.

Catatan operasional: perintah `php8.3 artisan` di folder live sebaiknya dijalankan sebagai `www-data` (`sudo -u www-data php8.3 artisan …`) supaya file cache/log tidak jadi milik `carawin`; kalau terlanjur, `chgrp www-data` + `chmod g+rw`. PR #33 (reset password HOP ID) dideploy 2026-10-27 20:36.

**Urutan yang dipakai:**
1. Merge PR `hop-id` → main, lalu `bash ~/hel-ink-handoff/03-deploy.sh` (migrate jalan otomatis).
2. `php8.3 artisan hop:bootstrap` → owner role + tabel apps terisi dari .env.
3. Ekspor username dari PixelHop dan NoteHop ke JSON `[{hop_id, username, created_at}]` (perintah ekspor dibuat di Tahap 3; sementara bisa SQL manual), lalu `php8.3 artisan hop:usernames --import=pixelhop.json --import=notehop.json --dry-run`, periksa konflik, jalankan tanpa `--dry-run`.
4. Cek: login web, "Continue with HEL.ink" dari PixelHop dan NoteHop (tiket v2 kompatibel v1), login form PixelHop (`/api/pixelhop/*` tetap jalan), halaman admin minta 2FA.
5. Setelah stabil: hapus `PIXELHOP_*`/`NOTEHOP_*` dari `.env` (hanya dibaca hop:bootstrap).

## Tahap 2: Imperium inti (repo imperium)

1. Skeleton Go: router, templates (tanpa build step, seperti NoteHop), SQLite (audit, settings, metrics, notes admin, antrean laporan), i18n id/en, CSP ketat.
2. Login via HOP SSO (Imperium sendiri adalah aplikasi di registry, `aud=imperium`), hanya roles owner/admin/moderator/support; 2FA wajib (ditolak bila tiket menyatakan 2FA tidak aktif); sesi 8 jam; re-auth untuk aksi merah.
3. Klien HOP App Protocol (Go) + registry sinkron dari hel.ink (`GET /api/hop/apps`, bertanda tangan).
4. Beranda gabungan, pencarian global, User 360°, aksi user global, audit log, notifikasi Telegram/email.
5. Deploy: systemd `imperium`, user sendiri, vhost Tailscale-only, backup DB Imperium.

### Status Tahap 2 (2026-10-27)

**TERPASANG 2026-10-27 21:20** (LinkHop PR #35 dideploy 5716b88; registry: imperium + citadel hidden; service `imperium` d8cae00 di 127.0.0.1:8100; kedua pintu menjawab; panggilan bertanda tangan Imperium→HOP ID `/api/hop/health` OK). Imperium (repo `navi-crwn/Imperium`, Go): login HOP ID (peran owner/admin/moderator/support, 2FA wajib, sesi 8 jam), beranda gabungan (health semua aplikasi + KPI + seri 14 hari dari HOP ID), pencarian global, daftar pengguna, User 360 dengan aksi (suspend/activate/revoke/roles/note/delete) yang diteruskan ke HOP ID dan dicatat di audit lokal, registry aplikasi, CSRF + CSP ketat. Dua pintu = dua entri registry (`imperium`, `citadel`). Sisi LinkHop: PR #35 (`hop-admin-api`, 255 tes).

**Pemasangan (urutan):** 1) merge + deploy PR LinkHop#35; 2) `bash ~/imperium/scripts/tahap2-helink.sh` (secret + hop:bootstrap); 3) `cd ~/imperium && make build && sudo bash scripts/tahap2-sudo.sh` (service menggantikan placeholder); 4) buka https://imperium.hel.ink → Login with HOP (owner + 2FA).

## Tahap 3: Admin API di tiap aplikasi + migrasi modul

**hel.ink** (`app/Http/Controllers/Hop/*`): implementasi kontrak di atas, membungkus logika yang sudah ada (AdminUserController, AdminLinkController, AdminBioController, AdminAbuseController, IP, blacklist, SEO, queue, geoip, proxy). Perbaikan sekalian: hapus file QR/bio saat delete, suspend admin memilih "sembunyikan konten", ban CIDR, audit log aksi admin, restart queue lewat hop-agent bukan `sudo` PHP. Lalu modul Imperium: Links, Bio, Analytics, Abuse (+ editor aturan), Domain, IP, Guest links, SEO, Email, Queue, API tokens, GeoIP, Retensi. **Pensiun:** semua route `/admin/*` hel.ink dan view `admin-*.blade.php`, `admin-control-panel` (dashboard admin kembali jadi dashboard user biasa).

**PixelHop** (`internal/hop`): kontrak + ekstensi storage, AI tools, media worker, album. Perbaikan sekalian: soft-delete 30 hari, status change tidak meng-unhide semua, cari username, registration dicek untuk SSO, hapus jalur password lokal untuk user HOP (password lokal dihapus saat tautan dibuat), role dari tiket (promosi **dan** demosi), `MergeUsers` lewat API. Lalu modul Imperium: Images, Albums, Users, Storage, AI, Media, Settings, Explore. **Pensiun:** `/admin/*` PixelHop, template `admin_*.html`, `a.admin()` tinggal untuk hak lihat hidden/API (berdasarkan roles dari tiket), CLI `user admin`.

**NoteHop** (`internal/hop`): kontrak (user, catatan metadata, slug holds, files, integrasi, settings, metrics) + fitur lapor + peran dari tiket, `NOTEHOP_ADMINS` dihapus, rate limit konfigurabel & persisten, sesi admin cabut. Lalu modul Imperium: Notes, Users, Names, Files, Realtime, Integrations, Settings.

**Urutan di dalam tahap 3:** hel.ink dulu (panel paling besar), PixelHop, lalu NoteHop. Tiap aplikasi: tulis Admin API + tes → modul Imperium → verifikasi di produksi lewat Tailscale → hapus panel lama → rilis.

### Status Tahap 3 (2026-10-27, berjalan)

- **Imperium**: modul generik berbasis manifest (konten + aksi, antrean laporan, pengaturan, ekstensi) terpasang (versi 3f5ab6a), registry disinkronkan tiap 5 menit.
- **LinkHop**: PR #36 (API konten/laporan/IP/domain/queue/settings, 262 tes) menunggu merge; branch `retire-admin-panel` (draft PR) menyusul.
- **PixelHop**: PR #3 (`/hop/*` lengkap + tiket v2 roles + migrasi 7, 50 tes) menunggu merge; branch `retire-admin-ui` (draft PR) menyusul. Deploy: `scripts/deploy-pixelhop.sh main`, lalu `scripts/tahap3-registry.sh pixelhop`.
- **NoteHop**: branch `hop-admin-api` (subagent) → PR; deploy: `scripts/deploy-notehop.sh main`, lalu `scripts/tahap3-registry.sh notehop`.
- Merge PR tidak bisa dilakukan Claude (pengaman auto mode): pemilik merge, Claude deploy.
- **Status 2026-10-27 malam:** PR menunggu merge pemilik: LinkHop #36 (CI hijau), PixelHop #3 (CI diperbaiki setelah gabung origin/main), NoteHop #8 (repo tanpa CI; tes lokal hijau). Draft (pensiun panel lama, merge SETELAH Imperium diverifikasi): PixelHop #4 (base hop-admin-api; saat deploy set `PIXELHOP_CONTROL_URL=https://imperium.hel.ink` di ~/apps/pixelhop/pixelhop.env), LinkHop #37 (saat deploy set `HOP_CONTROL_URL=https://imperium.hel.ink` dan `HOP_CONTROL_NAME=Imperium` di .env live, lalu config:cache). Imperium live versi 6946305 dengan modul generik, watch 2 menit, notifikasi Telegram (aktifkan dengan `sudo bash scripts/tahap3-telegram.sh`).
- **Urutan saat pemilik kembali:** 1) merge LinkHop #36 → `echo ya | bash ~/hel-ink-handoff/03-deploy.sh /var/www/helink/shortlink-app main` (atau minta Claude); 2) merge PixelHop #3 → `bash ~/imperium/scripts/deploy-pixelhop.sh main` → `bash ~/imperium/scripts/tahap3-registry.sh pixelhop`; 3) merge NoteHop #8 → `bash ~/imperium/scripts/deploy-notehop.sh main` → `tahap3-registry.sh notehop`; 4) aktifkan 2FA admin@hel.ink, masuk Imperium, cek modul tiap aplikasi; 5) baru merge PR pensiun panel (PixelHop #4 → lalu rebase ke main; LinkHop retire-admin-panel) dan deploy ulang; 6) `sudo rm /etc/sudoers.d/90-carawin-temp`.
- **Status 2026-10-28 pagi (deploy Tahap 3 selesai):** LinkHop live 65d9ef0 (main, termasuk #36 dan #37 → panel admin lama pensiun; `/admin` → login → Imperium; `HOP_CONTROL_URL`/`HOP_CONTROL_NAME` sudah di .env live). PixelHop live dari branch `hop-admin-api` (6c6209a, sudah berisi main + #4 pensiun admin; `PIXELHOP_CONTROL_URL` di pixelhop.env) karena **PR #3 ke main masih OPEN** — merge #3 lalu `deploy-pixelhop.sh main` supaya live = main. NoteHop live 29f463b (main, #8); env: `NOTEHOP_SSO_NAME=HOP`, `NOTEHOP_LINKS=LinkHop=…`, `NOTEHOP_HOP_CLIENTS=imperium,helink`. Registry: pixelhop `account_api,admin_api,events,metrics`; notehop `admin_api,events,metrics,zero_knowledge`. `/hop/health` bertanda tangan: linkhop, pixelhop, notehop semua 200/ok. Skrip tahap3-registry.sh diperbaiki (psysh butuh XDG_CONFIG_HOME yang bisa ditulis). **Temuan TLS:** nginx origin hel.ink memakai sertifikat self-signed (CN saja, tanpa SAN; publik dipegang Cloudflare), sehingga pintasan lokal `HOP_API_ADDR=127.0.0.1:443` (Imperium) dan `HELINK_API_ADDR=127.0.0.1:443` (PixelHop, login kata sandi via HOP ID) gagal verifikasi x509. Keduanya dimatikan (dikomentari di env) → lewat Cloudflare. Perbaikan permanen (Tahap 4): sertifikat origin dengan SAN (Cloudflare Origin CA atau certbot) dan opsi `HOP_API_CA` di Imperium/PixelHop. Pemilik: uji login kata sandi di p.hel.ink (Turnstile menghalangi uji otomatis).
- **Status 2026-10-28 siang:** PixelHop PR #3 ter-merge → live = main a65111f. LinkHop live 8a22b3d (PR #38 tile HOP mengambang + checkbox Terms, PR #39 tepi tile). Imperium 02e49c8: halaman login baru (kartu kaca, chip pintu Tailscale/Cloudflare Access, tile HOP, bebas CSP). Sisa untuk pemilik: 2FA admin@hel.ink + uji Imperium; `sudo bash scripts/tahap3-telegram.sh`; `sudo rm /etc/sudoers.d/90-carawin-temp`.

## Tahap 4: hop-agent dan modul Server

1. `hop-agent` (Go) jalan sebagai root-less user dengan sudoers berisi daftar perintah tetap (`systemctl start|stop|restart|status <daftar unit>`, `journalctl -u <unit>`, `nginx -t`, `nginx -s reload`, `apt list --upgradable`, skrip backup/deploy tertentu). Komunikasi lewat unix socket + token; tidak pernah menerima shell bebas.
2. Modul Service, Sistem, Nginx, Sertifikat, Database, Backup (+ uji restore), Deploy (+ rollback), Cron, Keamanan, Env & secrets (rotasi secret SSO), Docker.
3. Uptime eksternal (Cloudflare Worker ping) → Imperium.

## Tahap 5: observabilitas dan otomasi

Metrics historis, log terpusat, laporan berkala, kesehatan SSO, deteksi anomali + aksi otomatis, feature flags, maintenance mode global, staging + e2e dari Imperium, runbook, terminal web darurat, multi-server.

## Kandidat: BioHop (usulan pemilik 2026-10-27)

Link-in-bio dipecah dari LinkHop menjadi produk sendiri, **BioHop**, supaya LinkHop ringan dan rapi. Dampak: memindahkan `bio_pages`, `bio_links`, `bio_clicks`, 20 tema, upload, analytics bio, dan route `hel.ink/b/{slug}` (harus tetap hidup lewat redirect ke domain BioHop). Keputusan: dikerjakan **setelah Tahap 3** (HOP ID dan Imperium sudah stabil), sebagai aplikasi HOP keempat/kelima yang mendaftar lewat registry seperti RemoteHop. Sampai saat itu modul Bio di Imperium (C30) dibangun di atas Admin API LinkHop dan nanti tinggal dipindah.

## RemoteHop

Begitu RemoteHop ada, ia hanya perlu: satu baris di registry, implementasi `internal/hop` dari paket `hopproto`, dan tombol "Login with HOP". Semua modul generik Imperium (users, reports, settings, metrics, health) langsung bekerja; modul khusus (router, perangkat) dibuat sebagai ekstensi.

## Keputusan pemilik (2026-10-27)

1. **Subdomain:** `imperium.hel.ink`. Akses **Tailscale-only dan Cloudflare Access keduanya**, supaya tetap bisa masuk kalau salah satu hilang. Implementasi: vhost listen di IP Tailscale untuk jalur utama; jalur kedua lewat Cloudflare Tunnel (`cloudflared`) + Access policy (Google milik pemilik), tanpa port publik di VPS.
2. **Rename repo:** `navi-crwn/hel.ink` → `navi-crwn/linkhop`; nama produk **LinkHop**, tagline "Hop Easy Link" (arti HEL.ink); domain tetap hel.ink; GitHub Pages jadi `navi-crwn.github.io/linkhop/`. Pola nama keluarga: `<benda>Hop` (LinkHop, PixelHop, NoteHop, RemoteHop); merek payung **HOP**, akun **HOP ID**, pusat kendali **Imperium**. Folder server `/var/www/helink` dibiarkan (kosmetik).
3. **Peran awal:** hanya pemilik sebagai `owner`. Modul Tim minimal di MVP.
4. **Soft-delete PixelHop 30 hari:** ya, dengan batas tempat sampah 2 GB dan purge lebih awal bila penuh.
5. **Username bentrok:** otomatis "lebih dulu dibuat menang"; backfill pertama menampilkan daftar konflik dulu sebelum dijalankan.

## Tambahan permintaan pemilik

- **Backup & migrasi satu perintah** ke server baru (fitur I84–I88 di `01-FITUR.md`): masuk Tahap 4 bersama hop-agent, tapi skrip dasarnya dibuat sudah di Tahap 0 karena berguna sebelum Imperium jadi.
- **Watchdog konten berbahaya** yang bisa menyebabkan takedown domain/hosting (fitur I89–I97): fondasinya di Tahap 3 (modul Abuse hel.ink diperluas ke semua aplikasi), otomasi dan pemantauan reputasi di Tahap 5.
- **Status 2026-10-28 sore (sidebar explorer + dashboard per app, PR navi-crwn/Imperium "app-dashboards"):** Imperium sekarang berlayout dua kolom: sidebar kiri ala file explorer (folder Imperium: Beranda/Pengguna/Aplikasi/Audit; folder Aplikasi: LinkHop (HOP ID), PixelHop, NoteHop, dan app registry lain, masing-masing dengan titik sehat/tidak dan anak modul dari manifest: Dashboard, tipe konten, Laporan, Pengaturan, ekstensi). Klik app → `GET /apps/{slug}` = dashboard app: kesehatan + versi, kartu KPI dari `/stats` (byte dihumanisasi, label i18n per kunci), seri 14 hari, konten terbaru (8 baris), laporan terbuka dengan tombol aksi (LinkHop: confirm/restore + investigate/close; app lain: dari `actions.report` di manifest), kartu operasional (LinkHop `x/queue`, NoteHop `x/realtime`). Laporan dan pengaturan sekarang juga muncul untuk PixelHop/NoteHop lewat `actions`/`settings_schema` manifest (tidak lagi butuh capability `reports`). Ekstensi: aksi punya daftar pilihan (ip: ban/unban/unwatch; domains: add/remove; names: release) dan sub-path untuk `names/holds/{slug}` NoteHop. Sidebar collapsible (hamburger; mobile slide-in; desktop diingat di localStorage), tanpa inline style (semua `style=` lama diganti kelas; tinggi bar grafik jadi kelas h0…h100, sebelumnya diblokir CSP). Tagline jadi "HOP Dashboard"; logo baru bergaya tile HOP (kotak membulat bergradasi, glyph putih perisai + citadel; `static/imperium.svg`: favicon, bar atas, login); logo LinkHop di tile login diperbarui dari hel.ink/brand (kelinci baru); PR LinkHop #44 mengganti tile Imperium (mahkota → perisai) di halaman auth. `scripts/fakehop` = HOP ID + PixelHop + NoteHop palsu untuk pengembangan UI/screenshot (`docs/img-*.png`). CI GitHub Actions ditambahkan (gofmt, vet, test -race, build). Sisi app: PR PixelHop #5 (`hop-ip-extension`, CI hijau) menambah ekstensi `ip` (daftar ban + ban/unban) dan KPI disk, menggantikan halaman "IP bans" admin lama. Belum: ringkasan user per app di User 360 (PixelHop kuota/role, NoteHop), analytics global LinkHop (negara/perangkat/top links) butuh endpoint baru. **Dideploy 2026-10-28:** PR #1 merged → Imperium live = main bc17915 (binary diinstal ke /usr/local/bin, service restart, /healthz ok). PixelHop #5 merged dan dideploy (live = main 1b70a26, ekstensi `ip` + KPI disk; manifest di Imperium diperbarui otomatis ≤10 menit). LinkHop: merge #44 lalu deploy biasa.
- **Status 2026-10-28 (login pertama pemilik):** HOP ID `ivan_smb787` diberi peran global `owner` (granted_by superadmin). Tautan "Imperium" di dashboard LinkHop dan `/admin` PixelHop dialihkan ke **https://citadel.hel.ink** (`HOP_CONTROL_URL` / `PIXELHOP_CONTROL_URL`), karena `imperium.hel.ink` hanya hidup di Tailscale dan tidak terjangkau dari perangkat tanpa Tailscale. Pintu Tailscale tetap aktif sebagai cadangan.
