#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=zh_CN.UTF-8
out=docs/rounds/P13-jo
trap 'result=$?; printf "exit=%s\n" "$result" > "$out/gates-exit.txt"' EXIT
: > "$out/gates.txt"
gofmt -l cmd internal > "$out/gofmt.txt"
test ! -s "$out/gofmt.txt"
printf 'gofmt: empty\n' >> "$out/gates.txt"
run_gate() {
    local label="$1"
    shift
    "$@" > "$out/$label.txt" 2>&1
    printf '%s: exit=0\n' "$label" >> "$out/gates.txt"
}
run_gate vet-linux go vet ./...
run_gate vet-windows env GOOS=windows go vet ./...
run_gate vet-darwin env GOOS=darwin go vet ./...
run_gate build-linux env GOOS=linux go build ./...
run_gate build-windows env GOOS=windows go build ./...
run_gate build-darwin env GOOS=darwin go build ./...
run_gate full-test go test -count=1 ./...
run_gate full-verbose go test -count=1 -v ./...
run_gate race go test -race -count=1 -v ./internal/shell -run '^Test(CompletionSeverity|HealthCompletion)'
printf 'PASS=' >> "$out/gates.txt"
grep -c -- '--- PASS' "$out/full-verbose.txt" >> "$out/gates.txt"
printf 'functions=' >> "$out/gates.txt"
git grep -h '^func Test' -- '*_test.go' | wc -l >> "$out/gates.txt"
md5sum -c "$out/final-md5.txt" >> "$out/gates.txt"
