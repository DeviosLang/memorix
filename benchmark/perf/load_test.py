#!/usr/bin/env python3
"""
load_test.py — Performance load testing framework for memorix-server.

Runs configurable load tests against memorix API endpoints, collecting
latency statistics (P50/P95/P99), throughput (QPS), and error rates.
Supports YAML-defined scenarios with weighted operation distributions.
"""

import argparse
import json
import os
import random
import string
import sys
import threading
import time
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from queue import Queue

try:
    import yaml
except ImportError:
    print("ERROR: pyyaml is required. Install with: pip3 install pyyaml", file=sys.stderr)
    sys.exit(1)


# ANSI color codes for terminal output
class Colors:
    GREEN = "\033[92m"
    YELLOW = "\033[93m"
    RED = "\033[91m"
    CYAN = "\033[96m"
    BOLD = "\033[1m"
    RESET = "\033[0m"


def colorize(text: str, color: str) -> str:
    """Apply ANSI color to text."""
    return f"{color}{text}{Colors.RESET}"


@dataclass
class RequestResult:
    """Result of a single HTTP request."""
    operation: str
    status: int
    elapsed_ms: float
    error: Optional[str] = None
    response_body: Optional[Dict] = None
    memory_id: Optional[str] = None


@dataclass
class ScenarioConfig:
    """Parsed scenario configuration."""
    name: str
    description: str
    phases: List[Dict]
    setup: Optional[Dict] = None
    teardown: Optional[Dict] = None


@dataclass
class BenchmarkStats:
    """Aggregated statistics for benchmark run."""
    total_requests: int = 0
    successful_requests: int = 0
    failed_requests: int = 0
    latencies_ms: List[float] = field(default_factory=list)
    operation_stats: Dict[str, Dict] = field(default_factory=dict)
    start_time: float = 0.0
    end_time: float = 0.0

    def add_result(self, result: RequestResult):
        """Add a request result to statistics."""
        self.total_requests += 1

        if result.error or result.status >= 400:
            self.failed_requests += 1
        else:
            self.successful_requests += 1
            self.latencies_ms.append(result.elapsed_ms)

        # Track per-operation stats
        op = result.operation
        if op not in self.operation_stats:
            self.operation_stats[op] = {
                "count": 0,
                "success": 0,
                "failed": 0,
                "latencies": [],
            }

        self.operation_stats[op]["count"] += 1
        if result.error or result.status >= 400:
            self.operation_stats[op]["failed"] += 1
        else:
            self.operation_stats[op]["success"] += 1
            self.operation_stats[op]["latencies"].append(result.elapsed_ms)


