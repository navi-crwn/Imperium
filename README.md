# Imperium

**English** · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [Français](README.fr.md) · [Русский](README.ru.md)

The admin panel and operations console for the HOP app family (LinkHop, BioHop, PixelHop, NoteHop, FileHop, PayHop). It is open source like the rest of the family, and **optional**: every app runs without it. You need it only if you want one place to manage the users, content, abuse reports, settings and health of all your apps.

What it does:

- Signs admins in through HOP ID (the hub, [HEL.ink](https://github.com/navi-crwn/HEL.ink)); only accounts with an admin role get in, with two-factor sign-in by default.
- Reads the list of apps from the hub and talks to each one through its signed admin API (the HOP App Protocol in `protocol/`).
- Shows health, numbers, users, content, abuse reports and settings per app; groups the HTTP 500 errors the apps report; sends optional Telegram alerts.

What is in this repository:

- `cmd/imperium/`: the single Go server binary (web UI, signed server-to-server client)
- `internal/`: the application code
- `protocol/`: the HOP App Protocol, the admin API and webhook contract every app implements
- `docs/`: architecture notes, feature list and per-app admin inventories (written in Indonesian)
- `deploy/`: a hardened systemd unit, an environment example, and the nginx and Cloudflare tunnel files of the hel.ink production setup
- `scripts/`: backup and restore (`hop-backup.sh`, `hop-restore.sh`) and the hel.ink production setup scripts (`tahap*.sh`, `deploy-*.sh`)

## Run it

You need Go 1.27 or newer, and a running hub with at least one app.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ characters> ./bin/imperium
```

At the hub, set `HOP_CONTROL_URL` to Imperium's address and `HOP_SSO_SECRET_IMPERIUM` to the same secret, then run `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force` there: it registers Imperium as a hidden app that may call the admin API. `make test` runs the tests.

On a server, use `deploy/imperium.service` (systemd, with its own user and a read-only system) and an environment file like `deploy/imperium.env.example`. That example carries the hel.ink production values; replace the addresses with yours. Keep Imperium off the open internet: put it behind a VPN (Tailscale, WireGuard) or an access proxy, and serve it over HTTPS. The `tahap*.sh` and `deploy-*.sh` scripts set up the hel.ink production server and are examples, not an installer.

## Self-hosting

<!-- self-hosting:start -->
Everything Imperium needs is in public repositories: no private repository, no token and no hel.ink account. The family guide has the details: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md).

| Piece | For Imperium | What it does |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | **Required**: sign-in and the list of apps | Accounts and sign-in for every app |
| [Imperium](https://github.com/navi-crwn/Imperium) | This repository | Admin panel for the whole family; needs the hub |
| Family fonts and tiles | Nothing to install: loaded from `hel.ink/hop-assets` | Without hel.ink: system fonts, no floating tiles |

- **Install order**: the [hub](https://github.com/navi-crwn/HEL.ink), then the apps, then Imperium.
- **Minimum settings**: `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; at the hub: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Alone or in the family**: Imperium alone does nothing: it manages a hub and the apps registered there. Keep it off the open internet (VPN or access proxy).
<!-- self-hosting:end -->

## Languages

The interface language is set by the operator with `IMPERIUM_LANG` (see `deploy/imperium.env.example`). The default is `en`; `IMPERIUM_LANG` takes any of the nine codes listed below (for example `IMPERIUM_LANG=id` selects the Indonesian catalog). Unknown values fall back to English, and the page's `<html lang>` always matches the catalog in use.

Every page is translated into all nine languages of the HOP family (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`), and a test checks that each catalog has every key. Product names, HOP ID, 2FA, IP, JSON, slug and ISO are not translated.

## Notifications (HTTP 500 reports)

HOP apps report every HTTP 500 they serve to `POST /hop/notify` on Imperium's loopback listener, signed with the app's own registry secret (details, payload and limits: `protocol/PROTOCOL.md` section 10). Identical errors are grouped and shown on the **Notifications** page (the badge counts groups that are still new); an admin can acknowledge, resolve, mute (with a reason) or reopen a group, and every action is audited. Occurrences are kept 30 days, groups 90 days. Telegram gets at most one message per group per `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (default 60), never with user data. The database migrates itself on start (`schema_version`). To try it locally: `scripts/fakehop` plus `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## HOP family

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
| **Imperium** | not public-facing: optional, for admins | [navi-crwn/Imperium](https://github.com/navi-crwn/Imperium) | The HOP control panel. |
<!-- hop-family:end -->

## Security and license

Found a vulnerability? Report it privately through GitHub's security advisories for this repository: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). Imperium is released under the [MIT license](LICENSE).
