#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-ki
git show 7777fb41:internal/settingsui/parameditor.go > /tmp/floter-P13-ki-rest-param.go
git show 7777fb41:internal/settingsui/settingsui.go > /tmp/floter-P13-ki-rest-settings.go
printf '{"Replace":{"%s/internal/settingsui/ki_rest_test.go":"%s/%s/rest-probe.go.txt","%s/internal/settingsui/parameditor.go":"/tmp/floter-P13-ki-rest-param.go","%s/internal/settingsui/settingsui.go":"/tmp/floter-P13-ki-rest-settings.go"}}\n' "$PWD" "$PWD" "$out" "$PWD" "$PWD" > "$out/rest-before.json"
printf '{"Replace":{"%s/internal/settingsui/ki_rest_test.go":"%s/%s/rest-probe.go.txt"}}\n' "$PWD" "$PWD" "$out" > "$out/rest-after.json"
for stage in before after; do
    FLOTER_P13KI_REST="$stage" go test -count=1 -v -overlay="$out/rest-$stage.json" ./internal/settingsui -run '^TestKIRestingGeometry$' > "$out/rest-$stage.txt" 2>&1
done
for appearance in light dark; do
    for position in 0 1 2; do
        for ext in png txt; do
            cmp "/tmp/floter-P13-ki-rest-$appearance-$position-before.$ext" "/tmp/floter-P13-ki-rest-$appearance-$position-after.$ext"
            printf '%s position=%s %s identical\n' "$appearance" "$position" "$ext"
        done
    done
done > "$out/rest-comparison.txt"
cat "$out/rest-comparison.txt"