class MemorixClient:
    """HTTP client for memorix API."""

    def __init__(self, base_url: str, token: str, timeout: int = 30):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.created_ids: List[str] = []
        self.ids_lock = threading.Lock()

    def _make_request(
        self,
        method: str,
        path: str,
        data: Optional[Dict] = None,
        tenant_id: Optional[str] = None,
    ) -> RequestResult:
        """Make an HTTP request to memorix API."""
        url = f"{self.base_url}{path}"
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
        }

        body = None
        if data:
            body = json.dumps(data).encode("utf-8")

        req = urllib.request.Request(
            url,
            data=body,
            headers=headers,
            method=method,
        )

        start = time.monotonic()
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as response:
                response_body = response.read().decode("utf-8")
                elapsed_ms = (time.monotonic() - start) * 1000
                parsed_body = json.loads(response_body) if response_body else {}
                return RequestResult(
                    operation=path.split("/")[-1] or path,
                    status=response.status,
                    elapsed_ms=elapsed_ms,
                    response_body=parsed_body,
                )
        except urllib.error.HTTPError as e:
            elapsed_ms = (time.monotonic() - start) * 1000
            response_body = e.read().decode("utf-8") if e.fp else ""
            return RequestResult(
                operation=path.split("/")[-1] or path,
                status=e.code,
                elapsed_ms=elapsed_ms,
                error=f"HTTP {e.code}: {response_body[:200]}",
            )
        except urllib.error.URLError as e:
            elapsed_ms = (time.monotonic() - start) * 1000
            return RequestResult(
                operation=path.split("/")[-1] or path,
                status=0,
                elapsed_ms=elapsed_ms,
                error=str(e.reason),
            )
        except Exception as e:
            elapsed_ms = (time.monotonic() - start) * 1000
            return RequestResult(
                operation=path.split("/")[-1] or path,
                status=0,
                elapsed_ms=elapsed_ms,
                error=str(e),
            )

    def _get_tenant_path(self, tenant_id: str) -> str:
        """Get the tenant-scoped base path."""
        return f"/v1alpha1/memorix/{tenant_id}"

    def create_memory(
        self,
        tenant_id: str,
        content: str,
        tags: Optional[List[str]] = None,
        metadata: Optional[Dict] = None,
    ) -> RequestResult:
        """Create a new memory."""
        path = f"{self._get_tenant_path(tenant_id)}/memories"
        data = {
            "content": content,
            "tags": tags or [],
            "metadata": metadata or {},
        }
        result = self._make_request("POST", path, data)
        result.operation = "create_memory"

        # Store created ID for later use
        if result.response_body and "id" in result.response_body:
            with self.ids_lock:
                self.created_ids.append(result.response_body["id"])
                result.memory_id = result.response_body["id"]

        return result

    def get_memory(self, tenant_id: str, memory_id: str) -> RequestResult:
        """Get a memory by ID."""
        path = f"{self._get_tenant_path(tenant_id)}/memories/{memory_id}"
        result = self._make_request("GET", path)
        result.operation = "get_memory"
        return result

    def update_memory(
        self,
        tenant_id: str,
        memory_id: str,
        content: str,
        tags: Optional[List[str]] = None,
    ) -> RequestResult:
        """Update an existing memory."""
        path = f"{self._get_tenant_path(tenant_id)}/memories/{memory_id}"
        data = {
            "content": content,
            "tags": tags or [],
        }
        result = self._make_request("PUT", path, data)
        result.operation = "update_memory"
        return result

    def delete_memory(self, tenant_id: str, memory_id: str) -> RequestResult:
        """Delete a memory by ID."""
        path = f"{self._get_tenant_path(tenant_id)}/memories/{memory_id}"
        result = self._make_request("DELETE", path)
        result.operation = "delete_memory"
        return result

    def search_memory(
        self,
        tenant_id: str,
        query: str,
        k: int = 10,
    ) -> RequestResult:
        """Search memories (keyword search)."""
        path = f"{self._get_tenant_path(tenant_id)}/memories"
        result = self._make_request("GET", f"{path}?q={query}&limit={k}")
        result.operation = "search_memory"
        return result

    def search_hybrid(
        self,
        tenant_id: str,
        query: str,
        k: int = 10,
    ) -> RequestResult:
        """Hybrid search (keyword + semantic)."""
        path = f"{self._get_tenant_path(tenant_id)}/memories"
        # Hybrid search uses the same endpoint with semantic=true
        result = self._make_request("GET", f"{path}?q={query}&limit={k}&semantic=true")
        result.operation = "search_hybrid"
        return result

    def list_memories(
        self,
        tenant_id: str,
        limit: int = 50,
        offset: int = 0,
    ) -> RequestResult:
        """List memories with pagination."""
        path = f"{self._get_tenant_path(tenant_id)}/memories"
        result = self._make_request("GET", f"{path}?limit={limit}&offset={offset}")
        result.operation = "list_memories"
        return result

    def get_random_created_id(self) -> Optional[str]:
        """Get a random ID from previously created memories."""
        with self.ids_lock:
            if self.created_ids:
                return random.choice(self.created_ids)
        return None


def calculate_percentile(sorted_values: List[float], percentile: float) -> float:
    """Calculate percentile from sorted values."""
    if not sorted_values:
        return 0.0
    n = len(sorted_values)
    idx = int(n * percentile / 100)
    idx = min(idx, n - 1)
    return sorted_values[idx]


