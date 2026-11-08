#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
round_dir=/tmp/floter-P13-kr
run_gate() {
  local name=$1
  shift
  local result=0
  "$@" > "$round_dir/$name.txt" 2>&1 || result=$?
  printf '%s\n' "$result" > "$round_dir/$name.exit"
  test "$result" = 0
}
run_gate gofmt gofmt -l cmd internal
test ! -s "$round_dir/gofmt.txt"
for platform in linux windows darwin; do
  run_gate "$platform-vet" env GOOS="$platform" go vet ./...
  run_gate "$platform-build" env GOOS="$platform" go build ./...
done
run_gate full-test go test -count=1 ./...
run_gate full-verbose go test -count=1 -v ./...
grep -c -- '--- PASS' "$round_dir/full-verbose.txt" > "$round_dir/final-pass.txt"
git grep -h '^func Test' -- '*_test.go' | wc -l > "$round_dir/final-functions.txt"
date -u '+%Y-%m-%d %H:%M:%S UTC' > "$round_dir/gates-end.txt"
printf '0\n' > "$round_dir/gates.exit"
