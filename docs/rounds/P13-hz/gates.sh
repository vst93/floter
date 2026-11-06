#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=/tmp/floter-P13-hz
: > "$out/gates.txt"
failed=0
run() {
  label=$1
  shift
  "$@" > "$out/$label.txt" 2>&1
  rc=$?
  printf '%s exit=%s\n' "$label" "$rc" >> "$out/gates.txt"
  if test "$rc" -ne 0; then failed=1; fi
}
run gofmt gofmt -l cmd internal
if test -s "$out/gofmt.txt"; then failed=1; fi
run vet-linux env GOOS=linux go vet ./...
run vet-windows env GOOS=windows go vet ./...
run vet-darwin env GOOS=darwin go vet ./...
run full-test go test -count=1 ./...
run full-verbose go test -count=1 -v ./...
run build-linux env GOOS=linux go build ./...
run build-windows env GOOS=windows go build ./...
run build-darwin env GOOS=darwin go build ./...
printf 'PASS=' >> "$out/gates.txt"
rg -c -- '--- PASS' "$out/full-verbose.txt" >> "$out/gates.txt"
printf 'functions=' >> "$out/gates.txt"
git grep -h '^func Test' -- '*.go' | wc -l >> "$out/gates.txt"
cat "$out/gates.txt"
exit "$failed"
