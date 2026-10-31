# Imperium

[Baca dalam bahasa Inggris (English)](README.md)

HOP Dashboard & Operations Console untuk keluarga aplikasi HOP (LinkHop, BioHop, PixelHop, NoteHop, RemoteHop). Privat.

- `cmd/imperium/` binary server Go tunggal (UI web, server-to-server HMAC client)
- `docs/` analisis arsitektur, daftar fitur, planning, dan inventaris admin tiap aplikasi
- `protocol/` HOP App Protocol (kontrak Admin API & webhook semua aplikasi)
- `scripts/` `hop-backup.sh`, `hop-restore.sh`, skrip deploy dan registry
- `deploy/` vhost nginx (Tailscale-only), cloudflared, unit systemd

### Ekosistem Keluarga HOP

<!-- hop-family:start -->
<!-- Generated from HEL.ink resources/hop-assets/apps.source.json; edit there, then run scripts/hop_assets.py build. -->
| App | Address | Repository | What it does |
| --- | --- | --- | --- |
| **HOP ID** | [hel.ink](https://hel.ink) · accounts at `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | One account for every HOP app. |
| **LinkHop** | [link.hel.ink](https://link.hel.ink) · `hel.ink/l/…` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Short links, QR codes and click analytics. |
| **BioHop** | [bio.hel.ink](https://bio.hel.ink) · `hel.ink/b/…` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | One link-in-bio page for everything you do. |
| **PixelHop** | [pixel.hel.ink](https://pixel.hel.ink) · `p.hel.ink/…` | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Image hosting with albums and direct links. |
| **NoteHop** | [note.hel.ink](https://note.hel.ink) · `n.hel.ink/…` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Encrypted notes you share with a link. |
| **Imperium** | private (owner only) | private | The HOP control panel. |
<!-- hop-family:end -->

### Bahasa

Bahasa antarmuka diatur lewat `IMPERIUM_LANG` (lihat `deploy/imperium.env.example`).
Bawaannya `en`; `IMPERIUM_LANG=id` memakai katalog Indonesia. Nilai lain jatuh
ke bahasa Inggris, dan `<html lang>` pada halaman selalu mengikuti katalog yang
dipakai.
