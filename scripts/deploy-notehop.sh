#!/usr/bin/env bash
# Deploy NoteHop dari branch/tag git: build statis, lalu pasang-update.sh milik repo ops
# (backup binary lama, install.sh, restart, cek halaman). Butuh sudo tanpa password.
#   bash ~/imperium/scripts/deploy-notehop.sh [ref=main]
set -euo pipefail
REF=${1:-main}; SRC=/home/carawin/notehop/repo; GO=/home/carawin/sdk/go/bin/go
LOG=/home/carawin/imperium/backups/sudo-log.txt; S() { printf '%s  %s\n' "$(date -Is)" "$*" >> "$LOG"; sudo -n "$@"; }
sudo -v || { echo "Gagal mengautentikasi sudo"; exit 1; }
cd "$SRC" && git fetch -q origin && git checkout -q "$REF" && { git pull -q --ff-only origin "$REF" 2>/dev/null || true; }
echo "build $(git log --oneline -1)"
mkdir -p bin
CGO_ENABLED=0 $GO build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always)" -o bin/notehop-linux-amd64 ./cmd/notehop
(cd bin && sha256sum notehop-linux-amd64 > SHA256SUMS)
S bash /home/carawin/notehop/deploy-vps/pasang-update.sh
