# HOP App Protocol v1 (draft Tahap 0)

Kontrak antara **Imperium** (pusat kendali) dan setiap aplikasi keluarga HOP (LinkHop, PixelHop, NoteHop, RemoteHop, …). Aplikasi yang memenuhi kontrak ini otomatis muncul di Imperium tanpa kode khusus di sisi Imperium. Akun tetap dimiliki **HOP ID** (hel.ink); Imperium tidak pernah membuka database aplikasi lain.

Status: draft, sisi HOP ID (hel.ink) sudah diimplementasikan di branch `hop-id` (Tahap 1). Berubah sampai Tahap 3 selesai; setelah itu versi dibekukan dan perubahan hanya lewat `X-HOP-Version` baru.

## 1. Pihak dan arah panggilan

```
HOP ID (hel.ink) ──tiket SSO v2──▶ Aplikasi      (login user, sudah ada)
HOP ID (hel.ink) ──webhook────────▶ Aplikasi      (profile.updated, account.*, session.revoked)
Imperium ──────────Admin API──────▶ Aplikasi      (baca, moderasi, setting)   ← dokumen ini
Aplikasi ──────────push───────────▶ Imperium      (error 500, alert)          POST /hop/notify, lihat bagian 10
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

## 10. Aplikasi → Imperium: `POST /hop/notify`

Satu-satunya panggilan yang arahnya dari aplikasi ke Imperium. Dipakai aplikasi untuk melaporkan HTTP 500 yang baru saja ia sajikan; Imperium mengelompokkan laporan yang sama dan menampilkannya di halaman **Notifications** (badge jumlah kelompok baru). Tipe lain (backup gagal, disk penuh) memakai amplop yang sama nanti; saat ini hanya `error.500` yang diterima.

### Alamat dan pembatasan

- `POST http://127.0.0.1:8100/hop/notify`, langsung ke loopback (alamat `IMPERIUM_ADDR`). **Bukan** lewat nginx, Tailscale atau Cloudflare: permintaan yang membawa `X-Forwarded-For`, `X-Forwarded-Host/Proto`, `X-Real-IP`, `Forwarded`, `Via`, `CF-*`, `True-Client-IP`, `X-HOP-Door`, `Cookie` atau `Authorization` ditolak `403 forbidden_door`, begitu juga sumber koneksi yang bukan loopback.
- `Content-Type: application/json` (`415` jika lain). Tanpa query string (`400`). Isi maksimum **8 KB** (`413`). Imperium memberi batas baca 5 detik untuk route ini.
- Pengirim sebaiknya memakai timeout total 2 detik (connect 1 detik), tanpa retry, mengirim **setelah** respons 500 keluar, dan menelan semua error.

### Tanda tangan

Tanda tangan `hop1` biasa (bagian 2), dibuat oleh **aplikasi pengirim** dengan **secret-nya sendiri di registry HOP ID** (secret yang sama dengan secret SSO-nya):

```
X-HOP-App:       <slug pengirim>        biohop | linkhop | pixelhop | notehop | hub
X-HOP-Timestamp: <unix detik>           selisih maksimum 120 detik
X-HOP-Nonce:     <16-128 karakter acak, sekali pakai>
X-HOP-Actor:     system                 (tidak ikut ditandatangani, tidak dipakai)
X-HOP-Signature: base64url( HMAC-SHA256( secret, "hop1|<slug>|<ts>|<nonce>|POST|/hop/notify|<sha256hex(body)>" ) )
```

- Secret `hub` adalah secret baris `imperium` di registry (secret Imperium sendiri); aplikasi lain memakai secret miliknya. Imperium menyimpan secret milik aplikasi itu di kolom `apps.own_secret` saat sinkron registry (tiap 5 menit dan tiap login). Aplikasi yang belum tersinkron, atau secret-nya kurang dari 32 karakter, ditolak `401`.
- Kesalahan tanda tangan: `401 {"ok":false,"error":"bad_signature"|"stale_request"|"replay"}`. Cache nonce ada di memori; sesudah restart, idempotensi dijaga oleh `id` laporan.

### Isi (hanya field ini, field lain ditolak `422`)

```json
{
  "id": "01JC8Z3K5Q9X7M2N4P6R8T0V1W",
  "type": "error.500",
  "at": "2026-11-02T14:03:22Z",
  "app": "linkhop",
  "env": "production",
  "data": {
    "class": "Illuminate\\Database\\QueryException",
    "message": "SQLSTATE[HY000]: General error",
    "origin": "app/Http/Controllers/LinkController.php:88",
    "route": "GET /l/{code}",
    "method": "GET",
    "ref": "7f3a9c1e2b4d",
    "version": "abc1234"
  }
}
```