def generate_random_content(template: str, request_id: int) -> str:
    """Generate content from template with variable substitution."""
    result = template
    result = result.replace("{id}", str(request_id))
    result = result.replace("{timestamp}", datetime.now(timezone.utc).isoformat())
    result = result.replace("{random}", "".join(random.choices(string.ascii_lowercase, k=8)))
    return result


def execute_operation(
    client: MemorixClient,
    tenant_id: str,
    operation: Dict,
    request_id: int,
) -> RequestResult:
    """Execute a single operation based on configuration."""
    op_type = operation.get("type", "create_memory")
    params = operation.get("params", {})

    if op_type == "create_memory":
        content_template = params.get("content_template", "Test memory {id}")
        content = generate_random_content(content_template, request_id)
        tags = params.get("tags", [])
        metadata = params.get("metadata", {})
        return client.create_memory(tenant_id, content, tags, metadata)

    elif op_type == "get_memory":
        # Try to get ID from source or use specific ID
        id_source = params.get("id_source", "created_ids")
        if id_source == "created_ids":
            memory_id = client.get_random_created_id()
            if not memory_id:
                return RequestResult(
                    operation="get_memory",
                    status=0,
                    elapsed_ms=0,
                    error="No created IDs available",
                )
        else:
            memory_id = params.get("id", "unknown")
        return client.get_memory(tenant_id, memory_id)

    elif op_type == "update_memory":
        id_source = params.get("id_source", "created_ids")
        if id_source == "created_ids":
            memory_id = client.get_random_created_id()
            if not memory_id:
                return RequestResult(
                    operation="update_memory",
                    status=0,
                    elapsed_ms=0,
                    error="No created IDs available",
                )
        else:
            memory_id = params.get("id", "unknown")
        content_template = params.get("content_template", "Updated memory {id}")
        content = generate_random_content(content_template, request_id)
        tags = params.get("tags", [])
        return client.update_memory(tenant_id, memory_id, content, tags)

    elif op_type == "delete_memory":
        id_source = params.get("id_source", "created_ids")
        if id_source == "created_ids":
            memory_id = client.get_random_created_id()
            if not memory_id:
                return RequestResult(
                    operation="delete_memory",
                    status=0,
                    elapsed_ms=0,
                    error="No created IDs available",
                )
        else:
            memory_id = params.get("id", "unknown")
        return client.delete_memory(tenant_id, memory_id)

    elif op_type == "search_memory":
        query = params.get("query", "test query")
        k = params.get("k", 10)
        return client.search_memory(tenant_id, query, k)

    elif op_type == "search_hybrid":
        query = params.get("query", "test query")
        k = params.get("k", 10)
        return client.search_hybrid(tenant_id, query, k)

    elif op_type == "list_memories":
        limit = params.get("limit", 50)
        offset = params.get("offset", 0)
        return client.list_memories(tenant_id, limit, offset)

    else:
        return RequestResult(
            operation=op_type,
            status=0,
            elapsed_ms=0,
            error=f"Unknown operation type: {op_type}",
        )


def worker(
    worker_id: int,
    client: MemorixClient,
    tenant_id: str,
    operations: List[Dict],
    duration_seconds: int,
    stats: BenchmarkStats,
    stop_event: threading.Event,
    request_counter: List[int],
    counter_lock: threading.Lock,
):
    """Worker thread that executes operations for the duration of the test."""
    # Calculate total weight for weighted selection
    total_weight = sum(op.get("weight", 1) for op in operations)

    while not stop_event.is_set():
        # Select operation based on weight
        rand = random.uniform(0, total_weight)
        cumulative = 0
        selected_op = operations[0]
        for op in operations:
            cumulative += op.get("weight", 1)
            if rand <= cumulative:
                selected_op = op
                break

        # Get request ID
        with counter_lock:
            request_counter[0] += 1
            request_id = request_counter[0]

        # Execute operation
        result = execute_operation(client, tenant_id, selected_op, request_id)
        stats.add_result(result)


