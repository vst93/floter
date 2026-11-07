#!/usr/bin/env bash
set -u
export LANG=zh_CN.UTF-8 LC_ALL=zh_CN.UTF-8
out=/tmp/floter-P13-jr
failed=0
run_gate() {
  label=$1
  shift
  "$@" > "$out/$label.txt" 2>&1
  result=$?
  printf '%s exit=%s\n' "$label" "$result" >> "$out/gates.txt"
  if test "$result" -ne 0; then failed=1; fi
}
: > "$out/gates.txt"
gofmt -l cmd internal > "$out/gofmt.txt"
if test -s "$out/gofmt.txt"; then failed=1; fi
printf 'gofmt lines=%s\n' "$(wc -l < "$out/gofmt.txt")" >> "$out/gates.txt"
run_gate vet-linux go vet ./...
run_gate vet-windows env GOOS=windows go vet ./...
run_gate vet-darwin env GOOS=darwin go vet ./...
run_gate build-linux env GOOS=linux go build ./...
run_gate build-windows env GOOS=windows go build ./...
run_gate build-darwin env GOOS=darwin go build ./...
go test -count=1 ./... > "$out/full-test.txt" 2>&1 &
normal_pid=$!
go test -count=1 -v ./... > "$out/verbose-test.txt" 2>&1 &
verbose_pid=$!
wait "$normal_pid"
result=$?
printf 'full-test exit=%s\n' "$result" >> "$out/gates.txt"
if test "$result" -ne 0; then failed=1; fi
wait "$verbose_pid"
result=$?
printf 'verbose-test exit=%s\n' "$result" >> "$out/gates.txt"
if test "$result" -ne 0; then failed=1; fi
printf 'PASS=%s FAIL=%s functions=%s\n' "$(grep -c -- '--- PASS' "$out/verbose-test.txt")" "$(grep -c -- '--- FAIL' "$out/verbose-test.txt")" "$(git grep -h '^func Test' -- '*_test.go' | wc -l)" >> "$out/gates.txt"
cat "$out/gates.txt"
exit "$failed"
