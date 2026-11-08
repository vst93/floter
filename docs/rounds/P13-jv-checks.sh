#!/usr/bin/env bash
set -uo pipefail
export LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4
out=/tmp/floter-P13-jv
check() {
  local name=$1
  shift
  "$@" > "$out/$name.log" 2>&1
  local result=$?
  printf 'exit=%s\n' "$result" > "$out/$name.status"
  printf '%s exit=%s\n' "$name" "$result"
  return "$result"
}
case "$1" in
  platforms)
    check gofmt gofmt -l cmd internal && test ! -s "$out/gofmt.log" || exit 1
    for platform in linux windows darwin; do
      export GOOS=$platform
      check "vet-$platform" go vet ./... || exit 1
      check "build-$platform" go build ./... || exit 1
    done
    ;;
  normal) check final-normal go test -count=1 ./... ;;
  verbose) check final-verbose go test -count=1 -v ./... ;;
  race) check race go test -race -count=1 -v ./internal/settingsui ./internal/shell -run '^Test(OrphanBusy|Adoption)' ;;
esac
