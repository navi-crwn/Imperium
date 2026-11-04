#!/usr/bin/env bash
# Runs scripts/check-commit-messages.sh against a scratch repository.
set -eu
script=$(cd "$(dirname "$0")/.." && pwd)/scripts/check-commit-messages.sh
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
git init -q . && git config user.name t && git config user.email t@example.test
git commit -q --allow-empty -m "old commit

Co-Authored-By: Someone <noreply@anthropic.com>"
base=$(git rev-parse HEAD)
fail() { echo "FAIL: $1"; exit 1; }
expect_ok()  { git commit -q --allow-empty -m "$1"; "$script" "$base" HEAD >/dev/null 2>&1 || fail "rejected a clean message: $1"; git reset -q --hard "$2"; }
expect_bad() { git commit -q --allow-empty -m "$1"; "$script" "$base" HEAD >/dev/null 2>&1 && fail "accepted: $1"; git reset -q --hard "$2"; }
tip=$base
"$script" "$base" HEAD >/dev/null || fail "an empty range must pass"
expect_ok "Fix the redirect target" "$tip"
expect_ok "Wrap the opus-shaped tables and the haiku field" "$tip"   # words that are not model names
expect_bad "Fix it

Co-Authored-By: A Name <a@example.test>" "$tip"
expect_bad "Fix it

co-authored-by: lowercase works too" "$tip"
expect_bad "Fix it

Generated with a tool" "$tip"
expect_bad "Fix it, written by Claude" "$tip"
expect_bad "Fix it with GPT-5" "$tip"
expect_bad "Fix it

Opus 5.5" "$tip"
# a bad commit hidden under a clean one is still found, and the old history before BASE is ignored
git commit -q --allow-empty -m "bad

Co-Authored-By: x"; git commit -q --allow-empty -m "clean on top"
"$script" "$base" HEAD >/dev/null 2>&1 && fail "missed a bad commit below a clean one"
echo "ok: check-commit-messages behaves"
