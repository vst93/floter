#!/bin/sh
if [ "$1" = "--version" ]; then echo 'lister 1.0.0'; exit 0; fi
if [ "$1" = "--help" ]; then printf 'Options:\n  -old   Old flag\n'; exit 0; fi
echo done
