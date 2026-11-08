#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-kj
source=internal/settingsui/settingsui.go
saved=/tmp/floter-P13-kj-settings-final.go
cp "$source" "$saved"
focus=internal/settingsui/outputfocus.go
focusSaved=/tmp/floter-P13-kj-focus-final.go
cp "$focus" "$focusSaved"
md5sum "$source" "$focus" > "$out/mutations.md5"
trap 'cp "$saved" "$source"; cp "$focusSaved" "$focus"' EXIT
git diff --unified=0 286f3014 -- "$source" > "$out/M1.patch"
git diff --unified=0 ce5083d0 -- "$source" > "$out/M2.patch"
for mutation in M1 M2; do
    pattern='^TestToolFieldEdge(Name|Program|Args|VersionArgs|Description|Mode)$'
    if test "$mutation" = M2; then
        pattern='^TestToolFieldEdge(Choose|Language)$'
    fi
    git apply --reverse --unidiff-zero "$out/$mutation.patch"
    set +e
    FLOTER_P13KJ_DUMP=before go test -count=1 -v ./internal/settingsui -run "$pattern" > "$out/$mutation-red.txt" 2>&1
    code=$?
    set -e
    printf '%s\n' "$code" > "$out/$mutation-red.exit"
    test "$code" = 1
    grep -q 'active edge missing' "$out/$mutation-red.txt"
    cp "$saved" "$source"
    md5sum -c "$out/mutations.md5" > "$out/$mutation-restored-md5.txt"
    touch "$source"
    FLOTER_P13KJ_DUMP=after go test -count=1 -v ./internal/settingsui -run "$pattern" > "$out/$mutation-restored.txt" 2>&1
    printf '0\n' > "$out/$mutation-restored.exit"
    printf '%s red=%s restored PASS=%s\n' "$mutation" "$code" "$(grep -c -- '--- PASS' "$out/$mutation-restored.txt")"
done
git apply --unidiff-zero "$out/M3.patch"
set +e
go test -count=1 -v ./internal/settingsui -run '^TestContentVisibilityPreservesOtherEditor$' > "$out/M3-red.txt" 2>&1
code=$?
set -e
printf '%s\n' "$code" > "$out/M3-red.exit"
test "$code" = 1
grep -q 'retirement stole current editor or scroll' "$out/M3-red.txt"
cp "$focusSaved" "$focus"
md5sum -c "$out/mutations.md5" > "$out/M3-restored-md5.txt"
touch "$source" "$focus"
go test -count=1 -v ./internal/settingsui -run '^TestContentVisibilityPreservesOtherEditor$' > "$out/M3-restored.txt" 2>&1
printf '0\n' > "$out/M3-restored.exit"
printf 'M3 red=%s restored PASS=%s\n' "$code" "$(grep -c -- '--- PASS' "$out/M3-restored.txt")"
