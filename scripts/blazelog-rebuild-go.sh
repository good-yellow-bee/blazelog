#!/bin/bash
# Rebuild server binary (Go/templ) and restart the server

set -e

cd "$(dirname "$0")/.."

make build-server
./scripts/blazelog-restart-server.sh
