# HOP Imperium: analisis sebelum membangun

Dokumen 1 dari 3. Lihat juga `01-FITUR.md` (daftar fitur lengkap) dan `02-PLANNING.md` (urutan pengerjaan).

## 1. Situasi sekarang

| | hel.ink | PixelHop | NoteHop |
|---|---|---|---|
| Stack | Laravel 12, MariaDB | Go, SQLite | Go, SQLite |
| Akun | **pemilik akun** (password, Google, 2FA, verifikasi) | akun lokal, ditautkan via `helink_id` | akun lokal, ditautkan via `sso_id` |
| Profil | nama, email, avatar | username, nama, bio, website, foto, show_owner | username, display_name, avatar |
| Admin | panel lengkap di `/admin` | panel sendiri di `/admin` | tidak ada UI, hanya env `NOTEHOP_ADMINS` + token per catatan |
| Siapa admin | satu email di `.env` | kolom `role=admin` | daftar email di env |
| Data | 74 user, 180 link, 6 bio | 5 user, 72 gambar | (butuh sudo untuk membaca) |

Identitas sudah terpusat: `sub` di tiket SSO = `users.id` hel.ink. Yang belum terpusat: **profil** (username ada di dua aplikasi dengan aturan berbeda), **hak admin** (tiga mekanisme berbeda), dan **pengelolaan** (tiga tempat berbeda, satu di antaranya tidak ada).

## 2. Pro dan kontra

### Opsi A: Imperium = aplikasi baru yang berdiri sendiri (disarankan)

Aplikasi terpisah (Go, satu binary, DB sendiri) di subdomain rahasia. Dia tidak menyimpan user; dia **membaca dan memerintah** tiap aplikasi lewat API admin bertanda tangan. hel.ink tetap jadi server akun, tapi panel `/admin`-nya dipensiunkan.

Pro
- Satu tempat untuk semua, termasuk pengendalian server (systemd, nginx, backup, log) yang tidak pantas ditaruh di dalam aplikasi publik mana pun.
- Terpisah dari kode produk: ketiga repo tetap standalone dan open source, Imperium privat. Aplikasi baru cukup "mendaftar" ke Imperium.
- Permukaan serang kecil: bisa diikat ke Tailscale atau Cloudflare Access, tidak pernah tampil di internet publik. Kalau salah satu aplikasi publik bocor, admin tidak ikut bocor.
- Go satu binary cocok dengan PixelHop dan NoteHop; satu bahasa untuk sisi admin dan dua dari tiga aplikasi.
- Bisa dipakai untuk aplikasi di luar keluarga HOP nanti (carawinapp, nawalaku) tanpa menyentuh hel.ink.

Kontra
- Satu komponen lagi yang harus dijaga (service, backup, update).
- Setiap aksi admin harus lewat API: tiap aplikasi perlu endpoint admin yang lengkap, ini kerja terbesar (terutama NoteHop yang belum punya apa-apa).
- Dua panel lama harus dipensiunkan dengan rapi (redirect, hapus route, hapus view) supaya tidak ada dua sumber kebenaran.

### Opsi B: Imperium = perluasan panel admin hel.ink

Pro
- Panel, auth admin, 2FA, dan 74 user sudah ada di sana; paling cepat jalan.
- Tidak ada komponen baru.

Kontra
- Admin global menempel ke aplikasi publik yang paling sering diserang (shortlink = magnet spam dan phishing). Modul server/remote (restart service, baca log, nginx) di dalam Laravel yang jalan sebagai `www-data` berarti memberi `www-data` hak sudo. Itu buruk.
- hel.ink jadi gemuk: repo open source berisi kode pengelolaan VPS pribadi.
- Kalau hel.ink mati (queue, PHP-FPM, DB), admin ikut mati, padahal saat itulah admin paling dibutuhkan.
- Subdomain unik tetap bisa, tapi secara teknis hanya vhost kedua ke kode yang sama.

### Opsi C: Hibrida

Akun dan profil tetap di hel.ink (diperluas: username global, bio, website). Imperium aplikasi terpisah untuk pengelolaan. Ini sebenarnya Opsi A; dicatat di sini supaya jelas bahwa **akun tidak pindah**. Memindahkan akun keluar dari hel.ink ke "server akun" baru adalah proyek lain dan tidak dibutuhkan sekarang.

