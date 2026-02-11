#!/bin/bash
# Start BlazeLog server and agent for local development

set -e

cd "$(dirname "$0")/.."

# Source .env.local if it exists
if [ -f .env.local ]; then
    source .env.local
fi

# Require all secrets to be set - no defaults for security
missing_vars=""
[ -z "$BLAZELOG_MASTER_KEY" ] && missing_vars="$missing_vars BLAZELOG_MASTER_KEY"
[ -z "$BLAZELOG_DB_KEY" ] && missing_vars="$missing_vars BLAZELOG_DB_KEY"
[ -z "$BLAZELOG_JWT_SECRET" ] && missing_vars="$missing_vars BLAZELOG_JWT_SECRET"
[ -z "$BLAZELOG_CSRF_SECRET" ] && missing_vars="$missing_vars BLAZELOG_CSRF_SECRET"
[ -z "$BLAZELOG_BOOTSTRAP_ADMIN_PASSWORD" ] && missing_vars="$missing_vars BLAZELOG_BOOTSTRAP_ADMIN_PASSWORD"

if [ -n "$missing_vars" ]; then
    echo "ERROR: Required environment variables not set:$missing_vars"
    echo ""
    echo "Generate secrets with:"
    echo "  export BLAZELOG_MASTER_KEY=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_DB_KEY=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_JWT_SECRET=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_CSRF_SECRET=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_BOOTSTRAP_ADMIN_PASSWORD='<strong-password>'"
    exit 1
fi

SERVER_CONFIG="${BLAZELOG_SERVER_CONFIG:-configs/server.yaml}"
AGENT_CONFIG="${BLAZELOG_AGENT_CONFIG:-configs/agent.yaml}"
START_AGENT="${BLAZELOG_START_AGENT:-true}"

if [ ! -f "$SERVER_CONFIG" ]; then
    echo "ERROR: Server config not found: $SERVER_CONFIG"
    exit 1
fi

if [ "$START_AGENT" = "true" ] && [ ! -f "$AGENT_CONFIG" ]; then
    echo "ERROR: Agent config not found: $AGENT_CONFIG"
    exit 1
fi

mkdir -p logs data

# Create nginx log files if they don't exist (mounted from Docker)
if [ -n "$BLAZELOG_NGINX_LOGS_DIR" ]; then
    mkdir -p "$BLAZELOG_NGINX_LOGS_DIR"
    touch "$BLAZELOG_NGINX_LOGS_DIR/access.log" "$BLAZELOG_NGINX_LOGS_DIR/error.log" 2>/dev/null
fi

if [ -f .blazelog-server.pid ] && kill -0 "$(cat .blazelog-server.pid)" 2>/dev/null; then
    echo "ERROR: Server already running (PID: $(cat .blazelog-server.pid))."
    echo "Stop it with: ./scripts/blazelog-stop.sh"
    exit 1
fi

# Start server
echo "Starting server..."
nohup ./build/blazelog-server -c "$SERVER_CONFIG" > logs/server.log 2>&1 &
echo $! > .blazelog-server.pid
echo "Server started (PID: $!)"

sleep 2

# Check server is running
if ! kill -0 $(cat .blazelog-server.pid) 2>/dev/null; then
    echo "ERROR: Server failed to start"
    tail -20 logs/server.log
    exit 1
fi

if [ "$START_AGENT" = "true" ]; then
    # Start agent
    echo "Starting agent..."
    nohup ./build/blazelog-agent -c "$AGENT_CONFIG" > logs/agent.log 2>&1 &
    echo $! > .blazelog-agent.pid
    echo "Agent started (PID: $!)"
fi

echo ""
echo "BlazeLog running:"
echo "  Web UI: http://localhost:8080"
echo "  Server log: logs/server.log"
if [ "$START_AGENT" = "true" ]; then
    echo "  Agent log: logs/agent.log"
fi
echo ""
echo "Stop with: ./scripts/blazelog-stop.sh"
