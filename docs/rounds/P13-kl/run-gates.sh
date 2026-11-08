#!/usr/bin/env bash
set -u
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
evidence=docs/rounds/P13-kl
failed=0
run() {
    local tag=$1
    shift
    "$@" > "$evidence/$tag.txt" 2>&1
    local result=$?
    printf '%s\n' "$result" > "$evidence/$tag.exit"
    if test "$result" != 0; then failed=1; fi
}
run gofmt gofmt -l cmd internal
if test -s "$evidence/gofmt.txt"; then failed=1; fi
for platform in linux windows darwin; do
    run "vet-$platform" env GOOS="$platform" go vet ./...
    run "build-$platform" env GOOS="$platform" go build ./...
done
run tests go test -count=1 ./...
run tests-verbose go test -count=1 -v ./...
grep -c -- '--- PASS' "$evidence/tests-verbose.txt" > "$evidence/pass-count.txt"
git grep -h '^func Test' -- '*_test.go' | wc -l > "$evidence/function-count.txt"
printf '%s\n' "$failed" > "$evidence/gates.exit"
exit "$failed"
