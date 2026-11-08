#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=docs/rounds/P13-ke
failed=0
run() {
    name=$1
    shift
    "$@" > "$out/$name.txt" 2>&1
    result=$?
    printf '%s\n' "$result" > "$out/$name.exit"
    printf '%s exit=%s\n' "$name" "$result"
    if test "$result" -ne 0; then failed=1; fi
}
case "$1" in
    platforms)
        run gofmt gofmt -l cmd internal
        if test -s "$out/gofmt.txt"; then failed=1; fi
        for platform in linux windows darwin; do
            run "vet-$platform" env GOOS="$platform" go vet ./...
            run "build-$platform" env GOOS="$platform" go build ./...
        done
        ;;
    tests)
        run full-test go test -count=1 ./...
        run full-verbose go test -count=1 -v ./...
        rg -c -- '--- PASS' "$out/full-verbose.txt"
        ;;
esac
exit "$failed"
