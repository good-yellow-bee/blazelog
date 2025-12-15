#!/bin/bash
# BlazeLog Load Test Runner
# Runs Go benchmarks and k6 load tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}BlazeLog Load Test Runner${NC}"
echo "=========================="

# Default values
BLAZELOG_URL="${BLAZELOG_URL:-http://localhost:8080}"
BLAZELOG_USER="${BLAZELOG_USER:-admin}"
BLAZELOG_PASS="${BLAZELOG_PASS:-admin123}"
RUN_GO_BENCHMARKS=true
RUN_K6_TESTS=false
K6_DURATION="30s"
K6_VUS=10

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            BLAZELOG_URL="$2"
            shift 2
            ;;
        --user)
            BLAZELOG_USER="$2"
            shift 2
            ;;
        --pass)
            BLAZELOG_PASS="$2"
            shift 2
            ;;
        --go-only)
            RUN_GO_BENCHMARKS=true
            RUN_K6_TESTS=false
            shift
            ;;
        --k6-only)
            RUN_GO_BENCHMARKS=false
            RUN_K6_TESTS=true
            shift
            ;;
        --all)
            RUN_GO_BENCHMARKS=true
            RUN_K6_TESTS=true
            shift
            ;;
        --k6-duration)
            K6_DURATION="$2"
            shift 2
            ;;
        --k6-vus)
            K6_VUS="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --url URL        BlazeLog server URL (default: http://localhost:8080)"
            echo "  --user USER      Admin username (default: admin)"
            echo "  --pass PASS      Admin password (default: admin123)"
            echo "  --go-only        Run only Go benchmarks"
            echo "  --k6-only        Run only k6 load tests"
            echo "  --all            Run both Go benchmarks and k6 tests"
            echo "  --k6-duration    k6 test duration (default: 30s)"
            echo "  --k6-vus         k6 virtual users (default: 10)"
            echo "  --help           Show this help"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Check if server is running
echo -e "\n${YELLOW}Checking server availability...${NC}"
if curl -s -o /dev/null -w "%{http_code}" "$BLAZELOG_URL/health" | grep -q "200"; then
    echo -e "${GREEN}Server is running at $BLAZELOG_URL${NC}"
else
    echo -e "${YELLOW}Warning: Server may not be running at $BLAZELOG_URL${NC}"
    echo "Some tests may fail."
fi

# Run Go benchmarks
if [ "$RUN_GO_BENCHMARKS" = true ]; then
    echo -e "\n${GREEN}Running Go Benchmarks${NC}"
    echo "======================"

    cd "$PROJECT_ROOT"

    echo -e "\n${YELLOW}API Benchmarks:${NC}"
    go test -bench=. -benchmem -run=^$ ./internal/api/ 2>&1 | tee /tmp/api-bench.txt || true

    echo -e "\n${YELLOW}gRPC Server Benchmarks:${NC}"
    go test -bench=. -benchmem -run=^$ ./internal/server/ 2>&1 | tee /tmp/grpc-bench.txt || true

    echo -e "\n${YELLOW}Storage Benchmarks:${NC}"
    go test -bench=. -benchmem -run=^$ ./internal/storage/ 2>&1 | tee /tmp/storage-bench.txt || true

    echo -e "\n${GREEN}Go benchmark results saved to /tmp/*-bench.txt${NC}"
fi

# Run k6 tests
if [ "$RUN_K6_TESTS" = true ]; then
    echo -e "\n${GREEN}Running k6 Load Tests${NC}"
    echo "======================"

    # Check if k6 is installed
    if ! command -v k6 &> /dev/null; then
        echo -e "${RED}k6 is not installed. Install with:${NC}"
        echo "  brew install k6  # macOS"
        echo "  apt install k6   # Debian/Ubuntu"
        echo "  https://k6.io/docs/getting-started/installation/"
        exit 1
    fi

    export BLAZELOG_URL
    export BLAZELOG_USER
    export BLAZELOG_PASS

    echo -e "\n${YELLOW}API Load Test:${NC}"
    k6 run --vus "$K6_VUS" --duration "$K6_DURATION" "$SCRIPT_DIR/k6/api-load.js" 2>&1 | tee /tmp/k6-api.txt || true

    echo -e "\n${YELLOW}Auth Stress Test:${NC}"
    k6 run "$SCRIPT_DIR/k6/auth-stress.js" 2>&1 | tee /tmp/k6-auth.txt || true

    echo -e "\n${YELLOW}Log Query Test:${NC}"
    k6 run --vus 5 --duration "$K6_DURATION" "$SCRIPT_DIR/k6/log-query.js" 2>&1 | tee /tmp/k6-query.txt || true

    echo -e "\n${GREEN}k6 results saved to /tmp/k6-*.txt${NC}"
fi

# Summary
echo -e "\n${GREEN}Load Test Complete${NC}"
echo "==================="
echo ""
echo "Results saved to:"
if [ "$RUN_GO_BENCHMARKS" = true ]; then
    echo "  Go benchmarks: /tmp/*-bench.txt"
fi
if [ "$RUN_K6_TESTS" = true ]; then
    echo "  k6 tests:      /tmp/k6-*.txt"
fi
echo ""
echo "Target metrics (from PLAN.md):"
echo "  - Agent: <50MB memory, <5% CPU for 10k logs/sec"
echo "  - Server: Process 100k logs/sec"
echo "  - Query: <100ms for last 24h queries"
