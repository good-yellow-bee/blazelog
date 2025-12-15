#!/bin/sh
# BlazeLog Agent Docker Entrypoint
#
# This script handles agent initialization before starting the main process.

set -e

# Wait for server to be ready (optional)
if [ -n "$BLAZELOG_WAIT_FOR_SERVER" ]; then
    echo "Waiting for server at $BLAZELOG_SERVER_ADDRESS..."

    # Extract host and port from address
    HOST=$(echo "$BLAZELOG_SERVER_ADDRESS" | cut -d: -f1)
    PORT=$(echo "$BLAZELOG_SERVER_ADDRESS" | cut -d: -f2)

    # Wait up to 60 seconds
    for i in $(seq 1 60); do
        if nc -z "$HOST" "$PORT" 2>/dev/null; then
            echo "Server is ready!"
            break
        fi
        if [ $i -eq 60 ]; then
            echo "Timeout waiting for server"
            exit 1
        fi
        sleep 1
    done
fi

# Set agent name from hostname if not specified
if [ -z "$BLAZELOG_AGENT_NAME" ]; then
    export BLAZELOG_AGENT_NAME=$(hostname)
fi

# Create directories if needed
mkdir -p /var/lib/blazelog 2>/dev/null || true

echo "Starting BlazeLog Agent..."
exec blazelog-agent "$@"
