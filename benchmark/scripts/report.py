#!/usr/bin/env python3
"""
report.py — HTML report generator for memorix benchmarks.

Reads benchmark-results.json and produces an HTML comparison report.
"""

import argparse
import json
import os
import sys
from datetime import datetime
from typing import Any, Dict, List


HTML_TEMPLATE = """<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Memorix Benchmark Report</title>
    <style>
        :root {
            --bg-color: #1a1a2e;
            --card-bg: #16213e;
            --text-color: #eaeaea;
            --accent-color: #0f3460;
            --success-color: #4ade80;
            --error-color: #f87171;
            --warning-color: #fbbf24;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            margin: 0;
            padding: 20px;
            line-height: 1.6;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        h1 {
            color: var(--success-color);
            border-bottom: 2px solid var(--accent-color);
            padding-bottom: 10px;
        }

        h2 {
            color: var(--warning-color);
            margin-top: 30px;
        }

        .summary-cards {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }

        .card {
            background-color: var(--card-bg);
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
        }

        .card h3 {
            margin: 0 0 10px 0;
            color: var(--text-color);
            font-size: 14px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .card .value {
            font-size: 28px;
            font-weight: bold;
            color: var(--success-color);
        }

        .card.error .value {
            color: var(--error-color);
        }

        .turn {
            background-color: var(--card-bg);
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
            border-left: 4px solid var(--accent-color);
        }

        .turn.store {
            border-left-color: var(--warning-color);
        }

        .turn.query {
            border-left-color: var(--success-color);
        }

        .turn-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
        }

        .turn-header h3 {
            margin: 0;
        }

        .turn-type {
            background-color: var(--accent-color);
            padding: 4px 12px;
            border-radius: 4px;
            font-size: 12px;
            text-transform: uppercase;
        }

        .prompt {
            background-color: var(--bg-color);
            padding: 15px;
            border-radius: 4px;
            margin: 10px 0;
            font-family: 'Monaco', 'Consolas', monospace;
            white-space: pre-wrap;
            word-break: break-word;
        }

        .response {
            background-color: var(--bg-color);
            padding: 15px;
            border-radius: 4px;
            margin: 10px 0;
        }

        .meta {
            font-size: 12px;
            color: #888;
            margin-top: 10px;
        }

        .error {
            color: var(--error-color);
            font-weight: bold;
        }

        .success {
            color: var(--success-color);
        }

        table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }

        th, td {
            background-color: var(--card-bg);
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid var(--accent-color);
        }

        th {
            background-color: var(--accent-color);
        }

        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid var(--accent-color);
            text-align: center;
            color: #666;
            font-size: 12px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Memorix Benchmark Report</h1>

        <div class="summary-cards">
            <div class="card">
                <h3>Scenario</h3>
                <div class="value">{scenario_name}</div>
            </div>
            <div class="card">
                <h3>Total Turns</h3>
                <div class="value">{total_turns}</div>
            </div>
            <div class="card">
                <h3>Total Time</h3>
                <div class="value">{total_time:.2f}s</div>
            </div>
            <div class="card">
                <h3>Avg Latency</h3>
                <div class="value">{avg_latency:.3f}s</div>
            </div>
            <div class="card {error_class}">
                <h3>Errors</h3>
                <div class="value">{error_count}</div>
            </div>
        </div>

        <h2>Latency Distribution</h2>
        <table>
            <tr>
                <th>Metric</th>
                <th>Value</th>
            </tr>
            <tr><td>Min</td><td>{latency_min:.3f}s</td></tr>
            <tr><td>Max</td><td>{latency_max:.3f}s</td></tr>
            <tr><td>Mean</td><td>{latency_mean:.3f}s</td></tr>
            <tr><td>P50</td><td>{latency_p50:.3f}s</td></tr>
            <tr><td>P99</td><td>{latency_p99:.3f}s</td></tr>
        </table>

        <h2>Turn Details</h2>
        {turns_html}

        <div class="footer">
            <p>Generated: {timestamp}</p>
            <p>Memorix Benchmark Framework</p>
        </div>
    </div>
</body>
</html>
"""


def render_turn(turn: Dict[str, Any]) -> str:
    """Render a single turn as HTML."""
    response = turn.get("response", {})
    error = response.get("error")
    elapsed = response.get("elapsed_seconds", 0)
    status = response.get("status", "N/A")
    turn_type = turn.get("turn_type", "query")
    parsed = turn.get("parsed_response", "(no response)")

    status_class = "error" if error else "success"
    status_text = f"ERROR: {error}" if error else f"HTTP {status}"

    return f"""
        <div class="turn {turn_type}">
            <div class="turn-header">
                <h3>Turn {turn.get('turn', '?')}</h3>
                <span class="turn-type">{turn_type}</span>
            </div>
            <div class="prompt">{turn.get('prompt', '')}</div>
            <div class="response">
                <div class="{status_class}">{status_text}</div>
                <div>{parsed}</div>
            </div>
            <div class="meta">Elapsed: {elapsed}s</div>
        </div>
    """


def generate_report(results_path: str) -> str:
    """Generate HTML report from benchmark results."""
    with open(results_path) as f:
        data = json.load(f)

    turns = data.get("turns", [])
    summary = data.get("summary", {})

    # Calculate latency metrics
    latencies = [
        t.get("response", {}).get("elapsed_seconds", 0)
        for t in turns
        if not t.get("response", {}).get("error")
    ]

    if latencies:
        latencies.sort()
        n = len(latencies)
        latency_min = min(latencies)
        latency_max = max(latencies)
        latency_mean = sum(latencies) / n
        latency_p50 = latencies[n // 2]
        latency_p99 = latencies[int(n * 0.99)] if n > 1 else latencies[-1]
    else:
        latency_min = latency_max = latency_mean = latency_p50 = latency_p99 = 0

    total_time = summary.get("total_time_seconds", sum(latencies))
    error_count = summary.get("errors", 0)
    error_class = "error" if error_count > 0 else ""

    turns_html = "\n".join(render_turn(t) for t in turns)

    return HTML_TEMPLATE.format(
        scenario_name=data.get("scenario", "Unknown"),
        total_turns=len(turns),
        total_time=total_time,
        avg_latency=latency_mean,
        error_count=error_count,
        error_class=error_class,
        latency_min=latency_min,
        latency_max=latency_max,
        latency_mean=latency_mean,
        latency_p50=latency_p50,
        latency_p99=latency_p99,
        turns_html=turns_html,
        timestamp=datetime.now().isoformat(),
    )


def main():
    parser = argparse.ArgumentParser(description="Generate HTML benchmark report")
    parser.add_argument(
        "results_file",
        help="Path to benchmark-results.json",
    )
    parser.add_argument(
        "-o",
        "--output",
        help="Output file (default: stdout)",
        default=None,
    )
    args = parser.parse_args()

    if not os.path.exists(args.results_file):
        print(f"ERROR: Results file not found: {args.results_file}", file=sys.stderr)
        sys.exit(1)

    html = generate_report(args.results_file)

    if args.output:
        with open(args.output, "w") as f:
            f.write(html)
        print(f"Report written to {args.output}")
    else:
        print(html)


if __name__ == "__main__":
    main()
