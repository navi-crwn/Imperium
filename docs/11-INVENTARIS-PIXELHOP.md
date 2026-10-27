# Inventaris admin PixelHop

Read-only, 2026-10-27. **Perhatian versi:** checkout `~/pixelhop` di `main` = v3.5.1, binary live = v3.6.0 yang hanya ada sebagai tag di branch `claude/musing-shtern-2628ad`. Inventaris ini menggambarkan v3.6.0.

## Penjaga akses

- `a.admin()` (`internal/app/auth.go:134-142`): user login dengan `role == "admin"`, selain itu 403.
- Sesi lewat cookie `ph_session` atau `Authorization: Bearer ph_…`; hanya user `status == active` yang dilekatkan.
- Rantai middleware: accessLog → recoverer → withIP → securityHeaders → csrf → checkBan → withUser.

## Route admin (`internal/app/admin.go` v3.6.0)

| Path | Fungsi |
|---|---|
| GET /admin | KPI: gambar, byte, views, user, album, laporan terbuka, disk free/total, upload 30 hari, AI tool 7 hari, 12 upload terakhir |
| GET /admin/images | Semua gambar 60/halaman: q, sort (old/views/size), user, page |
| POST /admin/images/{id} | action: hide, unhide, delete, ban (hapus + ban IP) |
| GET /admin/users | 50/halaman, cari email/nama (bukan username), storage + jumlah gambar per user |
| POST /admin/users/{id} | action: quota (quota_gb), status (active/locked/suspended + note), role (user/admin), warn, password, delete (purgeUser) |
| GET/POST /admin/reports, POST /reports/{id} | tab open/actioned/dismissed; dismiss/hide/delete |
| GET/POST /admin/bans, POST /bans/delete | ban IP exact + reason; cache 30 s |
| GET/POST /admin/settings | guest_uploads, registration, announcement, seo_title, seo_description + kartu server read-only |

Kemampuan admin di luar /admin: lihat gambar hidden (451 untuk lainnya), GET/PATCH/DELETE gambar siapa pun lewat API, akses album siapa pun, ZIP download.

Perilaku penting: mengubah status ke non-active menghapus semua sesi; **status selain suspended meng-unhide semua gambar user, termasuk yang disembunyikan karena alasan lain**. Admin tidak bisa mengubah status/role/delete akun sendiri.

## Setting

**DB tabel `settings`:** guest_uploads, registration, announcement, seo_title, seo_description (+ counter `usage:YYYY-MM-DD:<tool>`). Tabel `secrets` untuk kunci otomatis.

**Env saja (`internal/config/config.go`):** PIXELHOP_ADDR, BASE_URL, DATA, SITE_NAME, SECRET; MAX_UPLOAD_MB (20), MAX_MEDIA_MB (100), GUEST_MAX_MB (20), MAX_MEGAPIXELS (80), MAX_BATCH (20), UPLOADS_PER_MINUTE (30, member ×4); GUEST_UPLOADS, REGISTRATION, DEFAULT_QUOTA_MB (1024); INACTIVE_DAYS; TRUSTED_PROXIES; STORAGE + S3_*; GOOGLE_*; CONTACT_EMAIL; HELINK_URL, HELINK_SSO_SECRET, HELINK_API_ADDR; TURNSTILE_*; REMBG_URL, REMBG_PER_DAY. Session TTL 30 hari hard-coded.

## User dan profil

Kolom: id, email (unique nocase), name (≤60), pass_hash (argon2id, bcrypt juga diterima), google_id, avatar_url, role, status (active/locked/suspended), status_note, quota_bytes, created_at, last_login, warning, helink_id, username (unique nocase), bio (≤300), website (≤200), avatar_key, show_owner.

Tabel terkait: sessions (ip, user_agent), api_keys (maks 20), webhooks (maks 5).

Profil (`profile.go`): POST /settings/profile (name, username, bio, website, show_owner); avatar upload ≤8 MB → JPEG 400px; POST /settings/password (disembunyikan untuk user HEL.ink tapi route tetap jalan); API keys; delete account; ack-warning.

Aturan username: 3–30, huruf kecil ASCII, angka, `- _ .`; mulai huruf/angka; tidak diakhiri `.`; tidak `..`; reserved: admin, api, about, help, support, settings, login, logout, register, dashboard, tools, docs, legal, static, pixelhop, helink, hel, root, system, moderator, staff, official, null, undefined, me, you, guest, anonymous. Auto-suggest dari nama atau `userNNNNN`, backfill saat startup dan login.

## SSO HEL.ink (`helink.go`, `helink_api.go`)

- `/auth/helink` → cookie `ph_helink` (state|next, 15 menit) → `{HELINK_URL}/sso/pixelhop?state&return_to`.
- Callback: tiket HMAC ≤600 s, state cocok, jti sekali pakai (memori).
- `linkHelinkUser`: cari helink_id → cari email (tautkan implisit, **bukan MergeUsers**) → buat baru (admin jika t.Admin atau user pertama; **setting registration tidak dicek**). Email selalu ditimpa dari HEL.ink; name hanya jika kosong; avatar_url ditimpa; t.Admin mempromosikan, **tidak pernah mendemosi**.
- Form login sendiri: cek pass_hash lokal dulu, baru HEL.ink. Konsekuensi: user tertaut dengan password lokal sisa masih bisa login melewati HEL.ink; "reset password" admin membuat password lokal untuk user HEL.ink.
- Google sign-in tetap ada berdampingan.
- `MergeUsers` hanya dari CLI `pixelhop user merge`.

## Moderasi

- reports: reason (nsfw/illegal/privacy/copyright/spam/other), note, ip, status (open/dismissed/actioned); 10/jam/IP, 1 per IP per gambar.
- bans: IP exact, cache 30 s, blokir semua non-GET.
- images.hidden (451), tidak ada soft-delete. Flag pemilik: private, pass_hash, listed, expires_at.
- user status: locked (tak bisa login, gambar tetap) / suspended (gambar disembunyikan).
- warning: modal sampai di-ack. Tidak ada audit log.

## Statistik dan job

Stats, UserStorage, `/metrics` Prometheus (loopback saja), member dashboard (UserSummary), image_views harian + image_referrers, PublicStats. Janitor 5 menit (expired/inactive, album kosong, sesi), chunks.sweep 10 menit, mediaWorker (ffmpeg, tabel media_jobs).

## API dan webhook

Bearer `ph_<token>`; `/api/v1/upload`, chunked uploads, `/api/v1/images[...]`, `/stats`, `/api/v1/me`, `/api/v1/stats`, `/api/v1/albums`, `/api/v1/users/{username}/images`, `/oembed`, `/api/v1/tools/{tool}` (remove-background 30/hari, faces 100, erase 30, upscale 10). Webhook per user maks 5, event image.uploaded / image.deleted, HMAC `X-PixelHop-Signature`, retry 3×. Tidak ada webhook global/admin.

## CLI

`serve`, `version`, `user add|passwd|admin|merge` (tidak ada demote), `import`, `login`, `push`.

## Celah

Tidak ada: unlink helink_id, ubah email, demosi dari HEL.ink, daftar/kill sesi, tampilan API key/webhook admin, audit log, ban CIDR, cari username; status change meng-unhide semua gambar; SSO membuat profil walau registration tutup.