def run_phase(
    client: MemorixClient,
    tenant_id: str,
    phase: Dict,
    stats: BenchmarkStats,
    phase_name: str,
) -> None:
    """Run a single phase of the benchmark."""
    duration = phase.get("duration_seconds", 60)
    concurrency = phase.get("concurrency", 10)
    ramp_up = phase.get("ramp_up_seconds", 0)
    operations = phase.get("operations", [])

    if not operations:
        print(f"  {Colors.YELLOW}Warning: No operations defined for phase {phase_name}{Colors.RESET}")
        return

    print(f"\n  {Colors.CYAN}Phase: {phase.get('name', phase_name)}{Colors.RESET}")
    print(f"    Duration: {duration}s | Concurrency: {concurrency} | Ramp-up: {ramp_up}s")

    # Prepare weighted operations list
    weighted_ops = []
    for op in operations:
        weight = op.get("weight", 1)
        weighted_ops.extend([op] * weight)

    stop_event = threading.Event()
    request_counter = [0]
    counter_lock = threading.Lock()

    # Create workers with ramp-up
    workers = []
    phase_start = time.monotonic()

    for i in range(concurrency):
        t = threading.Thread(
            target=worker,
            args=(
                i,
                client,
                tenant_id,
                operations,  # Use original list for weighted selection
                duration,
                stats,
                stop_event,
                request_counter,
                counter_lock,
            ),
        )
        workers.append(t)

        # Stagger start if ramp-up is configured
        if ramp_up > 0 and concurrency > 1:
            delay = (ramp_up / concurrency) * i
            time.sleep(delay)

        t.start()

    # Wait for duration
    time.sleep(duration)
    stop_event.set()

    # Wait for workers to finish
    for t in workers:
        t.join(timeout=5)

    elapsed = time.monotonic() - phase_start
    print(f"    Completed in {elapsed:.1f}s | Requests: {request_counter[0]}")


def run_setup(
    client: MemorixClient,
    tenant_id: str,
    setup_config: Dict,
    stats: BenchmarkStats,
) -> None:
    """Run setup phase (e.g., pre-populate data)."""
    print(f"\n  {Colors.CYAN}Setup: Pre-populating data...{Colors.RESET}")

    num_memories = setup_config.get("num_memories", 0)
    content_template = setup_config.get("content_template", "Setup memory {id}")
    tags = setup_config.get("tags", [])

    if num_memories == 0:
        return

    batch_size = setup_config.get("batch_size", 100)
    created = 0

    for i in range(0, num_memories, batch_size):
        batch_end = min(i + batch_size, num_memories)
        for j in range(i, batch_end):
            content = generate_random_content(content_template, j)
            result = client.create_memory(tenant_id, content, tags)
            stats.add_result(result)
            created += 1

            if created % 1000 == 0:
                print(f"    Created {created}/{num_memories} memories...")

    print(f"    Created {created} memories for setup")


def generate_report(stats: BenchmarkStats, scenario_name: str, config: Dict) -> Dict:
    """Generate structured JSON report from statistics."""
    duration = stats.end_time - stats.start_time
    qps = stats.successful_requests / duration if duration > 0 else 0

    # Sort latencies for percentile calculation
    sorted_latencies = sorted(stats.latencies_ms)

    report = {
        "scenario": scenario_name,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "config": config,
        "summary": {
            "total_requests": stats.total_requests,
            "successful_requests": stats.successful_requests,
            "failed_requests": stats.failed_requests,
            "error_rate": round(stats.failed_requests / stats.total_requests, 4) if stats.total_requests > 0 else 0,
            "requests_per_second": round(qps, 2),
            "duration_seconds": round(duration, 2),
        },
        "latency": {
            "min_ms": round(min(sorted_latencies), 2) if sorted_latencies else 0,
            "max_ms": round(max(sorted_latencies), 2) if sorted_latencies else 0,
            "mean_ms": round(sum(sorted_latencies) / len(sorted_latencies), 2) if sorted_latencies else 0,
            "p50_ms": round(calculate_percentile(sorted_latencies, 50), 2),
            "p95_ms": round(calculate_percentile(sorted_latencies, 95), 2),
            "p99_ms": round(calculate_percentile(sorted_latencies, 99), 2),
        },
        "operations": {},
    }

    # Calculate per-operation stats
    for op_name, op_stats in stats.operation_stats.items():
        op_latencies = sorted(op_stats["latencies"])
        report["operations"][op_name] = {
            "count": op_stats["count"],
            "success": op_stats["success"],
            "failed": op_stats["failed"],
            "latency": {
                "min_ms": round(min(op_latencies), 2) if op_latencies else 0,
                "max_ms": round(max(op_latencies), 2) if op_latencies else 0,
                "mean_ms": round(sum(op_latencies) / len(op_latencies), 2) if op_latencies else 0,
                "p50_ms": round(calculate_percentile(op_latencies, 50), 2),
                "p95_ms": round(calculate_percentile(op_latencies, 95), 2),
                "p99_ms": round(calculate_percentile(op_latencies, 99), 2),
            },
        }

    return report


