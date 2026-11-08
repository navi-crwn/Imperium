# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · **Español** · [Français](README.fr.md) · [Русский](README.ru.md)

El panel de administración y la consola de operaciones de la familia de apps HOP (LinkHop, BioHop, PixelHop, NoteHop, FileHop). Es de código abierto como el resto de la familia, y **opcional**: cada app funciona sin él. Solo lo necesitas si quieres un único lugar para gestionar los usuarios, el contenido, las denuncias de abuso, los ajustes y el estado de todas tus apps.

Qué hace:

- Inicia la sesión de los administradores con HOP ID (el hub, [HEL.ink](https://github.com/navi-crwn/HEL.ink)); solo entran las cuentas con rol de administrador, con inicio de sesión en dos pasos por defecto.
- Lee la lista de apps desde el hub y se comunica con cada una a través de su API de administración firmada (el HOP App Protocol en `protocol/`).
- Muestra por app el estado, las cifras, los usuarios, el contenido, las denuncias de abuso y los ajustes; agrupa los errores HTTP 500 que informan las apps; envía alertas opcionales por Telegram.

Qué hay en este repositorio:

- `cmd/imperium/`: el binario único del servidor Go (interfaz web, cliente servidor a servidor firmado)
- `internal/`: el código de la aplicación
- `protocol/`: el HOP App Protocol, el contrato de la API de administración y de los webhooks que implementa cada app
- `docs/`: notas de arquitectura, lista de funciones e inventarios de administración por app (escritos en indonesio)
- `deploy/`: una unidad systemd reforzada, un ejemplo de entorno y los archivos de nginx y del túnel de Cloudflare de la instalación de producción de hel.ink
- `scripts/`: copia de seguridad y restauración (`hop-backup.sh`, `hop-restore.sh`) y los scripts de configuración de la producción de hel.ink (`tahap*.sh`, `deploy-*.sh`)

## Ejecutarlo

Necesitas Go 1.27 o posterior, y un hub en marcha con al menos una app.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ caracteres> ./bin/imperium
```

En el hub, pon en `HOP_CONTROL_URL` la dirección de Imperium y en `HOP_SSO_SECRET_IMPERIUM` el mismo secreto. Después ejecuta allí `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force`: registra Imperium como una app oculta que puede llamar a la API de administración. `make test` ejecuta las pruebas.

En un servidor, usa `deploy/imperium.service` (systemd, con su propio usuario y un sistema de solo lectura) y un archivo de entorno como `deploy/imperium.env.example`. Ese ejemplo lleva los valores de producción de hel.ink; cambia las direcciones por las tuyas. Mantén Imperium fuera del internet abierto: ponlo detrás de una VPN (Tailscale, WireGuard) o de un proxy de acceso, y sírvelo por HTTPS. Los scripts `tahap*.sh` y `deploy-*.sh` configuran el servidor de producción de hel.ink. Son ejemplos, no un instalador.

## Autoalojamiento

<!-- self-hosting:start -->
Todo lo que Imperium necesita está en repositorios públicos: ningún repositorio privado, ningún token y ninguna cuenta en hel.ink. Los detalles están en la guía de la familia: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (en inglés).

| Pieza | Para Imperium | Qué hace |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | **Obligatorio**: inicio de sesión y lista de apps | Cuentas e inicio de sesión para todas las apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Este repositorio | Panel de administración de toda la familia; necesita el hub |
| Fuentes y mosaicos de la familia | Nada que instalar: se cargan desde `hel.ink/hop-assets` | Sin hel.ink: fuentes del sistema, sin mosaicos flotantes |

- **Orden de instalación**: el [hub](https://github.com/navi-crwn/HEL.ink), luego las apps, luego Imperium.
- **Ajustes mínimos**: `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; en el hub: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Solo o en la familia**: Imperium solo no hace nada: gestiona un hub y las apps registradas en él. Mantenlo fuera de internet abierto (VPN o proxy de acceso).
<!-- self-hosting:end -->

## Idiomas

El operador elige el idioma de la interfaz con `IMPERIUM_LANG` (consulta `deploy/imperium.env.example`). El valor por defecto es `en`; `IMPERIUM_LANG=id` selecciona el catálogo en indonesio. Los valores desconocidos vuelven al inglés, y el `<html lang>` de la página siempre coincide con el catálogo en uso.

Las páginas de Notificaciones están traducidas a los nueve idiomas de la familia HOP (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). Las páginas más antiguas solo existen en `en` e `id` por ahora; un texto que falta en un idioma se muestra en inglés.

## Notificaciones (informes de HTTP 500)

Las apps HOP informan de cada HTTP 500 que sirven a `POST /hop/notify` en el listener de loopback de Imperium, firmado con el secreto de registro de la propia app (detalles, payload y límites: `protocol/PROTOCOL.md` sección 10). Los errores idénticos se agrupan y se muestran en la página **Notificaciones** (la insignia cuenta los grupos que siguen siendo nuevos); un administrador puede confirmar, resolver, silenciar (con un motivo) o reabrir un grupo, y cada acción queda auditada. Las ocurrencias se guardan 30 días y los grupos 90 días. Telegram recibe como máximo un mensaje por grupo cada `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (por defecto 60), nunca con datos de usuarios. La base de datos se migra sola al arrancar (`schema_version`). Para probarlo en local: `scripts/fakehop` más `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## Familia HOP

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

## Seguridad y licencia

¿Has encontrado una vulnerabilidad? Infórmala en privado mediante los avisos de seguridad (security advisories) de GitHub de este repositorio: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). Imperium se publica bajo la [licencia MIT](LICENSE).
