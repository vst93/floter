#!/bin/sh
# R98 fixture: a long-running process group that survives soft signals, so the
# run-kill test can prove a disable/uninstall terminates the whole group (the
# direct child *and* its grandchild) instead of waiting out the run timeout.
#
# It answers the connect-time capability probes (`--help`/`--version`) with a
# clean exit so creating the integration does not itself leave the script
# running — and, crucially, so the probe never treats `--help` as the PID-file
# path it is handed at run time.
if [ "$1" = "--help" ] || [ "$1" = "--version" ]; then
  exit 0
fi

printf '%s\n' "$$" > "$1"
(sleep 30) &
printf '%s\n' "$!" > "$2"
trap '' TERM INT
while :; do sleep 1; done
