#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
round_dir=docs/rounds/P13-kn
check() {
    local gate_name=$1
    shift
    "$@" > "$round_dir/$gate_name.txt" 2>&1
    local gate_status=$?
    printf '%s\n' "$gate_status" > "$round_dir/$gate_name.exit"
    return "$gate_status"
}
case "$1" in
    test) check final-test go test -count=1 ./... ;;
    verbose) check final-verbose go test -count=1 -v ./... ;;
    static)
        check gofmt gofmt -l cmd internal
        for platform in linux windows darwin; do
            check "vet-$platform" env GOOS="$platform" go vet ./...
            check "build-$platform" env GOOS="$platform" go build ./...
        done
        ;;
esac
