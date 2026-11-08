#!/usr/bin/env bash
# Copy the HOP family assets (fonts, floating tiles) into the embedded static folder.
# Imperium's CSP is style-src 'self' and script-src 'self', so it serves its own copy
# instead of loading https://hel.ink. Source: a HEL.ink checkout (default ../HEL.ink)
# or, with --remote, https://hel.ink/hop-assets/v1.
#
#   scripts/sync-hop-assets.sh [--remote | /path/to/HEL.ink]
set -euo pipefail
cd "$(dirname "$0")/.."
dest=cmd/imperium/webfs/web/static/hop
files=(fonts.css hop-float.css hop-float.js apps/linkhop/mark-white.svg
  apps/hub/hop-id-mark.svg apps/linkhop/mark.svg apps/biohop/mark.svg
  apps/pixelhop/mark.svg apps/notehop/mark.svg
  fonts/LICENSE-syne.txt fonts/LICENSE-plus-jakarta-sans.txt fonts/LICENSE-jetbrains-mono.txt)
rm -rf "$dest.new" && mkdir -p "$dest.new/fonts"
for f in "${files[@]}"; do mkdir -p "$dest.new/$(dirname "$f")"; done
if [[ "${1:-}" == "--remote" ]]; then
  base=https://hel.ink/hop-assets/v1
  curl -fsS https://hel.ink/hop-assets/apps.json -o "$dest.new/apps.json"
  for f in "${files[@]}"; do sleep 1; curl -fsS "$base/$f" -o "$dest.new/$f"; done
  for f in $(grep -o 'fonts/[a-z0-9-]*\.woff2' "$dest.new/fonts.css" | sort -u); do sleep 1; curl -fsS "$base/$f" -o "$dest.new/$f"; done
else
  src="${1:-../HEL.ink}/public/hop-assets/v1"
  [[ -f "$src/fonts.css" ]] || { echo "no HOP assets at $src" >&2; exit 1; }
  cp "$src/../apps.json" "$dest.new/apps.json"
  for f in "${files[@]}"; do cp "$src/$f" "$dest.new/$f"; done
  cp "$src"/fonts/*.woff2 "$dest.new/fonts/"
fi
# FileHop joined after v1 was frozen: its mark lives in v2 (a new file there, nothing in v2 changed).
mkdir -p "$dest.new/apps/filehop"
if [[ "${1:-}" == "--remote" ]]; then
  curl -fsS https://hel.ink/hop-assets/v2/apps/filehop/mark.svg -o "$dest.new/apps/filehop/mark.svg"
else
  cp "$src/../v2/apps/filehop/mark.svg" "$dest.new/apps/filehop/mark.svg"
fi
rm -rf "$dest" && mv "$dest.new" "$dest"
git --no-pager diff --stat -- "$dest" || true
