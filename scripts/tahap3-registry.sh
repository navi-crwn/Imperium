#!/usr/bin/env bash
# Tahap 3: setelah PixelHop/NoteHop versi HOP App Protocol dideploy, nyatakan kemampuannya
# di registry HOP ID supaya Imperium memakainya dan HOP ID mengirim event.
#   bash ~/imperium/scripts/tahap3-registry.sh pixelhop   (atau notehop)
set -euo pipefail
APP=${1:?pixelhop|notehop}
case "$APP" in
  pixelhop) CAPS='["account_api","admin_api","events","metrics"]';;
  notehop)  CAPS='["admin_api","events","metrics","zero_knowledge"]';;
  *) echo "aplikasi tidak dikenal: $APP"; exit 1;;
esac
cd /var/www/helink/shortlink-app
sudo -n -u www-data php8.3 artisan tinker --execute="
\$a = App\Models\HopApp::where('slug', '$APP')->firstOrFail();
\$a->capabilities = json_decode('$CAPS', true); \$a->save();
echo \$a->slug.' capabilities='.json_encode(\$a->capabilities).\"\n\";" 2>/dev/null | tail -1
echo "Imperium menyinkronkan registry tiap 5 menit (atau Apps → Sinkronkan)."
