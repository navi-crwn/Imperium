# Inventaris admin hel.ink (HEL.ink, akan menjadi LinkHop)

Hasil pembacaan read-only kode live di `/var/www/helink/shortlink-app` (commit 859b69a), 2026-10-27. Dipakai sebagai daftar "apa yang harus dipindah ke Imperium dan dipensiunkan".

## Fakta kunci

- **Hanya ada satu admin.** `User::isAdmin()` = email sama dengan `SUPERADMIN_EMAIL`. Kolom `users.role` tidak pernah dipakai untuk otorisasi (`app/Models/User.php:93-106`).
- Middleware `admin` (`app/Http/AdminMiddleware.php`) hanya 403 untuk non-admin. **Grup admin tidak memakai middleware `2fa`.**
- Panel utama bukan di `/admin` (hanya redirect) tapi di `/dashboard` yang merender `admin-control-panel.blade.php` kalau user admin (`LinkController.php:98-153`).
- Tidak ada audit log aksi admin sama sekali. `ActivityLog` hanya mencatat aksi user biasa (link, folder, tag, profil).
- `LoginHistory` hanya dari login password; login Google tidak dicatat.

## Route admin (`routes/web.php:154-203`)

| Path | Fungsi |
|---|---|
| GET /admin/analytics/global | Total user/link/klik/link aktif, seri harian/mingguan/bulanan, top 20 link, negara, peta, kota, ISP, proxy, 12 klik terakhir, device split |
| GET/POST /admin/users | Daftar 20/halaman, cari nama/email, filter status, sort; buat user biasa |
| GET /admin/users/lookup | JSON 10 user untuk owner picker |
| GET /admin/users/{id}/inspect | Detail: link, folder, tag, klik, bio pages |
| PATCH /admin/users/{id}/status | active / suspended (bukan superadmin, bukan diri sendiri) |
| DELETE /admin/users/{id} | Hard delete + cascade (link, QR, folder, tag, bio, file bio) |
| GET /admin/links | Semua link, filter status/user/guest/folder/tag/q, sort |
| GET /admin/links/my | Link milik admin |
| PATCH /admin/links/{id}/status, DELETE, POST /bulk | aktif/nonaktif, hapus, massal (file QR tidak dihapus) |
| /admin/ip-bans | CRUD ban IP dengan reason + expires_at; dicek `CheckIpBan` (cache 60 s) |
| /admin/ip-watchlist | Daftar pantau IP (tidak memblokir) |
| /admin/domain-blacklist | Domain exact/wildcard + kategori (phishing, porn, malware, spam, scam, illegal, violence, hate, other) |
| /admin/seo | site_title, meta, og_*, favicon, logo |
| /admin/abuse | Laporan + temuan scan; scan manual (3/10 menit); ubah status laporan; confirm/restore flag |
| POST /admin/queue/restart, GET /queue/status | `queue:restart` + `sudo systemctl restart queue-worker` via shell_exec |
| /admin/geoip-monitor, /geoip-test | Provider GeoIP dan kuota |
| /admin/proxy-monitor, /proxy-test, /proxy-reset-stats | Detektor proxy dan statistik |
| /admin/bio, /bio/monitor, /bio/export, /bio/{id}, toggle, bulk-delete | Bio pages: daftar, monitor periode, CSV, detail, publish/unpublish (membuat AbuseFlag), hapus (file tidak dihapus) |

## Widget control panel (`admin-control-panel.blade.php`)

Users (+7 hari), Links (live), Total clicks (30 hari tersimpan), Clicks today vs kemarin, grafik 14 hari, Needs attention (laporan terbuka, temuan terbuka, user suspended, link nonaktif), Queue worker heartbeat + tombol restart, Abuse scan terakhir, 6 link terbaru, 6 user terbaru, top 8 negara, 4 laporan terakhir, hitungan IP ban/watchlist/blacklist.

## Apa yang bisa diubah admin pada user

Hanya `status`. Tidak ada UI untuk role, email, nama, password, reset 2FA, verifikasi email, kuota.

