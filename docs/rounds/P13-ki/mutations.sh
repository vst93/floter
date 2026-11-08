#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-ki
for group in M1 M2; do
    case "$group" in
        M1) source=internal/settingsui/parameditor.go; pattern='^TestEditorRing(Label|ID|Placeholder|Default|Flag|Options)$'; failures=6 ;;
        M2) source=internal/settingsui/settingsui.go; pattern='^TestEditorRing(Command|Version)$'; failures=2 ;;
    esac
    injected="/tmp/floter-P13-ki-$group.go"
    backup="/tmp/floter-P13-ki-$group-restored.go"
    cp "$source" "$injected"
    cp "$source" "$backup"
    md5sum "$injected" > "$out/$group.md5"
    trap 'cp "$backup" "$injected"' EXIT
    git show "7777fb41:$source" > "$injected"
    printf '{"Replace":{"%s/%s":"%s"}}\n' "$PWD" "$source" "$injected" > "$out/$group-overlay.json"
    set +e
    go test -count=1 -v -overlay="$out/$group-overlay.json" ./internal/settingsui -run "$pattern" > "$out/$group-red.txt" 2>&1
    result=$?
    set -e
    printf '%s\n' "$result" > "$out/$group-red.exit"
    test "$result" -ne 0
    test "$(grep -c '^--- FAIL: TestEditorRing' "$out/$group-red.txt")" = "$failures"
    cp "$backup" "$injected"
    md5sum -c "$out/$group.md5" > "$out/$group-restore.txt"
    touch "$injected"
    go test -count=1 -v -overlay="$out/$group-overlay.json" ./internal/settingsui -run "$pattern" > "$out/$group-green.txt" 2>&1
    printf '0\n' > "$out/$group-green.exit"
    printf '%s red=%s restored_PASS=%s\n' "$group" "$result" "$(grep -c -- '--- PASS' "$out/$group-green.txt")"
    trap - EXIT
done
