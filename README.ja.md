# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · **日本語** · [Português (Brasil)](README.pt-BR.md) · [Español](README.es.md) · [Français](README.fr.md) · [Русский](README.ru.md)

HOP アプリファミリー（LinkHop、BioHop、PixelHop、NoteHop、FileHop、PayHop）のための管理パネル兼運用コンソールです。ファミリーのほかのアプリと同じくオープンソースで、**任意**です。どのアプリも Imperium なしで動きます。すべてのアプリのユーザー、コンテンツ、不正利用の報告、設定、稼働状況を一か所で管理したい場合にだけ必要です。

できること:

- HOP ID（ハブ、[HEL.ink](https://github.com/navi-crwn/HEL.ink)）で管理者をサインインさせます。管理者ロールを持つアカウントだけが入れ、二要素サインインが既定で有効です。
- ハブからアプリの一覧を読み込み、各アプリの署名付き管理 API（`protocol/` にある HOP App Protocol）を通じて各アプリと通信します。
- アプリごとに稼働状況、数値、ユーザー、コンテンツ、不正利用の報告、設定を表示します。アプリが報告する HTTP 500 エラーをグループにまとめます。必要に応じて Telegram で通知を送ります。

このリポジトリの内容:

- `cmd/imperium/`: 単一の Go サーバーバイナリ（Web UI、署名付きのサーバー間クライアント）
- `internal/`: アプリケーションのコード
- `protocol/`: HOP App Protocol。すべてのアプリが実装する管理 API と webhook の取り決め
- `docs/`: アーキテクチャのメモ、機能一覧、アプリごとの管理項目一覧（インドネシア語）
- `deploy/`: 強化した systemd ユニット、環境設定の例、hel.ink 本番環境の nginx と Cloudflare トンネルのファイル
- `scripts/`: バックアップと復元（`hop-backup.sh`、`hop-restore.sh`）、hel.ink 本番環境のセットアップスクリプト（`tahap*.sh`、`deploy-*.sh`）

## 実行する

Go 1.27 以降と、少なくとも 1 つのアプリを持つ稼働中のハブが必要です。

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32 文字以上> ./bin/imperium
```

ハブ側で `HOP_CONTROL_URL` に Imperium のアドレスを、`HOP_SSO_SECRET_IMPERIUM` に同じシークレットを設定し、そこで `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force` を実行します。これで Imperium が、管理 API を呼び出せる非表示のアプリとして登録されます。`make test` でテストを実行します。

サーバーでは `deploy/imperium.service`（systemd、専用ユーザーと読み取り専用のシステム）と、`deploy/imperium.env.example` のような環境ファイルを使います。この例には hel.ink 本番環境の値が入っているので、アドレスを自分のものに置き換えてください。Imperium はインターネットに公開しないでください。VPN（Tailscale、WireGuard）やアクセスプロキシの背後に置き、HTTPS で提供します。`tahap*.sh` と `deploy-*.sh` のスクリプトは hel.ink の本番サーバーを構築するためのもので、インストーラーではなく例です。

## セルフホスティング

<!-- self-hosting:start -->
Imperium に必要なものはすべて公開リポジトリにあります。非公開リポジトリ、トークン、hel.ink のアカウントは不要です。詳しくはファミリーガイド [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md)（英語）を参照してください。

| 構成要素 | Imperium にとって | 役割 |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (ハブ、HOP ID) | **必須**：サインインとアプリ一覧 | すべてのアプリのアカウントとサインイン |
| [Imperium](https://github.com/navi-crwn/Imperium) | このリポジトリ | ファミリー全体の管理パネル。ハブが必要 |
| ファミリーのフォントとタイル | インストール不要：`hel.ink/hop-assets` から読み込み | hel.ink がなければシステムフォント、浮遊タイルなし |

- **インストール順**：[ハブ](https://github.com/navi-crwn/HEL.ink)、次にアプリ、最後に Imperium。
- **最小限の設定**：`IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; ハブ側: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`。
- **単体かファミリーか**：Imperium 単体では何もしません。ハブとそこに登録されたアプリを管理します。インターネットに直接公開しないでください（VPN かアクセスプロキシ）。
<!-- self-hosting:end -->

## 言語

画面の言語は運用者が `IMPERIUM_LANG` で設定します（`deploy/imperium.env.example` を参照）。既定値は `en` で、`IMPERIUM_LANG=id` を指定するとインドネシア語のカタログが選ばれます。不明な値は英語にフォールバックし、ページの `<html lang>` は常に使用中のカタログと一致します。

通知ページは HOP ファミリーの 9 言語すべて（`en`、`zh-CN`、`de`、`id`、`ja`、`pt-BR`、`es`、`fr`、`ru`）に翻訳されています。古いページは今のところ `en` と `id` のみです。ある言語に足りない文言は英語にフォールバックします。

## 通知（HTTP 500 レポート）

HOP アプリは、返したすべての HTTP 500 を Imperium のループバックリスナー上の `POST /hop/notify` に報告します。報告にはそのアプリ自身のレジストリシークレットで署名します（詳細、ペイロード、制限: `protocol/PROTOCOL.md` の 10 節）。同一のエラーはグループにまとめられ、**通知**ページに表示されます（バッジはまだ新しいグループの数を示します）。管理者はグループを確認済み、解決済み、ミュート（理由付き）にしたり、再オープンしたりでき、すべての操作が監査ログに記録されます。個々の発生は 30 日、グループは 90 日保存されます。Telegram へ送るメッセージは、グループごとに `IMPERIUM_NOTIFY_COOLDOWN_MINUTES`（既定 60）あたり最大 1 通で、ユーザーデータは決して含みません。データベースは起動時に自動でマイグレーションされます（`schema_version`）。ローカルで試すには: `scripts/fakehop` と `NOTIFY_SECRET=... go run ./scripts/notifysend`。

## HOP ファミリー

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
| **PayHop** | [pay.hel.ink](https://pay.hel.ink) | [navi-crwn/PayHop](https://github.com/navi-crwn/PayHop) | Make a payment QR and split a bill; nothing leaves your browser. |
| **Imperium** | not public-facing: optional, for admins | [navi-crwn/Imperium](https://github.com/navi-crwn/Imperium) | The HOP control panel. |
<!-- hop-family:end -->

## セキュリティとライセンス

脆弱性を見つけたら、このリポジトリの GitHub セキュリティアドバイザリから非公開で報告してください: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories)。Imperium は [MIT ライセンス](LICENSE)で公開されています。
