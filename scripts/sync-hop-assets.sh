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
  fonts/LICENSE-syne.txt fonts/LICENSE-plus-jakarta-sans.txt fonts/LICENSE-jetbrains-mono.txt)
rm -rf "$dest.new" && mkdir -p "$dest.new/fonts" "$dest.new/apps/linkhop"
if [[ "${1:-}" == "--remote" ]]; then
  base=https://hel.ink/hop-assets/v1
  for f in "${files[@]}"; do curl -fsS "$base/$f" -o "$dest.new/$f"; done
  for f in $(grep -o 'fonts/[a-z0-9-]*\.woff2' "$dest.new/fonts.css" | sort -u); do curl -fsS "$base/$f" -o "$dest.new/$f"; done
else
  src="${1:-../HEL.ink}/public/hop-assets/v1"
  [[ -f "$src/fonts.css" ]] || { echo "no HOP assets at $src" >&2; exit 1; }
  for f in "${files[@]}"; do cp "$src/$f" "$dest.new/$f"; done
  cp "$src"/fonts/*.woff2 "$dest.new/fonts/"
fi
rm -rf "$dest" && mv "$dest.new" "$dest"
git --no-pager diff --stat -- "$dest" || true
