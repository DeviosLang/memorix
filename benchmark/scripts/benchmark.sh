#!/usr/bin/env bash
#
# benchmark.sh — Main benchmark coordinator for memorix.
#
# This script orchestrates benchmark runs against a memorix server,
# provisioning fresh spaces and running test scenarios.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ---------------------------------------------------------------------------
# Configuration (override via env vars)
# ---------------------------------------------------------------------------
MNEMO_API_URL="${MNEMO_API_URL:-http://127.0.0.1:18081}"
MNEMO_API_URL="${MNEMO_API_URL%/}"
MNEMO_API_TOKEN="${MNEMO_API_TOKEN:-}"
BENCH_PROMPT_FILE="${BENCH_PROMPT_FILE:-}"
BENCH_PROMPT_TIMEOUT="${BENCH_PROMPT_TIMEOUT:-600}"
BENCH_CONCURRENCY="${BENCH_CONCURRENCY:-1}"
BENCH_CONFIG="${BENCH_CONFIG:-$ROOT/benchmark/configs/default.yaml}"

# ---------------------------------------------------------------------------
# Color output helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*" >&2; exit 1; }
step()  { echo ""; echo -e "${BLUE}===${NC} $*"; }

# ---------------------------------------------------------------------------
# Preflight checks
# ---------------------------------------------------------------------------
preflight() {
    step "Preflight checks"

    if [[ -z "$MNEMO_API_TOKEN" ]]; then
        fail "MNEMO_API_TOKEN is required but not set.\n  export MNEMO_API_TOKEN='mnemo_...'"
    fi

    if [[ -z "$BENCH_PROMPT_FILE" ]]; then
        fail "BENCH_PROMPT_FILE is required but not set.\n  export BENCH_PROMPT_FILE='benchmark/prompts/example.yaml'"
    fi

    if [[ ! -f "$BENCH_PROMPT_FILE" ]]; then
        fail "Prompt file not found: $BENCH_PROMPT_FILE"
    fi

    # Check required commands
    for cmd in jq curl python3; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            fail "$cmd is required but not installed."
        fi
    done
    ok "All required commands available"

    # Check Python dependencies
    if ! python3 -c "import yaml" 2>/dev/null; then
        fail "Python pyyaml is required. Install with: pip3 install pyyaml"
    fi
    ok "Python dependencies satisfied"

    # Check server connectivity
    info "Testing connection to memorix server at $MNEMO_API_URL"
    if ! curl -sf "${MNEMO_API_URL}/health" >/dev/null 2>&1; then
        fail "Cannot connect to memorix server at $MNEMO_API_URL\n  Make sure the server is running and accessible."
    fi
    ok "Server connection successful"
}

# ---------------------------------------------------------------------------
# Provision a fresh space for benchmarking
# ---------------------------------------------------------------------------
provision_space() {
    step "Provisioning fresh memorix space"

    info "Server: $MNEMO_API_URL"

    PROVISION_RESP=$(curl -sf -X POST "${MNEMO_API_URL}/api/spaces/provision" \
        -H "Authorization: Bearer $MNEMO_API_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{}')

    SPACE_TOKEN=$(echo "$PROVISION_RESP" | jq -r '.space_token // empty')

    if [[ -z "$SPACE_TOKEN" ]]; then
        fail "Failed to provision space:\n$PROVISION_RESP"
    fi

    SPACE_ID=$(echo "$PROVISION_RESP" | jq -r '.space_id // "unknown"')
    ok "Space provisioned: $SPACE_ID"
    echo "$SPACE_TOKEN"
}

# ---------------------------------------------------------------------------
# Run the benchmark
# ---------------------------------------------------------------------------
run_benchmark() {
    local space_token="$1"

    step "Running benchmark"

    RESULTS_DIR="$ROOT/benchmark/results/$(date -u +%Y%m%d-%H%M%S)"
    mkdir -p "$RESULTS_DIR"
    info "Results directory: $RESULTS_DIR"

    # Run the Python driver
    python3 "$SCRIPT_DIR/drive-session.py" \
        --api-url "$MNEMO_API_URL" \
        --api-token "$space_token" \
        --prompt-file "$BENCH_PROMPT_FILE" \
        --results-dir "$RESULTS_DIR" \
        --timeout "$BENCH_PROMPT_TIMEOUT" \
        --concurrency "$BENCH_CONCURRENCY"

    ok "Benchmark completed"
    echo "$RESULTS_DIR"
}

# ---------------------------------------------------------------------------
# Generate report
# ---------------------------------------------------------------------------
generate_report() {
    local results_dir="$1"

    step "Generating report"

    if [[ -f "$results_dir/benchmark-results.json" ]]; then
        python3 "$SCRIPT_DIR/report.py" \
            "$results_dir/benchmark-results.json" \
            > "$results_dir/report.html"
        ok "Report written to $results_dir/report.html"
    else
        warn "No benchmark-results.json found, skipping report generation"
    fi
}

# ---------------------------------------------------------------------------
# Summary output
# ---------------------------------------------------------------------------
print_summary() {
    local results_dir="$1"
    local space_token="$2"

    echo ""
    echo "============================================================"
    echo "  Benchmark Complete!"
    echo "============================================================"
    echo ""
    echo "  Server: $MNEMO_API_URL"
    echo "  Prompt file: $BENCH_PROMPT_FILE"
    echo "  Results: $results_dir"
    echo ""
    if [[ -f "$results_dir/report.html" ]]; then
        echo "  HTML report: $results_dir/report.html"
    fi
    if [[ -f "$results_dir/transcript.md" ]]; then
        echo "  Transcript: $results_dir/transcript.md"
    fi
    if [[ -f "$results_dir/benchmark-results.json" ]]; then
        echo "  JSON output: $results_dir/benchmark-results.json"
    fi
    echo ""
    echo "============================================================"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    echo ""
    echo "============================================================"
    echo "  Memorix Benchmark Runner"
    echo "============================================================"
    echo ""
    echo "  API URL: $MNEMO_API_URL"
    echo "  Prompt file: $BENCH_PROMPT_FILE"
    echo "  Timeout: ${BENCH_PROMPT_TIMEOUT}s"
    echo "  Concurrency: $BENCH_CONCURRENCY"
    echo ""

    preflight
    SPACE_TOKEN=$(provision_space)
    RESULTS_DIR=$(run_benchmark "$SPACE_TOKEN")
    generate_report "$RESULTS_DIR"
    print_summary "$RESULTS_DIR" "$SPACE_TOKEN"
}

main "$@"
