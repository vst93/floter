#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
round_dir=/tmp/floter-P13-kq
date -u '+%Y-%m-%d %H:%M:%S UTC' > "$round_dir/gates-start.txt"
gofmt -l cmd internal > "$round_dir/gofmt.log"
test ! -s "$round_dir/gofmt.log" || exit 1
for platform in linux windows darwin; do
  for check in vet build; do
    GOOS="$platform" go "$check" ./... > "$round_dir/$platform-$check.log" 2>&1
    result=$?
    printf '%s\n' "$result" > "$round_dir/$platform-$check.exit"
    test "$result" = 0 || exit "$result"
  done
done
go test -count=1 ./... > "$round_dir/full-test.log" 2>&1
result=$?
printf '%s\n' "$result" > "$round_dir/full-test.exit"
test "$result" = 0 || exit "$result"
go test -count=1 -v ./... > "$round_dir/full-verbose.log" 2>&1
result=$?
printf '%s\n' "$result" > "$round_dir/full-verbose.exit"
test "$result" = 0 || exit "$result"
grep -c -- '--- PASS' "$round_dir/full-verbose.log" > "$round_dir/final-pass.txt"
git grep -h '^func Test' -- '*_test.go' | wc -l > "$round_dir/final-functions.txt"
date -u '+%Y-%m-%d %H:%M:%S UTC' > "$round_dir/gates-end.txt"
printf '0\n' > "$round_dir/gates.exit"
