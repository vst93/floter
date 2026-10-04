#!/bin/sh
case "$1" in
  --version)  echo "floter-tool 1.2.3"; exit 0 ;;
  --help)     echo "Usage: floter-tool [options]"; exit 0 ;;
  --features) echo "json markdown"; exit 0 ;;
  --defunct)  echo "not supported"; exit 3 ;;
  *) echo "unknown flag: $1" >&2; exit 1 ;;
esac
