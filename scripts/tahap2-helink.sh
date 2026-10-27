#!/usr/bin/env bash
# Tahap 2, sisi LinkHop (tanpa sudo): tambah secret Imperium + citadel ke .env live,
# bangun ulang cache config, daftarkan keduanya di registry HOP ID.
# Jalankan SETELAH PR hop-admin-api dideploy:  bash ~/imperium/scripts/tahap2-helink.sh
set -euo pipefail
APP=/var/www/helink/shortlink-app
cd "$APP"
gen() { head -c 48 /dev/urandom | base64 | tr -d '/+=\n' | cut -c1-48; }
add() { grep -qE "^$1=" .env && echo "   $1 sudah ada" || { printf '%s=%s\n' "$1" "$2" >> .env; echo "   $1 ditambahkan"; }; }
cp -p .env ".env.bak-tahap2-$(date +%Y%m%d%H%M%S)"
add IMPERIUM_URL https://imperium.hel.ink
add IMPERIUM_SSO_SECRET "$(gen)"
add CITADEL_URL https://citadel.hel.ink
add CITADEL_SSO_SECRET "$(gen)"
php8.3 artisan config:cache -q
php8.3 artisan hop:bootstrap
# file cache yang baru dibuat sebagai carawin harus bisa ditulis www-data
find storage bootstrap/cache -user carawin ! -group www-data -exec chgrp www-data {} + -exec chmod g+rw {} + 2>/dev/null || true
echo "Selesai. Lanjutkan: cd ~/imperium && make build && sudo bash scripts/tahap2-sudo.sh"
