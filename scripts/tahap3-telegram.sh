#!/usr/bin/env bash
# Sambungkan notifikasi Imperium ke bot Telegram router-bot (grup & topik "alert" yang sama).
# Dijalankan pemilik sendiri (menyalin token dari ~/router-bot/bot.conf ke /etc/imperium/imperium.env):
#   sudo bash ~/imperium/scripts/tahap3-telegram.sh
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "jalankan dengan sudo"; exit 1; }
SRC=/home/carawin/router-bot/bot.conf; ENVF=/etc/imperium/imperium.env
val() { grep -E "^$1=" "$SRC" | head -1 | cut -d= -f2- | tr -d '"'"'"' '; }
TOK=$(val BOT_TOKEN); CHAT=$(val GROUP_CHAT_ID); THR=$(val THREAD_ALERT)
[ -n "$TOK" ] && [ -n "$CHAT" ] || { echo "BOT_TOKEN/GROUP_CHAT_ID tidak ditemukan di $SRC"; exit 1; }
sed -i '/^IMPERIUM_TELEGRAM_/d; /^IMPERIUM_WATCH_MINUTES=/d' "$ENVF"
printf 'IMPERIUM_TELEGRAM_TOKEN=%s\nIMPERIUM_TELEGRAM_CHAT=%s\nIMPERIUM_TELEGRAM_THREAD=%s\nIMPERIUM_WATCH_MINUTES=2\n' "$TOK" "$CHAT" "$THR" >> "$ENVF"
systemctl restart imperium && sleep 1 && curl -fsS http://127.0.0.1:8100/healthz && echo && echo "Telegram aktif untuk Imperium (grup router-bot, topik alert)."
