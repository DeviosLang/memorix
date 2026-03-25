# Memorix Performance Benchmarks

This directory contains load testing and performance benchmark tools for evaluating memorix-server API latency, throughput, and scalability.

## Overview

The performance benchmark framework runs HTTP load tests against memorix-server to establish baseline metrics for core API operations. It supports configurable concurrency levels, test durations, and workload scenarios defined in YAML files.

### Test Scenarios

| Scenario | Description |
|----------|-------------|
| `crud_baseline` | Baseline CRUD operations: pure write → pure read → mixed operations with increasing concurrency |
| `mixed_workload` | Simulates real agent usage: 50% query, 30% write, 20% update |
| `search_comparison` | Compares keyword search vs hybrid search performance on the same dataset |
| `scale_test` | Search latency curves with pre-populated datasets (1K/10K/100K memories) |

## Prerequisites

- **memorix-server** running (default `http://127.0.0.1:18081`, override with `MNEMO_API_URL`)
- **Python 3.8+** with `pyyaml` package
- **curl** for API interaction

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MNEMO_API_URL` | No | `http://127.0.0.1:18081` | memorix server base URL |
| `MNEMO_API_TOKEN` | Yes | - | User API token for authentication |
| `BENCH_PERF_SCENARIO` | No | `crud_baseline` | Scenario YAML file (without extension) |
| `BENCH_PERF_DURATION` | No | `60` | Test duration in seconds |
| `BENCH_PERF_CONCURRENCY` | No | `10` | Number of concurrent workers |
| `BENCH_PERF_RAMP_UP` | No | `0` | Ramp-up period in seconds |

## Quick Start

```bash
# 1. Build and start memorix-server
make build
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" make run

# 2. Provision a tenant token (one-time)
curl -s -X POST http://127.0.0.1:18081/v1alpha1/memorix | jq .
# → { "tenant_id": "...", "api_token": "mnemo_..." }

# 3. Set environment variables
export MNEMO_API_TOKEN="mnemo_..."  # from step 2
export BENCH_PERF_SCENARIO="crud_baseline"
export BENCH_PERF_DURATION=30
export BENCH_PERF_CONCURRENCY=10

# 4. Run performance benchmark
make bench-perf

# 5. Generate detailed report
make bench-perf-report
```

## Directory Layout

| Path | Description |
|------|-------------|
| `load_test.py` | Main load testing framework |
| `scenarios/*.yaml` | Test scenario definitions |
| `results/` | Benchmark outputs (git-ignored) |

## Scenario YAML Format

Scenarios define the workload distribution for load tests:

```yaml
name: "scenario-name"
description: "Human-readable description"
phases:
  - name: "phase-name"
    duration_seconds: 30
    concurrency: 10
    operations:
      - type: "create_memory"
        weight: 30
        params:
          content_template: "Test memory {id}"
      - type: "search_memory"
        weight: 50
        params:
          query: "test query"
          k: 10
      - type: "get_memory"
        weight: 20
        params:
          id_source: "created_ids"  # Use IDs from previous operations
```

### Operation Types

| Operation | Description | Key Parameters |
|-----------|-------------|----------------|
| `create_memory` | Create a new memory | `content_template`, `tags`, `metadata` |
| `get_memory` | Get memory by ID | `id_source` or `id` |
| `update_memory` | Update existing memory | `id_source`, `content_template` |
| `delete_memory` | Delete memory by ID | `id_source` or `id` |
| `search_memory` | Search memories (keyword) | `query`, `k` |
| `search_hybrid` | Hybrid search (keyword + semantic) | `query`, `k` |
| `list_memories` | List memories with pagination | `limit`, `offset` |

### Weight Distribution

Operations within a phase use `weight` to determine the probability of selection. Weights are normalized, so a 30/50/20 distribution means 30% create, 50% search, 20% get.

## Output Format

### JSON Results

Each benchmark run produces a timestamped JSON file in `results/`:

```json
{
  "scenario": "crud_baseline",
  "timestamp": "2024-01-15T14:30:00Z",
  "config": {
    "duration_seconds": 60,
    "concurrency": 10,
    "ramp_up_seconds": 0
  },
  "summary": {
    "total_requests": 5432,
    "successful_requests": 5401,
    "failed_requests": 31,
    "error_rate": 0.0057,
    "requests_per_second": 90.53
  },
  "latency": {
    "min_ms": 5.2,
    "max_ms": 1523.4,
    "mean_ms": 108.3,
    "p50_ms": 89.2,
    "p95_ms": 234.1,
    "p99_ms": 456.7
  },
  "operations": {
    "create_memory": { ... },
    "search_memory": { ... }
  }
}
```

### Terminal Summary

After each run, a human-readable summary is printed:

```
================================================================================
Performance Benchmark Results: crud_baseline
================================================================================

Configuration:
  Duration:    60s
  Concurrency: 10 workers
  Ramp-up:     0s

Summary:
  Total Requests:    5432
  Successful:        5401 (99.43%)
  Failed:            31 (0.57%)
  Throughput:        90.53 req/s

Latency (ms):
  Min:    5.2
  Max:    1523.4
  Mean:   108.3
  P50:    89.2
  P95:    234.1
  P99:    456.7

By Operation:
┌─────────────────┬───────────┬───────────┬───────────┬───────────┐
│ Operation       │ Count     │ P50 (ms)  │ P95 (ms)  │ P99 (ms)  │
├─────────────────┼───────────┼───────────┼───────────┼───────────┤
│ create_memory   │ 1629      │ 125.3     │ 287.4     │ 521.8     │
│ search_memory   │ 2716      │ 78.4      │ 198.2     │ 412.3     │
│ get_memory      │ 1087      │ 45.2      │ 123.1     │ 234.5     │
└─────────────────┴───────────┴───────────┴───────────┴───────────┘
================================================================================
```

## Make Targets

| Target | Description |
|--------|-------------|
| `make bench-perf` | Run performance benchmark |
| `make bench-perf-report` | Generate detailed report from latest results |
| `make bench-perf-clean` | Clean performance benchmark results |

## Best Practices

1. **Warm-up**: Run a short warm-up phase before measuring to allow connection pooling and caching to stabilize.

2. **Multiple Runs**: Run each scenario 3+ times and report averages to account for variance.

3. **Isolation**: Run benchmarks on dedicated hardware or during low-traffic periods to minimize noise.

4. **Monitoring**: Monitor server resource usage (CPU, memory, DB connections) during tests to identify bottlenecks.

5. **Baseline Comparison**: Save baseline results and compare new runs against them to detect performance regressions.
