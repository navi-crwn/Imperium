# Imperium

[Read this in Indonesian (Bahasa Indonesia)](README.id.md)

HOP Dashboard & Operations Console for the HOP app family (LinkHop, BioHop, PixelHop, NoteHop, RemoteHop). Private.

- `cmd/imperium/` single Go server binary (web UI, server-to-server HMAC client)
- `docs/` architecture analysis, feature list, planning, and per-app admin inventories
- `protocol/` HOP App Protocol (the Admin API & webhook contract for every app)
- `scripts/` `hop-backup.sh`, `hop-restore.sh`, deploy and registry scripts
- `deploy/` nginx vhost (Tailscale-only), cloudflared, systemd units

### HOP Family Ecosystem

| App | Live address | Repository | Purpose |
| --- | --- | --- | --- |
| **Hub** | `hel.ink` · `hub.hel.ink` | [navi-crwn/HEL.ink](https://github.com/navi-crwn/HEL.ink) | HOP ID accounts, SSO issuer, launcher, central admin |
| **LinkHop** | `link.hel.ink` · `hel.ink/l/{slug}` | [navi-crwn/LinkHop](https://github.com/navi-crwn/LinkHop) | Link shortener, QR codes, click analytics, API |
| **BioHop** | `bio.hel.ink` · `hel.ink/b/{slug}` | [navi-crwn/BioHop](https://github.com/navi-crwn/BioHop) | Link-in-bio pages with blocks, themes, stats |
| **PixelHop** | `p.hel.ink` (soon `pixel.hel.ink`) | [navi-crwn/PixelHop](https://github.com/navi-crwn/PixelHop) | Media & image hosting, GPS scrubbing, photo studio |
| **NoteHop** | `note.hel.ink` | [navi-crwn/NoteHop](https://github.com/navi-crwn/NoteHop) | Real-time collaborative notes, zero-knowledge encrypted |
| **Imperium** (this repo) | Internal / Tailscale | Private | Unified cross-app operations & admin console |

### Languages

The UI language is set by the operator with `IMPERIUM_LANG` (see
`deploy/imperium.env.example`). The default is `en`; `IMPERIUM_LANG=id`
selects the Indonesian catalog. Unknown values fall back to English, and the
rendered `<html lang>` always reflects the catalog in use.
