# Prompt: Audit total keluarga HOP

Salin seluruh isi di bawah garis ke agent auditor. Peta sistem ditulis 2026-10-29 dari keadaan live; agent wajib memverifikasi ulang sebelum memakainya.

---

Kamu adalah **Senior Engineer sekaligus Security Auditor** untuk keluarga aplikasi HOP milik navi-crwn. Tugasmu audit total: keamanan, alur akun lintas aplikasi, kelengkapan dashboard admin, rancangan dashboard untuk app yang belum punya, rancangan homepage BioHop, standar bahasa semua app, kesehatan repo, jejak AI di semua repo GitHub, kepatuhan pada standar Structured Agentic Engineering, pemindahan catatan internal ke repo hop-ops, dan link pendek. Jawaban "kode sudah aman" tanpa bukti tidak diterima. Setiap temuan wajib menyertakan **file dan nomor baris**, perintah atau request yang kamu jalankan, dan hasil yang terlihat.

## 0. Aturan kerja: hanya membaca

Seluruh audit ini **read-only**. Pemilik akan memeriksa hasilnya dulu, lalu mengeksekusi perbaikan sendiri.

1. **Jangan mengubah apa pun di VPS ini.** Larangan ini mencakup:
   - Tidak mengedit, menambah, atau menghapus file di `/var/www`, `/etc`, `~/apps`, `/usr/local/bin`, `/var/lib`, maupun di checkout git mana pun (`~/imperium`, `~/linkhop`, `~/pixelhop`, `~/notehop`, dan lainnya).
   - Tidak `git commit`, `git checkout`, `git pull`, `git stash`, `git worktree`, atau `git fetch` di checkout yang ada.
   - Tidak restart atau reload service, tidak menjalankan migrasi, `artisan` yang menulis (cache, config, queue), `tinker` yang menulis, atau deploy.
2. **Jangan mengubah apa pun di GitHub.** Tidak push, tidak membuat PR, issue, komentar, atau label, tidak mengedit atau menghapus branch, dan tidak mengubah pengaturan repo. Pakai `gh` hanya untuk membaca (`gh repo view`, `gh pr list`, `gh api` dengan GET).
3. **Satu-satunya tempat menulis** adalah `~/audit-hop/<YYYY-MM-DD>/`, folder di luar checkout git mana pun. Laporan, skrip uji, hasil Playwright, dan klon repo semuanya disimpan di sana.
   - Untuk membaca riwayat atau menjalankan tes, klon ulang ke `~/audit-hop/<tanggal>/work/<repo>` (`git clone --mirror`, atau `git clone` dari GitHub atau dari path lokal). Jangan memakai checkout aslinya.
4. **Data produksi.** Uji yang mengubah data produksi, seperti mendaftar akun, mengedit profil, menghapus akun, unggah, atau membuat link, **hanya boleh dijalankan setelah pemilik menulis izin eksplisit di chat**. Sebelum izin itu ada:
   - Jalankan skenario di lingkungan lokal: tes app, `tests/e2e/serve.sh` LinkHop, atau `scripts/fakehop` Imperium, dari klon di folder audit.
   - Siapkan skrip Playwright dan daftar langkah manual agar pemilik bisa menjalankannya di produksi.
   - Uji produksi tanpa izin hanya boleh berupa GET anonim (status, header, redirect).
   - Bila izin diberikan, pakai akun uji `audit+<n>@hel.ink`, jangan akun pemilik (`ivan_smb787` atau `superadmin`), dan hapus akun uji di akhir. Jangan mengakali Turnstile atau Google. Tulis langkah manual bila keduanya menghalangi.
5. **Rahasia.** Jangan pernah menampilkan isi secret, token, password, atau cookie. Cukup sebut nama variabelnya dan apakah terisi. Jangan meminta password ke pemilik di chat.
6. **sudo** hanya untuk membaca, lewat `~/imperium/scripts/s <perintah>` (tercatat di `~/imperium/backups/sudo-log.txt`). Contohnya `cat`, `grep`, `journalctl`, `sshd -T`, `nginx -T`. Artisan baca-saja dijalankan sebagai www-data dengan `php8.3`. Untuk tinker baca-saja: `env XDG_CONFIG_HOME=/tmp/psysh-www HOME=/tmp/psysh-www`.
7. **Bahasa dan jejak AI.** Tulis laporan dalam Bahasa Indonesia. Pemilik ingin **semua repo bebas jejak AI**, jadi rekomendasi perbaikanmu tidak boleh menyarankan menambah trailer AI, footer "Generated with", atau file instruksi agent ke repo.
8. **Jangan menyentuh** situs lain di VPS yang sama (carawinapp, nawalaku, router-bot), kecuali untuk mencatat repo GitHub-nya di bagian Jejak AI.

## 1. Peta sistem (verifikasi dulu)

Semua aplikasi berjalan di satu VPS. Semua domain publik lewat Cloudflare (proxied), dengan origin nginx.

### Aplikasi dalam cakupan

| Komponen | Domain | Lokasi live | Repo GitHub | Stack |
|---|---|---|---|---|
| Hub / HOP ID (penyedia akun, SSO, beranda publik) | `hub.hel.ink`, beranda di `hel.ink` | `/var/www/helink/hub` | navi-crwn/HEL.ink | Laravel 12, php8.3-fpm |
| LinkHop (shortener) | `link.hel.ink`, link pendek `hel.ink/l/{slug}` | `/var/www/helink/linkhop` | navi-crwn/LinkHop | Laravel 12 |
| BioHop (link-in-bio) | `bio.hel.ink`, halaman bio `hel.ink/b/{slug}` | `/var/www/helink/biohop` | navi-crwn/BioHop (private) | Laravel 12 |
| PixelHop (hosting gambar/video/PDF) | `p.hel.ink` (rencana: app di `pixel.hel.ink`, link tetap `p.hel.ink`) | binary `~/apps/pixelhop/pixelhop`, env `~/apps/pixelhop/pixelhop.env`, service `pixelhop` :8080 | navi-crwn/PixelHop | Go + SQLite |
| NoteHop (catatan terenkripsi zero-knowledge) | `note.hel.ink` (rencana: domain share pendek, mis. `n.hel.ink`) | `/usr/local/bin/notehop`, env `/etc/notehop/notehop.env`, data `/var/lib/notehop`, service `notehop` :8090, user `notehop` | navi-crwn/NoteHop | Go + SQLite |
| Imperium (dashboard admin global) | `imperium.hel.ink` (hanya Tailscale), `citadel.hel.ink` (Cloudflare Tunnel + Access) | `/usr/local/bin/imperium`, env `/etc/imperium/imperium.env`, DB `/var/lib/imperium/imperium.db`, service `imperium` :8100 | navi-crwn/Imperium (private) | Go + SQLite |
| Paket bersama Laravel | dipakai hub, LinkHop, BioHop | `vendor/` masing-masing | navi-crwn/hop-auth, hop-ui, hop-safety | Composer |

