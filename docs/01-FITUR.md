# HOP Imperium: daftar fitur lengkap

Dokumen 2 dari 3. Semua fitur yang masuk akal untuk satu pusat kendali keluarga HOP, tanpa memikirkan durasi dan biaya. Tanda:
- **[M]** = migrasi dari panel yang sudah ada (hel.ink / PixelHop), panel lama dipensiunkan setelah modul ini hidup.
- **[B]** = baru, belum ada di mana pun.
- **[ZK]** = dibatasi zero-knowledge NoteHop (hanya metadata).

Keluarga HOP saat ini: **LinkHop** (hel.ink, tagline "Hop Easy Link"), **PixelHop** (p.hel.ink), **NoteHop** (note.hel.ink), **RemoteHop** (direncanakan). Aplikasi lain di VPS (carawinapp, nawalaku) bisa ikut sebagai "aplikasi eksternal" hanya untuk pemantauan server.

---

## A. Fondasi: HOP ID (akun dan profil terpadu)

Tetap di hel.ink sebagai server akun, diperluas.

1. **Profil global** [B]: username global, nama tampilan, bio, website, lokasi, avatar (upload ke hel.ink, URL stabil `hel.ink/avatar/{id}`), tautan sosial, pronoun, bahasa UI, zona waktu, flag `show_owner` global.
2. **Satu aturan username** [B]: 3–30 karakter, huruf kecil, angka, `_`, `-`, `.`; mulai huruf/angka; tidak `..`; reserved gabungan dari ketiga aplikasi + prefix (admin-, official-, hop-, staff-, …); daftar kata terlarang gabungan; hold 90 hari setelah diganti; riwayat username lama tetap redirect.
3. **Backfill dan resolusi konflik** [B]: username PixelHop dan NoteHop ditarik ke hel.ink; bentrok diselesaikan "siapa lebih dulu dibuat"; yang kalah dapat notifikasi dan diminta memilih ulang saat login berikutnya.
4. **Peran** [B]: `owner`, `admin`, `moderator`, `support` (baca saja + jawab laporan), `app:<nama>` (admin satu aplikasi). Disimpan di hel.ink, dikirim di tiket SSO (`roles[]`) dan API. Mengganti `SUPERADMIN_EMAIL`, `role=admin` PixelHop, dan `NOTEHOP_ADMINS`.
5. **Tiket SSO v2** [B]: menambah `username`, `roles`, `profile_version`, `verified`, `created_at`; `aud` wajib; versi tiket; rotasi secret per aplikasi tanpa downtime (dua secret aktif).
6. **Profile push** [B]: setiap perubahan profil di hel.ink dikirim ke semua aplikasi (webhook bertanda tangan `profile.updated`), aplikasi menyimpan salinan lokal untuk render cepat. Aplikasi tidak lagi punya form edit profil sendiri; link "Edit profil" membuka hel.ink.
7. **Pendaftaran aplikasi (app registry)** [B]: tabel `apps` di hel.ink: nama, slug, base URL, callback, secret (terenkripsi), ikon, warna, status (aktif/maintenance/off), halaman yang bisa dituju, kemampuan (profile, admin API, metrics, webhooks). Mengganti konstanta `APPS`. Aplikasi baru = satu baris + env di sisi aplikasi.
8. **Branding tombol** [B]: "Login with HOP", "Join HOP", "Continue with HOP", "Continue with Google via HOP". Satu komponen tombol + ikon HOP yang dipakai semua aplikasi; halaman login hel.ink menampilkan logo aplikasi asal ("Masuk ke NoteHop dengan HOP ID").
9. **Satu login, semua aplikasi** [B]: setelah login HOP, aplikasi lain tidak minta login lagi (silent SSO lewat iframe/redirect ringan atau cookie domain `.hel.ink` khusus penanda "sudah login").
10. **Logout global** [B]: logout di satu tempat mencabut sesi di semua aplikasi (`session.revoked` webhook + daftar sesi per aplikasi).
11. **Passkey/WebAuthn** [B] dan **recovery codes** untuk HOP ID; wajib 2FA untuk peran admin ke atas.
12. **Penggabungan akun** [B]: akun PixelHop lokal (password/Google) dan akun NoteHop password lama digabung ke HOP ID lewat verifikasi email; `MergeUsers` dipanggil dari API, bukan CLI.
13. **Hapus akun global** [B]: satu tombol di hel.ink menghapus/menganonimkan data di semua aplikasi (`account.deleted` webhook, laporan hasil per aplikasi), dengan masa tenggang 14 hari dan ekspor data gabungan (ZIP: link, bio, gambar, catatan publik, metadata catatan).
14. **Status akun global** [B]: active / locked / suspended / banned / deleted, dengan catatan internal, alasan yang ditampilkan ke user, dan tanggal kedaluwarsa (suspend sementara). Disebarkan ke semua aplikasi.

