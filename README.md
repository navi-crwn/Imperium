# Imperium

HOP Dashboard & Operations Console untuk keluarga aplikasi HOP (LinkHop, BioHop, PixelHop, NoteHop, RemoteHop). Privat.

- `cmd/imperium/` binary server Go tunggal (UI web, server-to-server HMAC client)
- `docs/` analisis arsitektur, daftar fitur, planning, dan inventaris admin tiap aplikasi
- `protocol/` HOP App Protocol (kontrak Admin API & webhook semua aplikasi)
- `scripts/` `hop-backup.sh`, `hop-restore.sh`, skrip deploy dan registry
- `deploy/` vhost nginx (Tailscale-only), cloudflared, unit systemd

### Ekosistem Keluarga HOP

| App | Alamat Live | Repository | Kegunaan |
| --- | --- | --- | --- |
| **Hub** | `hel.ink` · `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | Akun HOP ID, SSO issuer, launcher, admin pusat |
| **LinkHop** | `link.hel.ink` · `hel.ink/l/{slug}` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Pemendek link, QR code, analitik klik, API |
| **BioHop** | `bio.hel.ink` · `hel.ink/b/{slug}` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | Halaman link-in-bio dengan blok, tema, statistik |
| **PixelHop** | `p.hel.ink` (segera `pixel.hel.ink`) | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Hosting media & gambar, pembersihan GPS, studio foto |
| **NoteHop** | `note.hel.ink` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Catatan kolaboratif real-time terenkripsi zero-knowledge |
| **Imperium** (repo ini) | Internal / Tailscale | Privat | Konsol operasional & admin terpadu lintas aplikasi |

## The HOP family

Part of **HOP**, a family of free, open-source apps that share one account (HOP ID).

<!-- hop-family:start -->
<!-- Generated from HEL.ink resources/hop-assets/apps.source.json; edit there, then run scripts/hop_assets.py build. -->
| App | Address | Repository | What it does |
| --- | --- | --- | --- |
| **HOP ID** | [hel.ink](https://hel.ink) · accounts at `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | One account for every HOP app. |
| **LinkHop** | [link.hel.ink](https://link.hel.ink) · `hel.ink/l/…` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Short links, QR codes and click analytics. |
| **BioHop** | [bio.hel.ink](https://bio.hel.ink) · `hel.ink/b/…` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | One link-in-bio page for everything you do. |
| **PixelHop** | [p.hel.ink](https://p.hel.ink) | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Image hosting with albums and direct links. |
| **NoteHop** | [note.hel.ink](https://note.hel.ink) · `n.hel.ink/…` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Encrypted notes you share with a link. |
| **Imperium** | private (owner only) | private | The HOP control panel. |
<!-- hop-family:end -->