def print_terminal_summary(report: Dict) -> None:
    """Print human-readable summary to terminal."""
    print("\n" + "=" * 80)
    print(colorize(f"Performance Benchmark Results: {report['scenario']}", Colors.BOLD))
    print("=" * 80)

    print(f"\n{Colors.CYAN}Configuration:{Colors.RESET}")
    config = report.get("config", {})
    print(f"  Duration:    {config.get('duration_seconds', 'N/A')}s")
    print(f"  Concurrency: {config.get('concurrency', 'N/A')} workers")
    print(f"  Ramp-up:     {config.get('ramp_up_seconds', 0)}s")

    summary = report.get("summary", {})
    print(f"\n{Colors.CYAN}Summary:{Colors.RESET}")
    total = summary.get("total_requests", 0)
    success = summary.get("successful_requests", 0)
    failed = summary.get("failed_requests", 0)
    error_rate = summary.get("error_rate", 0)

    success_color = Colors.GREEN if error_rate < 0.01 else (Colors.YELLOW if error_rate < 0.05 else Colors.RED)
    print(f"  Total Requests:    {total}")
    print(f"  Successful:        {colorize(f'{success} ({100*(1-error_rate):.2f}%)', success_color)}")
    print(f"  Failed:            {colorize(str(failed), success_color)}")
    print(f"  Throughput:        {colorize(f\"{summary.get('requests_per_second', 0):.2f} req/s\", Colors.GREEN)}")

    latency = report.get("latency", {})
    print(f"\n{Colors.CYAN}Latency (ms):{Colors.RESET}")
    print(f"  Min:    {latency.get('min_ms', 0):.2f}")
    print(f"  Max:    {latency.get('max_ms', 0):.2f}")
    print(f"  Mean:   {latency.get('mean_ms', 0):.2f}")
    print(f"  P50:    {colorize(f\"{latency.get('p50_ms', 0):.2f}\", Colors.GREEN)}")
    print(f"  P95:    {latency.get('p95_ms', 0):.2f}")
    print(f"  P99:    {colorize(f\"{latency.get('p99_ms', 0):.2f}\", Colors.YELLOW)}")

    operations = report.get("operations", {})
    if operations:
        print(f"\n{Colors.CYAN}By Operation:{Colors.RESET}")
        print("┌" + "─" * 18 + "┬" + "─" * 12 + "┬" + "─" * 12 + "┬" + "─" * 12 + "┬" + "─" * 12 + "┐")
        print(f"│ {'Operation':<16} │ {'Count':>10} │ {'P50 (ms)':>10} │ {'P95 (ms)':>10} │ {'P99 (ms)':>10} │")
        print("├" + "─" * 18 + "┼" + "─" * 12 + "┼" + "─" * 12 + "┼" + "─" * 12 + "┼" + "─" * 12 + "┤")

        for op_name, op_data in sorted(operations.items()):
            count = op_data.get("count", 0)
            op_latency = op_data.get("latency", {})
            p50 = op_latency.get("p50_ms", 0)
            p95 = op_latency.get("p95_ms", 0)
            p99 = op_latency.get("p99_ms", 0)
            print(f"│ {op_name:<16} │ {count:>10} │ {p50:>10.2f} │ {p95:>10.2f} │ {p99:>10.2f} │")

        print("└" + "─" * 18 + "┴" + "─" * 12 + "┴" + "─" * 12 + "┴" + "─" * 12 + "┴" + "─" * 12 + "┘")

    print("\n" + "=" * 80)