## B. Imperium: inti panel

15. **Login Imperium** = HOP ID dengan peran admin, 2FA wajib, passkey, sesi 8 jam, re-auth untuk aksi berbahaya (hapus, ban, restart), IP allowlist, Tailscale-only.
16. **Beranda** [M]: satu layar untuk semua aplikasi: KPI per aplikasi (user, konten, aktivitas hari ini vs kemarin), "Needs attention" gabungan (laporan terbuka, temuan abuse, antrean mati, disk penuh, backup gagal, cert mau habis), status service (hijau/kuning/merah), grafik 14 hari gabungan.
17. **Pencarian global** [B]: satu kotak: username, email, id HOP, slug link, id gambar, token catatan, IP, domain. Hasil lintas aplikasi.
18. **Halaman user 360°** [B]: profil HOP + untuk setiap aplikasi: konten, kuota, sesi aktif, API key, webhook, laporan terhadapnya, riwayat login (termasuk Google dan SSO yang sekarang tidak dicatat), riwayat aksi admin terhadapnya, catatan internal (notes) oleh admin, tag (VIP, watch, bot).
19. **Aksi user global** [M+B]: suspend/lock/ban/aktifkan (dengan alasan, durasi, opsi "sembunyikan semua konten" atau "biarkan"), verifikasi email paksa, reset 2FA, cabut semua sesi, cabut semua API key, ubah email/nama/username, ubah peran, set kuota per aplikasi, kirim peringatan (modal di semua aplikasi), kirim email, impersonate baca-saja ("lihat sebagai user", tanpa bisa menulis), hapus akun global, ekspor data.
20. **Audit log** [B]: setiap aksi admin di Imperium dan setiap aksi yang Imperium kirim ke aplikasi dicatat: siapa, kapan, dari IP mana, aksi, sasaran, before/after, alasan. Tidak bisa dihapus dari UI, bisa diekspor, retensi panjang. Admin lain bisa melihat; `owner` bisa melihat semua.
21. **Antrean laporan terpadu** [M+B]: laporan dari hel.ink (abuse_reports), PixelHop (reports), NoteHop (fitur lapor baru), form kontak, email abuse@ yang diparsing. Satu antrean dengan prioritas, kategori, penugasan ke moderator, SLA, template balasan, tindakan satu klik yang langsung ke aplikasi terkait, dan "tindakan lintas aplikasi" (satu pelaku, konten di tiga aplikasi).
22. **Notifikasi** [B]: Telegram (bot sudah ada infrastrukturnya di router-bot), email, dan web push ke admin untuk: laporan baru, suspend otomatis, service down, disk > 85 %, backup gagal, login admin baru, cert < 14 hari, lonjakan traffic/abuse.
23. **Peran & tim** [B]: undang admin/moderator lain, lihat siapa sedang online di Imperium, batasi modul per peran, 2FA wajib, cabut akses.
24. **Mode baca-saja / maintenance** [B]: tombol "maintenance" per aplikasi (halaman 503 cantik + pesan), "bekukan pendaftaran" per aplikasi, "bekukan upload/buat" per aplikasi, banner pengumuman global ke semua aplikasi sekaligus.
25. **Feature flags** [B]: nyalakan/matikan fitur per aplikasi dari Imperium tanpa deploy (AI tools PixelHop, SSH NoteHop, Telegram NoteHop, Explore PixelHop, guest upload, guest link), per persentase user atau per daftar user (beta).
26. **Pengaturan terpusat** [M+B]: semua yang sekarang di `.env` dan hard-coded dibuat bisa diubah di Imperium dan disimpan di tabel `settings` tiap aplikasi (lihat inventaris), dengan validasi, riwayat perubahan, dan "revert". Rahasia (API key pihak ketiga) disimpan terenkripsi dan hanya ditampilkan 4 karakter terakhir.
27. **Dark/light, responsif, PWA, command palette (⌘K)**, keyboard shortcut, tabel dengan kolom bisa dipilih, filter tersimpan, ekspor CSV/JSON di setiap tabel.
28. **i18n Imperium**: Indonesia dan Inggris minimal.

