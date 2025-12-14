#!/bin/bash
# Test ClickHouse integration
# Usage: ./scripts/test-clickhouse.sh

set -e

CONTAINER_NAME="blazelog-clickhouse-test"
CH_PORT=9000

echo "Starting ClickHouse container..."
docker run -d --name $CONTAINER_NAME \
    -p $CH_PORT:9000 \
    -p 8123:8123 \
    clickhouse/clickhouse-server:latest

echo "Waiting for ClickHouse to be ready..."
for i in {1..30}; do
    if docker exec $CONTAINER_NAME clickhouse-client --query "SELECT 1" > /dev/null 2>&1; then
        echo "ClickHouse is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Timeout waiting for ClickHouse"
        docker rm -f $CONTAINER_NAME
        exit 1
    fi
    sleep 1
done

# Create test database
echo "Creating test database..."
docker exec $CONTAINER_NAME clickhouse-client --query "CREATE DATABASE IF NOT EXISTS blazelog_test"

# Run integration tests
echo "Running integration tests..."
go test -tags=integration ./internal/storage/... -v

# Cleanup
echo "Cleaning up..."
docker rm -f $CONTAINER_NAME

echo "Done!"
