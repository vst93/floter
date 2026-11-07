#!/usr/bin/env bash
set -eu
batch=$1
product=$2
pattern=$3
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
base=/tmp/floter-P13-ih
cp "$product" "$base/product-$batch"
md5sum "$product" > "$base/m$batch.md5"
git diff -- "$product" > "$base/m$batch.patch"
trap 'cp "$base/product-$batch" "$product"' EXIT
git apply -R "$base/m$batch.patch"
set +e
go test -count=1 -v ./internal/settingsui -run "$pattern" > "$base/m$batch-red.txt" 2>&1
red=$?
set -e
cp "$base/product-$batch" "$product"
md5sum -c "$base/m$batch.md5" > "$base/m$batch-restore.txt"
touch "$product"
test "$red" -ne 0
grep -- '--- FAIL' "$base/m$batch-red.txt"
FLOTER_P13IH_DUMP=after go test -count=1 -v ./internal/launcher ./internal/settingsui -run "$pattern|Test(ConfigLifetime|ConfigEpoch|LocalToolForm|LocalToolEdit|EditCustom)" > "$base/batch$batch-green.txt" 2>&1
go vet ./internal/launcher ./internal/settingsui > "$base/batch$batch-vet.txt" 2>&1
test -z "$(gofmt -l "$product" internal/settingsui/configfetch_test.go)"
git diff --check
printf 'batch %s PASS=' "$batch"
grep -c -- '--- PASS' "$base/batch$batch-green.txt"
trap - EXIT
