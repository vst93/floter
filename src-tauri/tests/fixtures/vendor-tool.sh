#!/bin/sh
if [ "$1" = "--version" ]; then echo 'vendor 9.9.9'; exit 0; fi
if [ "$1" = "--help" ]; then printf 'Options:\n  -shouldNotBeDerived   Nope\n'; exit 0; fi
exit 0