**Keputusan (disetujui pemilik 2026-10-27): Opsi A, nama Imperium.** Akun tetap di hel.ink dengan profil diperluas, Imperium berdiri sendiri, dan **semua UI admin di tiap aplikasi dihapus** (panel hel.ink, panel PixelHop, `NOTEHOP_ADMINS`). Yang tersisa di aplikasi hanya Admin API, perilaku produk berbasis peran dari tiket HOP, dan perintah CLI darurat lewat SSH.

### Hal yang harus diputuskan sadar-sadar

1. **Panel lama dipensiunkan, bukan dibiarkan.** Begitu modul Imperium untuk sebuah area selesai (misalnya "users"), route admin lama untuk area itu dihapus dan diarahkan ke Imperium. Tidak ada masa "dua-duanya hidup" lebih dari satu rilis.
2. **NoteHop zero-knowledge.** Admin tidak akan pernah bisa membaca isi catatan terenkripsi. Imperium hanya bisa melihat metadata (pemilik, ukuran, umur, mode, jumlah lampiran) dan menghapus/takedown. Catatan publik (`encrypted=false`) bisa dibaca.
3. **Profil global = hel.ink pemilik kebenaran.** Username global satu aturan untuk semua (huruf kecil, angka, titik, strip, garis bawah, 3–30), dicek ke daftar reserved gabungan. Username yang sudah ada di PixelHop dan NoteHop di-backfill ke hel.ink; kalau bentrok, yang lebih dulu dibuat menang dan yang lain diberi tahu untuk memilih ulang.
4. **Peran.** Mengganti "satu email superadmin" dengan peran: `owner` (Anda), `admin` (semua aplikasi), `moderator` (konten saja), `app:<nama>` (admin satu aplikasi). Disimpan di hel.ink, dikirim di tiket SSO dan di API.
5. **Hak server.** Modul remote (restart, log, backup, nginx) berjalan lewat helper kecil dengan sudoers yang sangat sempit (daftar perintah tetap), bukan shell bebas.

## 3. Nama subdomain

Syarat: unik, tidak bisa ditebak dari kata "admin/panel/dashboard/manage/console", tidak ada di wordlist subdomain-bruteforce umum, enak diucapkan.

Kandidat (semua `*.hel.ink`, satu sertifikat Cloudflare origin sudah menutup wildcard):

| Nama | Kesan | Catatan |
|---|---|---|
| `imperium.hel.ink` | kekaisaran, pusat kuasa | sesuai nama proyek, tapi "imperium" ada di beberapa wordlist (kecil) |
| `citadel.hel.ink` | benteng dalam kota | kuat, tapi agak umum |
| `aerie.hel.ink` | sarang elang di puncak | jarang, pendek |
| `basilica.hel.ink` | pusat yang megah | jarang |
| `throne.hel.ink` | singgasana | mudah ditebak maknanya |
| `obsidian.hel.ink` | kaca vulkanik hitam | jarang, gaya |
| `aether.hel.ink` | langit atas | jarang |
| `hop.hel.ink` | jangan: terlalu jelas |

**Saran: `imperium.hel.ink` untuk nama resmi**, tetapi nama itu sendiri **bukan** lapisan keamanan. Lapisan keamanannya:

1. **Tailscale-only** (disarankan): nginx vhost `imperium.hel.ink` hanya listen di IP Tailscale VPS (100.x), DNS record di Cloudflare mengarah ke IP Tailscale (bukan proxied). Orang luar tidak dapat apa-apa, bahkan tidak 404. Anda sudah punya tiga perangkat di tailnet.
2. Alternatif kalau perlu akses dari perangkat tanpa Tailscale: **Cloudflare Access** (Zero Trust) di depan, login Google milik Anda, baru tiba di Imperium.
3. Di dalam Imperium sendiri: login lewat HOP SSO dengan peran `owner/admin`, **wajib 2FA**, passkey (WebAuthn) sebagai opsi, sesi pendek, audit log setiap aksi, IP allowlist tambahan.

Dengan Tailscale-only, nama subdomain boleh apa saja; nama hanya soal rasa.
