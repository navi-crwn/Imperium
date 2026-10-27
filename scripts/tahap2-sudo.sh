#!/usr/bin/env bash
# Tahap 2, bagian root: pasang Imperium sebagai service (menggantikan placeholder).
#   sudo bash ~/imperium/scripts/tahap2-sudo.sh
# Sebelumnya: binary sudah dibangun di ~/imperium/bin/imperium (make build) dan
# LinkHop sudah punya IMPERIUM_SSO_SECRET + CITADEL_SSO_SECRET di .env (lihat tahap2-helink.sh).
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "jalankan dengan sudo"; exit 1; }
REPO=/home/carawin/imperium
BIN=$REPO/bin/imperium
[ -x "$BIN" ] || { echo "binary $BIN belum ada: jalankan 'make build' di $REPO dulu"; exit 1; }
ENVF=/etc/imperium/imperium.env
id imperium >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin -d /var/lib/imperium imperium
mkdir -p /var/lib/imperium /etc/imperium && chown imperium:imperium /var/lib/imperium && chmod 750 /var/lib/imperium
install -m 755 "$BIN" /usr/local/bin/imperium
if [ ! -f "$ENVF" ]; then
  cp "$REPO/deploy/imperium.env.example" "$ENVF"
  HELINK_ENV=/var/www/helink/shortlink-app/.env
  S=$(grep -E '^IMPERIUM_SSO_SECRET=' "$HELINK_ENV" | cut -d= -f2- | tr -d '"'); A=$(grep -E '^CITADEL_SSO_SECRET=' "$HELINK_ENV" | cut -d= -f2- | tr -d '"')
  sed -i "s#^HOP_SSO_SECRET=.*#HOP_SSO_SECRET=$S#; s#^IMPERIUM_ALT_SECRET=.*#IMPERIUM_ALT_SECRET=$A#" "$ENVF"
  echo "   $ENVF dibuat dari .env LinkHop"
fi
chown root:imperium "$ENVF" && chmod 640 "$ENVF"
cp "$REPO/deploy/imperium.service" /etc/systemd/system/imperium.service
systemctl daemon-reload && systemctl enable imperium >/dev/null && systemctl restart imperium && sleep 1
curl -fsS http://127.0.0.1:8100/healthz && echo && echo "   imperium OK"
rm -rf /var/lib/imperium-placeholder
echo "Selesai. Buka https://imperium.hel.ink (Tailscale) atau https://citadel.hel.ink (Access) dan Login with HOP."
