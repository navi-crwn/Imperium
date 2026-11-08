# Imperium

[Read this in Indonesian (Bahasa Indonesia)](README.id.md)

HOP Dashboard & Operations Console for the HOP app family (LinkHop, BioHop, PixelHop, NoteHop, FileHop, RemoteHop). Private.

- `cmd/imperium/` single Go server binary (web UI, server-to-server HMAC client)
- `docs/` architecture analysis, feature list, planning, and per-app admin inventories
- `protocol/` HOP App Protocol (the Admin API & webhook contract for every app)
- `scripts/` `hop-backup.sh`, `hop-restore.sh`, deploy and registry scripts
- `deploy/` nginx vhost (Tailscale-only), cloudflared, systemd units

### HOP Family Ecosystem

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
| **Imperium** | private (owner only) | private | The HOP control panel. |
<!-- hop-family:end -->

### Languages

The UI language is set by the operator with `IMPERIUM_LANG` (see
`deploy/imperium.env.example`). The default is `en`; `IMPERIUM_LANG=id`
selects the Indonesian catalog. Unknown values fall back to English, and the
rendered `<html lang>` always reflects the catalog in use.

The Notifications pages are translated into all nine languages of the HOP family
(`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). The older pages still
exist in `en` and `id` only; a text a language lacks falls back to English.

### Notifications (HTTP 500 reports)

HOP apps report every HTTP 500 they serve to `POST /hop/notify` on Imperium's
loopback listener, signed with the app's own registry secret (details, payload
and limits: `protocol/PROTOCOL.md` section 10). Identical errors are grouped and
shown on the **Notifications** page (badge = groups still new); an admin can
acknowledge, resolve, mute (with a reason) or reopen a group, and every action is
audited. Occurrences are kept 30 days, groups 90 days. Telegram gets at most one
message per group per `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (default 60), never with
user data. The database migrates itself on start (`schema_version`). To try it
locally: `scripts/fakehop` plus `NOTIFY_SECRET=... go run ./scripts/notifysend`.
