#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
export TMPDIR=/home/tar/.fih
mkdir -p "$TMPDIR"
base=/tmp/floter-P13-ih
gofmt -l cmd internal > "$base/gofmt.txt"
test ! -s "$base/gofmt.txt"
git diff --check
for platform in linux windows darwin; do
    GOOS=$platform go vet ./... > "$base/vet-$platform.txt" 2>&1
    printf '%s vet PASS\n' "$platform"
    GOOS=$platform go build ./... > "$base/build-$platform.txt" 2>&1
    printf '%s build PASS\n' "$platform"
done
go test -count=1 ./... > "$base/full-test.txt" 2>&1
printf 'ordinary test PASS\n'
go test -count=1 -v ./... > "$base/full-verbose.txt" 2>&1
printf 'verbose PASS='
grep -c -- '--- PASS' "$base/full-verbose.txt"
printf 'test functions='
git grep -E '^func Test[^ (]*\(' -- '*_test.go' | wc -l
md5sum internal/launcher/config.go internal/settingsui/settingsui.go internal/settingsui/configfetch_test.go internal/settingsui/toolcallback_test.go > "$base/final-source.md5"
