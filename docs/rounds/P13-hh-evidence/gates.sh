#!/usr/bin/env bash
set -uo pipefail
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
evidence=docs/rounds/P13-hh-evidence
failed=0
run_check() {
  local name=$1
  shift
  "$@" > "$evidence/$name.txt" 2>&1
  local code=$?
  printf '%s exit=%s\n' "$name" "$code"
  if test "$code" -ne 0; then tail -30 "$evidence/$name.txt"; failed=1; fi
}
run_check gofmt gofmt -l cmd internal
if test -s "$evidence/gofmt.txt"; then cat "$evidence/gofmt.txt"; failed=1; fi
for platform in linux windows darwin; do
  run_check "vet-$platform" env GOOS="$platform" go vet ./...
  run_check "build-$platform" env GOOS="$platform" go build ./...
done
run_check final-test go test -count=1 ./...
run_check final-verbose go test -count=1 -v ./...
printf 'PASS='
grep -c -- '--- PASS' "$evidence/final-verbose.txt"
printf 'FAIL='
grep -c -- '--- FAIL' "$evidence/final-verbose.txt" || true
exit "$failed"
