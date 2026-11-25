#!/usr/bin/env bash
# The format gate: gofmt has nothing to change.
set -euo pipefail
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "gofmt would change:" >&2
  echo "$unformatted" >&2
  exit 1
fi
