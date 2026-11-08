#!/usr/bin/env bash
set -u
cd /home/tar/workspace/floter
export LC_ALL=zh_CN.UTF-8
evidence=/tmp/floter-P13-ko
run_gate() {
  gate=$1
  shift
  "$@" > "$evidence/$gate.txt" 2>&1
  code=$?
  printf '%s\n' "$code" > "$evidence/$gate.exit"
  printf '%s exit=%s\n' "$gate" "$code"
  return "$code"
}
run_gate gofmt gofmt -l cmd internal || exit $?
test ! -s "$evidence/gofmt.txt" || exit 1
for platform in linux windows darwin; do
  run_gate "vet-$platform" env GOOS="$platform" go vet ./... || exit $?
  run_gate "build-$platform" env GOOS="$platform" go build ./... || exit $?
done
run_gate full-test go test -count=1 ./... || exit $?
run_gate full-verbose go test -count=1 -v ./... || exit $?
grep -c -- '--- PASS' "$evidence/full-verbose.txt" > "$evidence/final-pass-count.txt"
printf 'complete\n' > "$evidence/gates.complete"