## C. Modul LinkHop (hel.ink)

29. **Links** [M]: semua link, filter (status, pemilik, guest, folder, tag, domain tujuan, negara klik, skor abuse), sort, aksi: aktif/nonaktif, hapus (sekaligus QR file), ubah tujuan, pindah pemilik, reset klik, bulk, lihat klik mentah (30 hari), preview aman (screenshot via headless), cek keamanan ulang (LinkSafety).
30. **Bio pages** [M]: daftar, monitor periode, CSV, detail, publish/unpublish, hapus (sekaligus file), lihat sebagai pengunjung, edit paksa (hapus blok tertentu), pindah pemilik.
31. **Analytics global** [M]: semua grafik yang ada (klik, negara, kota, ISP, proxy, device, referrer) + per periode custom, perbandingan periode, top domain tujuan, top pemilik, retensi user (kohort), link guest vs akun.
32. **Abuse** [M+B]: temuan scan, confirm/restore, scan manual, **editor aturan** (regex, bobot, threshold, kategori, daftar shortener/TLD berisiko) tanpa deploy, uji aturan terhadap link yang ada ("dry-run: berapa yang kena"), feed URLhaus/OpenPhish/Safe Browsing status + perbaiki bug `SAFE_BROWSING_KEY`.
33. **Domain blacklist / whitelist** [M+B]: exact, wildcard, regex, kategori, impor/ekspor, uji URL.
34. **IP** [M+B]: ban (exact + **CIDR**, sementara/permanen, alasan), watchlist, lihat aktivitas IP lintas aplikasi (klik, upload, catatan, login), GeoIP/proxy lookup satu klik, ban global (berlaku di semua aplikasi).
35. **Guest links** [M]: kebijakan masa hidup, retired slugs (lihat/lepas), prune manual dengan dry-run.
36. **SEO & site** [M]: judul, meta, OG, favicon, logo, halaman marketing, sitemap regenerate.
37. **Email** [B]: lihat antrean email, kirim ulang verifikasi, template email (welcome, verifikasi, reset) bisa diedit, log pengiriman (status SMTP), uji kirim.
38. **Queue & scheduler** [M+B]: heartbeat, restart (lewat helper, bukan `sudo` dari PHP), failed_jobs (lihat, retry, hapus), jalankan perintah terjadwal manual (`abuse:scan`, `links:prune-guest`, `geolite:update`, `backup:run`) dengan output live.
39. **API tokens** [B]: lihat semua token user, cabut, rate limit per token, statistik pemakaian (`api_usage_logs`).
40. **GeoIP & proxy** [M]: monitor provider, kuota, uji, urutan prioritas bisa diubah.
41. **Retensi data** [B]: angka retensi (30/90/180 hari) bisa diubah, prune manual dengan laporan.

## D. Modul PixelHop

