#!/bin/sh
if [ "$1" = "--version" ]; then echo 'drifting 1.0.0'; exit 0; fi
if [ "$1" = "--help" ]; then printf 'Options:\n  -old   Old flag\n'; exit 0; fi
exit 0
