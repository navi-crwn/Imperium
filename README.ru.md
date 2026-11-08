# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [Français](README.fr.md) · **Русский**

Панель администратора и консоль эксплуатации для семейства приложений HOP (LinkHop, BioHop, PixelHop, NoteHop, FileHop). Как и остальные приложения семейства, Imperium открыт и **необязателен**: каждое приложение работает без него. Он нужен, только если вы хотите управлять пользователями, контентом, жалобами на нарушения, настройками и состоянием всех своих приложений из одного места.

Что он делает:

- Выполняет вход администраторов через HOP ID (хаб, [HEL.ink](https://github.com/navi-crwn/HEL.ink)); войти могут только учётные записи с ролью администратора, по умолчанию с двухфакторным входом.
- Получает список приложений из хаба и общается с каждым через его подписанный API администрирования (HOP App Protocol в `protocol/`).
- Показывает по каждому приложению состояние, показатели, пользователей, контент, жалобы на нарушения и настройки; группирует ошибки HTTP 500, о которых сообщают приложения; при желании отправляет оповещения в Telegram.

Что есть в этом репозитории:

- `cmd/imperium/`: единый бинарный файл сервера на Go (веб-интерфейс, подписанный клиент для связи сервер–сервер)
- `internal/`: код приложения
- `protocol/`: HOP App Protocol — контракт API администрирования и webhook, который реализует каждое приложение
- `docs/`: заметки об архитектуре, список функций и перечни админ-функций по приложениям (на индонезийском)
- `deploy/`: усиленный юнит systemd, пример окружения, а также файлы nginx и туннеля Cloudflare из продакшен-установки hel.ink
- `scripts/`: резервное копирование и восстановление (`hop-backup.sh`, `hop-restore.sh`) и скрипты настройки продакшена hel.ink (`tahap*.sh`, `deploy-*.sh`)

## Запуск

Нужен Go 1.27 или новее и работающий хаб хотя бы с одним приложением.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ символа> ./bin/imperium
```

На хабе укажите в `HOP_CONTROL_URL` адрес Imperium, а в `HOP_SSO_SECRET_IMPERIUM` тот же секрет, затем выполните там `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force`: команда регистрирует Imperium как скрытое приложение, которому разрешено вызывать API администрирования. `make test` запускает тесты.

На сервере используйте `deploy/imperium.service` (systemd, с отдельным пользователем и системой только для чтения) и файл окружения по образцу `deploy/imperium.env.example`. В этом примере стоят продакшен-значения hel.ink; замените адреса своими. Не открывайте Imperium в интернет: разместите его за VPN (Tailscale, WireGuard) или прокси доступа и отдавайте по HTTPS. Скрипты `tahap*.sh` и `deploy-*.sh` настраивают продакшен-сервер hel.ink. Это примеры, а не установщик.

## Собственный хостинг

<!-- self-hosting:start -->
Всё, что нужно Imperium, лежит в публичных репозиториях: не нужны ни приватный репозиторий, ни токен, ни учётная запись на hel.ink. Подробности — в руководстве по семейству: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (на английском).

| Компонент | Для Imperium | Зачем |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (хаб, HOP ID) | **Обязательно**: вход и список приложений | Учётные записи и вход для всех приложений |
| [Imperium](https://github.com/navi-crwn/Imperium) | Этот репозиторий | Панель администратора для всего семейства; нужен хаб |
| Шрифты и плитки семейства | Ничего устанавливать не нужно: загружаются с `hel.ink/hop-assets` | Без hel.ink: системные шрифты, без плавающих плиток |

- **Порядок установки**: [хаб](https://github.com/navi-crwn/HEL.ink), затем приложения, затем Imperium.
- **Минимальные настройки**: `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; на хабе: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Отдельно или в семействе**: Imperium сам по себе ничего не делает: он управляет хабом и зарегистрированными в нём приложениями. Не открывайте его в интернет (VPN или прокси доступа).
<!-- self-hosting:end -->

## Языки

Язык интерфейса задаёт оператор через `IMPERIUM_LANG` (см. `deploy/imperium.env.example`). По умолчанию используется `en`; `IMPERIUM_LANG=id` выбирает индонезийский каталог. Неизвестные значения откатываются на английский, а `<html lang>` страницы всегда соответствует используемому каталогу.

Страницы уведомлений переведены на все девять языков семейства HOP (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). Более старые страницы пока есть только на `en` и `id`; если в языке нет какого-то текста, он показывается на английском.

## Уведомления (отчёты о HTTP 500)

Приложения HOP сообщают о каждом отданном HTTP 500 на `POST /hop/notify` на loopback-адресе Imperium и подписывают отчёт собственным секретом приложения из реестра (подробности, формат данных и ограничения: `protocol/PROTOCOL.md`, раздел 10). Одинаковые ошибки группируются и показываются на странице **Уведомления** (значок считает группы, которые ещё новые); администратор может подтвердить, закрыть как решённую, заглушить (с указанием причины) или снова открыть группу, и каждое действие попадает в журнал аудита. Отдельные случаи хранятся 30 дней, группы — 90 дней. Telegram получает не больше одного сообщения на группу за `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (по умолчанию 60), и никогда с данными пользователей. База данных сама выполняет миграции при запуске (`schema_version`). Чтобы попробовать локально: `scripts/fakehop` и `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## Семейство HOP

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

## Безопасность и лицензия

Нашли уязвимость? Сообщите о ней приватно через security advisories GitHub для этого репозитория: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). Imperium распространяется по [лицензии MIT](LICENSE).
