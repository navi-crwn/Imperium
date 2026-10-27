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
5. **Repo:**
   - `navi-crwn/imperium` (privat): Imperium (Go, satu binary, SQLite untuk audit/settings/metrics) + `hop-agent` + protokol HOP (dokumen + paket Go `hopproto` + stub PHP).
   - `navi-crwn/hel.ink` → direname **`navi-crwn/linkhop`** (GitHub redirect otomatis).
   - `navi-crwn/pixelhop`, `navi-crwn/NoteHop`: tambah paket `internal/hop` yang mengimplementasi kontrak.
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

**Deploy Tahap 1 (urutan):**
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

## Tahap 3: Admin API di tiap aplikasi + migrasi modul

**hel.ink** (`app/Http/Controllers/Hop/*`): implementasi kontrak di atas, membungkus logika yang sudah ada (AdminUserController, AdminLinkController, AdminBioController, AdminAbuseController, IP, blacklist, SEO, queue, geoip, proxy). Perbaikan sekalian: hapus file QR/bio saat delete, suspend admin memilih "sembunyikan konten", ban CIDR, audit log aksi admin, restart queue lewat hop-agent bukan `sudo` PHP. Lalu modul Imperium: Links, Bio, Analytics, Abuse (+ editor aturan), Domain, IP, Guest links, SEO, Email, Queue, API tokens, GeoIP, Retensi. **Pensiun:** semua route `/admin/*` hel.ink dan view `admin-*.blade.php`, `admin-control-panel` (dashboard admin kembali jadi dashboard user biasa).

**PixelHop** (`internal/hop`): kontrak + ekstensi storage, AI tools, media worker, album. Perbaikan sekalian: soft-delete 30 hari, status change tidak meng-unhide semua, cari username, registration dicek untuk SSO, hapus jalur password lokal untuk user HOP (password lokal dihapus saat tautan dibuat), role dari tiket (promosi **dan** demosi), `MergeUsers` lewat API. Lalu modul Imperium: Images, Albums, Users, Storage, AI, Media, Settings, Explore. **Pensiun:** `/admin/*` PixelHop, template `admin_*.html`, `a.admin()` tinggal untuk hak lihat hidden/API (berdasarkan roles dari tiket), CLI `user admin`.

**NoteHop** (`internal/hop`): kontrak (user, catatan metadata, slug holds, files, integrasi, settings, metrics) + fitur lapor + peran dari tiket, `NOTEHOP_ADMINS` dihapus, rate limit konfigurabel & persisten, sesi admin cabut. Lalu modul Imperium: Notes, Users, Names, Files, Realtime, Integrations, Settings.

**Urutan di dalam tahap 3:** hel.ink dulu (panel paling besar), PixelHop, lalu NoteHop. Tiap aplikasi: tulis Admin API + tes → modul Imperium → verifikasi di produksi lewat Tailscale → hapus panel lama → rilis.

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
