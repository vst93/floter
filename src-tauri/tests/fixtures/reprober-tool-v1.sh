#!/bin/sh
if [ "$1" = "--help" ]; then
printf 'Available Plugins\nalpha 1.0.0 (aliases: al)\n    First gadget\n'
exit 0
fi
if [ "$1" = "alpha" ]; then
printf 'Options:\n  -f         Format output\n'
exit 0
fi
echo done
