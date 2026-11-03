#!/usr/bin/env bash
# A mistyped subcommand must exit non-zero and must NOT start a server.
# usage: check-cli-unknown-arg.sh BINARY ADDR_ENV_NAME PORT [DB_ENV_NAME]
# Example: scripts/check-cli-unknown-arg.sh ./bin/notehop NOTEHOP_ADDR 18080 NOTEHOP_DB
set -u
bin=$1; addr_env=$2; port=$3; db_env=${4:-}
tmp=$(mktemp -d); trap 'kill $pid 2>/dev/null; rm -rf "$tmp"' EXIT
env "$addr_env=127.0.0.1:$port" ${db_env:+"$db_env=$tmp/x.db"} "$bin" definitely-not-a-command >"$tmp/out" 2>&1 &
pid=$!
sleep 2
if curl -fsS -m 2 "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
  echo "FAIL: '$bin definitely-not-a-command' started a server on :$port"; exit 1
fi
wait $pid 2>/dev/null; rc=$?
if [ "$rc" -eq 0 ]; then echo "FAIL: exit status 0 for an unknown command"; exit 1; fi
echo "ok: unknown command rejected (exit $rc): $(head -1 "$tmp/out")"
