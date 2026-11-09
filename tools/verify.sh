#!/usr/bin/env bash
# One entry point; complete logs stay on disk, stdout stays below 30 lines.
set -u
# AGENT-NOTES.md: visual pins use the CJK font selected by this locale.
export LC_ALL=zh_CN.UTF-8
cd "$(dirname "$0")/.." || exit 1
round=${1:?usage: bash tools/verify.sh ROUND}
case "$round" in *[!A-Za-z0-9_-]*) exit 2 ;; esac
logs="docs/rounds/$round/logs"
mkdir -p "$logs"
failed=0
resume=${2:-}
run() {
  local name=$1 result attempt=0
  shift
  # Resume an externally interrupted invocation without repeating green gates.
  if [ "$resume" = --resume ] && [ -f "$logs/$name.exit" ] && [ "$(<"$logs/$name.exit")" = 0 ]; then
    printf 'PASS %-16s %s (completed)\n' "$name" "$logs/$name.log"
    return
  fi
  if [ -f "$logs/$name.attempts" ]; then
    read -r attempt <"$logs/$name.attempts"
  fi
  if [ "$attempt" -ge 2 ]; then
    failed=$((failed + 1))
    printf 'FAIL %-16s retry limit reached | %s/%s.log\n' "$name" "$logs" "$name"
    return
  fi
  attempt=$((attempt + 1))
  printf '%s\n' "$attempt" >"$logs/$name.attempts"
  "$@" >"$logs/$name.log" 2>&1
  result=$?
  printf '%s\n' "$result" >"$logs/$name.exit"
  mkdir -p "$logs/attempt$attempt"
  cp "$logs/$name.log" "$logs/$name.exit" "$logs/attempt$attempt/"
  if [ "$result" -eq 0 ]; then
    printf 'PASS %-16s %s\n' "$name" "$logs/$name.log"
  else
    failed=$((failed + 1))
    printf 'FAIL %-16s %s | ' "$name" "$logs/$name.log"
    # Verbose test logs start with RUN lines; show the actual diagnostics.
    if ! grep -m 3 -E '(^|[[:space:]])(FAIL|--- FAIL|panic:|fatal error:)|:[0-9]+(:[0-9]+)?:' "$logs/$name.log" >"$logs/$name.failure"; then
      head -n 3 "$logs/$name.log" >"$logs/$name.failure"
    fi
    tr '\n' ' ' <"$logs/$name.failure" | head -c 180
    printf '\n'
  fi
}
if [ "$resume" = --focused ]; then
  pattern=${3:?focused test pattern required}
  batch=${4:?focused evidence name required}
  case "$batch" in *[!A-Za-z0-9_-]*) exit 2 ;; esac
  run "$batch" go test -count=1 -v "${5:-./internal/settingsui}" -run "$pattern"
  printf 'FOCUSED PASS=%s evidence=%s/%s.log\n' "$(grep -c -- '--- PASS' "$logs/$batch.log" || true)" "$logs" "$batch"
  test "$failed" -eq 0
  exit
fi
run gofmt bash -c 'out=$(gofmt -l cmd internal); printf "%s" "$out"; test -z "$out"'
run vet-linux env GOOS=linux go vet ./...
run vet-windows env GOOS=windows go vet ./...
run vet-darwin env GOOS=darwin go vet ./...
run test go test -count=1 ./...
run verbose go test -count=1 -v ./...
for platform in linux windows darwin; do
  run "build-$platform" env GOOS="$platform" go build ./...
done
passes=$(grep -c -- '--- PASS' "$logs/verbose.log" || true)
functions=$(git grep -h '^func Test' -- '*_test.go' | wc -l)
printf 'TOTAL PASS=%s test_functions=%s failed_gates=%s\n' "$passes" "$functions" "$failed"
test "$failed" -eq 0
