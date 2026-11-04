#!/usr/bin/env bash
# Deploy PixelHop dari branch/tag git ke ~/apps/pixelhop (service pixelhop). Butuh sudo tanpa password
# untuk restart service. Backup DB + binary lama dulu, rollback otomatis bila healthcheck gagal.
#   bash ~/imperium/scripts/deploy-pixelhop.sh [ref=main]
set -euo pipefail
REF=${1:-main}; SRC=/home/carawin/pixelhop; APP=/home/carawin/apps/pixelhop; GO=/home/carawin/sdk/go/bin/go
LOG=/home/carawin/imperium/backups/sudo-log.txt; S() { printf '%s  %s\n' "$(date -Is)" "$*" >> "$LOG"; sudo -n "$@"; }
sudo -n true 2>/dev/null || sudo -v || { echo "Gagal mengautentikasi sudo"; exit 1; }
cd "$SRC" && git fetch -q origin && git checkout -q "$REF" && { git pull -q --ff-only origin "$REF" 2>/dev/null || true; }
echo "build $(git log --oneline -1)"
CGO_ENABLED=0 $GO build -trimpath -ldflags "-s -w -X github.com/navi-crwn/pixelhop/internal/app.Version=$(git describe --tags --always)" -o /tmp/pixelhop.new ./cmd/pixelhop
STAMP=$(date +%Y%m%d-%H%M%S)
python3 -I -c "import sqlite3,sys; sqlite3.connect(sys.argv[1]).backup(sqlite3.connect(sys.argv[2]))" "$APP/data/pixelhop.db" "$APP/backups/pixelhop-before-$STAMP.db"
cp -p "$APP/pixelhop" "$APP/backups/pixelhop.bin-$STAMP"
install -m 755 /tmp/pixelhop.new "$APP/pixelhop" && rm -f /tmp/pixelhop.new
S systemctl restart pixelhop; sleep 2
if curl -fsS -m 10 http://127.0.0.1:8080/api/v1/stats >/dev/null; then
  echo "pixelhop OK: $("$APP/pixelhop" version)"
else
  echo "HEALTHCHECK GAGAL, rollback"; cp -p "$APP/backups/pixelhop.bin-$STAMP" "$APP/pixelhop"; S systemctl restart pixelhop; exit 1
fi