Repo lain yang relevan: navi-crwn/helink-ops (catatan ops, private). RemoteHop belum jadi dan berada di luar cakupan audit fungsional, tetapi tetap masuk audit jejak AI.

### Fakta penting

- **Database.** Hub, LinkHop, dan BioHop berbagi satu database MariaDB `helink_db`. PixelHop dan NoteHop punya SQLite sendiri. Identitas pusat: `sub` = `users.id` di hub. PixelHop menyimpannya di `users.helink_id`, NoteHop di `users.sso_id`.
- **Routing nginx.** Konfigurasi ada di `/etc/nginx/sites-enabled/hel.ink`, `p.hel.ink`, `note.hel.ink`, `imperium.hel.ink`, ditambah `snippets/hop-laravel.conf`.
  - Di `hel.ink`: `/l/` ke index.php LinkHop, `/b/` ke index.php BioHop, `/api/` dialihkan 308 ke link.hel.ink (sementara sampai 27 Nov 2026), sisanya ke hub dengan pengalihan slug lama `$hop_legacy_redirect`.
  - Di `hub.hel.ink`: `/api/hop/(manifest|content|reports|x/|settings)` dilayani **LinkHop**. Ini API moderasi untuk link, halaman bio, laporan, daftar IP dan domain, serta pengaturan. Sisa `/api/hop/*` (users, stats, apps) dilayani hub.
- **SSO.** Aplikasi mengarahkan ke hub `/sso/{client}?state=&return_to=`. Hub mengembalikan tiket HMAC berumur 120 detik dengan `aud`, roles, status, dan mfa. Daftar klien ada di **dua tempat**:
  - Konfigurasi `config/hop.php` di hub (`apps`: linkhop, biohop, pixelhop, notehop, beserta callback, logout, dan secret).
  - Tabel `hop_apps` (registry: citadel, imperium, notehop, pixelhop).

  Periksa apakah keduanya konsisten dan secret mana yang dipakai tiap klien. Contoh: `HOP_SSO_SECRET_LINKHOP` dan `HOP_SSO_SECRET_BIOHOP` tampaknya tidak diset, sehingga ada kemungkinan jatuh ke secret bersama.
- **HOP App Protocol v1** (server ke server):
  - Header: `X-HOP-App`, `X-HOP-Timestamp`, `X-HOP-Nonce`, `X-HOP-Actor`, `X-HOP-Signature`.
  - String yang ditandatangani: `hop1|app|ts|nonce|METHOD|PATH|sha256hex(body)`, HMAC-SHA256, base64url tanpa padding.
  - Endpoint: hub dan LinkHop di `/api/hop/*`; PixelHop dan NoteHop di `/hop/*` (health, manifest, users, content, reports, settings, ekstensi `x/*`).
  - Imperium memanggil semuanya dan menyinkronkan registry setiap 5 menit.
