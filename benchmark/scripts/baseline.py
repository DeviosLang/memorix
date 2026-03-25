#!/usr/bin/env python3
"""
baseline.py — Benchmark baseline management and regression detection.

Manages baseline performance metrics and detects regressions by comparing
new benchmark results against stored baselines.

Usage:
  # Save current results as new baseline
  python3 benchmark/scripts/baseline.py save benchmark/perf/results/perf-latest.json

  # Compare results against baseline
  python3 benchmark/scripts/baseline.py compare benchmark/perf/results/perf-latest.json

  # Show current baseline
  python3 benchmark/scripts/baseline.py show

Regression thresholds:
  - P99 latency increase > 20% → REGRESSION
  - QPS decrease > 15% → REGRESSION
"""

import argparse
import json
import os
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

# ANSI color codes
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


# Default paths
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
ROOT_DIR = os.path.dirname(os.path.dirname(SCRIPT_DIR))
DEFAULT_BASELINE_PATH = os.path.join(ROOT_DIR, "benchmark", "results", "baseline.json")


@dataclass
class RegressionResult:
    """Result of regression detection."""
    has_regression: bool
    p99_change_pct: float
    qps_change_pct: float
    p99_regressed: bool
    qps_regressed: bool
    details: List[str]


def load_json_file(path: str) -> Dict[str, Any]:
    """Load JSON file."""
    with open(path, "r") as f:
        return json.load(f)


