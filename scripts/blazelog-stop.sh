#!/bin/bash
# Stop BlazeLog server and agent

cd "$(dirname "$0")/.."

if [ -f .blazelog-agent.pid ]; then
    pid=$(cat .blazelog-agent.pid)
    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid"
        echo "Agent stopped (PID: $pid)"
    fi
    rm .blazelog-agent.pid
fi

if [ -f .blazelog-server.pid ]; then
    pid=$(cat .blazelog-server.pid)
    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid"
        echo "Server stopped (PID: $pid)"
    fi
    rm .blazelog-server.pid
fi

echo "BlazeLog stopped"
