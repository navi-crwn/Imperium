#!/usr/bin/env bash
# Copy the HOP family assets (fonts, floating tiles, app marks) into the embedded static folder.
# Imperium's CSP is style-src 'self' and script-src 'self', so it serves its own copy
# instead of loading https://hel.ink. Source: a HEL.ink checkout (default ../HEL.ink)
# or, with --remote, https://hel.ink/hop-assets/v8.
#
#   scripts/sync-hop-assets.sh [--remote | /path/to/HEL.ink]
#
# The family type is Inter + Instrument Serif + JetBrains Mono: hop-assets serves it as
# fonts-amnesia.css, which is copied here as fonts.css with every font file it names.
set -euo pipefail
cd "$(dirname "$0")/.."
dest=cmd/imperium/webfs/web/static/hop
ver=v8
files=(hop-float.css hop-float.js
  apps/hub/hop-id-mark.svg apps/hub/rabbit-mark.png
  apps/linkhop/mark.svg apps/linkhop/mark-white.svg apps/biohop/mark.svg
  apps/pixelhop/mark.svg apps/notehop/mark.svg apps/filehop/mark.svg
  fonts/LICENSE-inter.txt fonts/LICENSE-instrument-serif.txt fonts/LICENSE-jetbrains-mono.txt
  fonts/LICENSE-plus-jakarta-sans.txt fonts/LICENSE-syne.txt)
rm -rf "$dest.new" && mkdir -p "$dest.new/fonts"
for f in "${files[@]}"; do mkdir -p "$dest.new/$(dirname "$f")"; done
get() { # get PATH-IN-VERSION DEST
  if [[ "${remote:-0}" == 1 ]]; then sleep 1; curl -fsS "https://hel.ink/hop-assets/$ver/$1" -o "$2"; else cp "$src/$1" "$2"; fi
}
if [[ "${1:-}" == "--remote" ]]; then
  remote=1
  curl -fsS https://hel.ink/hop-assets/apps.json -o "$dest.new/apps.json"
else
  src="${1:-../HEL.ink}/public/hop-assets/$ver"
  [[ -f "$src/fonts-amnesia.css" ]] || { echo "no HOP assets at $src" >&2; exit 1; }
  cp "$src/../apps.json" "$dest.new/apps.json"
fi
get fonts-amnesia.css "$dest.new/fonts.css"
for f in "${files[@]}"; do get "$f" "$dest.new/$f"; done
for f in $(grep -oE 'fonts/[a-z0-9-]+\.woff2?' "$dest.new/fonts.css" | sort -u); do get "$f" "$dest.new/$f"; done
# The Syne and Plus Jakarta Sans files shipped before the amnesia set are kept (internal/app/hopassets_test.go serves one).
for f in $(cd "$dest/fonts" 2>/dev/null && ls syne-* plus-jakarta-sans-* 2>/dev/null); do get "fonts/$f" "$dest.new/fonts/$f"; done
rm -rf "$dest" && mv "$dest.new" "$dest"
git --no-pager diff --stat -- "$dest" || true
