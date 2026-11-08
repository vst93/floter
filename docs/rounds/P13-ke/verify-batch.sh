#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-ke
batch=$1
pattern=$2
prior=$3
source=$4
saved=$(mktemp /tmp/floter-P13-ke-source.XXXXXX)
cp "$source" "$saved"
md5sum "$source" > "$out/M$batch.md5"
trap 'cp "$saved" "$source"; rm -f "$saved"' EXIT
go test -count=1 -v ./internal/settingsui -run "$pattern" > "$out/batch$batch.txt" 2>&1
git show "$prior:$source" > "$source"
touch "$source"
set +e
FLOTER_P13KE_DUMP=before go test -count=1 -v ./internal/settingsui -run "$pattern" > "$out/M$batch-red.txt" 2>&1
red=$?
set -e
printf '%s\n' "$red" > "$out/M$batch-red.exit"
cp "$saved" "$source"
md5sum -c "$out/M$batch.md5" > "$out/M$batch-restore-md5.txt"
touch "$source"
test "$red" -ne 0
rg -q -- '--- FAIL: Test' "$out/M$batch-red.txt"
FLOTER_P13KE_DUMP=after go test -count=1 -v ./internal/settingsui -run "$pattern" > "$out/M$batch-restored.txt" 2>&1
go vet ./internal/settingsui > "$out/batch$batch-vet.txt" 2>&1
gofmt -l cmd internal > "$out/batch$batch-gofmt.txt"
test ! -s "$out/batch$batch-gofmt.txt"
printf 'batch%s restored PASS=' "$batch"
rg -c -- '--- PASS' "$out/M$batch-restored.txt"
rg 'outside viewport' "$out/M$batch-red.txt" | head -4