- **Google OAuth.** Hub memakai Laravel Socialite (`GOOGLE_CLIENT_ID` terisi). **PixelHop juga punya kredensial Google sendiri** (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI` di pixelhop.env). Petakan apakah login Google di PixelHop melewati hub atau langsung, dan apa akibatnya terhadap identitas tunggal.
- **Logout berantai.** Hub memiliki `registry_logout` dan path `logout` per app. LinkHop dan BioHop memakai `/auth/sso/logout`. PixelHop (PR #6, `GET /auth/helink/logout`) dan NoteHop (PR #12, `GET /auth/sso/logout`) sudah ter-merge di main.
- **Imperium.**
  - Login lewat SSO hub. Peran yang diterima: `owner`, `admin`, `moderator`, `support`, dan `admin:*`. 2FA wajib, sesi 8 jam.
  - Alias tampilan `IMPERIUM_ALIASES` menyamarkan identitas pemilik.
  - Sidebar berisi folder Aplikasi. Dashboard per app ada di `/apps/{slug}`, dengan modul dari manifest tiap app.
  - CSP ketat (`style-src 'self'`, tanpa inline style). Telegram alert opsional.
  - **BioHop belum punya dashboard sendiri di Imperium dan belum punya admin API.** Halaman bio hanya muncul sebagai tipe konten di modul LinkHop. BioHop masih punya halaman admin internal sendiri (`resources/views/admin`, route `admin.*` di `routes/web.php`), padahal app lain sudah memensiunkannya demi Imperium.
- **Homepage dan bahasa.**
  - BioHop belum punya homepage: `routes/web.php:26` langsung mengalihkan ke dashboard atau `/auth/sso`.
  - Saudaranya sudah punya: beranda hub (`hub/resources/views/home.blade.php`, `marketing/`), landing LinkHop di link.hel.ink, serta landing PixelHop dan NoteHop.
  - Aset 3D keluarga HOP berupa tile mengambang: komponen `hop-float` dan `resources/js/hop-float.js` di LinkHop, dan `static/hop-float.js` di Imperium.
  - NoteHop sudah memakai 9 bahasa (`web/i18n/`: id, en, zh, es, ar, pt, fr, ru, ja). Imperium hanya id dan en. Hub, LinkHop, dan BioHop memakai `APP_LOCALE=en` tanpa folder `lang/`. PixelHop belum diketahui.
- **Cron dan backup.** Backup PixelHop harian, healthcheck PixelHop tiap 5 menit, `hop-backup.sh` (restic) harian, berkas `/etc/cron.d/hop*`, dan watchdog hub (`config('hop.watch')`).

### Keputusan pemilik (2026-10-29)

1. **Admin BioHop pindah ke Imperium.** Halaman `/admin/bio` di BioHop dipensiunkan setelah dashboard `/apps/biohop` siap.
2. **Dashboard pengguna BioHop digabung ke hub.** Hub sudah punya dashboard akun di `hub.hel.ink/dashboard` (`LauncherController`). Saat ini pengelolaan halaman bio pengguna masih ada di `bio.hel.ink` (route `bio.index`). Rancangannya ada di bagian 3b.
3. **BioHop wajib punya homepage** yang semirip mungkin dengan saudaranya, dengan aset 3D. Lihat bagian 3c.
4. **Bahasa:** Indonesia ditambah 7 bahasa populer, wajib di semua app. Lihat bagian 3c.
5. **Standar Structured Agentic Engineering wajib untuk semua app.** Lihat bagian 6.
6. **Semua repo bebas jejak AI.** Catatan internal (jejak AI, tujuan, rencana, dan pekerjaan yang belum selesai) dipindahkan ke satu repo ops terpisah, `hop-ops`. Lihat bagian 5 dan 7.

### Petunjuk awal yang harus kamu konfirmasi atau bantah

1. PixelHop live tertinggal dari main. Binary melapor `pixelhop 3.6.0`, sedangkan main (30c9375) sudah 3.7.0 dengan logout berantai (#6) dan pemindahan ke pixel.hel.ink.
2. NoteHop live tertinggal dari main. Live 6aa89f3 (#9), sedangkan main 6dd906c berisi #10 (link share pendek), #11 (catatan di `/name`), dan #12 (logout berantai).
3. Hub belum mengisi `PIXELHOP_LOGOUT_PATH` dan `NOTEHOP_LOGOUT_PATH`. Akibatnya logout di hub kemungkinan tidak ikut mengeluarkan sesi PixelHop dan NoteHop.
4. `pixel.hel.ink` dan `n.hel.ink` sudah punya DNS di Cloudflare, tetapi belum ada vhost nginx untuk keduanya. Request jatuh ke `00-default`.
5. Registry `hop_apps` tidak memuat linkhop dan biohop. `registry_repos` PixelHop bernilai `null`, padahal repo-nya publik.
6. `NOTEHOP_LINKS` masih menunjuk `LinkHop=https://hel.ink` dan `PixelHop=https://p.hel.ink`. Control URL Imperium dan PixelHop menunjuk citadel.hel.ink.
7. Direktori lama `/var/www/helink/shortlink-app` (a371f72) masih ada. Pastikan tidak ada yang melayaninya lagi dan tidak ada `.env` berisi secret yang terbuka, lalu catat apakah boleh diarsipkan.
8. Pintasan TLS lokal (`HOP_API_ADDR`, `HELINK_API_ADDR`) dimatikan karena sertifikat origin dulu tidak punya SAN. nginx kini memakai `hel.ink-origin.crt`. Periksa SAN-nya dan apakah pintasan itu aman diaktifkan lagi.

## 2. Skenario alur akun (inti audit)

Untuk setiap skenario, isi matriks **app × hasil** dengan kolom hub, LinkHop, BioHop, PixelHop, NoteHop, dan Imperium. Nilai tiap sel: lulus, gagal, atau tidak berlaku. Sertakan bukti: request, status, cookie yang terlihat, baris log, dan file:baris penyebab.

Ingat aturan 0.4. Skenario yang menulis data dijalankan di lingkungan lokal. Untuk produksi, siapkan skripnya sampai pemilik memberi izin.

**A. Login dengan HOP ID dari tiap app.** Mulai dari tombol "Login with HOP" di setiap app, dalam keadaan belum login di mana pun. Periksa:
- `state` diverifikasi.
- Tiket ditolak bila kedaluwarsa, dipakai ulang (replay `jti`), `aud`-nya salah, atau tanda tangannya salah.
- `return_to` dan `next` tidak bisa dipakai untuk open redirect.
- Akun berstatus suspended atau banned ditolak.
- Akun di sisi app dibuat atau ditautkan dengan benar (`helink_id`, `sso_id`).
- Halaman pendaftaran (Terms, Turnstile) berfungsi.

**B. Login dengan Google OAuth dari tiap app** yang menawarkannya (hub, PixelHop, dan app lain bila ada). Periksa:
- Email Google yang sudah terdaftar ditautkan ke akun HOP ID yang sama, bukan membuat akun ganda. Uji juga email yang belum terverifikasi.
- PixelHop memakai Google langsung atau lewat hub, dan apakah `sub` tetap sama.
- `redirect_uri` sesuai domain baru (link.hel.ink, bio.hel.ink, pixel.hel.ink).
- Ada proteksi `state`/CSRF untuk OAuth.

**C. Sekali login, semua masuk (SSO).** Setelah login di satu app, buka app lain. Harapannya tidak perlu mengisi kredensial lagi, cukup satu klik atau otomatis sesuai desain. Catat perilaku tiap pasangan app. Periksa juga atribut cookie sesi di setiap domain (`Secure`, `HttpOnly`, `SameSite`, `Domain`) dan apakah `SESSION_DOMAIN=null` memang disengaja.

**D. Logout dari mana pun, semua keluar.**
- Logout dari tiap app satu per satu (lima titik awal), lalu periksa sesi di semua app lain dan di Imperium.
- Telusuri rantai redirect logout (`form-action` CSP, `GET /logout`). Pastikan tidak ada yang macet atau berakhir di halaman error.
- Uji juga logout saat salah satu app sedang mati.

**E. Edit profil.** Ubah username, nama, avatar, bio, dan email di hub. Periksa:
- Apakah perubahan sampai ke PixelHop, NoteHop, LinkHop, BioHop, dan Imperium, dan berapa lama.
- Event `SendHopEvent`, cache profil, dan aturan `username_hold_days`.
- Username yang dicadangkan (`reserved_usernames`) dan username yang bentrok.

**F. Hapus akun.** Hapus akun uji dari hub, lalu periksa satu per satu:
- Data di `helink_db`: links, bio_pages, klik, sesi, token, dan roles.
- PixelHop: file di disk atau S3, album, dan baris SQLite.
- NoteHop: catatan, vault, dan link share.
- Sisa sesi yang masih aktif.
- Data di Imperium: audit tetap ada, tetapi tanpa PII berlebih.
- Backup restic: dokumentasikan retensinya.

Bandingkan hasilnya dengan janji di halaman privasi dan Terms. Uji juga ekspor data (`export`) sebelum akun dihapus.

**G. Pakai semua app dengan satu akun.** Jalankan aksi utama tiap app dengan akun uji yang sama:
- LinkHop: buat link, QR, dan lihat analytics.
- BioHop: buat halaman bio, pilih tema, uji klik.
- PixelHop: unggah, album, link langsung, ShareX dan API.
- NoteHop: buat catatan, bagikan, kolaborasi live.

Periksa juga kuota dan role per app, serta navigasi lintas app (hop-ui).

**H. Akses admin.** Periksa:
- Pengguna biasa tidak bisa membuka `/admin` lama, endpoint `/hop/*`, atau `/api/hop/*` tanpa tanda tangan.
- Imperium menolak akun tanpa peran atau tanpa 2FA, lewat **kedua pintu** (Tailscale dan Citadel).
- Peran `support` hanya bisa membaca.
- Aksi Imperium (ban, suspend, hapus konten, settings) benar-benar terjadi di app dan tercatat di audit kedua sisi. Di produksi, cek ini lewat log dan kode saja.

## 3. Dashboard Imperium

### 3a. Kelengkapan dashboard yang sudah ada

Untuk LinkHop, PixelHop, NoteHop, dan hub (Pengguna), buat tabel **fitur aplikasi × tersedia di Imperium** dengan nilai ya, sebagian, atau tidak. Sumber daftar fitur:
- Kode app (routes, controller, handler).
- Panel admin lama yang sudah dipensiunkan: LinkHop sebelum PR #37 (`resources/views/admin*`, `app/Http/Controllers/Admin*`) dan PixelHop sebelum PR #4.
- Inventaris di `~/imperium/docs/10-12-INVENTARIS-*.md` dan daftar fitur di `docs/01-FITUR.md`.

Cakup:
- Pengguna dan User 360
- Konten
- Laporan abuse
- IP ban
- Domain
- Antrean
- Pengaturan
- Analytics (negara, perangkat, top links)
- Kuota
- Metrik

Tandai juga modul Imperium yang tampil tetapi error, kosong, atau masih memakai label i18n mentah.

### 3b. Rancangan dashboard baru untuk app yang belum punya (BioHop, dan app lain bila ditemukan)

Jangan membangunnya. Tulis **rancangan siap eksekusi** di `DASHBOARD-<app>.md` agar pemilik bisa memeriksanya dulu. Isinya:

1. **Inventaris fitur BioHop** dari kode `/var/www/helink/biohop` (baca saja): halaman bio, link di dalamnya, 20+ tema, upload, gerbang password dan umur, klik dan analytics, ekspor, serta pengaturan. Untuk tiap fitur, catat aksi admin yang dibutuhkan (lihat, sembunyikan, hapus, kembalikan, kunci, reset tema, hapus upload, tangani laporan).
2. **Keadaan sekarang.** Apa yang sudah bisa dilakukan untuk bio lewat API konten LinkHop di `hub.hel.ink/api/hop/*`, dan apa yang belum.
3. **Keputusan arsitektur.** Pilih salah satu opsi dan beri alasan:
   - BioHop punya admin API sendiri (`bio.hel.ink/api/hop/*` atau `/hop/*`) dengan manifest sendiri dan baris registry `biohop`.
   - Tetap menumpang LinkHop.

   Pertimbangkan database bersama, rencana BioHop berdiri sendiri, dan pola PixelHop dan NoteHop.
4. **Kontrak API.** Daftar endpoint, bentuk JSON (`manifest`: content types, actions, settings_schema, extensions; `stats`: KPI dan seri 14 hari; `content`; `reports`; `settings`), serta capability registry. Semua harus konsisten dengan HOP App Protocol v1 dan cara Imperium membaca manifest (`internal/app/content.go`, `nav.go`, `dashboard.html`).
5. **Isi dashboard `/apps/biohop` di Imperium.**
   - Kartu kesehatan dan versi.
   - KPI: halaman aktif, halaman baru 7 hari, klik hari ini, laporan terbuka, penyimpanan upload.
   - Seri 14 hari.
   - Halaman terbaru.
   - Laporan terbuka beserta aksinya.
   - Daftar tema populer.
   - Ekstensi bila perlu, misalnya daftar domain terlarang di link bio.

   Sertakan sketsa tata letak dalam teks atau ASCII, mengikuti gaya dashboard LinkHop, PixelHop, dan NoteHop yang sudah ada (lihat `~/imperium/docs/img-dash-*.png`).
6. **Rencana kerja per repo.** Urutkan berkas yang perlu dibuat atau diubah di BioHop, Imperium (bila ada yang tidak generik), dan hub (registry, nginx). Sertakan tes yang dibutuhkan dan urutan deploy. Pecah menjadi PR kecil yang bisa dieksekusi pemilik atau agent lain.

7. **Dashboard pengguna BioHop ke hub.**
   - Inventaris semua halaman pengguna di BioHop: daftar halaman bio, editor, tema, analytics, pengaturan, dan ekspor.
   - Tentukan mana yang pindah ke `hub.hel.ink/dashboard` dan mana yang tetap di `bio.hel.ink` sebagai alat yang dibuka dari hub. Beri rekomendasi beserta alasannya. Contoh: ringkasan dan daftar halaman pindah ke hub, sedangkan editor visual tetap di BioHop.
   - Bandingkan dengan cara LinkHop dan app lain tampil di launcher hub, supaya polanya seragam.
   - Database sudah sama (`helink_db`). Catat model, policy, dan route yang perlu dibagi lewat paket bersama, dan risiko dua app menulis ke tabel yang sama.
   - Tulis rencana pengalihan URL lama (`bio.hel.ink/dashboard`, `bio.index`), dampaknya pada SSO dan logout, serta urutan PR.
8. **Pensiunkan admin internal BioHop.** Hapus `resources/views/admin` dan route `admin.*` setelah dashboard Imperium siap, dengan pola yang sama seperti LinkHop PR #37 dan PixelHop PR #4.

Lakukan hal yang sama untuk app lain yang ternyata belum punya dashboard.

### 3c. Homepage BioHop dan standar bahasa untuk semua app

Bagian ini juga berupa rancangan, belum dibangun.

**Homepage BioHop (`HOMEPAGE-BIOHOP.md`).**

1. **Inventaris gaya saudara.** Ambil tangkapan layar homepage hub, LinkHop, PixelHop, dan NoteHop dengan Playwright, lewat GET anonim, dalam mode terang, gelap, dan ponsel. Simpan di folder audit. Catat elemen yang sama di semua homepage:
   - Header dan navigasi lintas app (hop-ui).
   - Hero dengan judul dan tagline.
   - Aset 3D (tile HOP mengambang).
   - Bagian fitur, CTA "Login with HOP", dan footer berisi tautan keluarga.
   - Tipografi, warna, radius, dan mode gelap.
2. **Rancangan homepage BioHop yang mirip saudaranya.**
   - Hero berisi tagline "One link-in-bio page for everything you do".
   - **Aset 3D wajib.** Usulkan bentuknya, misalnya tile BioHop sebagai pusat dengan tile saudara mengambang, atau mockup ponsel berisi halaman bio di antara tile 3D. Gunakan ulang `hop-float` dan pertimbangkan memindahkannya ke paket hop-ui agar semua app memakai satu sumber.
   - Bagian fitur: tema, analytics, gerbang password dan umur, domain `hel.ink/b/…`.
   - Contoh halaman bio, CTA, dan footer.
   - Perubahan route: `/` di `bio.hel.ink` menampilkan homepage untuk semua orang. Pengguna yang sudah login mendapat tombol ke dashboard di hub, karena dashboard pengguna pindah ke hub. Bila ada pilihan lain yang lebih baik, usulkan dan bandingkan.
   - Sketsa tata letak dalam teks.
3. **Batasan teknis.** CSP tanpa inline style dan script, `prefers-reduced-motion`, jeda saat tab tersembunyi, performa (LCP, CLS), aksesibilitas (aset 3D `aria-hidden`), serta meta SEO dan OpenGraph per bahasa.
4. **Rencana kerja.** Daftar berkas yang dibuat atau diubah di BioHop dan hop-ui, tes, dan urutan PR.

**Standar bahasa untuk semua app (`BAHASA.md`).** Pemilik mewajibkan **Bahasa Indonesia ditambah 7 bahasa populer** di semua app (hub, LinkHop, BioHop, PixelHop, NoteHop, Imperium).

1. **Usulan set standar.** Pakai set NoteHop yang sudah jalan: id, en, zh, es, ar, pt, fr, ru, ja. Set ini berisi Indonesia ditambah 8 bahasa, satu lebih banyak dari permintaan. Bila pemilik ingin tepat 8, usulkan satu bahasa yang dibuang beserta alasannya.
2. **Audit per app.** Buat tabel app × bahasa. Untuk tiap app, catat:
   - Cara memilih bahasa: `Accept-Language`, pemilih bahasa, cookie, atau preferensi di profil HOP ID.
   - Jumlah string yang belum diterjemahkan atau masih tertanam di kode.
   - Dukungan RTL untuk Arab.
   - Format tanggal dan angka.
   - Email dan pesan error yang ikut diterjemahkan.
   - Tag `lang` dan `hreflang`.
3. **Rencana penyeragaman.**
   - Satu sumber kebenaran untuk preferensi bahasa: disimpan di profil HOP ID dan dibawa lewat tiket SSO atau event ke semua app.
   - Mekanisme per stack: file `lang/` Laravel, terjemahan bersama di hop-ui, JSON seperti NoteHop untuk app Go, dan katalog Imperium.
   - Alur kerja penerjemahan dan pemeriksaan kunci yang hilang di CI.
   - Urutan pengerjaan per app.

## 4. Repo GitHub

Untuk setiap repo dalam cakupan (HEL.ink, LinkHop, BioHop, PixelHop, NoteHop, Imperium, hop-auth, hop-ui, hop-safety), periksa:

- **Live dan main.** Bandingkan commit live di server dengan `origin/main`. Baca `.git` di checkout live **tanpa fetch**; gunakan klon di folder audit untuk mendapatkan main terbaru. Catat juga PR terbuka atau draft yang tertinggal.
- **Identitas repo.** README, deskripsi repo, homepage, topik, dan tangkapan layar sesuai produk dan domain sekarang. Contoh: deskripsi LinkHop masih menyebut "Link in Bio" padahal fitur itu sudah pindah ke BioHop. HEL.ink dan paket `hop-*` belum punya deskripsi.
- **Dokumen pendukung.** `.env.example` lengkap, LICENSE, SECURITY.md, dan visibilitas (public atau private) sesuai niat.
- **CI dan pengaman.** CI ada dan hijau di main. Branch protection, Dependabot, dan secret scanning aktif.
- **Secret di riwayat.** Pastikan tidak ada secret di riwayat git. Jalankan `gitleaks` atau `trufflehog` pada klon bila tersedia, tanpa mencetak nilainya.
- **Paket bersama.** Versi `hop-*` yang terpasang di tiap app sama dan tidak tertinggal.

## 5. Jejak AI di semua repo GitHub (hanya mencatat)

Pemilik ingin **semua repo bebas jejak AI** dari agent mana pun: Claude atau Claude Code, Gemini atau Gemini Code Assist, GitHub Copilot, Cursor, Codex atau OpenAI, Devin, Aider, Windsurf, Continue, dan sejenisnya. Tugasmu **hanya mencatat dan menulis cara singkat menghapusnya**. Pemilik yang akan mengeksekusi. Jangan menghapus, mengedit, atau force-push apa pun.

**Cakupan:** semua repo milik navi-crwn, bukan hanya keluarga HOP. Ambil daftarnya dengan perintah ini, lalu buat `git clone --mirror` masing-masing ke `~/audit-hop/<tanggal>/mirror/`:

```bash
gh repo list navi-crwn --limit 200 --json name,visibility,isArchived
```

**Yang dicari:**

1. **Trailer dan pesan commit.**
   - `Co-Authored-By` yang menyebut claude, anthropic, gemini, google-labs, copilot, cursor, codex, openai, devin, atau aider.
   - Kalimat seperti "Generated with", "🤖", "Claude Code", "AI-generated" di pesan commit.
   - Perintah:
     ```bash
     git log --all --format='%H%x09%s%x09%(trailers:key=Co-authored-by,valueonly,separator=%x2C)'
     ```
     lalu disaring dengan `grep -iE`.
2. **Author dan committer.** Email seperti `noreply@anthropic.com`, akun `*[bot]` milik layanan AI (`copilot-swe-agent[bot]`, `gemini-code-assist[bot]`, `claude[bot]`, dan sejenisnya), serta nama seperti "Claude". Perintah: `git log --all --format='%an <%ae>|%cn <%ce>' | sort | uniq -c`.
3. **Nama branch.** `claude/*`, `copilot/*`, `codex/*`, `gemini/*`, `cursor/*`, `devin/*`, baik yang masih ada di remote maupun yang tercatat sebagai `headRefName` di PR lama.
4. **PR, issue, dan review.**
   - Judul dan body yang memuat footer "🤖 Generated with [Claude Code]" atau sejenisnya.
   - Komentar dan review dari bot AI.
   - Ringkasan otomatis dari Gemini Code Assist atau Copilot.
   - Perintah: `gh pr list --state all --limit 1000 --json number,title,body,headRefName,author`, `gh api repos/navi-crwn/<repo>/issues/comments --paginate`, `gh api repos/navi-crwn/<repo>/pulls/comments --paginate`, dan review per PR.
5. **File di HEAD dan di riwayat.**
   - File instruksi agent: `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `.claude/`, `.gemini/`, `.cursor/`, `.cursorrules`, `.windsurfrules`, `.github/copilot-instructions.md`, `.aider*`, `.codex/`, `.continue/`, `.mcp.json`, serta dokumen prompt seperti dokumen ini.
   - Perintah untuk riwayat nama file: `git log --all --name-only --format= | sort -u | grep -iE '<pola>'`.
   - Isi file atau komentar kode yang menyebut nama agent: `git grep -iE` di HEAD.
6. **Workflow dan integrasi.** Workflow yang memakai `anthropics/claude-code-action`, `google-github-actions/run-gemini-cli`, atau action AI lain. GitHub App yang terpasang (Claude, Gemini Code Assist, Copilot code review). Gunakan `gh api` bila izinnya cukup; bila tidak, catat sebagai langkah manual di Settings → Integrations.
7. **Tempat lain.** Release notes dan tag, wiki, branch `gh-pages`, badge README, dan grafik Contributors. Co-Authored-By membuat agent tampil sebagai kontributor.

**Keluaran `JEJAK-AI.md`:**
- Tabel per repo dengan kolom: jenis jejak, jumlah, contoh (SHA pendek, nomor PR, atau path), dan tindakan yang disarankan.
- Ringkasan total.
- Bagian "Cara menghapus" yang singkat dan bisa langsung dieksekusi pemilik. Sertakan setidaknya poin berikut:
  1. **File di HEAD.** Hapus lewat commit biasa. Bila pemilik ingin tetap memakainya secara lokal, masukkan ke `.git/info/exclude` atau gitignore global, bukan ke `.gitignore` repo.
  2. **Riwayat commit.** Jalankan `git filter-repo` pada klon `--mirror` yang baru:
     - `--message-callback` untuk membuang baris trailer dan footer AI.
     - `--mailmap` untuk author dan committer AI.
     - `--path <file> --invert-paths` untuk file instruksi agent.

     Setelah itu, `git push --force --all` dan `git push --force --tags`. Jangan memakai `--mirror`, karena GitHub menolak `refs/pull/*`. Tulis contoh perintah lengkap yang sudah kamu uji pada **salinan** mirror di folder audit, tanpa push.
  3. **Dampak penulisan ulang riwayat.** Semua SHA berubah. Akibatnya:
     - Checkout live (`/var/www/helink/*`, `~/pixelhop`, `~/notehop/repo`, `~/imperium`) perlu di-reset ke main baru.
     - PR terbuka rusak.
     - Skrip deploy dan string versi yang memakai SHA ikut berubah.
     - GitHub tetap menyimpan commit lama lewat `refs/pull/*` di halaman PR. Untuk benar-benar hilang, perlu permintaan ke GitHub Support.
     - Grafik Contributors perlu waktu untuk dihitung ulang.

     Usulkan urutan yang aman: backup, satu repo uji dulu, lalu sisanya.
  4. **PR dan komentar.**
     - Body PR: `gh pr edit <n> --body-file <file>`.
     - Komentar bot: `gh api -X DELETE repos/<o>/<r>/issues/comments/<id>` dan `.../pulls/comments/<id>`.
     - Branch sisa: `git push origin --delete <branch>`.
     - Nama branch PR yang sudah di-merge tidak bisa diubah. Catat ini sebagai batasan.
  5. **Integrasi.** Copot GitHub App AI, hapus atau nonaktifkan workflow AI, dan matikan Copilot code review di pengaturan repo.
  6. **Pencegahan.** Matikan atribusi otomatis di agent yang dipakai (pengaturan attribution atau `includeCoAuthoredBy` di settings Claude Code, opsi sejenis di agent lain). Tambahkan hook `commit-msg` lokal yang menolak trailer AI. Gitignore global untuk file instruksi agent.
- **Prioritas.** Repo publik lebih dulu, karena jejaknya terlihat orang lain.

## 6. Standar Structured Agentic Engineering (wajib untuk semua app)

Pemilik mewajibkan standar ini untuk semua app. Prinsipnya: manusia menetapkan arsitektur, batas keamanan, dan kontrak data secara deterministik, sedangkan AI hanya mengisi implementasi dalam potongan kecil yang terisolasi dan terverifikasi.

Risiko yang ingin dicegah, beserta penangkalnya:

| Risiko | Gejala | Penangkal |
|---|---|---|
| Celah OWASP Top 10 | SQLi, XSS, autentikasi lemah | Validasi input ketat (struct atau FormRequest yang strict), ORM berparameter, sesi Auditor terpisah |
| Kredensial terbuka | API key atau token di kode | Pemindai rahasia di pre-commit dan CI, rahasia hanya dari env |
| Dependency berbahaya atau halusinasi | Paket tak teraudit, slopsquatting | Dependency dibekukan, paket baru hanya dengan audit manual |
| Arsitektur rapuh | Logika putus di tengah | Contract-first (`ARCHITECTURE.md`, kontrak API, skema), vertical slice |
| Looping regression | Satu perbaikan merusak fitur lain | Konteks sempit per fitur, tes otomatis, kembali ke commit stabil |
| Debugging buntu | Kode spaghetti tanpa dokumentasi | Strict typing, linter ketat, kontrak terdokumentasi per modul |
| Keahlian tumpul | Developer tidak paham kodenya | 1 tugas = 1 commit, review manual setiap diff |
| Proyek mangkrak saat kuota habis | Konteks hilang bersama tool | Spesifikasi dan daftar tugas tertulis, bisa dilanjutkan dengan tool lain atau manual |

**Untuk setiap repo, nilai kepatuhan (ada, sebagian, atau tidak) beserta bukti:**

1. **Contract-first.** Periksa:
   - Ada dokumen arsitektur dan kontrak yang netral (`ARCHITECTURE.md`, spesifikasi API, skema database).
   - Kontrak HOP App Protocol terdokumentasi di satu tempat dan dipakai sama oleh semua app.
   - Strict typing: `declare(strict_types=1)`, level PHPStan/Larastan, tipe Go tanpa `any` atau `interface{}` yang tidak perlu.
2. **Vertical slice.** Fitur terisolasi dengan batas input dan output yang jelas. Catat modul yang saling bergantung, file di atas 300 baris, dan "god class".
3. **Micro-checkpoint.** Analisis riwayat git:
   - Ukuran commit (baris per commit) dan commit yang mencampur banyak urusan.
   - Pola looping regression: rangkaian commit "fix" berulang pada file yang sama, revert, atau perubahan bolak-balik.
4. **Gerbang deterministik.** Periksa:
   - Pre-commit hook (gitleaks atau trufflehog, format, lint).
   - CI dengan SAST: `gosec`, `staticcheck`, `govulncheck` untuk Go; Larastan, `composer audit`, dan semgrep untuk Laravel.
   - Branch protection dengan required checks, sehingga merge ditolak bila gerbang merah.
5. **Pemisahan Planner, Worker, dan Auditor.** Periksa apakah PR mendapat review terpisah (diff saja) sebelum merge. Hitung PR yang di-merge tanpa review dan PR besar yang di-merge beberapa menit setelah dibuka.
6. **Kunci dependency.** Periksa:
   - Lockfile ter-commit.
   - Penambahan paket baru dalam 90 hari terakhir: siapa yang menambah, di PR mana, dan apakah ditinjau.
   - Setiap paket benar-benar ada di registry resmi, dengan maintainer dan unduhan yang wajar. Tandai nama yang mirip paket populer.
7. **Human-in-the-loop.** Ukur ukuran PR. Usulkan batas ukuran PR dan aturan bahwa pemilik hanya merge diff yang ia pahami.
8. **Portabilitas konteks.** Periksa apakah spesifikasi, keputusan, dan daftar tugas tertulis cukup lengkap sehingga pekerjaan bisa dilanjutkan dengan tool lain atau manual. **Tempatnya di hop-ops, bukan di repo app** (lihat bagian 7). Repo app hanya menyimpan dokumen teknis yang netral.
9. **Tes per slice.** Unit dan integration test per fitur, serta tes regresi untuk bug yang pernah diperbaiki.

**Keluaran `STANDAR-REKAYASA.md`:**
- Tabel app × 9 butir di atas, dengan bukti untuk setiap sel.
- Rencana adopsi per repo, berupa usulan konkret yang belum dipasang: isi `.pre-commit-config.yaml` atau hook, job CI, aturan branch protection, kerangka `ARCHITECTURE.md` per app, dan template PR.
- Urutan adopsi yang paling murah dan paling berdampak dulu.

## 7. Catatan internal ke repo hop-ops

Pemilik ingin semua jejak AI, tujuan (goals), rencana, keputusan, dan pekerjaan yang belum selesai ditulis ke file Markdown, lalu dipindahkan dari repo app ke **satu repo ops private bernama `hop-ops`**.

Repo `navi-crwn/helink-ops` (private) sudah ada dan berisi `README.md`, `backups/`, `design-reference/`, dan `notehop/`. Usulkan apakah repo itu di-rename menjadi `hop-ops` (GitHub mempertahankan redirect dari nama lama) atau dibuat repo baru. Jelaskan dampaknya.

**Langkah:**

1. **Inventaris** di semua repo dan di server (baca saja):
   - Dokumen planning dan prompt: `~/imperium/docs/00-*`, `01-*`, `02-PLANNING.md`, `10-12-INVENTARIS-*`, `20-PROMPT-AUDIT.md`, dan tangkapan layar `docs/img-*`.
   - Catatan serah-terima seperti `~/hel-ink-handoff/` dan dokumen sejenis di checkout lain.
   - File instruksi agent dan jejak AI dari bagian 5.
   - Komentar `TODO`, `FIXME`, `HACK`, dan `XXX` di kode.
   - Rencana dan daftar "belum" di body PR atau issue.
   - Skrip ops di `~/imperium/scripts/`.
2. **Klasifikasi.** Untuk setiap item, catat path asal, jenisnya (jejak AI, tujuan, rencana, keputusan, TODO, runbook, atau skrip), tujuan di hop-ops, dan perlakuannya di repo asal (dihapus, diganti ringkasan netral, atau dibiarkan karena merupakan dokumen teknis netral).
3. **Susun isinya, belum dipindahkan.** Tulis file Markdown hasil gabungan di `~/audit-hop/<tanggal>/hop-ops/` dengan struktur yang siap disalin. Usulan struktur:
   - `README.md`
   - `apps/<app>/GOALS.md`, `TODO.md`, `DECISIONS.md`
   - `family/ROADMAP.md` (Tahap 4, Tahap 5, RemoteHop)
   - `audit/<tanggal>/` (semua keluaran audit ini)
   - `ai/` (JEJAK-AI, prompt, dan instruksi agent yang masih ingin dipakai)
   - `runbooks/`
   - `scripts/`

   Perbaiki strukturnya bila ada yang lebih baik.
4. **Tulis `PINDAH-KE-HOP-OPS.md`** berisi langkah singkat untuk pemilik:
   - Rename atau buat repo.
   - Salin folder, lalu commit.
   - Hapus item dari repo asal lewat PR biasa. Bila perlu dibersihkan dari riwayat juga, rujuk langkah `git filter-repo` di bagian 5.
   - Sesuaikan skrip yang memakai path lama, misalnya `~/imperium/scripts/*`.

## 8. Link pendek tiap app

Uji dari luar (lewat Cloudflare) dengan GET anonim, lalu catat status, header `Location`, dan cache.

- **LinkHop.** Periksa:
  - `hel.ink/l/{slug}`.
  - Slug lama di root `hel.ink/{slug}` (`$hop_legacy_redirect`).
  - Link berpassword (POST): baca kodenya, uji di lokal.
  - Link kedaluwarsa atau dinonaktifkan, dan slug yang tidak ada.
  - Slug yang bentrok dengan path hub.
  - Pemeriksaan Safe Browsing atau hop-safety untuk URL berbahaya.
- **BioHop.** `hel.ink/b/{slug}`, gerbang password atau umur, klik tracking, dan pengalihan dari path lama di LinkHop.
- **PixelHop.** Link langsung `p.hel.ink/i/...`, link album, dan perilaku setelah rencana pindah ke `pixel.hel.ink`. Log menunjukkan 404 berulang pada `/i/2026/04/08/...`, telusuri penyebabnya.
- **NoteHop.** Link share, domain share pendek (`NOTEHOP_SHARE_URL` kosong), catatan di `/name`, dan pengalihan link lama.
- **Umum.** Periksa:
  - Panjang tiap bentuk link.
  - Konsistensi skema antar app.
  - `robots` dan `noindex` untuk link privat.
  - Open redirect.
  - Pengalihan sementara (`/api/` dan slug root) punya tanggal pencabutan yang tercatat.

## 9. Standar audit teknis (50 butir)

Terapkan ke **setiap app** sesuai stack (Laravel/PHP atau Go). Butir yang tidak berlaku ditandai "tidak berlaku" beserta alasannya, jangan dilewati diam-diam. Semua dijalankan pada klon di folder audit atau dengan membaca konfigurasi live.

**Keamanan**
1. Injeksi: lacak input (body, query, params, header) sampai ke query DB atau shell. Wajib memakai parameter binding atau ORM dan validasi ketat. Tunjukkan setiap penggabungan string mentah.
2. Access control dan IDOR: setiap operasi baca, ubah, dan hapus memeriksa kepemilikan berdasarkan sesi, bukan ID dari klien.
3. Secret tertanam di kode, konfigurasi, skrip, atau Dockerfile. Semua harus berasal dari env, dan izin file env harus ketat.
4. Sesi, tiket, dan cookie: algoritma tanda tangan diverifikasi, TTL wajar, ada anti-replay, flag `HttpOnly`/`Secure`/`SameSite` terpasang, dan sesi dirotasi saat login.
5. Rate limit dan brute force di login, pendaftaran, reset password, 2FA, pembuatan link, upload, endpoint SSO, dan `/hop/*`, berbasis IP dan user.
6. XSS (Blade `{!! !!}`, `template.HTML`, markdown), CSRF di semua mutasi, CORS tanpa wildcard, dan CSP tiap app.
7. Upload file (PixelHop, BioHop, avatar): periksa magic bytes, batas ukuran, nama file aman dari `../`, penyimpanan tanpa izin eksekusi, dan penghapusan EXIF/GPS.
8. Kebocoran di log dan respons error: tidak ada stack trace atau SQL, PII dan token di-mask, dan `APP_DEBUG=false`.
9. SSRF dan open redirect: fitur pratinjau URL, pemeriksaan link, webhook, `return_to`, dan `next`. Blokir IP lokal dan alamat metadata.
10. Dependensi: jalankan `composer audit`, `govulncheck`, dan `npm audit` pada klon. Laporkan CVE aktif dan paket usang.

**Kesiapan produksi**
11. Paritas env: bandingkan kode dengan `.env.example`. Aplikasi harus gagal cepat bila variabel penting kosong.
12. Migrasi idempoten, dan indeks untuk query utama (`WHERE`/`JOIN`/`ORDER BY`), terutama di tabel bersama `helink_db`.
13. Health check riil (DB, storage, dependensi) dengan timeout. Bandingkan `/up`, `/healthz`, dan `/hop/health`.
14. Graceful shutdown saat menerima `SIGTERM`, untuk service Go dan worker atau queue Laravel.
15. Retensi dan pembersihan otomatis: sesi, token, OTP, log, tempat sampah PixelHop (30 hari, 2 GB), dan log watchdog.
16. nginx dan Cloudflare: kompresi, `client_max_body_size`, security header, IP asli pengunjung (`CF-Connecting-IP`, trust proxy), dan TLS origin.
17. Observability: log terstruktur, request ID, dan alert yang benar-benar terkirim (Telegram, email).
18. Layanan eksternal (Google, SMTP, Safe Browsing, Telegram, S3): ada timeout dan retry, dan aplikasi tetap berfungsi saat layanan itu mati.
19. SEO dan crawler: robots.txt, sitemap, OpenGraph, serta `noindex` untuk admin, Imperium, dan link privat.
20. Smoke test E2E pada build produksi untuk alur kritis (daftar, login, aksi utama, baca data, logout), dijalankan di lokal.

**Kebersihan repo**
21. Sisa debug dan kode mati (`dd()`, `dump`, `fmt.Println`, bypass sementara).
22. `.gitignore`: tidak ada `.env`, dump DB, binary, atau log yang ter-commit.
23. Lockfile sinkron (`composer.lock`, `package-lock.json`, `go.sum`) dan lulus `go mod verify` atau `composer validate`.
24. Linter dan type checker: `gofmt`, `go vet`, `staticcheck`, Pint, dan PHPStan/Larastan bila ada, tanpa jalan pintas mengabaikan peringatan.
25. Tes: semua suite lulus di klon. Catat area yang belum punya tes, terutama SSO, logout berantai, dan hapus akun.
26. Container (bila ada Dockerfile): multi-stage, versi base image dikunci, dan berjalan sebagai user non-root.
27. Kompatibilitas mundur: kontrak HOP App Protocol, tiket v1 dan v2, dan link lama.
28. Dokumentasi: README, `.env.example`, dan spesifikasi API sesuai kode.
29. CI mencakup lint, tes, dan build, serta sesuai struktur folder terbaru.
30. Riwayat commit dan PR rapi, ada changelog atau versi rilis, dan tanpa jejak AI (lihat bagian 5).

**Stabilitas backend**
31. Race condition: kuota, pembuatan slug unik, username, penghitung klik, dan penulisan SQLite bersamaan.
32. Kebocoran resource: `defer Close()` pada body, rows, dan file; listener dan interval di frontend dibersihkan.
33. Query N+1, dan loop yang berisi query atau panggilan HTTP, termasuk saat Imperium memanggil beberapa app.
34. Transaksi DB untuk operasi dengan beberapa penulisan, termasuk hapus akun dan pemindahan data antar-app.
35. Edge case: input null atau kosong, Unicode dan emoji panjang, payload besar, dan angka negatif.
36. Idempotensi: callback SSO, event antar-app, aksi admin yang diulang, dan webhook.
37. Konsistensi cache dan pencegahan cache stampede: cache profil, manifest, dan registry Imperium.
38. Isolasi error di goroutine, job, dan queue, termasuk pemulihan dari panic.
39. Mesin status: transisi status akun (active, suspended, banned, deleted), link, dan laporan.
40. Waktu konsisten dalam UTC, presisi angka, dan `LIMIT` atau pagination di semua daftar.

**Penyelesaian**
41. Format respons API seragam dan pesan error yang manusiawi, dalam bahasa yang konsisten.
42. State UI: loading, kosong, error dengan tombol coba lagi, dan tanda sukses.
43. Proteksi submit ganda, tombol Enter dan Esc, serta autofocus.
44. Responsif di lebar 320–375px, tablet, dan desktop, tanpa overflow horizontal.
45. Performa frontend: ukuran bundle, gambar, lazy load, dan CLS.
46. Konsol browser bersih: tidak ada error, peringatan CSP, atau 404 aset dan favicon.
47. Konsistensi visual dan mode gelap/terang antar app (hop-ui).
48. Aksesibilitas: HTML semantik, `aria-label` pada tombol ikon, dan urutan Tab yang benar.
49. Keterbacaan kode: tandai fungsi atau file yang terlalu panjang.
50. Runbook: deploy, rollback, migrasi, backup dan restore, serta langkah pertama saat insiden, dalam satu dokumen untuk seluruh keluarga.

## 10. Cara menguji

- **Browser.** Gunakan Playwright dari `~/linkhop/node_modules/@playwright/test` (chromium sudah ada) lewat skrip node di folder audit. Skrip harus memakai path absolut ke modul itu dan tidak menulis apa pun ke `~/linkhop`. Playwright MCP tidak punya Chrome.
- **Panggilan bertanda tangan.** Hanya GET, misalnya `/hop/health`, manifest, dan daftar. Buat helper kecil di folder audit yang membaca secret lewat `scripts/s` tanpa mencetaknya.
- **Tes lokal di klon.**
  - Laravel: `composer install` lalu `php8.3 artisan test`; e2e LinkHop memakai `tests/e2e/serve.sh` (butuh shim `php` yang menunjuk php8.3 di PATH).
  - Go: `go test -race ./...` dengan `/home/carawin/sdk/go/bin/go`.
  - Imperium: `make test`, ditambah `scripts/fakehop` untuk HOP palsu.

  Jalankan semuanya di port bebas, dengan database sementara di folder audit.
- **Larangan.** Jangan menjalankan load test atau scanner agresif ke domain produksi. Cloudflare bisa memblokir IP VPS, dan ini juga dianggap serangan.

## 11. Keluaran

Semua berkas disimpan di `~/audit-hop/<YYYY-MM-DD>/`. Jangan di-commit ke mana pun. Seluruh folder ini nantinya dipindahkan pemilik ke `hop-ops/audit/<tanggal>/`.

1. **`LAPORAN.md`**, berisi:
   - Ringkasan eksekutif dalam 10 baris.
   - Tabel temuan dengan kolom: ID, app, kategori, keparahan (Kritis/Tinggi/Sedang/Rendah/Info), ringkasan, dan file:baris.
   - Detail tiap temuan: bukti, cara reproduksi, dampak, dan rekomendasi perbaikan.
   - Matriks skenario A–H.
   - Tabel kelengkapan dashboard (3a).
   - Tabel status repo.
   - Tabel link pendek.
   - Hasil 50 butir per app.
   - Daftar apa saja yang tidak bisa diverifikasi tanpa izin menulis.
2. **`JEJAK-AI.md`**: temuan jejak AI per repo dan cara singkat menghapusnya (bagian 5).
3. **`DASHBOARD-BIOHOP.md`**, dan `DASHBOARD-<app>.md` lain bila ada (bagian 3b).
4. **`HOMEPAGE-BIOHOP.md`** dan **`BAHASA.md`** (bagian 3c), beserta tangkapan layar homepage saudara di `img/`.
5. **`STANDAR-REKAYASA.md`** (bagian 6).
6. **`hop-ops/`** (isi siap salin) dan **`PINDAH-KE-HOP-OPS.md`** (bagian 7).
7. **`RENCANA-PERBAIKAN.md`**:
   - Urutan perbaikan dari temuan Kritis.
   - Pembagian menjadi PR kecil per repo: judul, berkas, dan tes, dengan tautan ke ID temuan.
   - Urutan deploy yang disarankan.
   - Uji produksi yang menunggu izin pemilik, beserta skrip dan langkah manualnya.
8. **`skrip/`**: skrip Playwright dan helper yang dipakai atau disiapkan.

Tutup pekerjaan dengan ringkasan singkat di chat:
- Lokasi folder hasil.
- Lima temuan paling penting.
- Jumlah jejak AI per repo.
- Keputusan yang dibutuhkan dari pemilik.
