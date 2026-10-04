#!/bin/sh
if [ "$1" = "--help" ]; then
cat <<'EOF'
subber - Gadgets under the terminal
Version: dev  🏠 https://example.com/subber

Available Plugins
==================================================
📦 alpha 1.0.0 👤 vst  (aliases: al)
  First gadget does things
📦 beta 0.2.0 👤 vst
  Second gadget does other things

Run subber <command> -h for detailed help.
EOF
exit 0
fi
if [ "$1" = "alpha" ]; then
printf 'Modes:\n  -f         Format (pretty-print)\nOptions:\n  -sort   Sort object keys alphabetically\n'
exit 0
fi
if [ "$1" = "beta" ]; then
printf 'Options:\n  -raw   Disable colored output\n'
exit 0
fi
echo done
