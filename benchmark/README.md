# Memorix Benchmark Harness

This directory contains benchmark tools for evaluating memorix memory recall quality and API performance.

## Table of Contents

- [Quick Start](#quick-start)
- [Test Scenarios](#test-scenarios)
- [Configuration](#configuration)
- [Custom Test Scenarios](#custom-test-scenarios)
- [Performance Testing](#performance-testing)
- [Performance Tuning](#performance-tuning)
- [CI Integration](#ci-integration)
- [Troubleshooting](#troubleshooting)

## Quick Start

Run your first benchmark in under 5 minutes:

```bash
# 1. Build and start memorix-server (from project root)
make build
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" \
  MNEMO_LLM_API_KEY="sk-..." \
  MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
  make run

# 2. Provision a user token (one-time setup)
curl -s -X POST http://127.0.0.1:18081/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"benchmark-user"}' | jq .
# Output: { "ok": true, "user_id": "...", "api_token": "mnemo_..." }

# 3. Set environment variables
export MNEMO_API_TOKEN="mnemo_..."  # from step 2
export BENCH_PROMPT_FILE="benchmark/prompts/example.yaml"

# 4. Run the benchmark
cd benchmark && make bench

# 5. View the report
open results/YYYYMMDD-HHMMSS/report.html
```

### Prerequisites

| Requirement | Version | Install |
|-------------|---------|---------|
| Python | 3.8+ | `apt install python3` or `brew install python` |
| pyyaml | Latest | `pip3 install pyyaml` |
| jq | Latest | `apt install jq` or `brew install jq` |
| curl | Any | Usually pre-installed |

## Test Scenarios

### Functional Benchmarks

Functional benchmarks test memory recall quality through scripted conversations.

| Scenario | File | Description |
|----------|------|-------------|
| `simple-recall` | `prompts/simple-recall.yaml` | Basic memory storage and recall test with expected facts validation |
| `hybrid-search` | `prompts/hybrid-search.yaml` | Semantic similarity search (different wording) |
| `smart-ingest` | `prompts/smart-ingest.yaml` | LLM fact extraction and recall |
| `example` | `prompts/example.yaml` | Simple example scenario |

### Running Scenarios

```bash
# Run a specific scenario
bash scripts/benchmark.sh --scenario simple-recall

# Run all scenarios
bash scripts/benchmark.sh --all

# Or use Makefile targets
export MNEMO_API_TOKEN="mnemo_..."
export BENCH_PROMPT_FILE="benchmark/prompts/simple-recall.yaml"
make bench
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MNEMO_API_URL` | No | `http://127.0.0.1:18081` | memorix server base URL |
| `MNEMO_API_TOKEN` | Yes | - | User API token for authentication |
| `BENCH_PROMPT_FILE` | Yes* | - | Path to YAML prompt file |
| `BENCH_PROMPT_TIMEOUT` | No | `600` | Per-prompt timeout in seconds |
| `BENCH_CONCURRENCY` | No | `1` | Number of concurrent sessions |
| `BENCH_WAIT` | No | `3` | Seconds to wait after store operations |

*Required for `make bench`. Use `--scenario` flag with `benchmark.sh` as an alternative.

### Configuration File

The `configs/default.yaml` file defines default benchmark parameters:

```yaml
server:
  base_url: "${MNEMO_API_URL:http://127.0.0.1:18081}"
  timeout_seconds: 30
  max_retries: 3

benchmark:
  concurrency: 1
  prompt_timeout_seconds: 600
  gc_enabled: true
  warmup_iterations: 0

embedding:
  model: "${MNEMO_EMBED_MODEL:text-embedding-3-small}"
  dimension: 1536

storage:
  mode: "sections"
  auto_embed: true

metrics:
  collect:
    - latency_p50
    - latency_p99
    - recall_at_k
    - throughput
  output_format: "json"
```

## Custom Test Scenarios

### YAML Format

Test scenarios are defined in YAML files under `prompts/`:

```yaml
name: "my-custom-test"
description: "Description of what this test validates"

prompts:
  # Store operations - save information to memory
  - id: "store-1"
    type: "store"
    text: "Remember that the database uses PostgreSQL version 15"
    tags: ["database", "infrastructure"]

  - id: "store-2"
    type: "store"
    text: "Remember that the API key for production is sk-prod-12345"
    tags: ["secrets", "api"]

  # Query operations - test recall
  - id: "query-1"
    type: "query"
    text: "What database does the project use?"
    expected_facts:
      - "PostgreSQL"
      - "15"

  - id: "query-2"
    type: "query"
    text: "What is the production API key?"
    expected_facts:
      - "sk-prod-12345"
```

### Prompt Types

| Type | Description | Required Fields |
|------|-------------|-----------------|
| `store` | Save information to memory | `id`, `type`, `text`, `tags` (optional) |
| `query` | Test recall with expected facts | `id`, `type`, `text`, `expected_facts` |

### Expected Facts Validation

The `expected_facts` field in query prompts enables automatic validation:

```yaml
- id: "query-1"
  type: "query"
  text: "What is the deployment namespace?"
  expected_facts:
    - "memorix-prod"     # Fact must appear in response
    - "3 replicas"       # Supports partial matching
```

Validation is case-insensitive and uses substring matching.

### Creating a New Scenario

```bash
# 1. Create the YAML file
cat > benchmark/prompts/my-scenario.yaml << 'EOF'
name: "my-scenario"
description: "My custom test scenario"
prompts:
  - id: "store-1"
    type: "store"
    text: "Remember that the cache TTL is 300 seconds"
    tags: ["cache", "config"]
  - id: "query-1"
    type: "query"
    text: "What is the cache TTL?"
    expected_facts:
      - "300"
EOF

# 2. Run the scenario
bash scripts/benchmark.sh --scenario my-scenario
```

## Performance Testing

Performance benchmarks measure API latency, throughput, and scalability under load.

### Quick Start

```bash
# 1. Set environment variables
export MNEMO_API_TOKEN="mnemo_..."
export BENCH_PERF_SCENARIO="crud_baseline"
export BENCH_PERF_DURATION=30
export BENCH_PERF_CONCURRENCY=10

# 2. Run performance benchmark
make bench-perf

# 3. View results
make bench-perf-report
```

### Performance Scenarios

| Scenario | File | Description |
|----------|------|-------------|
| `crud_baseline` | `perf/scenarios/crud_baseline.yaml` | Baseline CRUD operations: write → read → mixed |
| `mixed_workload` | `perf/scenarios/mixed_workload.yaml` | Real-world usage: 50% query, 30% write, 20% update |
| `search_comparison` | `perf/scenarios/search_comparison.yaml` | Keyword vs hybrid search performance |
| `scale_1k` | `perf/scenarios/scale_1k.yaml` | Search latency with 1K memories |
| `scale_10k` | `perf/scenarios/scale_10k.yaml` | Search latency with 10K memories |
| `scale_100k` | `perf/scenarios/scale_100k.yaml` | Search latency with 100K memories |

### Performance Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BENCH_PERF_SCENARIO` | `crud_baseline` | Scenario name (without `.yaml`) |
| `BENCH_PERF_DURATION` | `60` | Test duration in seconds |
| `BENCH_PERF_CONCURRENCY` | `10` | Number of concurrent workers |
| `BENCH_PERF_RAMP_UP` | `0` | Ramp-up period in seconds |

### Scenario YAML Format

Performance scenarios define workload phases:

```yaml
name: "my-perf-test"
description: "Custom performance test"

phases:
  - name: "write-phase"
    description: "Write operations"
    duration_seconds: 20
    concurrency: 10
    ramp_up_seconds: 2
    operations:
      - type: "create_memory"
        weight: 100
        params:
          content_template: "Test memory {id} at {timestamp}"
          tags: ["perf-test"]

  - name: "mixed-phase"
    description: "Mixed operations"
    duration_seconds: 30
    concurrency: 20
    operations:
      - type: "search_memory"
        weight: 50
        params:
          query: "test"
          k: 10
      - type: "create_memory"
        weight: 30
        params:
          content_template: "Memory {id}"
      - type: "get_memory"
        weight: 20
        params:
          id_source: "created_ids"
```

### Operation Types

| Operation | Description | Key Parameters |
|-----------|-------------|----------------|
| `create_memory` | Create a new memory | `content_template`, `tags`, `metadata` |
| `get_memory` | Get memory by ID | `id_source` or `id` |
| `update_memory` | Update existing memory | `id_source`, `content_template` |
| `delete_memory` | Delete memory by ID | `id_source` or `id` |
| `search_memory` | Keyword search | `query`, `k` |
| `search_hybrid` | Hybrid search | `query`, `k` |
| `list_memories` | List with pagination | `limit`, `offset` |

### Weight Distribution

Operations use `weight` for probability distribution. Weights are normalized:

```yaml
operations:
  - type: "search_memory"
    weight: 50    # 50% probability
  - type: "create_memory"
    weight: 30    # 30% probability
  - type: "get_memory"
    weight: 20    # 20% probability
```

## Performance Tuning

### Server-Side Tuning

Adjust memorix-server configuration for better benchmark results:

```bash
# Connection pool settings
MNEMO_TENANT_POOL_MAX_IDLE=10
MNEMO_TENANT_POOL_MAX_OPEN=20
MNEMO_TENANT_POOL_TOTAL_LIMIT=500

# Rate limiting (increase for load tests)
MNEMO_RATE_LIMIT=1000
MNEMO_RATE_BURST=2000
```

### Benchmark Tuning

| Parameter | Low Load | High Load | Purpose |
|-----------|----------|-----------|---------|
| `BENCH_PERF_CONCURRENCY` | 5-10 | 50-100 | Simulates concurrent users |
| `BENCH_PERF_DURATION` | 30s | 120s+ | Longer = more stable averages |
| `BENCH_PERF_RAMP_UP` | 0s | 10s | Gradual worker start |

### Database Tuning

For TiDB/MySQL backends:

```sql
-- Increase connection limits
SET GLOBAL max_connections = 500;

-- Optimize for write-heavy workloads
SET GLOBAL tidb_txn_mode = 'optimistic';
```

### Interpreting Results

| Metric | Good | Warning | Critical |
|--------|------|---------|----------|
| Error rate | < 0.1% | 0.1% - 1% | > 1% |
| P50 latency | < 50ms | 50-200ms | > 200ms |
| P99 latency | < 200ms | 200-500ms | > 500ms |
| Throughput | > 100 req/s | 50-100 req/s | < 50 req/s |

## CI Integration

### GitHub Actions

Add a workflow for automated benchmarking:

```yaml
# .github/workflows/benchmark.yml
name: Benchmark

on:
  pull_request:
    branches: [main]
  workflow_dispatch:

jobs:
  benchmark:
    runs-on: ubuntu-latest
    services:
      tidb:
        image: pingcap/tidb:latest
        ports:
          - 4000:4000

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Setup Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          pip install pyyaml
          sudo apt-get install -y jq

      - name: Build server
        run: make build

      - name: Start server
        run: |
          MNEMO_DSN="root@tcp(localhost:4000)/memorix?parseTime=true" \
            MNEMO_LLM_API_KEY="${{ secrets.OPENAI_API_KEY }}" \
            MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
            ./server/bin/memorix-server &
          sleep 5

      - name: Provision test user
        id: provision
        run: |
          RESP=$(curl -s -X POST http://localhost:8080/api/users \
            -H "Content-Type: application/json" \
            -d '{"name":"ci-benchmark"}')
          echo "token=$(echo $RESP | jq -r '.api_token')" >> $GITHUB_OUTPUT

      - name: Run functional benchmarks
        env:
          MNEMO_API_TOKEN: ${{ steps.provision.outputs.token }}
        run: |
          bash benchmark/scripts/benchmark.sh --all

      - name: Run performance benchmarks
        env:
          MNEMO_API_TOKEN: ${{ steps.provision.outputs.token }}
          BENCH_PERF_DURATION: 30
          BENCH_PERF_CONCURRENCY: 10
        run: |
          cd benchmark && make bench-perf

      - name: Upload results
        uses: actions/upload-artifact@v4
        with:
          name: benchmark-results
          path: benchmark/results/
```

### Baseline Comparison

Store baseline results and compare against them:

```bash
# Save baseline
cp benchmark/results/perf-*.json benchmark/results/baseline.json

# In CI, compare against baseline
python3 benchmark/scripts/baseline.py \
  --baseline benchmark/results/baseline.json \
  --current benchmark/results/perf-latest.json \
  --threshold 10  # Fail if >10% regression
```

### CI Setup Script

Use the provided CI setup script:

```bash
# For self-hosted CI runners
bash benchmark/scripts/ci-setup.sh
```

## Troubleshooting

### Common Issues

#### Connection Refused

```
ERROR: Cannot connect to memorix server at http://127.0.0.1:18081
```

**Solution**: Ensure memorix-server is running:

```bash
# Check server status
curl http://localhost:18081/healthz

# Start server if not running
make run
```

#### Authentication Failed

```
ERROR: Failed to provision space: {"error":"unauthorized"}
```

**Solution**: Check your API token:

```bash
# Verify token format (should start with "mnemo_")
echo $MNEMO_API_TOKEN

# Re-provision if needed
curl -s -X POST http://localhost:18081/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"benchmark-user"}' | jq .
```

#### Python Module Not Found

```
ERROR: Python pyyaml is required.
```

**Solution**: Install dependencies:

```bash
pip3 install pyyaml
```

#### Timeout Errors

```
ERROR: Request timeout after 600s
```

**Solution**: Increase timeout or reduce workload:

```bash
# Increase timeout
export BENCH_PROMPT_TIMEOUT=1200

# Reduce concurrency
export BENCH_PERF_CONCURRENCY=5
```

#### High Error Rate

If benchmarks show >1% error rate:

1. Check server logs for errors
2. Verify database connectivity
3. Reduce concurrency
4. Check rate limit settings

```bash
# Check server health
curl http://localhost:18081/api/dashboard/overview

# Reduce load
export BENCH_PERF_CONCURRENCY=5
export MNEMO_RATE_LIMIT=1000
```

### Debug Mode

Enable verbose logging:

```bash
# For benchmark scripts
DEBUG=1 bash scripts/benchmark.sh --scenario example

# For performance tests
python3 perf/load_test.py --verbose ...
```

### Getting Help

1. Check the [docs/DESIGN.md](../docs/DESIGN.md) for architecture details
2. Review [perf/README.md](perf/README.md) for performance-specific docs
3. Open an issue at https://github.com/DeviosLang/memorix/issues

## Directory Layout

```
benchmark/
├── README.md                 # This file
├── Makefile                  # Make targets
├── configs/
│   └── default.yaml          # Default configuration
├── prompts/                  # Functional test scenarios
│   ├── example.yaml
│   ├── simple-recall.yaml
│   ├── hybrid-search.yaml
│   └── smart-ingest.yaml
├── perf/                     # Performance benchmarks
│   ├── README.md
│   ├── load_test.py
│   ├── scenarios/
│   │   ├── crud_baseline.yaml
│   │   ├── mixed_workload.yaml
│   │   └── scale_*.yaml
│   └── results/
├── scripts/
│   ├── benchmark.sh          # Main benchmark runner
│   ├── drive-session.py      # Functional test driver
│   ├── drive-ab-session.py   # A/B test driver
│   ├── report.py             # HTML report generator
│   ├── baseline.py           # Baseline comparison
│   └── ci-setup.sh           # CI environment setup
├── results/                  # Benchmark outputs (git-ignored)
└── workspace/                # Shared context files
```

## Make Targets

| Target | Description |
|--------|-------------|
| `make bench` | Run functional benchmark |
| `make bench-report` | Generate HTML report from latest results |
| `make bench-clean` | Clean functional benchmark results |
| `make bench-perf` | Run performance benchmark |
| `make bench-perf-report` | Show latest performance results |
| `make bench-perf-clean` | Clean performance benchmark results |
| `make help` | Show all available targets |
