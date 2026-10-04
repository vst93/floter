#!/bin/sh
if [ "$1" = "--help" ]; then
cat <<'EOF'
Usage: demo-tool [options]

Options:
  -o, --output <FILE>    Write result to FILE
      --verbose          Enable verbose logging
  -h, --help             Show this help
EOF
exit 0
fi
echo done
