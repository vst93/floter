#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-jx
name=$1
source_file=$2
old_file=$3
package=$4
pattern=$5
saved=$(mktemp /tmp/floter-P13-jx-source.XXXXXX)
cp "$source_file" "$saved"
md5sum "$source_file" > "$out/$name.md5"
restore() { cp "$saved" "$source_file"; }
trap restore EXIT
cp "$old_file" "$source_file"
set +e
FLOTER_P13JX_DUMP=before go test -count=1 -v "$package" -run "$pattern" > "$out/$name-red.txt" 2>&1
result=$?
set -e
printf '%s\n' "$result" > "$out/$name-red.exit"
restore
md5sum -c "$out/$name.md5" > "$out/$name-restore.txt"
touch "$source_file"
FLOTER_P13JX_DUMP=after go test -count=1 -v "$package" -run "$pattern" > "$out/$name-restored.txt" 2>&1
printf '0\n' > "$out/$name-restored.exit"
grep -c -- '--- PASS' "$out/$name-restored.txt"
if test "$result" -eq 0; then printf 'mutation did not turn red\n'; exit 1; fi
if ! grep -q -- '--- FAIL' "$out/$name-red.txt"; then printf 'mutation lacked assertion failure\n'; exit 1; fi
grep -E 'lost retry|FAIL|permits|focus|warning|blank' "$out/$name-red.txt" | head -20
