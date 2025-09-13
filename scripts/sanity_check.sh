#!/usr/bin/env bash
set -euo pipefail
CFG="${1:-config.yaml}"
if [ ! -f "$CFG" ]; then
  echo "config.yaml missing"
  exit 1
fi
echo "Config present: $CFG"
exit 0