42. **Gambar/media** [M+B]: semua gambar/video/PDF, filter (pemilik, guest, IP, ukuran, jenis, hidden, private, password, expiring, listed, backend), grid/tabel, aksi: hide/unhide, delete (soft-delete 30 hari **baru**, lalu purge), ban IP + hapus, pindah pemilik, ubah expiry, ubah visibilitas, paksa re-strip EXIF, lihat referrer dan views, bulk.
43. **Album** [B]: daftar, detail, hapus, pindah pemilik.
44. **Laporan** [M]: masuk antrean terpadu (B21), aksi dismiss/hide/delete tetap.
45. **User PixelHop** [M]: kuota (GB), status, role (dihapus: peran dari HOP), warning, hapus profil, lihat storage, lihat API key dan webhook user, cabut; cari username (bug sekarang).
46. **Storage** [B]: pemakaian disk/S3, biaya perkiraan, file yatim (storage tanpa baris DB) dan baris yatim (DB tanpa file), bersihkan, varian cache ukuran, migrasi local→S3 per batch.
47. **AI tools** [M+B]: status `pixelhop-ai` service, model yang dimuat, idle, antrean, pemakaian per tool per hari per user, limit per user/global bisa diubah, nyalakan/matikan per tool.
48. **Media worker** [B]: antrean ffmpeg (media_jobs), job macet, retry, batas CPU.
49. **Pengaturan** [M]: guest_uploads, registration, announcement, SEO, dan semua env limit (upload MB, media MB, guest MB, megapixel, batch, per-menit, default quota, inactive days, Turnstile, Google, S3).
50. **Explore & oEmbed** [B]: moderasi feed Explore (sembunyikan dari feed tanpa hide), domain yang boleh embed.
51. **Metrics** [B]: scrape `/metrics` Prometheus ke Imperium untuk grafik historis.

## E. Modul NoteHop

52. **Catatan** [M(CLI)+B][ZK]: daftar metadata (token/slug, pemilik, encrypted/public, mode link, ukuran, views/max_views, expiry, frozen, created/updated/last_open, fitur aktif dari config, jumlah entry/versi/lampiran/inbox), filter, cari slug/pemilik; aksi: takedown (hold 365 hari), hapus, ubah expiry, lepas hold slug, bekukan (frozen), matikan inbox/watch, lihat isi **hanya jika publik**, buka via link `#k=` yang dikirim pelapor (dalam sandbox, tidak disimpan).
53. **Fitur lapor di NoteHop** [B]: tombol lapor di halaman catatan (kirim token + alasan, opsional link lengkap), masuk antrean terpadu.
54. **User NoteHop** [B]: daftar (password vs SSO), sesi aktif + cabut, hapus akun, hapus semua catatan, status dari HOP, vault_check reset (user kehilangan dashboard tapi bukan catatan).
55. **Nama** [B]: reserved dan blocked list bisa diedit (sekarang file embedded), slug_holds lihat/lepas, takedown holds, username rules disamakan ke HOP.
56. **Lampiran** [B]: pemakaian vs FILES_TOTAL_MB, per IP, hapus file, TTL bisa diubah.
57. **Real-time & nearby** [B]: room live aktif (jumlah peer), nearby rooms, tutup room, batas peer/pesan bisa diubah.
58. **Integrasi** [B]: Telegram bot (status, chat terhubung, allowlist chat, putus), SSH server (on/off, port, koneksi terakhir), webhook watch (job tertunda, gagal, allow private hooks), rate limit per bucket bisa diubah dan persisten.
59. **Perbaikan keamanan** [B]: `NOTEHOP_ADMINS` dihapus, diganti peran dari tiket HOP; nama di daftar admin tidak bisa didaftarkan orang lain.
60. **Pengaturan** [M]: semua env NoteHop + limit hard-coded (1 MB/catatan, 2000 entry, expiry maks, dll.) jadi setting.

