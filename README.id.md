# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · **Bahasa Indonesia** · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [Français](README.fr.md) · [Русский](README.ru.md)

Panel admin dan konsol operasional untuk keluarga aplikasi HOP (LinkHop, BioHop, PixelHop, NoteHop, FileHop, PayHop). Seperti aplikasi lain dalam keluarga ini, Imperium bersifat open source dan **opsional**: setiap aplikasi tetap berjalan tanpanya. Kamu hanya membutuhkannya jika ingin satu tempat untuk mengelola pengguna, konten, laporan penyalahgunaan, pengaturan, dan kesehatan semua aplikasimu.

Yang dilakukannya:

- Memasukkan admin lewat HOP ID (hub, [HEL.ink](https://github.com/navi-crwn/HEL.ink)); hanya akun dengan peran admin yang bisa masuk, dengan login dua faktor sebagai bawaan.
- Membaca daftar aplikasi dari hub dan berkomunikasi dengan setiap aplikasi lewat API admin bertanda tangan milik aplikasi itu (HOP App Protocol di `protocol/`).
- Menampilkan kesehatan, angka, pengguna, konten, laporan penyalahgunaan, dan pengaturan per aplikasi; mengelompokkan error HTTP 500 yang dilaporkan aplikasi; mengirim peringatan Telegram opsional.

Isi repositori ini:

- `cmd/imperium/`: satu binary server Go (UI web, klien server-ke-server bertanda tangan)
- `internal/`: kode aplikasi
- `protocol/`: HOP App Protocol, kontrak API admin dan webhook yang diterapkan setiap aplikasi
- `docs/`: catatan arsitektur, daftar fitur, dan inventaris admin per aplikasi (ditulis dalam bahasa Indonesia)
- `deploy/`: unit systemd yang diperketat, contoh environment, serta berkas nginx dan tunnel Cloudflare dari setup produksi hel.ink
- `scripts/`: backup dan restore (`hop-backup.sh`, `hop-restore.sh`) serta skrip setup produksi hel.ink (`tahap*.sh`, `deploy-*.sh`)

## Menjalankan

Kamu butuh Go 1.27 atau lebih baru, dan hub yang berjalan dengan minimal satu aplikasi.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ karakter> ./bin/imperium
```

Di hub, isi `HOP_CONTROL_URL` dengan alamat Imperium dan `HOP_SSO_SECRET_IMPERIUM` dengan secret yang sama, lalu jalankan `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force` di sana: perintah ini mendaftarkan Imperium sebagai aplikasi tersembunyi yang boleh memanggil API admin. `make test` menjalankan tes.

Di server, pakai `deploy/imperium.service` (systemd, dengan user sendiri dan sistem read-only) dan berkas environment seperti `deploy/imperium.env.example`. Contoh itu berisi nilai produksi hel.ink; ganti alamatnya dengan milikmu. Jauhkan Imperium dari internet terbuka: taruh di balik VPN (Tailscale, WireGuard) atau access proxy, dan sajikan lewat HTTPS. Skrip `tahap*.sh` dan `deploy-*.sh` menyiapkan server produksi hel.ink dan hanya contoh, bukan installer.

## Hosting sendiri

<!-- self-hosting:start -->
Semua yang dibutuhkan Imperium ada di repositori publik: tanpa repositori privat, tanpa token, dan tanpa akun hel.ink. Rinciannya ada di panduan keluarga: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (bahasa Inggris).

| Komponen | Untuk Imperium | Fungsinya |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | **Wajib**: proses masuk dan daftar aplikasi | Akun dan proses masuk untuk semua aplikasi |
| [Imperium](https://github.com/navi-crwn/Imperium) | Repositori ini | Panel admin untuk seluruh keluarga; butuh hub |
| Font dan ubin keluarga | Tidak perlu dipasang: dimuat dari `hel.ink/hop-assets` | Tanpa hel.ink: font sistem, tanpa ubin melayang |

- **Urutan pemasangan**: [hub](https://github.com/navi-crwn/HEL.ink), lalu aplikasinya, lalu Imperium.
- **Pengaturan minimum**: `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; di hub: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Sendiri atau dalam keluarga**: Imperium sendiri tidak berbuat apa-apa: ia mengelola hub dan aplikasi yang terdaftar di sana. Jauhkan dari internet terbuka (VPN atau proxy akses).
<!-- self-hosting:end -->

## Bahasa

Bahasa antarmuka diatur oleh operator lewat `IMPERIUM_LANG` (lihat `deploy/imperium.env.example`). Bawaannya `en`; `IMPERIUM_LANG=id` memilih katalog bahasa Indonesia. Nilai yang tidak dikenal jatuh ke bahasa Inggris, dan `<html lang>` pada halaman selalu sesuai dengan katalog yang dipakai.

Halaman Notifikasi sudah diterjemahkan ke sembilan bahasa keluarga HOP (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). Halaman yang lebih lama sejauh ini hanya ada dalam `en` dan `id`; teks yang belum ada dalam suatu bahasa jatuh ke bahasa Inggris.

## Notifikasi (laporan HTTP 500)

Aplikasi HOP melaporkan setiap HTTP 500 yang mereka sajikan ke `POST /hop/notify` di listener loopback Imperium, ditandatangani dengan secret registry milik aplikasi itu sendiri (rincian, payload, dan batas: `protocol/PROTOCOL.md` bagian 10). Error yang identik dikelompokkan dan ditampilkan di halaman **Notifikasi** (badge menghitung kelompok yang masih baru); admin bisa menandai diketahui, menyelesaikan, membisukan (dengan alasan), atau membuka kembali sebuah kelompok, dan setiap aksi dicatat dalam audit. Kejadian disimpan 30 hari, kelompok 90 hari. Telegram menerima paling banyak satu pesan per kelompok per `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (bawaan 60), tanpa data pengguna. Database bermigrasi sendiri saat start (`schema_version`). Untuk mencobanya secara lokal: `scripts/fakehop` ditambah `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## Keluarga HOP

<!-- hop-family:start -->
<!-- Generated from HEL.ink resources/hop-assets/apps.source.json; edit there, then run scripts/hop_assets.py build. -->
| App | Address | Repository | What it does |
| --- | --- | --- | --- |
| **HOP ID** | [hel.ink](https://hel.ink) · accounts at `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | One account for every HOP app. |
| **LinkHop** | [link.hel.ink](https://link.hel.ink) · `hel.ink/l/…` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Short links, QR codes and click analytics. |
| **BioHop** | [bio.hel.ink](https://bio.hel.ink) · `hel.ink/b/…` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | One link-in-bio page for everything you do. |
| **PixelHop** | [pixel.hel.ink](https://pixel.hel.ink) · `p.hel.ink/…` | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Image hosting with albums and direct links. |
| **NoteHop** | [note.hel.ink](https://note.hel.ink) · `n.hel.ink/…` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Encrypted notes you share with a link. |
| **FileHop** | [file.hel.ink](https://file.hel.ink) | [navi-crwn/FileHop](https://github.com/navi-crwn/FileHop) | Convert files in your browser; nothing is uploaded. |
| **PayHop** | [pay.hel.ink](https://pay.hel.ink) | [navi-crwn/PayHop](https://github.com/navi-crwn/PayHop) | Make a payment QR and split a bill; nothing leaves your browser. |
| **Imperium** | not public-facing: optional, for admins | [navi-crwn/Imperium](https://github.com/navi-crwn/Imperium) | The HOP control panel. |
<!-- hop-family:end -->

## Keamanan dan lisensi

Menemukan celah keamanan? Laporkan secara privat lewat security advisories GitHub untuk repositori ini: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). Imperium dirilis di bawah [lisensi MIT](LICENSE).
