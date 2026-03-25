#!/usr/bin/env bash
#
# benchmark.sh — Main benchmark coordinator for memorix.
#
# This script orchestrates benchmark runs against a memorix server,
# provisioning fresh spaces and running test scenarios.
#
# Supports:
#   - Single scenario: bash benchmark/scripts/benchmark.sh
#   - Specific scenario: bash benchmark/scripts/benchmark.sh --scenario simple-recall
#   - All scenarios: bash benchmark/scripts/benchmark.sh --all
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PROMPTS_DIR="$ROOT/benchmark/prompts"

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
BENCH_WAIT="${BENCH_WAIT:-3}"

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
    if ! curl -sf "${MNEMO_API_URL}/healthz" >/dev/null 2>&1; then
        warn "Health check failed, trying /health..."
        if ! curl -sf "${MNEMO_API_URL}/health" >/dev/null 2>&1; then
            fail "Cannot connect to memorix server at $MNEMO_API_URL\n  Make sure the server is running and accessible."
        fi
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
    local prompt_file="$2"

    step "Running benchmark"

    RESULTS_DIR="$ROOT/benchmark/results/$(date -u +%Y%m%d-%H%M%S)"
    mkdir -p "$RESULTS_DIR"
    info "Results directory: $RESULTS_DIR"
    info "Prompt file: $prompt_file"

    # Determine which driver to use
    # Use drive-ab-session.py for scenarios with expected_facts
    local driver_script="$SCRIPT_DIR/drive-session.py"
    if grep -q "expected_facts" "$prompt_file" 2>/dev/null; then
        driver_script="$SCRIPT_DIR/drive-ab-session.py"
        info "Using A/B driver with expected_facts validation"
    fi

    # Run the Python driver
    python3 "$driver_script" \
        --api-url "$MNEMO_API_URL" \
        --api-token "$space_token" \
        --prompt-file "$prompt_file" \
        --results-dir "$RESULTS_DIR" \
        --timeout "$BENCH_PROMPT_TIMEOUT" \
        --wait "$BENCH_WAIT"

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

    echo ""
    echo "============================================================"
    echo "  Benchmark Complete!"
    echo "============================================================"
    echo ""
    echo "  Server: $MNEMO_API_URL"
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
        echo ""
        echo "  Summary:"
        jq -r '.summary | "    Turns: \(.total_turns), Errors: \(.errors), Recall: \(.recall_success // "N/A")/\(.recall_turns // "N/A")"' "$results_dir/benchmark-results.json" 2>/dev/null || true
    fi
    echo ""
    echo "============================================================"
}

# ---------------------------------------------------------------------------
# Show usage
# ---------------------------------------------------------------------------
show_usage() {
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --scenario, -s NAME   Run specific scenario (without .yaml extension)"
    echo "  --all, -a             Run all scenarios in prompts/ directory"
    echo "  --help, -h            Show this help message"
    echo ""
    echo "Available scenarios:"
    for f in "$PROMPTS_DIR"/*.yaml; do
        [ -f "$f" ] && echo "  - $(basename "$f" .yaml)"
    done
    echo ""
    echo "Environment variables:"
    echo "  MNEMO_API_URL       Server base URL (default: http://127.0.0.1:18081)"
    echo "  MNEMO_API_TOKEN     User API token (required)"
    echo "  BENCH_PROMPT_FILE   Prompt file path (alternative to --scenario)"
    echo "  BENCH_WAIT          Seconds to wait after store ops (default: 3)"
    echo ""
    exit 0
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    local scenarios=()

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --scenario|-s)
                shift
                local scenario_name="$1"
                local scenario_file="$PROMPTS_DIR/${scenario_name}.yaml"
                if [ ! -f "$scenario_file" ]; then
                    fail "Scenario file not found: $scenario_file"
                fi
                scenarios+=("$scenario_file")
                shift
                ;;
            --all|-a)
                for f in "$PROMPTS_DIR"/*.yaml; do
                    [ -f "$f" ] && scenarios+=("$f")
                done
                shift
                ;;
            --help|-h)
                show_usage
                ;;
            *)
                fail "Unknown option: $1. Use --help for usage."
                ;;
        esac
    done

    # Default to BENCH_PROMPT_FILE or show usage
    if [ ${#scenarios[@]} -eq 0 ]; then
        if [ -n "$BENCH_PROMPT_FILE" ] && [ -f "$BENCH_PROMPT_FILE" ]; then
            scenarios+=("$BENCH_PROMPT_FILE")
        else
            show_usage
        fi
    fi

    echo ""
    echo "============================================================"
    echo "  Memorix Benchmark Runner"
    echo "============================================================"
    echo ""
    echo "  API URL: $MNEMO_API_URL"
    echo "  Scenarios: ${#scenarios[@]}"
    echo "  Timeout: ${BENCH_PROMPT_TIMEOUT}s"
    echo ""

    preflight

    local pass=0
    local fail_count=0
    local failed_scenarios=()

    for scenario_file in "${scenarios[@]}"; do
        local scenario_name
        scenario_name=$(basename "$scenario_file" .yaml)

        echo ""
        echo "============================================================"
        echo "  Running: $scenario_name"
        echo "============================================================"

        SPACE_TOKEN=$(provision_space)
        RESULTS_DIR=$(run_benchmark "$SPACE_TOKEN" "$scenario_file")
        generate_report "$RESULTS_DIR"
        print_summary "$RESULTS_DIR"

        # Check for errors
        local errors
        errors=$(jq -r '.summary.errors // 0' "$RESULTS_DIR/benchmark-results.json" 2>/dev/null || echo "1")
        if [ "$errors" -eq 0 ]; then
            ((pass++)) || true
        else
            ((fail_count++)) || true
            failed_scenarios+=("$scenario_name")
        fi
    done

    echo ""
    echo "============================================================"
    echo "  ALL BENCHMARKS COMPLETE"
    echo "============================================================"
    echo "  Passed: $pass"
    echo "  Failed: $fail_count"
    if [ $fail_count -gt 0 ]; then
        echo "  Failed scenarios: ${failed_scenarios[*]}"
    fi
    echo "============================================================"

    if [ $fail_count -gt 0 ]; then
        exit 1
    fi
}

main "$@"