## F. Modul Server / Remote (yang Anda sebut "admin remote")

Dijalankan lewat **hop-agent**: daemon kecil di VPS (Go) dengan sudoers sempit, dipanggil Imperium lewat socket lokal/Tailscale. Bisa dipasang di VPS lain nanti (multi-server).

61. **Service** [B]: status, start/stop/restart (nginx, php-fpm, mariadb, redis, pixelhop, pixelhop-ai, notehop, queue-worker, docker container nawalaku), uptime, memori/CPU per service, auto-restart counter, log `journalctl` live (tail + filter) per service.
62. **Sistem** [B]: CPU, RAM, swap, load, disk per mount, inode, network in/out, suhu kalau ada, proses teratas, uptime, kernel, update paket tersedia (apt list), reboot terjadwal.
63. **Nginx** [B]: daftar vhost, cek konfigurasi (`nginx -t`), reload, access/error log live per vhost, top IP/top path/top status 5 menit terakhir, blok IP di level nginx (deny list), rate limit zone.
64. **Sertifikat** [B]: semua cert (hel.ink, cloudflare origin), tanggal habis, peringatan 30/14/7 hari, renew.
65. **Database** [B]: MariaDB status, ukuran tiap DB/tabel, slow query, koneksi aktif, SQLite ukuran + WAL + integrity_check, VACUUM terjadwal.
66. **Backup** [B]: satu tampilan untuk semua backup (spatie hel.ink, PixelHop backup.sh, NoteHop timer, carawinapp), terakhir sukses, ukuran, retensi, jalankan sekarang, **uji restore otomatis ke folder sementara** dan laporkan, simpan juga ke S3/R2 off-site, enkripsi.
67. **Deploy** [B]: versi yang jalan vs tag git terbaru per aplikasi, tombol "update" (pull/build/restart dengan backup dulu dan rollback otomatis bila healthcheck gagal), riwayat deploy, diff changelog. Memperbaiki situasi PixelHop sekarang (checkout main ≠ binary live).
68. **Cron** [B]: semua crontab (carawin, www-data, root) dan systemd timer dalam satu daftar, terakhir jalan, exit code, output, jalankan manual, nonaktifkan.
69. **Keamanan server** [B]: login SSH gagal (auth.log), fail2ban status, port terbuka (`ss -tlnp`), ufw rules, Tailscale peers, ZeroTier, user sudo, kunci SSH terdaftar, perubahan file penting (checksum nginx/sudoers/env).
70. **Env & secrets** [B]: lihat kunci (bukan nilai) env tiap aplikasi, edit nilai dengan konfirmasi + restart, rotasi secret SSO sekali klik (diperbarui di hel.ink dan aplikasi, dua secret aktif selama transisi).
71. **Docker** [B]: container (nawalaku), status, log, restart, image update.
72. **Terminal web** [B] (hanya `owner`, dengan re-auth + 2FA + audit rekaman sesi): shell terbatas lewat Tailscale untuk keadaan darurat.
73. **Uptime eksternal** [B]: ping dari luar (Cloudflare Worker atau UptimeRobot) ke tiap domain, status page internal, riwayat downtime.
74. **Multi-server** [B]: tambah VPS lain dengan hop-agent, Imperium menampilkan semua.

## G. Pengamatan dan laporan

75. **Metrics & grafik historis** [B]: Imperium menyimpan time series (SQLite/VictoriaMetrics) dari `/metrics` PixelHop, NoteHop (ditambahkan), hel.ink (ditambahkan), hop-agent; grafik 1 jam sampai 1 tahun.
76. **Log terpusat** [B]: Laravel log, PixelHop accessLog, NoteHop journal, nginx; pencarian, filter level, tautan dari error ke user/konten terkait, deteksi error berulang.
77. **Laporan berkala** [B]: ringkasan harian/mingguan ke Telegram/email: pertumbuhan user, konten, abuse, uptime, disk, backup.
78. **Kesehatan SSO** [B]: tiket diterbitkan/gagal per aplikasi, waktu respons API akun, secret mismatch, jam server melenceng (tiket kedaluwarsa dini).
79. **Anomali** [B]: lonjakan pendaftaran dari satu IP/ASN, upload massal, pembuatan link massal, login gagal berulang, pola bot; aksi otomatis yang bisa diatur (rate limit, captcha paksa, suspend sementara) dengan tombol "batalkan".