| Field | Aturan |
|---|---|
| `id` | wajib, 8-64 karakter `A-Za-z0-9_-`, acak per laporan. Kunci idempotensi per aplikasi: kirim ulang `id` yang sama tidak menambah hitungan. |
| `type` | wajib, harus `error.500` |
| `at` | wajib, RFC 3339 **UTC berakhiran `Z`**, tidak lebih dari 24 jam lalu dan tidak lebih dari 5 menit ke depan |
| `app` | wajib, harus sama dengan `X-HOP-App` |
| `env` | wajib, 1-24 karakter `a-z0-9_-` |
| `data.class` | wajib, 1-160 karakter `A-Za-z0-9_.\$/-`. Untuk Go: `panic` atau `http_500`. Potong kelas anonim Laravel mulai dari `@`. |
| `data.message` | opsional, **maksimum 200 karakter**, sudah di-scrub oleh aplikasi (lihat bawah). NoteHop tidak mengirim pesan. Imperium men-scrub sekali lagi sebelum menyimpan. |
| `data.origin` | wajib: `file:baris` pertama dari kode aplikasi, **relatif**, tanpa argumen; atau `unknown`. Path absolut, `..` dan alamat IP ditolak. |
| `data.route` | wajib: **template** route, bukan URL asli (`GET /l/{code}`, `l/{code}`, `users/{id?}`; `unmatched` bila tidak ada yang cocok). Boleh diawali `METHOD `. Query string, `%`, `#`, `@`, alamat IP, dan segmen yang menyerupai token/hex panjang ditolak karena itu tanda URL asli. |
| `data.method` | wajib, `GET POST PUT PATCH DELETE HEAD OPTIONS` |
| `data.ref` | wajib, 8-32 hex huruf kecil; kode yang tampil di halaman 500 dan tertulis di log aplikasi |
| `data.version` | opsional, maksimum 40 karakter `A-Za-z0-9._+-` |

**Jangan pernah dikirim**: isi permintaan/respons, header, cookie, id sesi, token, query string, alamat IP, user agent, username, email, id pengguna, URL asli, stack trace selain `origin`. Field semacam itu membuat seluruh laporan ditolak (`422 bad_payload`, pesan menyebut nama field, tidak pernah nilainya).

Scrub `message` di sisi aplikasi: untuk `QueryException`/`PDOException` kirim hanya kode `SQLSTATE` (SQL berisi nilai ikatan, mis. email dan token); untuk pesan lain: ganti email, URL berquery, `Bearer ...`, alamat IP dan deretan 20+ karakter hex/base64 dengan `[redacted]`.

### Jawaban

- `202 {"ok":true,"data":{"group":"12","duplicate":false}}`: tersimpan (`duplicate:true` bila `id` sudah pernah diterima).
- `422 {"ok":false,"error":"bad_payload","message":"data.route: looks like a real path, send the template"}`
- `429` + `Retry-After`: batas laju terlampaui (per aplikasi 30 laporan/menit dengan burst 60; semua aplikasi 120/menit), atau inbox penuh (`inbox_full`). Laporan yang ditolak karena batas laju menambah penghitung `suppressed` pada kelompoknya, tanpa menyimpan kejadian.
- `401` / `403` / `413` / `415` seperti di atas; `500 internal` bila penyimpanan gagal (jangan di-retry).

### Pengelompokan, penyimpanan, retensi

- Sidik jari = `sha256(app | env | class | origin | route)`; sidik jari sama = satu kelompok dengan jumlah, `first_seen`, `last_seen`, dan **20 kejadian terbaru** (`ref`, waktu, versi).
- Status kelompok: `new` → `seen` (ack) → `resolved` / `muted`. Laporan baru pada kelompok `resolved` membuka lagi kelompok itu (`new`, ditandai regresi). Kelompok `muted` dan `seen` tetap dihitung tetapi tidak memicu Telegram.
- Retensi: kejadian dihapus setelah 30 hari; kelompok dihapus bila `last_seen` lebih dari 90 hari (kelompok `resolved`: 30 hari). Dijalankan tiap 5 menit di `Sweep`. Batas ukuran: 5.000 kelompok, 1.000 per aplikasi; bila penuh, kelompok non-`new` paling lama dibuang, jika tidak ada maka `429 inbox_full`.
- Telegram (bila dikonfigurasi): satu pesan per kelompok per cooldown (`IMPERIUM_NOTIFY_COOLDOWN_MINUTES`, bawaan 60) untuk kelompok baru, regresi, atau hitungan melewati 10/100/1000. Isinya hanya aplikasi, kelas, template route dan hitungan.
- Semua aksi di halaman (ack, resolve, mute dengan alasan, reopen) ditulis ke audit lokal dalam transaksi yang sama dengan perubahan status.
