#!/usr/bin/env bash
# Fails when a commit in BASE..HEAD carries an AI attribution trailer or names an AI agent or model in its
# message. Only the commits of the pull request are looked at, never the whole history.
# usage: check-commit-messages.sh BASE_SHA HEAD_SHA
set -eu
base=${1:?usage: check-commit-messages.sh BASE_SHA HEAD_SHA}; head=${2:?usage: check-commit-messages.sh BASE_SHA HEAD_SHA}
pattern='co-authored-by|generated (with|by)|noreply@anthropic\.com|\b(claude|anthropic|openai|chatgpt|copilot|gemini|codex)\b|\bgpt-?[0-9]|\b(opus|sonnet|haiku|fable)[ -]?[0-9]'
bad=0
for sha in $(git rev-list --no-merges "$base..$head"); do
  if hit=$(git log -1 --format=%B "$sha" | grep -niE "$pattern"); then
    echo "commit $(git log -1 --format='%h %s' "$sha")"
    printf '  %s\n' "$hit"
    bad=1
  fi
done
if [ "$bad" -ne 0 ]; then
  echo "A commit message must not carry Co-Authored-By, 'Generated with' or the name of an agent or model. Reword the commit." >&2
  exit 1
fi
echo "ok: no attribution trailer or agent name in $(git rev-list --no-merges --count "$base..$head") commit message(s)"