## H. Pengembangan dan kualitas

80. **Health check gabungan** [B]: `/healthz` tiap aplikasi + DB + storage + SSO, satu endpoint untuk uptime monitor.
81. **Sandbox/staging** [B]: tombol untuk menyalin DB (anonim) ke instance staging di port lain dan menjalankan e2e Playwright, hasil tampil di Imperium.
82. **Dokumentasi internal** [B]: runbook per insiden (service down, disk penuh, abuse massal, restore backup) tersimpan di Imperium, tautan dari alert ke runbook.
83. **Changelog & roadmap** [B]: catatan rilis per aplikasi, daftar ide (dari ROADMAP.md) dengan status.

## I. Backup, migrasi, dan watchdog konten berbahaya (permintaan tambahan)

### Backup & migrasi satu perintah

84. **Satu arsip untuk seluruh keluarga HOP** [B]: `hop-backup` membuat satu paket bertanda waktu berisi: dump MariaDB `helink_db`, `pixelhop.db` + folder gambar (atau manifest S3), `notehop.db`, DB Imperium, semua `.env` (terenkripsi dengan age/GPG, kunci di luar server), vhost nginx, unit systemd + timer, crontab semua user, sertifikat, sudoers hop-agent, daftar paket (apt, Go, PHP ext), dan `MANIFEST.json` (versi tiap aplikasi, commit, checksum). Dijalankan oleh timer harian, dari Imperium sekali klik, atau manual lewat SSH.
85. **Off-site otomatis** [B]: salinan ke S3/R2/B2 (rclone) dengan retensi harian 14, mingguan 8, bulanan 12; verifikasi checksum setelah upload; peringatan kalau backup terlambat.
86. **Uji restore otomatis** [B]: tiap minggu arsip terakhir dipulihkan ke folder sementara dan DB sementara, dicek integritas (`integrity_check`, `mysqlcheck`), dilaporkan ke Imperium/Telegram. Backup yang tidak pernah diuji dianggap tidak ada.
87. **Migrasi ke server baru satu perintah** [B]: `hop-restore --from <arsip> --target <ssh host>` di server baru yang masih kosong: pasang paket (nginx, PHP 8.3, MariaDB, redis, Go binary), buat user sistem, pulihkan DB dan file, pasang env (minta kunci dekripsi), vhost, systemd, cron, sertifikat, jalankan healthcheck tiap aplikasi, lalu cetak daftar DNS yang harus diubah. Idempoten: bisa diulang. Ditulis sebagai skrip Bash + Go kecil, **tanpa** bergantung pada Imperium supaya tetap bisa dipakai saat Imperium ikut hilang.
88. **Mode cutover** [B]: sinkronisasi terakhir (rsync file + dump DB) saat aplikasi lama dalam maintenance mode, lalu DNS pindah; Imperium menampilkan checklist cutover dan rollback (balik DNS).

### Watchdog konten berbahaya (anti takedown domain/hosting)

Tujuan: menangkap konten yang membuat registrar, Cloudflare, hoster, atau Google menandai domain (phishing, malware, CSAM, judi, penipuan, pelanggaran hak cipta massal) **sebelum** pihak luar menemukannya, dan punya bukti tindakan bila laporan datang.

