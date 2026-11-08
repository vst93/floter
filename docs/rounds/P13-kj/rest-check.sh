#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-kj
git show 286f3014:internal/settingsui/settingsui.go > /tmp/floter-P13-kj-rest-settings.go
git show 286f3014:internal/settingsui/outputfocus.go > /tmp/floter-P13-kj-rest-focus.go
printf '{"Replace":{"%s/internal/settingsui/kj_rest_test.go":"%s/%s/rest-probe.go.txt","%s/internal/settingsui/settingsui.go":"/tmp/floter-P13-kj-rest-settings.go","%s/internal/settingsui/outputfocus.go":"/tmp/floter-P13-kj-rest-focus.go"}}\n' "$PWD" "$PWD" "$out" "$PWD" "$PWD" > "$out/rest-before.json"
printf '{"Replace":{"%s/internal/settingsui/kj_rest_test.go":"%s/%s/rest-probe.go.txt"}}\n' "$PWD" "$PWD" "$out" > "$out/rest-after.json"
for stage in before after; do
    FLOTER_P13KJ_REST="$stage" go test -count=1 -v -overlay="$out/rest-$stage.json" ./internal/settingsui -run '^TestKJRestingGeometry$' > "$out/rest-$stage.txt" 2>&1
done
for mode in program script; do
    for appearance in light dark; do
        for position in 0 1 2; do
            for ext in png txt; do
                cmp "/tmp/floter-P13-kj-rest-$mode-$appearance-$position-before.$ext" "/tmp/floter-P13-kj-rest-$mode-$appearance-$position-after.$ext"
                printf '%s %s position=%s %s identical\n' "$mode" "$appearance" "$position" "$ext"
            done
        done
    done
done > "$out/rest-comparison.txt"
cat "$out/rest-comparison.txt"