def main():
    parser = argparse.ArgumentParser(description="memorix performance load test")
    parser.add_argument("--api-url", help="memorix API base URL")
    parser.add_argument("--api-token", help="API token")
    parser.add_argument("--scenario-file", required=True, help="YAML scenario file")
    parser.add_argument("--results-dir", required=True, help="Output directory for results")
    parser.add_argument("--duration", type=int, default=60, help="Test duration (seconds)")
    parser.add_argument("--concurrency", type=int, default=10, help="Number of concurrent workers")
    parser.add_argument("--ramp-up", type=int, default=0, help="Ramp-up period (seconds)")
    parser.add_argument("--timeout", type=int, default=30, help="HTTP request timeout (seconds)")
    args = parser.parse_args()

    # Get configuration from args or environment
    api_url = args.api_url or os.environ.get("MNEMO_API_URL", "http://127.0.0.1:18081")
    api_token = args.api_token or os.environ.get("MNEMO_API_TOKEN")

    if not api_token:
        print("ERROR: MNEMO_API_TOKEN is required", file=sys.stderr)
        sys.exit(1)

    # Load scenario
    with open(args.scenario_file) as f:
        scenario = yaml.safe_load(f)

    scenario_name = scenario.get("name", "unnamed")
    description = scenario.get("description", "")

    print(f"\n{Colors.BOLD}memorix Performance Benchmark{Colors.RESET}")
    print(f"  Scenario: {scenario_name}")
    print(f"  Server: {api_url}")
    print(f"  Description: {description}")
    print()

    # Provision tenant
    print(f"{Colors.CYAN}Provisioning tenant...{Colors.RESET}")
    client = MemorixClient(api_url, api_token, args.timeout)

    provision_req = urllib.request.Request(
        f"{api_url}/v1alpha1/memorix",
        data=b"{}",
        headers={
            "Authorization": f"Bearer {api_token}",
            "Content-Type": "application/json",
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(provision_req, timeout=30) as resp:
            provision_data = json.loads(resp.read().decode("utf-8"))
            tenant_id = provision_data.get("tenant_id")
            print(f"  Tenant ID: {tenant_id}")
    except Exception as e:
        print(f"  {Colors.RED}Failed to provision tenant: {e}{Colors.RESET}")
        sys.exit(1)

    # Initialize stats
    stats = BenchmarkStats()
    stats.start_time = time.monotonic()

    # Run setup if defined
    if scenario.get("setup"):
        run_setup(client, tenant_id, scenario["setup"], stats)

    # Run phases
    phases = scenario.get("phases", [])
    if not phases:
        # Single phase from command-line args
        phases = [{
            "name": "default",
            "duration_seconds": args.duration,
            "concurrency": args.concurrency,
            "ramp_up_seconds": args.ramp_up,
            "operations": scenario.get("operations", []),
        }]

    for i, phase in enumerate(phases):
        run_phase(client, tenant_id, phase, stats, f"phase_{i+1}")

    stats.end_time = time.monotonic()

    # Generate report
    config = {
        "duration_seconds": args.duration,
        "concurrency": args.concurrency,
        "ramp_up_seconds": args.ramp_up,
        "api_url": api_url,
        "scenario": scenario_name,
    }

    report = generate_report(stats, scenario_name, config)

    # Create results directory
    os.makedirs(args.results_dir, exist_ok=True)

    # Save JSON results
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    json_path = os.path.join(args.results_dir, f"perf-{timestamp}.json")
    with open(json_path, "w") as f:
        json.dump(report, f, indent=2, ensure_ascii=False)
    print(f"\n{Colors.GREEN}Results saved to {json_path}{Colors.RESET}")

    # Print terminal summary
    print_terminal_summary(report)


if __name__ == "__main__":
    main()
