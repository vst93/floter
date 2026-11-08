#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
dir=docs/rounds/P13-kc
go test -count=1 ./... > "$dir/full-test.txt" 2>&1
result=$?; printf '%s\n' "$result" > "$dir/full-test.exit"
test "$result" = 0 || exit "$result"
go test -count=1 -v ./... > "$dir/full-verbose.txt" 2>&1
result=$?; printf '%s\n' "$result" > "$dir/full-verbose.exit"
test "$result" = 0 || exit "$result"
rg -c -- '--- PASS' "$dir/full-verbose.txt" > "$dir/pass-count.txt"
rg -n '^func Test' cmd internal -g '*_test.go' | wc -l > "$dir/function-count.txt"
printf '0\n' > "$dir/gates.exit"
