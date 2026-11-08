#!/usr/bin/env bash
# One entry point; complete logs stay on disk, stdout stays below 30 lines.
set -u
cd "$(dirname "$0")/.." || exit 1
round=${1:?usage: bash tools/verify.sh ROUND}
case "$round" in *[!A-Za-z0-9_-]*) exit 2 ;; esac
logs="docs/rounds/$round/logs"
mkdir -p "$logs"
failed=0
run() {
  local name=$1 result
  shift
  "$@" >"$logs/$name.log" 2>&1
  result=$?
  printf '%s\n' "$result" >"$logs/$name.exit"
  if [ "$result" -eq 0 ]; then
    printf 'PASS %-16s %s\n' "$name" "$logs/$name.log"
  else
    failed=$((failed + 1))
    printf 'FAIL %-16s %s | ' "$name" "$logs/$name.log"
    head -n 3 "$logs/$name.log" | tr '\n' ' ' | head -c 180
    printf '\n'
  fi
}
run gofmt bash -c 'out=$(gofmt -l cmd internal); printf "%s" "$out"; test -z "$out"'
run vet-linux env GOOS=linux go vet ./...
run vet-windows env GOOS=windows go vet ./...
run vet-darwin env GOOS=darwin go vet ./...
run test go test -count=1 ./...
run verbose go test -count=1 -v ./...
for platform in linux windows darwin; do
  run "build-$platform" env GOOS="$platform" go build ./...
done
passes=$(grep -c -- '--- PASS' "$logs/verbose.log" || true)
functions=$(git grep -h '^func Test' -- 'cmd/*_test.go' 'internal/*_test.go' | wc -l)
printf 'TOTAL PASS=%s test_functions=%s failed_gates=%s\n' "$passes" "$functions" "$failed"
test "$failed" -eq 0
