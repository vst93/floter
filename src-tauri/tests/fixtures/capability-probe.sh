#!/bin/sh
case "$1" in
  --check|--required) exit 0 ;;
  --version) printf '1.0.0\n' ;;
  --sleep) exec sleep 10 ;;
  *) exit 1 ;;
esac
