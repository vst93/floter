#!/usr/bin/env bash
set -eu
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-kj
run() {
    label=$1
    shift
    set +e
    "$@" > "$out/$label.txt" 2>&1
    code=$?
    set -e
    printf '%s\n' "$code" > "$out/$label.exit"
    test "$code" = 0
}
if test "${1:-}" = baseline; then
    git show ce5083d0:internal/settingsui/settingsui.go > /tmp/floter-P13-kj-baseline-settings.go
    git show ce5083d0:internal/settingsui/toolfieldedge_test.go > /tmp/floter-P13-kj-baseline-test.go
    printf '{"Replace":{"%s/internal/settingsui/settingsui.go":"/tmp/floter-P13-kj-baseline-settings.go","%s/internal/settingsui/toolfieldedge_test.go":"/tmp/floter-P13-kj-baseline-test.go"}}\n' "$PWD" "$PWD" > "$out/resume-baseline.json"
    run resume-baseline go test -count=1 -v -overlay="$out/resume-baseline.json" ./...
else
    run gofmt gofmt -l cmd internal
    test ! -s "$out/gofmt.txt"
    for platform in linux windows darwin; do
        run "vet-$platform" env GOOS="$platform" go vet ./...
        run "build-$platform" env GOOS="$platform" go build ./...
    done
    run tests go test -count=1 ./...
    run tests-verbose go test -count=1 -v ./...
fi
