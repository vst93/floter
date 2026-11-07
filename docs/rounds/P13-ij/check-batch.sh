#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
round_dir=docs/rounds/P13-ij
batch=$1
pattern=$2
shift 2
go vet ./internal/settingsui > "$round_dir/batch${batch}-vet.txt" 2>&1
git diff -- "$@" > "$round_dir/M${batch}.patch"
test -s "$round_dir/M${batch}.patch"
md5sum "$@" > "$round_dir/M${batch}-source.md5"
git apply -R "$round_dir/M${batch}.patch"
set +e
FLOTER_P13IJ_DUMP=before go test -count=1 -v ./internal/settingsui -run "$pattern" > "$round_dir/M${batch}-red.txt" 2>&1
red=$?
set -e
git apply "$round_dir/M${batch}.patch"
md5sum -c "$round_dir/M${batch}-source.md5"
touch "$@"
test "$red" -ne 0
grep -q -- '--- FAIL:' "$round_dir/M${batch}-red.txt"
FLOTER_P13IJ_DUMP=after go test -count=1 -v ./internal/settingsui -run "$pattern" > "$round_dir/M${batch}-restored.txt" 2>&1
printf 'M%s RED=%s restored PASS=%s FAIL=%s\n' "$batch" "$(grep -c -- '--- FAIL' "$round_dir/M${batch}-red.txt")" "$(grep -c -- '--- PASS' "$round_dir/M${batch}-restored.txt")" "$(grep -c -- '--- FAIL' "$round_dir/M${batch}-restored.txt")"
grep -E 'lost|stole|fallback|copied|sidebar' "$round_dir/M${batch}-red.txt" || true
