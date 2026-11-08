# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · **Deutsch** · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [Français](README.fr.md) · [Русский](README.ru.md)

Das Admin-Panel und die Betriebskonsole für die HOP-App-Familie (LinkHop, BioHop, PixelHop, NoteHop, FileHop). Wie der Rest der Familie ist es Open Source und **optional**: Jede App läuft auch ohne. Du brauchst es nur, wenn du Nutzer, Inhalte, Missbrauchsmeldungen, Einstellungen und den Zustand all deiner Apps an einem Ort verwalten willst.

Was es tut:

- Meldet Admins über HOP ID an (den Hub, [HEL.ink](https://github.com/navi-crwn/HEL.ink)); nur Konten mit Admin-Rolle kommen hinein, standardmäßig mit Zwei-Faktor-Anmeldung.
- Liest die Liste der Apps vom Hub und spricht mit jeder App über ihre signierte Admin-API (das HOP App Protocol in `protocol/`).
- Zeigt pro App Zustand, Kennzahlen, Nutzer, Inhalte, Missbrauchsmeldungen und Einstellungen; gruppiert die HTTP-500-Fehler, die die Apps melden; sendet auf Wunsch Benachrichtigungen über Telegram.

Was in diesem Repository liegt:

- `cmd/imperium/`: die einzelne Go-Server-Binary (Web-UI, signierter Server-zu-Server-Client)
- `internal/`: der Anwendungscode
- `protocol/`: das HOP App Protocol, der Vertrag für Admin-API und Webhooks, den jede App umsetzt
- `docs/`: Architekturnotizen, Funktionsliste und Admin-Inventare pro App (auf Indonesisch)
- `deploy/`: eine gehärtete systemd-Unit, ein Beispiel für die Umgebung sowie die nginx- und Cloudflare-Tunnel-Dateien des hel.ink-Produktionssetups
- `scripts/`: Backup und Wiederherstellung (`hop-backup.sh`, `hop-restore.sh`) und die Einrichtungsskripte der hel.ink-Produktion (`tahap*.sh`, `deploy-*.sh`)

## Starten

Du brauchst Go 1.27 oder neuer und einen laufenden Hub mit mindestens einer App.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ Zeichen> ./bin/imperium
```

Setze im Hub `HOP_CONTROL_URL` auf die Adresse von Imperium und `HOP_SSO_SECRET_IMPERIUM` auf dasselbe Secret. Führe dort dann `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force` aus: Damit wird Imperium als versteckte App registriert, die die Admin-API aufrufen darf. `make test` führt die Tests aus.

Auf einem Server nimmst du `deploy/imperium.service` (systemd, mit eigenem Benutzer und schreibgeschütztem System) und eine Umgebungsdatei wie `deploy/imperium.env.example`. Dieses Beispiel enthält die Produktionswerte von hel.ink; ersetze die Adressen durch deine eigenen. Halte Imperium vom offenen Internet fern: Stelle es hinter ein VPN (Tailscale, WireGuard) oder einen Zugangs-Proxy und liefere es über HTTPS aus. Die Skripte `tahap*.sh` und `deploy-*.sh` richten den hel.ink-Produktionsserver ein. Sie sind Beispiele, kein Installer.

## Selbst hosten

<!-- self-hosting:start -->
Alles, was Imperium braucht, liegt in öffentlichen Repositorys: kein privates Repository, kein Token und kein hel.ink-Konto. Details stehen in der Familienanleitung: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (Englisch).

| Baustein | Für Imperium | Wozu |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (Hub, HOP ID) | **Erforderlich**: Anmeldung und App-Liste | Konten und Anmeldung für alle Apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Dieses Repository | Admin-Panel für die ganze Familie; braucht den Hub |
| Familien-Schriften und -Kacheln | Nichts zu installieren: kommen von `hel.ink/hop-assets` | Ohne hel.ink: Systemschriften, keine schwebenden Kacheln |

- **Reihenfolge**: der [Hub](https://github.com/navi-crwn/HEL.ink), dann die Apps, dann Imperium.
- **Mindest-Einstellungen**: `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; im Hub: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Allein oder in der Familie**: Imperium allein tut nichts: Es verwaltet einen Hub und die dort registrierten Apps. Halte es vom offenen Internet fern (VPN oder Zugangs-Proxy).
<!-- self-hosting:end -->

## Sprachen

Die Sprache der Oberfläche legt der Betreiber mit `IMPERIUM_LANG` fest (siehe `deploy/imperium.env.example`). Standard ist `en`; `IMPERIUM_LANG=id` wählt den indonesischen Katalog. Unbekannte Werte fallen auf Englisch zurück, und `<html lang>` der Seite passt immer zum verwendeten Katalog.

Die Benachrichtigungsseiten sind in alle neun Sprachen der HOP-Familie übersetzt (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). Die älteren Seiten gibt es bisher nur auf `en` und `id`; fehlt einer Sprache ein Text, wird Englisch angezeigt.

## Benachrichtigungen (HTTP-500-Meldungen)

HOP-Apps melden jeden HTTP-500-Fehler, den sie ausliefern, an `POST /hop/notify` auf dem Loopback-Listener von Imperium, signiert mit dem eigenen Registry-Secret der App (Details, Payload und Limits: `protocol/PROTOCOL.md` Abschnitt 10). Gleiche Fehler werden gruppiert und auf der Seite **Benachrichtigungen** angezeigt (das Badge zählt Gruppen, die noch neu sind). Ein Admin kann eine Gruppe bestätigen, als gelöst markieren, stummschalten (mit Begründung) oder wieder öffnen, und jede Aktion wird protokolliert. Vorkommen werden 30 Tage aufbewahrt, Gruppen 90 Tage. Telegram bekommt höchstens eine Nachricht pro Gruppe pro `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (Standard 60), nie mit Nutzerdaten. Die Datenbank migriert sich beim Start selbst (`schema_version`). Zum lokalen Ausprobieren: `scripts/fakehop` plus `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## HOP-Familie

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

## Sicherheit und Lizenz

Eine Schwachstelle gefunden? Melde sie vertraulich über die Security Advisories von GitHub für dieses Repository: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). Imperium steht unter der [MIT-Lizenz](LICENSE).
