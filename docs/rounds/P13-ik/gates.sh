#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
round_dir=docs/rounds/P13-ik
if test "$1" = tests; then
  go test -count=1 ./... > "$round_dir/full-test.txt" 2>&1
  plain=$?
  printf 'ordinary exit=%s\n' "$plain"
  go test -count=1 -v ./... > "$round_dir/full-verbose.txt" 2>&1
  verbose=$?
  printf 'verbose exit=%s PASS=%s FAIL=%s functions=%s\n' "$verbose" "$(grep -c -- '--- PASS' "$round_dir/full-verbose.txt")" "$(grep -c -- '--- FAIL' "$round_dir/full-verbose.txt")" "$(git grep -h '^func Test' -- '*.go' | wc -l)"
  test "$plain" -eq 0 && test "$verbose" -eq 0
else
  failed=0
  gofmt -l cmd internal > "$round_dir/gofmt.txt"
  test ! -s "$round_dir/gofmt.txt" || failed=1
  for platform in linux windows darwin; do
    for task in vet build; do
      GOOS=$platform go "$task" ./... > "$round_dir/$task-$platform.txt" 2>&1
      result=$?
      printf '%s %s exit=%s\n' "$platform" "$task" "$result"
      test "$result" -eq 0 || failed=1
    done
  done
  exit "$failed"
fi
