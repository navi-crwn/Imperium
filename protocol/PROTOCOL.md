# HOP App Protocol v1 (draft Tahap 0)

Kontrak antara **Imperium** (pusat kendali) dan setiap aplikasi keluarga HOP (LinkHop, PixelHop, NoteHop, RemoteHop, …). Aplikasi yang memenuhi kontrak ini otomatis muncul di Imperium tanpa kode khusus di sisi Imperium. Akun tetap dimiliki **HOP ID** (hel.ink); Imperium tidak pernah membuka database aplikasi lain.

Status: draft, sisi HOP ID (hel.ink) sudah diimplementasikan di branch `hop-id` (Tahap 1). Berubah sampai Tahap 3 selesai; setelah itu versi dibekukan dan perubahan hanya lewat `X-HOP-Version` baru.

## 1. Pihak dan arah panggilan

```
HOP ID (hel.ink) ──tiket SSO v2──▶ Aplikasi      (login user, sudah ada)
HOP ID (hel.ink) ──webhook────────▶ Aplikasi      (profile.updated, account.*, session.revoked)
Imperium ──────────Admin API──────▶ Aplikasi      (baca, moderasi, setting)   ← dokumen ini
Aplikasi ──────────push───────────▶ Imperium      (laporan baru, alert)       (opsional)
Imperium ──────────registry───────▶ HOP ID        (daftar aplikasi + secret)
```

## 2. Tanda tangan permintaan

Semua panggilan server-ke-server memakai pola yang sudah ada di `PixelHopAccountController` dan `helink_api.go`, digeneralisasi:

```
X-HOP-App:       <slug aplikasi pemanggil>      (mis. "imperium", "helink")
X-HOP-Timestamp: <unix detik>
X-HOP-Nonce:     <16+ byte base64url acak>
X-HOP-Actor:     <hop user id admin yang memicu, atau "system">
X-HOP-Signature: base64url( HMAC-SHA256( secret, "hop1|" + app + "|" + ts + "|" + nonce + "|" + METHOD + "|" + PATH + "|" + sha256hex(body) ) )
```

- `PATH` = target permintaan **termasuk query string** (`/api/hop/profiles?ids=1,2`), tanpa host. Body kosong → sha256 dari string kosong.
- `secret` = secret milik **aplikasi yang terdaftar** di registry HOP ID (32+ karakter), yaitu pihak yang bukan HOP ID: aplikasi → HOP ID ditandatangani dengan secret aplikasi itu sendiri (`X-HOP-App` = slug-nya); HOP ID → aplikasi (webhook) dan Imperium → aplikasi dengan secret aplikasi tujuan (`X-HOP-App` = `helink` / `imperium`). Imperium menyimpan salinan semua secret (terenkripsi).
- Implementasi acuan: `app/Support/HopSignature.php` di repo linkhop (sign + verify + nonce sekali pakai).
- Umur maksimum 120 detik; nonce disimpan sampai kedaluwarsa (anti-replay).
- Rotasi: registry menyimpan `secret` dan `secret_prev` + `prev_until`; penerima mencoba keduanya selama masa transisi.
- Jawaban error tanda tangan: `401 {"error":"bad_signature"|"stale_request"|"replay"}`; secret belum dikonfigurasi: `404 {"error":"not_configured"}`.

## 3. Bentuk jawaban

```json
{ "ok": true,  "data": …, "page": {"next": "<cursor>", "total": 123} }
{ "ok": false, "error": "code_snake_case", "message": "teks untuk manusia", "details": {} }
```

Semua waktu ISO-8601 UTC. Id user selalu **HOP ID** (string `sub` dari tiket), bukan id lokal aplikasi. Aplikasi boleh menyertakan `local_id`.

## 4. Endpoint wajib (`/hop/*`, hanya bisa diakses dengan tanda tangan)

