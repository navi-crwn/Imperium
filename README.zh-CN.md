# Imperium

[English](README.md) · **简体中文** · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [Français](README.fr.md) · [Русский](README.ru.md)

HOP 应用家族（LinkHop、BioHop、PixelHop、NoteHop、FileHop）的管理面板和运维控制台。它和家族中的其他应用一样开源，并且是**可选的**：每个应用都可以在没有它的情况下运行。只有当你想在一个地方管理所有应用的用户、内容、滥用举报、设置和运行状况时，才需要它。

它的功能：

- 通过 HOP ID（即中心服务，[HEL.ink](https://github.com/navi-crwn/HEL.ink)）为管理员登录；只有具备管理员角色的账号才能进入，默认启用双因素登录。
- 从中心服务读取应用列表，并通过每个应用的签名管理 API（`protocol/` 中的 HOP App Protocol）与其通信。
- 按应用显示运行状况、统计数字、用户、内容、滥用举报和设置；将各应用上报的 HTTP 500 错误分组；可选发送 Telegram 提醒。

本仓库包含：

- `cmd/imperium/`：单个 Go 服务器二进制文件（Web 界面、带签名的服务器间客户端）
- `internal/`：应用代码
- `protocol/`：HOP App Protocol，即每个应用都要实现的管理 API 和 webhook 约定
- `docs/`：架构说明、功能列表和各应用的管理清单（以印尼语撰写）
- `deploy/`：一个加固的 systemd 单元、一个环境变量示例，以及 hel.ink 生产环境的 nginx 和 Cloudflare 隧道文件
- `scripts/`：备份与恢复（`hop-backup.sh`、`hop-restore.sh`），以及 hel.ink 生产环境的部署脚本（`tahap*.sh`、`deploy-*.sh`）

## 运行

你需要 Go 1.27 或更新版本，以及一个正在运行、至少带有一个应用的中心服务。

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32 个以上字符> ./bin/imperium
```

在中心服务上，把 `HOP_CONTROL_URL` 设为 Imperium 的地址，把 `HOP_SSO_SECRET_IMPERIUM` 设为同一个密钥，然后在那里运行 `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force`：它会把 Imperium 注册为一个可以调用管理 API 的隐藏应用。`make test` 用于运行测试。

在服务器上，请使用 `deploy/imperium.service`（systemd，使用独立用户和只读系统）以及一个类似 `deploy/imperium.env.example` 的环境文件。该示例包含 hel.ink 生产环境的取值；请把其中的地址换成你自己的。不要让 Imperium 暴露在公网上：把它放在 VPN（Tailscale、WireGuard）或访问代理之后，并通过 HTTPS 提供服务。`tahap*.sh` 和 `deploy-*.sh` 脚本用于搭建 hel.ink 生产服务器，它们是示例，不是安装程序。

## 自行托管

<!-- self-hosting:start -->
Imperium 所需的一切都在公开仓库中：不需要私有仓库、令牌或 hel.ink 账户。详细说明见家族指南：[docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md)（英文）。

| 组件 | 对 Imperium 而言 | 作用 |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (中心，HOP ID) | **必需**：登录与应用列表都来自它 | 所有应用的账户与登录 |
| [Imperium](https://github.com/navi-crwn/Imperium) | 就是本仓库 | 整个家族的管理面板；需要中心 |
| 家族字体与浮动图块 | 无需安装：从 `hel.ink/hop-assets` 加载 | 无法访问 hel.ink 时：使用系统字体，没有浮动图块 |

- **安装顺序**：先装[中心](https://github.com/navi-crwn/HEL.ink)，再装各应用，最后装 Imperium。
- **最少设置**：`IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; 在中心上: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`。
- **单独运行或加入家族**：Imperium 单独运行没有用处：它管理一个中心及其登记的应用。不要把它暴露在公网上（使用 VPN 或访问代理）。
<!-- self-hosting:end -->

## 语言

界面语言由运维者通过 `IMPERIUM_LANG` 设置（见 `deploy/imperium.env.example`）。默认值为 `en`；`IMPERIUM_LANG=id` 选择印尼语文本目录。未知的值会回退到英语，页面的 `<html lang>` 始终与正在使用的文本目录一致。

通知页面已翻译成 HOP 家族的全部九种语言（`en`、`zh-CN`、`de`、`id`、`ja`、`pt-BR`、`es`、`fr`、`ru`）。较早的页面目前只有 `en` 和 `id`；某种语言缺少的文本会回退到英语。

## 通知（HTTP 500 报告）

HOP 应用会把它们返回的每个 HTTP 500 报告到 Imperium 回环监听地址上的 `POST /hop/notify`，并用该应用自己的注册表密钥签名（细节、载荷和限制：`protocol/PROTOCOL.md` 第 10 节）。相同的错误会被分组，并显示在**通知**页面上（徽标计数的是仍为新状态的分组）；管理员可以确认、解决、静音（需填写原因）或重新打开一个分组，每个操作都会记入审计日志。单次发生记录保留 30 天，分组保留 90 天。每个分组在每个 `IMPERIUM_NOTIFY_COOLDOWN_MINUTES`（默认 60）内最多向 Telegram 发送一条消息，且绝不包含用户数据。数据库在启动时自动迁移（`schema_version`）。在本地试用：`scripts/fakehop` 加上 `NOTIFY_SECRET=... go run ./scripts/notifysend`。

## HOP 家族

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

## 安全与许可证

发现了漏洞？请通过本仓库在 GitHub 上的安全公告功能私下报告：[github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories)。Imperium 以 [MIT 许可证](LICENSE)发布。
