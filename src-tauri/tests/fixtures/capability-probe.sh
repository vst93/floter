#!/bin/sh
case "$1" in
  --stored-version|--health|--usage)
    printf '%s|%s\n' "$1" "${FLOTER_PROBE_VALUE:-missing}" >> "$FLOTER_PROBE_MARKER"
    [ "$FLOTER_PROBE_VALUE" = platform ] || exit 31
    IFS= read -r behavior < "$FLOTER_PROBE_CONTROL"
    case "$1:$behavior" in
      --health:fail-health|--usage:fail-help|--stored-version:fail-version)
        printf 'fixture rejected %s\n' "$1" >&2
        exit 23 ;;
      --health:timeout) exec sleep 10 ;;
    esac
    printf '1.2.3\n'
    ;;
  --check|--required) exit 0 ;;
  --version) printf '1.0.0\n' ;;
  --sleep) exec sleep 10 ;;
  *) exit 1 ;;
esac