89. **Pemindaian lintas aplikasi** [M+B]: mesin `abuse:scan` hel.ink (pola, bobot, threshold, Safe Browsing, URLhaus, OpenPhish, Spamhaus DBL) diangkat ke Imperium dan dipakai untuk: link dan bio hel.ink, judul/alt/teks OCR/hash gambar PixelHop, catatan publik dan slug/judul NoteHop, username dan bio profil HOP. Jadwal: tiap jam inkremental, tiap malam penuh.
90. **Pemindaian gambar** [B]: hash PhotoDNA-setara lewat layanan pihak ketiga yang legal untuk CSAM (misal Cloudflare CSAM Scanning Tool, gratis untuk zona Cloudflare), NSFW classifier lokal di `pixelhop-ai` (model open-source) untuk menandai dan menyembunyikan dari Explore, OCR teks pada gambar untuk pola judi/phishing.
91. **Reputasi domain & IP** [B]: cek berkala hel.ink, p.hel.ink, note.hel.ink, IP VPS terhadap Google Safe Browsing, VirusTotal, Spamhaus, SURBL, URLhaus, PhishTank, AbuseIPDB, Microsoft SmartScreen (lewat laporan), dan daftar blokir Indonesia (Trust Positif, API yang sama dengan nawalaku). Peringatan segera bila masuk daftar, dengan tautan ke form permintaan review tiap penyedia.
92. **Kotak masuk abuse** [B]: email `abuse@`, `dmca@`, `security@` masuk ke antrean terpadu (IMAP poll), diparsing (URL yang dilaporkan, pengirim, jenis), SLA 24 jam, template balasan, dan bukti tindakan tersimpan (siapa menghapus apa, kapan). Registrar dan Cloudflare biasanya menilai responsivitas ini sebelum takedown.
93. **Aksi otomatis bertingkat** [B]: skor rendah → tandai; sedang → sembunyikan + beri tahu pemilik; tinggi → nonaktifkan + suspend pemilik + ban IP; kategori CSAM → hapus, simpan hash dan metadata untuk laporan, blokir hash, dan notifikasi ke owner saat itu juga. Semua tingkat punya tombol "batalkan" dan masuk audit log.
94. **Kill switch** [B]: satu tombol untuk mematikan guest upload, guest link, pendaftaran, dan Explore di semua aplikasi sekaligus saat serangan massal, plus captcha paksa di semua form.
95. **Pencegahan di pintu masuk** [B]: domain tujuan baru dicek sebelum link dibuat (sudah ada di hel.ink, diperluas ke semua aplikasi), TLD berisiko dan shortener bertingkat ditolak, upload dari IP dengan skor AbuseIPDB tinggi diminta captcha, pembatasan per ASN (hosting/VPN) untuk guest.
96. **Halaman transparansi** [B]: `/abuse` publik di tiap aplikasi dengan form lapor, alamat kontak, dan janji SLA; laporan transparansi bulanan (jumlah laporan, tindakan). Menjadi bukti itikad baik saat ada sengketa.
97. **Latihan takedown** [B]: runbook "domain ditandai" (langkah: identifikasi konten, hapus, kumpulkan bukti, ajukan review ke Safe Browsing/Cloudflare/registrar, pantau), dan simulasi tiap kuartal dari Imperium.

---

## Prioritas yang saya sarankan untuk MVP (tetap besar, tapi berurutan)

1. Fondasi HOP ID (A1–A8, A14) dan registry aplikasi, karena semua modul lain bergantung padanya.
2. Imperium inti (B15–B20) + modul Users 360° dengan aksi global.
3. Antrean laporan terpadu (B21) + migrasi modul hel.ink Links/Bio/Abuse (C29–C34) dan PixelHop Images/Users/Reports (D42–D45). Panel lama dipensiunkan di sini.
4. NoteHop: API admin + fitur lapor + perbaikan keamanan (E52–E55, E59).
5. hop-agent + modul Server (F61–F68).
6. Backup & migrasi (I84–I88) bersamaan dengan hop-agent; watchdog (I89–I97) fondasinya saat migrasi modul Abuse.
7. Sisanya (observabilitas, anomali, feature flags, multi-server, terminal web).
