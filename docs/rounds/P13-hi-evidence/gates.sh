#!/bin/bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
evidence=docs/rounds/P13-hi-evidence
gofmt -l cmd internal > "$evidence/gofmt.txt"
test ! -s "$evidence/gofmt.txt"
for target in linux windows darwin; do
  GOOS="$target" go vet ./... > "$evidence/vet-$target.txt" 2>&1
  GOOS="$target" go build ./... > "$evidence/build-$target.txt" 2>&1
done
go test -count=1 ./... > "$evidence/tests.txt" 2>&1
go test -count=1 -v ./... > "$evidence/tests-verbose.txt" 2>&1
printf 'PASS '
rg -c -- '--- PASS' "$evidence/tests-verbose.txt"
git grep '^func Test' -- '*.go' | wc -l
git diff --check
