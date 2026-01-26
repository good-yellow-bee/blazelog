#!/bin/bash
# Restart only the server process (keeps agent running)

set -e

cd "$(dirname "$0")/.."

if [ -f .blazelog-server.pid ]; then
    pid=$(cat .blazelog-server.pid)
    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid"
        echo "Server stopped (PID: $pid)"
    fi
    rm -f .blazelog-server.pid
fi

BLAZELOG_START_AGENT=false ./scripts/blazelog-start.sh