| Endpoint | Jawaban |
|---|---|
| `GET /hop/health` | `{app, version, commit, uptime_s, db: "ok", storage: "ok", sso: "ok", checks: [{name, ok, detail}]}` |
| `GET /hop/manifest` | Deskripsi diri: `{app, name, icon, base_url, capabilities: [...], content_types: [...], settings_schema: [...], actions: {...}}`. Imperium merender UI dari sini. |
| `GET /hop/stats?since=&until=&step=day` | `{kpi: {users, content, active_today, …}, series: {<kpi>: [[ts, n], …]}}` |
| `GET /hop/users/{hop_id}` | Ringkasan user di aplikasi ini: status lokal, kuota, jumlah konten, sesi aktif, api keys, webhooks, catatan admin, `last_seen` |
| `POST /hop/users/{hop_id}/actions` | `{action, reason, params}`; action wajib: `suspend`, `lock`, `activate`, `revoke_sessions`, `revoke_keys`, `warn`, `set_quota`, `purge` (hapus semua data user di aplikasi ini), `export` (kembalikan URL unduhan sementara) |
| `POST /hop/profile` | Terima profil terbaru `{hop_id, username, name, bio, website, avatar_url, locale, show_owner, roles, status, profile_version}`; idempoten; aplikasi menyimpan salinan |
| `GET /hop/content?type=&q=&owner=&status=&cursor=&limit=` | Daftar konten generik: `[{id, type, title, owner, status, created, size, url, flags, preview}]`. `type` dari manifest (mis. `link`, `bio`, `image`, `album`, `note`) |
| `GET /hop/content/{type}/{id}` | Detail + aksi yang tersedia |
| `POST /hop/content/{type}/{id}/actions` | `{action, reason, params}`; action umum: `hide`, `unhide`, `disable`, `enable`, `delete`, `takedown`, `transfer` (ke hop_id lain) |
| `GET /hop/reports?status=&cursor=` | Laporan abuse di aplikasi ini (atau `[]` kalau aplikasi push ke Imperium) |
| `POST /hop/reports/{id}/actions` | `dismiss`, `resolve`, `escalate` |
| `GET /hop/settings` | `[{key, type, value, default, description, restart_required}]` (nilai rahasia dimask) |
| `PUT /hop/settings` | `{key: value, …}` → jawab nilai efektif + `restart_required` |
| `GET /hop/search?q=` | `[{kind: "user"|"content"|"ip", id, label, url}]` |
| `GET /hop/metrics` | Teks Prometheus |
| `GET /hop/audit?since=&cursor=` | Aksi admin yang dicatat aplikasi (hasil panggilan Imperium + aksi lokal) |

## 5. Ekstensi opsional

Dideklarasikan di `manifest.capabilities`, dengan endpoint di bawah `/hop/x/<nama>/…`. Contoh: `storage` (PixelHop), `ai_tools` (PixelHop), `names` (NoteHop: reserved, blocked, slug holds), `files` (NoteHop), `realtime`, `integrations`, `queue` (LinkHop), `abuse_rules` (LinkHop), `kill_switch` (semua: `guest_create`, `registration`, `explore`, `captcha_force`).

## 6. Webhook dari HOP ID ke aplikasi

`POST <base_url>/hop/events`, ditandatangani sama (app = `helink`). Body `{id, type, at, data}`. Tipe: `profile.updated`, `account.status` (`active|locked|suspended|banned`), `account.deleted` (aplikasi wajib purge dan menjawab `{purged: true, counts: {...}}` secepatnya atau lewat job), `session.revoked`, `app.secret_rotated`. Retry eksponensial sampai 24 jam; aplikasi harus idempoten berdasarkan `id`.

## 7. Tiket SSO v2 (perubahan dari v1)

Tambahan klaim: `username`, `roles` (`["owner"]`, `["admin"]`, `["moderator"]`, `["app:pixelhop"]`), `profile_version`, `verified`, `mfa` (bool), `v: 2`. `aud` wajib dan harus sama dengan slug aplikasi. Aplikasi memakai `roles` untuk promosi **dan** demosi admin lokal; `admin` lama tetap dikirim selama transisi.

## 8. Aturan untuk aplikasi zero-knowledge

Endpoint konten hanya mengembalikan metadata; `preview` kosong untuk konten terenkripsi. Aksi `delete`/`takedown` tetap tersedia. Imperium menandai konten `encrypted: true` dan tidak pernah meminta isi.

## 9. Implementasi

- Paket Go `hopproto` (repo imperium): tanda tangan + verifikasi, tipe jawaban, klien, server middleware; dipakai PixelHop, NoteHop, RemoteHop, Imperium.
- Stub PHP `Hop\Signature` untuk LinkHop (Laravel), di repo linkhop.
- Vektor uji bersama di `protocol/testvectors.json` supaya dua implementasi menghasilkan tanda tangan identik.
