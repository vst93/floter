#!/bin/sh
if [ "$1" = "--help" ]; then
printf 'Options:\n  -old   Old flag\n  -new   New flag\n'
exit 0
fi
echo done
