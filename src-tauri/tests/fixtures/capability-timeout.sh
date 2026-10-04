#!/bin/sh
printf '%s\n' "$$" > "$1"
trap '' TERM
while :; do sleep 1; done
