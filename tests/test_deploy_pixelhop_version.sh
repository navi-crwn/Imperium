#!/usr/bin/env bash
# PixelHop reports its version without the tag's v, like its release binaries. Runs the version line of
# scripts/deploy-pixelhop.sh with a stand-in git and checks that the build uses the result.
set -eu
script=$(cd "$(dirname "$0")/.." && pwd)/scripts/deploy-pixelhop.sh
fail() { echo "FAIL: $1"; exit 1; }
git() { echo v3.7.0-85-g457354f; }
line=$(grep '^VERSION=' "$script") || fail "the script does not work out a version before the build"
eval "$line"
[ "$VERSION" = "3.7.0-85-g457354f" ] || fail "version is $VERSION"
grep -q 'internal/app\.Version=\$VERSION"' "$script" || fail "the build does not use \$VERSION"
echo ok
