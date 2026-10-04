#!/bin/sh
if [ "$1" = "--version" ]; then echo 'subber 2.0.0'; exit 0; fi
if [ "$1" = "--help" ]; then printf 'Available Plugins\n==================================================\n📦 alpha 1.0.0 👤 vst\n  Alpha thing\n'; exit 0; fi
echo done
