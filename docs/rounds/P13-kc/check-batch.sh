#!/usr/bin/env bash
set -eu
batch=$1
pattern=$2
shift 2
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
dir=/tmp/floter-P13-kc
mkdir -p "docs/rounds/P13-kc"
gofmt -l cmd internal > "$dir/$batch-gofmt.txt"
test ! -s "$dir/$batch-gofmt.txt"
go vet ./internal/settingsui > "$dir/$batch-vet.txt" 2>&1
md5sum "$@" > "$dir/$batch-source.md5"
for file in "$@"; do cp "$file" "$dir/$batch-fixed-$(basename "$file")"; git show "HEAD:$file" > "$file"; done
set +e
FLOTER_P13KC_DUMP=before go test -count=1 -v ./internal/settingsui -run "$pattern" > "$dir/$batch-red.txt" 2>&1
result=$?
set -e
for file in "$@"; do cp "$dir/$batch-fixed-$(basename "$file")" "$file"; done
test "$result" -ne 0
rg -- '--- FAIL|outside viewport|lost focus' "$dir/$batch-red.txt"
md5sum -c "$dir/$batch-source.md5" | tee "$dir/$batch-restored.txt"
touch "$@"
FLOTER_P13KC_DUMP=after go test -count=1 -v ./internal/settingsui -run "$pattern" > "$dir/$batch-green.txt" 2>&1
rg -c -- '--- PASS' "$dir/$batch-green.txt"
cp "$dir/$batch-"*.txt "$dir/$batch-source.md5" docs/rounds/P13-kc/
