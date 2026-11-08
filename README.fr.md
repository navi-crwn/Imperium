# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · **Français** · [Русский](README.ru.md)

Le panneau d'administration et la console d'exploitation de la famille d'apps HOP (LinkHop, BioHop, PixelHop, NoteHop, FileHop). Il est open source comme le reste de la famille, et **facultatif** : chaque app fonctionne sans lui. Vous n'en avez besoin que si vous voulez un seul endroit pour gérer les utilisateurs, le contenu, les signalements d'abus, les réglages et l'état de santé de toutes vos apps.

Ce qu'il fait :

- Connecte les admins via HOP ID (le hub, [HEL.ink](https://github.com/navi-crwn/HEL.ink)) ; seuls les comptes avec un rôle admin peuvent entrer, avec une connexion à deux facteurs par défaut.
- Lit la liste des apps depuis le hub et communique avec chacune via son API d'administration signée (le HOP App Protocol dans `protocol/`).
- Affiche par app l'état de santé, les chiffres, les utilisateurs, le contenu, les signalements d'abus et les réglages ; regroupe les erreurs HTTP 500 signalées par les apps ; envoie des alertes Telegram facultatives.

Contenu de ce dépôt :

- `cmd/imperium/` : l'unique binaire serveur Go (interface web, client serveur à serveur signé)
- `internal/` : le code de l'application
- `protocol/` : le HOP App Protocol, le contrat de l'API d'administration et des webhooks que chaque app implémente
- `docs/` : notes d'architecture, liste des fonctionnalités et inventaires d'administration par app (rédigés en indonésien)
- `deploy/` : une unité systemd renforcée, un exemple d'environnement, et les fichiers nginx et de tunnel Cloudflare de l'installation de production de hel.ink
- `scripts/` : sauvegarde et restauration (`hop-backup.sh`, `hop-restore.sh`) et les scripts d'installation de la production de hel.ink (`tahap*.sh`, `deploy-*.sh`)

## Lancer Imperium

Il vous faut Go 1.27 ou plus récent, et un hub en fonctionnement avec au moins une app.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ caractères> ./bin/imperium
```

Sur le hub, mettez l'adresse d'Imperium dans `HOP_CONTROL_URL` et le même secret dans `HOP_SSO_SECRET_IMPERIUM`, puis lancez-y `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force` : cette commande enregistre Imperium comme une app cachée autorisée à appeler l'API d'administration. `make test` lance les tests.

Sur un serveur, utilisez `deploy/imperium.service` (systemd, avec son propre utilisateur et un système en lecture seule) et un fichier d'environnement comme `deploy/imperium.env.example`. Cet exemple contient les valeurs de production de hel.ink ; remplacez les adresses par les vôtres. Gardez Imperium hors de l'internet public : placez-le derrière un VPN (Tailscale, WireGuard) ou un proxy d'accès, et servez-le en HTTPS. Les scripts `tahap*.sh` et `deploy-*.sh` installent le serveur de production de hel.ink. Ce sont des exemples, pas un installateur.

## Auto-hébergement

<!-- self-hosting:start -->
Tout ce dont Imperium a besoin se trouve dans des dépôts publics : aucun dépôt privé, aucun jeton et aucun compte hel.ink. Les détails sont dans le guide de la famille : [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (en anglais).

| Élément | Pour Imperium | Rôle |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | **Obligatoire** : connexion et liste des apps | Comptes et connexion pour toutes les apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Ce dépôt | Panneau d'administration de toute la famille ; nécessite le hub |
| Polices et tuiles de la famille | Rien à installer : chargés depuis `hel.ink/hop-assets` | Sans hel.ink : polices système, pas de tuiles flottantes |

- **Ordre d'installation** : le [hub](https://github.com/navi-crwn/HEL.ink), puis les apps, puis Imperium.
- **Réglages minimaux** : `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; sur le hub: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Seul ou en famille** : Imperium seul ne fait rien : il gère un hub et les apps qui y sont enregistrées. Gardez-le hors de l'internet ouvert (VPN ou proxy d'accès).
<!-- self-hosting:end -->

## Langues

La langue de l'interface est choisie par l'opérateur avec `IMPERIUM_LANG` (voir `deploy/imperium.env.example`). La valeur par défaut est `en` ; `IMPERIUM_LANG=id` sélectionne le catalogue indonésien. Les valeurs inconnues reviennent à l'anglais, et le `<html lang>` de la page correspond toujours au catalogue utilisé.

Les pages Notifications sont traduites dans les neuf langues de la famille HOP (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). Les pages plus anciennes n'existent pour l'instant qu'en `en` et `id` ; un texte absent dans une langue s'affiche en anglais.

## Notifications (rapports HTTP 500)

Les apps HOP signalent chaque HTTP 500 qu'elles servent à `POST /hop/notify` sur l'écouteur loopback d'Imperium, signé avec le secret de registre propre à l'app (détails, payload et limites : `protocol/PROTOCOL.md` section 10). Les erreurs identiques sont regroupées et affichées sur la page **Notifications** (le badge compte les groupes encore nouveaux) ; un admin peut prendre en compte, résoudre, mettre en sourdine (avec un motif) ou rouvrir un groupe, et chaque action est journalisée pour l'audit. Les occurrences sont conservées 30 jours, les groupes 90 jours. Telegram reçoit au plus un message par groupe par période de `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (60 par défaut), jamais avec des données d'utilisateur. La base de données se migre d'elle-même au démarrage (`schema_version`). Pour l'essayer en local : `scripts/fakehop` plus `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## Famille HOP

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

## Sécurité et licence

Vous avez trouvé une vulnérabilité ? Signalez-la en privé via les avis de sécurité (security advisories) GitHub de ce dépôt : [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). Imperium est publié sous [licence MIT](LICENSE).