def save_json_file(path: str, data: Dict[str, Any]) -> None:
    """Save JSON file with pretty formatting."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(data, f, indent=2, ensure_ascii=False)


def extract_metrics(report: Dict[str, Any]) -> Dict[str, float]:
    """Extract key metrics from a benchmark report."""
    latency = report.get("latency", {})
    summary = report.get("summary", {})

    return {
        "p50_ms": latency.get("p50_ms", 0),
        "p95_ms": latency.get("p95_ms", 0),
        "p99_ms": latency.get("p99_ms", 0),
        "mean_ms": latency.get("mean_ms", 0),
        "qps": summary.get("requests_per_second", 0),
        "total_requests": summary.get("total_requests", 0),
        "successful_requests": summary.get("successful_requests", 0),
        "failed_requests": summary.get("failed_requests", 0),
        "error_rate": summary.get("error_rate", 0),
    }


def calculate_change_pct(new_value: float, baseline_value: float) -> float:
    """Calculate percentage change (positive = increase, negative = decrease)."""
    if baseline_value == 0:
        return 0.0
    return ((new_value - baseline_value) / baseline_value) * 100


def detect_regression(
    new_metrics: Dict[str, float],
    baseline_metrics: Dict[str, float],
    p99_threshold: float = 20.0,
    qps_threshold: float = 15.0,
) -> RegressionResult:
    """
    Detect performance regression by comparing metrics.

    Args:
        new_metrics: Latest benchmark metrics
        baseline_metrics: Baseline metrics to compare against
        p99_threshold: P99 latency increase threshold (%)
        qps_threshold: QPS decrease threshold (%)

    Returns:
        RegressionResult with details
    """
    details = []

    # P99 latency change (increase is bad)
    p99_change = calculate_change_pct(new_metrics["p99_ms"], baseline_metrics["p99_ms"])
    p99_regressed = p99_change > p99_threshold

    if p99_regressed:
        details.append(
            f"P99 latency regression: {baseline_metrics['p99_ms']:.2f}ms → "
            f"{new_metrics['p99_ms']:.2f}ms (+{p99_change:.1f}%)"
        )
    else:
        details.append(
            f"P99 latency: {baseline_metrics['p99_ms']:.2f}ms → "
            f"{new_metrics['p99_ms']:.2f}ms ({p99_change:+.1f}%)"
        )

    # QPS change (decrease is bad)
    qps_change = calculate_change_pct(new_metrics["qps"], baseline_metrics["qps"])
    qps_regressed = qps_change < -qps_threshold

    if qps_regressed:
        details.append(
            f"QPS regression: {baseline_metrics['qps']:.2f} → "
            f"{new_metrics['qps']:.2f} ({qps_change:.1f}%)"
        )
    else:
        details.append(
            f"QPS: {baseline_metrics['qps']:.2f} → "
            f"{new_metrics['qps']:.2f} ({qps_change:+.1f}%)"
        )

    # Additional metrics
    p50_change = calculate_change_pct(new_metrics["p50_ms"], baseline_metrics["p50_ms"])
    p95_change = calculate_change_pct(new_metrics["p95_ms"], baseline_metrics["p95_ms"])
    details.append(
        f"P50 latency: {baseline_metrics['p50_ms']:.2f}ms → "
        f"{new_metrics['p50_ms']:.2f}ms ({p50_change:+.1f}%)"
    )
    details.append(
        f"P95 latency: {baseline_metrics['p95_ms']:.2f}ms → "
        f"{new_metrics['p95_ms']:.2f}ms ({p95_change:+.1f}%)"
    )

    return RegressionResult(
        has_regression=p99_regressed or qps_regressed,
        p99_change_pct=p99_change,
        qps_change_pct=qps_change,
        p99_regressed=p99_regressed,
        qps_regressed=qps_regressed,
        details=details,
    )


def save_baseline(report_path: str, baseline_path: str = DEFAULT_BASELINE_PATH) -> None:
    """Save benchmark results as new baseline."""
    report = load_json_file(report_path)
    metrics = extract_metrics(report)

    baseline = {
        "version": 1,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "scenario": report.get("scenario", "unknown"),
        "metrics": metrics,
        "operations": report.get("operations", {}),
        "source_file": os.path.basename(report_path),
    }

    save_json_file(baseline_path, baseline)
    print(f"{Colors.GREEN}Baseline saved to {baseline_path}{Colors.RESET}")
    print_baseline_summary(baseline)


def compare_baseline(report_path: str, baseline_path: str = DEFAULT_BASELINE_PATH) -> int:
    """
    Compare benchmark results against baseline.

    Returns exit code: 0 = no regression, 1 = regression detected
    """
    if not os.path.exists(baseline_path):
        print(f"{Colors.YELLOW}No baseline found at {baseline_path}{Colors.RESET}")
        print("Run 'python3 benchmark/scripts/baseline.py save <results.json>' to create one.")
        return 0

    report = load_json_file(report_path)
    baseline = load_json_file(baseline_path)

    new_metrics = extract_metrics(report)
    baseline_metrics = baseline.get("metrics", {})

    print(f"\n{Colors.BOLD}Performance Comparison{Colors.RESET}")
    print(f"  Baseline: {baseline.get('timestamp', 'unknown')}")
    print(f"  Current:  {datetime.now(timezone.utc).isoformat()}")
    print(f"  Scenario: {report.get('scenario', 'unknown')}")
    print()

    result = detect_regression(new_metrics, baseline_metrics)

    for detail in result.details:
        print(f"  {detail}")

    print()

    if result.has_regression:
        print(f"{Colors.RED}{Colors.BOLD}PERFORMANCE REGRESSION DETECTED{Colors.RESET}")
        if result.p99_regressed:
            print(f"  - P99 latency increased by {result.p99_change_pct:.1f}% (threshold: 20%)")
        if result.qps_regressed:
            print(f"  - QPS decreased by {-result.qps_change_pct:.1f}% (threshold: 15%)")
        return 1
    else:
        print(f"{Colors.GREEN}No performance regression detected{Colors.RESET}")
        return 0


def show_baseline(baseline_path: str = DEFAULT_BASELINE_PATH) -> int:
    """Display current baseline."""
    if not os.path.exists(baseline_path):
        print(f"{Colors.YELLOW}No baseline found at {baseline_path}{Colors.RESET}")
        return 1

    baseline = load_json_file(baseline_path)
    print_baseline_summary(baseline)
    return 0


def print_baseline_summary(baseline: Dict[str, Any]) -> None:
    """Print a summary of the baseline."""
    metrics = baseline.get("metrics", {})

    print(f"\n{Colors.BOLD}Baseline Summary{Colors.RESET}")
    print(f"  Version:   {baseline.get('version', 1)}")
    print(f"  Timestamp: {baseline.get('timestamp', 'unknown')}")
    print(f"  Scenario:  {baseline.get('scenario', 'unknown')}")
    print()
    print(f"{Colors.CYAN}Key Metrics:{Colors.RESET}")
    print(f"  P50 Latency: {metrics.get('p50_ms', 0):.2f} ms")
    print(f"  P95 Latency: {metrics.get('p95_ms', 0):.2f} ms")
    print(f"  P99 Latency: {metrics.get('p99_ms', 0):.2f} ms")
    print(f"  QPS:         {metrics.get('qps', 0):.2f} req/s")
    print()

    operations = baseline.get("operations", {})
    if operations:
        print(f"{Colors.CYAN}Per-Operation Baselines:{Colors.RESET}")
        for op_name, op_data in sorted(operations.items()):
            op_latency = op_data.get("latency", {})
            print(f"  {op_name}:")
            print(f"    P50: {op_latency.get('p50_ms', 0):.2f}ms, "
                  f"P99: {op_latency.get('p99_ms', 0):.2f}ms, "
                  f"Count: {op_data.get('count', 0)}")


def output_github_summary(
    report: Dict[str, Any],
    baseline: Optional[Dict[str, Any]],
    result: Optional[RegressionResult],
) -> str:
    """Generate GitHub Actions summary markdown."""
    lines = []
    lines.append("## Benchmark Results\n")

    # Current metrics
    metrics = extract_metrics(report)
    lines.append("### Current Results\n")
    lines.append("| Metric | Value |")
    lines.append("|--------|-------|")
    lines.append(f"| P50 Latency | {metrics['p50_ms']:.2f} ms |")
    lines.append(f"| P95 Latency | {metrics['p95_ms']:.2f} ms |")
    lines.append(f"| P99 Latency | {metrics['p99_ms']:.2f} ms |")
    lines.append(f"| QPS | {metrics['qps']:.2f} req/s |")
    lines.append(f"| Total Requests | {metrics['total_requests']} |")
    lines.append(f"| Error Rate | {metrics['error_rate'] * 100:.2f}% |")
    lines.append("")

    # Comparison with baseline
    if baseline and result:
        lines.append("### Comparison with Baseline\n")
        lines.append("| Metric | Baseline | Current | Change | Status |")
        lines.append("|--------|----------|---------|--------|--------|")

        baseline_metrics = baseline.get("metrics", {})

        # P99
        p99_status = "🔴" if result.p99_regressed else "🟢"
        lines.append(
            f"| P99 Latency | {baseline_metrics['p99_ms']:.2f}ms | "
            f"{metrics['p99_ms']:.2f}ms | {result.p99_change_pct:+.1f}% | {p99_status} |"
        )

        # QPS
        qps_status = "🔴" if result.qps_regressed else "🟢"
        lines.append(
            f"| QPS | {baseline_metrics['qps']:.2f} | "
            f"{metrics['qps']:.2f} | {result.qps_change_pct:+.1f}% | {qps_status} |"
        )

        lines.append("")

        if result.has_regression:
            lines.append("### ⚠️ Performance Regression Detected\n")
            if result.p99_regressed:
                lines.append(f"- P99 latency increased by {result.p99_change_pct:.1f}% (threshold: 20%)")
            if result.qps_regressed:
                lines.append(f"- QPS decreased by {-result.qps_change_pct:.1f}% (threshold: 15%)")
        else:
            lines.append("### ✅ No Performance Regression\n")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="Benchmark baseline management and regression detection"
    )
    parser.add_argument(
        "--baseline-path",
        default=DEFAULT_BASELINE_PATH,
        help="Path to baseline JSON file",
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    # save command
    save_parser = subparsers.add_parser("save", help="Save benchmark results as baseline")
    save_parser.add_argument("results_file", help="Path to benchmark results JSON")

    # compare command
    compare_parser = subparsers.add_parser("compare", help="Compare results against baseline")
    compare_parser.add_argument("results_file", help="Path to benchmark results JSON")
    compare_parser.add_argument(
        "--output-summary",
        help="Path to write GitHub Actions summary markdown",
    )
    compare_parser.add_argument(
        "--save-if-passed",
        action="store_true",
        help="Save as new baseline if no regression detected",
    )

    # show command
    subparsers.add_parser("show", help="Show current baseline")

    args = parser.parse_args()

    if args.command == "save":
        save_baseline(args.results_file, args.baseline_path)

    elif args.command == "compare":
        exit_code = compare_baseline(args.results_file, args.baseline_path)

        # Optionally output GitHub summary
        if hasattr(args, "output_summary") and args.output_summary:
            report = load_json_file(args.results_file)
            baseline = None
            result = None
            if os.path.exists(args.baseline_path):
                baseline = load_json_file(args.baseline_path)
                new_metrics = extract_metrics(report)
                baseline_metrics = baseline.get("metrics", {})
                result = detect_regression(new_metrics, baseline_metrics)

            summary = output_github_summary(report, baseline, result)
            with open(args.output_summary, "w") as f:
                f.write(summary)

        # Save as new baseline if passed and requested
        if exit_code == 0 and hasattr(args, "save_if_passed") and args.save_if_passed:
            print(f"\n{Colors.CYAN}Saving as new baseline...{Colors.RESET}")
            save_baseline(args.results_file, args.baseline_path)

        sys.exit(exit_code)

    elif args.command == "show":
        sys.exit(show_baseline(args.baseline_path))


if __name__ == "__main__":
    main()