**Suspend** hanya mengubah status: logout paksa, login ditolak (password, Google, API token, API PixelHop, tiket SSO). **Tidak** menonaktifkan link/bio (berbeda dengan suspend otomatis `abuse:scan` yang menonaktifkan semuanya).

**Delete** = hard delete, tanpa audit.

## Jadwal (`routes/console.php`)

| Jadwal | Perintah |
|---|---|
| harian | `clicks:purge` (link_clicks > 30 hari) |
| 03:15 | `privacy:prune` (bio_clicks 30 h, login_histories 90 h, activity_logs 90 h, abuse_reports 180 h) |
| Senin 02:00 | `geolite:update` |
| tiap 15 menit | `queue:health` |
| tiap 2 menit | job `QueueHeartbeat` |
| tiap jam | `abuse:scan --since=75m` |
| 04:30 | `abuse:scan --full --purge` |
| 00:00 | `links:prune-guest` |
| Senin 03:00 | `disposable:update` |
| 03:45 | `abuse:feeds` (URLhaus, OpenPhish) |
| 02:30 / 02:45 / 09:00 | `backup:clean` / `backup:run` / `backup:monitor` (spatie) |

`abuse:scan`: skor ≥ 60 nonaktifkan + flag, ≥ 100 atau ≥ 3 item nonaktif → suspend pemilik, `--purge` hapus yang nonaktif > 30 hari.

## Setting yang hanya ada di .env (nama kunci)

SUPERADMIN_EMAIL; TURNSTILE_SITE_KEY/SECRET_KEY; SAFE_BROWSING_KEY, URLHAUS_AUTH_KEY, SPAMHAUS_DBL, ABUSE_REGISTER_PER_HOUR; GUEST_LINK_MAX_DAYS, GUEST_LINK_IDLE_DAYS, GUEST_LINK_TIMEZONE, RETIRED_SLUG_DAYS, BIO_SLUG_MIN_LENGTH; LIMIT_IP_PER_MINUTE, LIMIT_IP_PER_DAY, LIMIT_GUEST_PER_DAY, LIMIT_GUEST_TOTAL, LIMIT_USER_PER_DAY, LIMIT_USER_ACTIVE; ADMIN_EMAIL, SUPPORT_EMAIL, SECURITY_EMAIL, FEEDBACK_EMAIL; BACKUP_*; GEOLITE_*, IPINFO_API_KEY, ABSTRACTAPI_KEY, PROXYCHECK_API_KEY, IPHUB_API_KEY, IPQUALITYSCORE_API_KEY; PIXELHOP_URL/SSO_SECRET, NOTEHOP_URL/SSO_SECRET; GOOGLE_*; SESSION_*; MAIL_*; SENTRY_*.

Hard-coded tanpa kunci env: pola regex abuse, bobot, threshold 60/100, suspend_after_disabled_items 3, purge 30 hari, retensi privasi, rate limit, reserved_slugs, panjang slug.

**Bug ditemukan:** config membaca `SAFE_BROWSING_KEY` tapi `.env` mengisi `SAFE_BROWSING_API_KEY`, jadi Google Safe Browsing tidak pernah jalan. Kunci di .env yang tidak dibaca kode: SAFE_BROWSING_API_KEY, VIRUSTOTAL_API_KEY, GEMINI_API_KEY, IPAPI_API_KEY, SESSION_INACTIVITY_LOGOUT.

## API yang sudah ada

- `POST /api/shorten` (Bearer token), `GET /api/links`, `POST /api/shorten-web`, `POST /api/bio/shorten`, `POST /api/utils/fetch-title` (sesi).
- `POST /api/pixelhop/login|register`: server-to-server, HMAC `X-PixelHop-Timestamp` + `X-PixelHop-Signature` = base64url(HMAC-SHA256(secret, "pixelhop-api|ts|body")), umur 120 s. Payload: sub, email, name, avatar, admin, verified.
- `GET /sso/{app}`, `GET /sso/{app}/google`, `GET /pixelhop`, `GET /notehop` (AppSsoController).

## Celah lain

- Admin route tanpa 2FA; delete link/bio meninggalkan file; `bio.bulk-delete` dan `queue.status` tidak dipakai view; restart queue lewat `sudo` dari PHP; panel mengkueri tabel besar tanpa cache.
