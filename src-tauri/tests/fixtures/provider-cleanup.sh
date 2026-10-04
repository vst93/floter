#!/bin/sh

if [ "$1" = immediate ]; then exit 0; fi
printf '%s\n' "$$" > "$1"
(sleep 30) &
printf '%s\n' "$!" > "$2"
trap '' TERM INT
while :; do sleep 1; done
