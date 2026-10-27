# Inventaris admin NoteHop

Read-only, 2026-10-27, repo `~/notehop/repo` (89df118 = binary live). Tidak ada paket `internal/config`; env dibaca di `cmd/notehop/main.go:120-142`.

## Mekanisme admin sekarang

**A. `NOTEHOP_ADMINS`** = daftar username (bukan role di DB). `siteAdmin(u)` (`names.go:95-97`). Hanya memberi hak penamaan: lewati minimum 7 karakter slug, lewati reserved, lewati hold; boleh mendaftarkan nama reserved miliknya.
**Celah:** siapa pun yang mendaftar duluan dengan username yang ada di daftar (misal SSO email `admin@x` → username `admin`) otomatis jadi site admin. Harus diperbaiki sebelum memberi kuasa admin sungguhan. Tidak ada UI admin; template tidak pernah mengecek admin.

**B. Token admin per catatan (`X-Admin`)**: browser membuat kunci 32 byte, server simpan SHA-256 di `notes.admin_hash`. Link admin = `#k=<K>&a=<admin>`. Melewati mode link, IP allowlist, time capsule, lock, hitung views, burn. Route admin-only: hapus entry, freeze, tambah/hapus link, hapus catatan (admin atau pemilik), config (lock_at, open_at, sign_off, ip_allow ≤50, reset, dms_days, watermark, watch, profile, check_in), decoy, suggestions, claim, inbox, versions.

**C. CLI**: `notehop takedown URL|TOKEN` (hold semua nama 365 hari, owner NULL, hapus catatan), `takedown -file ID`, `backup out.db` (VACUUM INTO), `deploy/backup.sh` harian.

**Tidak ada sama sekali:** daftar user, hapus/ban user, cabut sesi, reset password, hapus akun, lepas slug hold (hanya SQL manual), cari catatan per pemilik, audit log, fitur lapor (hanya halaman `/contact`).

## Env

NOTEHOP_ADDR, DB, SITE, BASE_URL, SHARE_URL, LINKS, CONTACT, SECURE_COOKIES, TRUST_PROXY, REGISTRATION, ADMINS, RESERVED, SSO_URL, SSO_SECRET, SSO_NAME, SSO_CLIENT, SSO_PROVIDERS, FILE_MB, FILES_TOTAL_MB, FILES_IP_MB, PRIVATE_HOOKS, TELEGRAM_BOT_TOKEN, SSH_ADDR, SSH_KEY; CLI: SERVER, PASSWORD; backup: BACKUP_DIR, BACKUP_KEEP_DAYS.
Hard-coded: 1 MB/catatan, 64 KB/entry, 2000 entry, 1000 views, expiry 365 hari, 50 burn block, 200 suggestion, 20 voter/jaringan, 1000 signature, sesi 30 hari, 100 versi.

## Skema dan apa yang bisa dibaca server

| Tabel | Terbaca server |
|---|---|
| users (username, auth_hash, created, sso_id, email, display_name, avatar, vault_check) | username, sso_id, email, display_name, avatar, created plaintext; hash argon2id |
| sessions | hash saja; bisa dihitung/dicabut |
| notes | `content` ciphertext jika encrypted=1, **plaintext JSON jika encrypted=0**; metadata plaintext: config (watch, ip_allow, profile, timer), checks, views, max_views, expires, frozen, last_open |
| links (token, note_id, mode) | plaintext; slug custom = token huruf kecil |
| entries, versions, suggestions, signatures | ikut status enkripsi catatan |
| votes, poll_voters | hash jaringan 8 byte, tanpa IP mentah |
| files (id, size, data, expires 24 h) | ciphertext, **tidak terhubung ke catatan/user** |
| inboxes, telegram_chats | pubkey plaintext, chat_id plaintext |
| slug_holds | plaintext, 90 hari / 365 hari takedown |

Janitor `Sweep` tiap menit. `NotesByOwner` maks 500 baris.

## SSO (`sso.go`)

Tiket sama dengan PixelHop plus `aud`. **`Admin` di tiket diparsing tapi tidak pernah dipakai** (satu-satunya hook siap pakai). `linkSSOUser`: cari sso_id → refresh email/name/avatar; kalau tidak ada dan registration tutup → tolak; kalau tidak, `ssoUsername(email)` maks 6 percobaan, fallback `user_xxxxx`. Akun password lama tidak pernah ditautkan. Vault: `POST /api/vault` menyimpan HMAC frasa.

## Profil dan nama

`/@username` hanya menampilkan catatan publik dengan `profile:true` milik user; 404 jika kosong. Username `^[a-z0-9_]{3,32}$` (berbeda dengan PixelHop yang mengizinkan `-` dan `.`). Slug `^[a-z0-9][a-z0-9-]{2,47}$`, min 7. Reserved `names/reserved.txt` + prefix (notehop-, official-, admin-, support-, staff-, team-, security-, verify-). Blocked `names/blocked.txt`. Hold 90 hari; takedown 365 hari.

## Abuse dan moderasi

Rate limit memori per IP per menit: raw/mock 120, tulis 120, POST /api/notes 30, files 20, inbox 60, login/register/vault 15, nearby 30, SSH 20. Tidak konfigurabel, tidak persisten. Tidak ada ban IP global. IP allowlist per catatan. Files: FILE_MB, kuota harian per IP (memori), FILES_TOTAL_MB (507 bila penuh), TTL 24 h, takedown terpisah. Webhook https-only + SSRF guard. Telegram bot tanpa allowlist. SSH tanpa auth, catatan publik 30 hari. Nearby memori saja.

## Statistik yang bisa dihitung

SQL: user total/password vs SSO/signup per waktu/vault/sesi aktif; notes total/encrypted vs public/password/burn/expiring/frozen/owned vs anonim/ukuran/views/last_open; adopsi fitur dari config; link per mode, slug custom vs random; entries, versions, suggestions, signatures, votes, decoys, margins, burn_keys; files count + size vs budget; inbox + telegram; slug_holds aktif/takedown; ukuran DB/WAL.
Memori (perlu accessor): liveHub rooms/peers, nearbyHub, limiter buckets, dayQuota, watcher jobs.

## Ide belum dibangun (docs)

Kuota catatan per IP, recovery code, rate limit konfigurabel, lepas hold lewat admin, index profil hanya akun terverifikasi, watch via Telegram/email, X-Robots-Tag. **Tidak ada dokumen yang menyebut admin dashboard, antrean laporan, manajemen user, atau ban list: semuanya baru.** Aturan CLAUDE.md: panel hanya boleh membaca isi catatan publik.
