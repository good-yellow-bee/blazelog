#!/bin/bash
# Rebuild frontend assets (Tailwind CSS only)

set -e

cd "$(dirname "$0")/.."

make web-build
echo "CSS rebuilt. If you changed .templ files, run ./scripts/blazelog-rebuild-go.sh"
