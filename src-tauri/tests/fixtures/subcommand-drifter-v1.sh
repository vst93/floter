#!/bin/sh
if [ "$1" = "--version" ]; then echo 'subber 1.0.0'; exit 0; fi
if [ "$1" = "--help" ]; then printf 'Options:\n  -v   Verbose\n'; exit 0; fi
echo done
