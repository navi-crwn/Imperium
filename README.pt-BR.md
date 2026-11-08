# Imperium

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Bahasa Indonesia](README.id.md) · [日本語](README.ja.md) · **Português (Brasil)** · [Español](README.es.md) · [Français](README.fr.md) · [Русский](README.ru.md)

O painel de administração e console de operações da família de apps HOP (LinkHop, BioHop, PixelHop, NoteHop, FileHop). Ele é open source como o resto da família, e **opcional**: todo app funciona sem ele. Você só precisa dele se quiser um único lugar para gerenciar os usuários, o conteúdo, as denúncias de abuso, as configurações e a saúde de todos os seus apps.

O que ele faz:

- Faz o login dos admins pelo HOP ID (o hub, [HEL.ink](https://github.com/navi-crwn/HEL.ink)); só entram contas com papel de admin, com login em dois fatores por padrão.
- Lê a lista de apps do hub e conversa com cada um pela API de administração assinada dele (o HOP App Protocol em `protocol/`).
- Mostra saúde, números, usuários, conteúdo, denúncias de abuso e configurações por app; agrupa os erros HTTP 500 que os apps reportam; envia alertas opcionais pelo Telegram.

O que há neste repositório:

- `cmd/imperium/`: o binário único do servidor Go (interface web, cliente servidor-a-servidor assinado)
- `internal/`: o código da aplicação
- `protocol/`: o HOP App Protocol, o contrato da API de administração e dos webhooks que todo app implementa
- `docs/`: notas de arquitetura, lista de recursos e inventários de administração por app (escritos em indonésio)
- `deploy/`: uma unidade systemd reforçada, um exemplo de ambiente e os arquivos de nginx e de túnel Cloudflare da instalação de produção do hel.ink
- `scripts/`: backup e restauração (`hop-backup.sh`, `hop-restore.sh`) e os scripts de configuração da produção do hel.ink (`tahap*.sh`, `deploy-*.sh`)

## Como executar

Você precisa do Go 1.27 ou mais recente e de um hub em execução com pelo menos um app.

```bash
git clone https://github.com/navi-crwn/Imperium.git && cd Imperium
make build                       # writes bin/imperium
IMPERIUM_ADDR=127.0.0.1:8100 IMPERIUM_DB=imperium.db \
IMPERIUM_BASE_URL=http://127.0.0.1:8100 HOP_URL=http://localhost:8000 \
HOP_SSO_CLIENT=imperium HOP_SSO_SECRET=<32+ caracteres> ./bin/imperium
```

No hub, defina `HOP_CONTROL_URL` com o endereço do Imperium e `HOP_SSO_SECRET_IMPERIUM` com o mesmo segredo. Depois rode `php artisan db:seed --class='Database\Seeders\HopAppsSeeder' --force` lá: isso registra o Imperium como um app oculto que pode chamar a API de administração. `make test` roda os testes.

Em um servidor, use `deploy/imperium.service` (systemd, com usuário próprio e sistema somente leitura) e um arquivo de ambiente como `deploy/imperium.env.example`. Esse exemplo traz os valores de produção do hel.ink; troque os endereços pelos seus. Mantenha o Imperium fora da internet aberta: coloque-o atrás de uma VPN (Tailscale, WireGuard) ou de um proxy de acesso, e sirva-o por HTTPS. Os scripts `tahap*.sh` e `deploy-*.sh` configuram o servidor de produção do hel.ink. Eles são exemplos, não um instalador.

## Hospedagem própria

<!-- self-hosting:start -->
Tudo o que o Imperium precisa está em repositórios públicos: nenhum repositório privado, nenhum token e nenhuma conta no hel.ink. Os detalhes estão no guia da família: [docs/self-hosting.md](https://github.com/navi-crwn/HEL.ink/blob/main/docs/self-hosting.md) (em inglês).

| Peça | Para o Imperium | O que faz |
| --- | --- | --- |
| [HEL.ink](https://github.com/navi-crwn/HEL.ink) (hub, HOP ID) | **Obrigatório**: login e lista de apps | Contas e login para todos os apps |
| [Imperium](https://github.com/navi-crwn/Imperium) | Este repositório | Painel de administração de toda a família; precisa do hub |
| Fontes e blocos da família | Nada a instalar: carregados de `hel.ink/hop-assets` | Sem hel.ink: fontes do sistema, sem blocos flutuantes |

- **Ordem de instalação**: o [hub](https://github.com/navi-crwn/HEL.ink), depois os apps, depois o Imperium.
- **Configuração mínima**: `IMPERIUM_BASE_URL`, `IMPERIUM_DB`, `HOP_URL`, `HOP_SSO_CLIENT=imperium`, `HOP_SSO_SECRET`; no hub: `HOP_CONTROL_URL`, `HOP_SSO_SECRET_IMPERIUM`.
- **Sozinho ou na família**: o Imperium sozinho não faz nada: ele gerencia um hub e os apps registrados nele. Mantenha-o fora da internet aberta (VPN ou proxy de acesso).
<!-- self-hosting:end -->

## Idiomas

O idioma da interface é definido pelo operador com `IMPERIUM_LANG` (veja `deploy/imperium.env.example`). O padrão é `en`; `IMPERIUM_LANG=id` seleciona o catálogo em indonésio. Valores desconhecidos voltam para o inglês, e o `<html lang>` da página sempre corresponde ao catálogo em uso.

As páginas de Notificações estão traduzidas para os nove idiomas da família HOP (`en`, `zh-CN`, `de`, `id`, `ja`, `pt-BR`, `es`, `fr`, `ru`). As páginas mais antigas existem só em `en` e `id` por enquanto; um texto que falta em um idioma aparece em inglês.

## Notificações (relatórios de HTTP 500)

Os apps HOP reportam cada HTTP 500 que servem para `POST /hop/notify` no listener de loopback do Imperium, assinado com o segredo de registro do próprio app (detalhes, payload e limites: `protocol/PROTOCOL.md` seção 10). Erros idênticos são agrupados e mostrados na página **Notificações** (o selo conta os grupos que ainda são novos); um admin pode reconhecer, resolver, silenciar (com um motivo) ou reabrir um grupo, e toda ação fica registrada na auditoria. As ocorrências são mantidas por 30 dias, os grupos por 90 dias. O Telegram recebe no máximo uma mensagem por grupo a cada `IMPERIUM_NOTIFY_COOLDOWN_MINUTES` (padrão 60), nunca com dados de usuários. O banco de dados se migra sozinho ao iniciar (`schema_version`). Para testar localmente: `scripts/fakehop` mais `NOTIFY_SECRET=... go run ./scripts/notifysend`.

## Família HOP

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

## Segurança e licença

Encontrou uma vulnerabilidade? Reporte-a de forma privada pelos alertas de segurança (security advisories) do GitHub deste repositório: [github.com/navi-crwn/Imperium/security/advisories](https://github.com/navi-crwn/Imperium/security/advisories). O Imperium é distribuído sob a [licença MIT](LICENSE).
