#!/usr/bin/env bash
# The test gate: runs the suite with coverage and prints {"total": N, "coverage": P},
# the line scc check reads.
set -euo pipefail
profile=$(mktemp)
events=$(mktemp)
trap 'rm -f "$profile" "$events"' EXIT
status=0
go test -count=1 -json -coverprofile="$profile" ./... > "$events" || status=$?
grep -v '"Action":"output"' "$events" | grep '"Action":"fail"' >&2 || true
total=$(grep -c '"Action":"pass","Package":"[^"]*","Test":' "$events" || true)
coverage=$(go tool cover -func="$profile" | awk '/^total:/ {sub("%", "", $3); print $3}')
echo "{\"total\": ${total}, \"coverage\": ${coverage}}"
exit "$status"
