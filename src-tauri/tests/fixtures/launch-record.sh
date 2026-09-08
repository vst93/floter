#!/bin/sh
case "$1" in
  --launch)
    printf '%s\0' "$0" "$PWD" "$TERM" "$COLORTERM" "$TERM_PROGRAM" "${FLOTER_LAUNCH_VALUE:-missing}" "$@" > "$FLOTER_LAUNCH_RECORD"
    ;;
  --health|--optional)
    printf '%s|%s\n' "$1" "${FLOTER_LAUNCH_VALUE:-missing}" >> "$FLOTER_LAUNCH_PROBES"
    [ "${FLOTER_LAUNCH_FAIL:-}" != "$1" ] || exit 23
    ;;
  *) exit 31 ;;
esac
