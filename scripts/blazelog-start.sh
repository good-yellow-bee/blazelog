#!/bin/bash
# Start BlazeLog server and agent for Inpost Magento development

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

if [ -n "$missing_vars" ]; then
    echo "ERROR: Required environment variables not set:$missing_vars"
    echo ""
    echo "Generate secrets with:"
    echo "  export BLAZELOG_MASTER_KEY=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_DB_KEY=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_JWT_SECRET=\$(openssl rand -base64 32)"
    echo "  export BLAZELOG_CSRF_SECRET=\$(openssl rand -base64 32)"
    exit 1
fi

mkdir -p logs data

# Create nginx log files if they don't exist (mounted from Docker)
NGINX_LOGS="/Users/storm/PhpstormProjects/sii/inpost/magento-docker-configuration/logs/nginx"
mkdir -p "$NGINX_LOGS"
touch "$NGINX_LOGS/access.log" "$NGINX_LOGS/error.log" 2>/dev/null

# Start server
echo "Starting server..."
nohup ./build/blazelog-server -c configs/server-dev.yaml > logs/server.log 2>&1 &
echo $! > .blazelog-server.pid
echo "Server started (PID: $!)"

sleep 2

# Check server is running
if ! kill -0 $(cat .blazelog-server.pid) 2>/dev/null; then
    echo "ERROR: Server failed to start"
    tail -20 logs/server.log
    exit 1
fi

# Start agent
echo "Starting agent..."
nohup ./build/blazelog-agent -c configs/agent-inpost.yaml > logs/agent.log 2>&1 &
echo $! > .blazelog-agent.pid
echo "Agent started (PID: $!)"

echo ""
echo "BlazeLog running:"
echo "  Web UI: http://localhost:8080"
echo "  Server log: logs/server.log"
echo "  Agent log: logs/agent.log"
echo ""
echo "Stop with: ./scripts/blazelog-stop.sh"
