# Memorix Benchmark Harnesses

This directory contains benchmark helpers and datasets for evaluating memorix memory recall capabilities.

## Overview

The benchmark framework runs A/B tests comparing different memory configurations to evaluate recall quality, performance, and correctness. It is designed to test memorix's core memory operations through scripted scenarios.

## Prerequisites

- **memorix-server** running (default `http://127.0.0.1:18081`, override with `MNEMO_API_URL`)
- **Python 3.8+** with `pyyaml` package
- **jq** installed (for JSON processing in shell scripts)
- **curl** for HTTP requests

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MNEMO_API_URL` | No | `http://127.0.0.1:18081` | memorix server base URL |
| `MNEMO_API_TOKEN` | Yes | - | User API token for authentication |
| `BENCH_PROMPT_FILE` | Yes | - | Path to YAML prompt file |
| `BENCH_PROMPT_TIMEOUT` | No | `600` | Per-prompt timeout in seconds |
| `BENCH_CONCURRENCY` | No | `1` | Number of concurrent sessions |

## Quick Start

```bash
# 1. Build and start memorix-server
make build
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" make run

# 2. Provision a user token (one-time, no auth required)
curl -s -X POST http://127.0.0.1:18081/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"benchmark-user"}' | jq .
# → { "ok": true, "user_id": "...", "api_token": "mnemo_..." }

# 3. Set environment variables
export MNEMO_API_TOKEN="mnemo_..."  # from step 2
export BENCH_PROMPT_FILE="benchmark/prompts/example.yaml"

# 4. Run benchmark
make bench

# 5. Generate report
make bench-report
```

## Directory Layout

| Path | Description |
|------|-------------|
| `configs/default.yaml` | Default benchmark configuration |
| `prompts/` | Test scenario YAML files |
| `scripts/benchmark.sh` | Main benchmark coordinator |
| `scripts/drive-session.py` | Parallel prompt driver |
| `scripts/report.py` | HTML report generator |
| `workspace/` | Shared context files for benchmark sessions |
| `results/` | Benchmark outputs (git-ignored) |

## Configuration

The `configs/default.yaml` file defines benchmark parameters:

```yaml
server:
  base_url: "http://127.0.0.1:18081"
  timeout_seconds: 30

benchmark:
  concurrency: 1
  prompt_timeout_seconds: 600
  gc_enabled: true

embedding:
  model: "text-embedding-3-small"
```

## Results Structure

Each benchmark run creates a timestamped directory under `results/`:

```
results/
└── 20240115-143000/
    ├── benchmark-results.json    # Structured test results
    ├── report.html               # HTML comparison report
    ├── transcript.md             # Human-readable transcript
    └── metrics.json              # Performance metrics
```

## Make Targets

| Target | Description |
|--------|-------------|
| `make bench` | Run benchmark (requires MNEMO_API_TOKEN and BENCH_PROMPT_FILE) |
| `make bench-report` | Generate HTML report from latest results |
| `make bench-clean` | Clean benchmark results directory |

## Writing Test Scenarios

Test scenarios are defined in YAML files under `prompts/`:

```yaml
name: "basic-recall"
description: "Test basic memory recall capabilities"
prompts:
  - "Remember that my favorite color is blue."
  - "What is my favorite color?"
```

## Notes

- Each benchmark run provisions a fresh space via `POST /api/spaces/provision`
- Benchmarks use the memorix Go backend directly, not agent plugins
- The framework supports both functional and performance benchmarks
- Results are automatically timestamped and stored in `results/`
