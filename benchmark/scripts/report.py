#!/usr/bin/env python3
"""
report.py — HTML report generator for memorix benchmarks.

Reads benchmark-results.json and produces an interactive HTML comparison report
with dark theme, responsive layout, and memorix-specific metrics visualization.
"""

import argparse
import json
import os
import sys
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple


# Color scheme for turn types
TURN_TYPE_COLORS = {
    "store": "#3b82f6",   # Blue
    "query": "#a855f7",   # Purple
    "story": "#22c55e",   # Green
    "default": "#6b7280", # Gray
}


def get_turn_color(turn_type: str) -> str:
    """Get the color for a turn type."""
    return TURN_TYPE_COLORS.get(turn_type, TURN_TYPE_COLORS["default"])


def calculate_statistics(turns: List[Dict]) -> Dict[str, Any]:
    """Calculate summary statistics from turns."""
    store_turns = [t for t in turns if t.get("turn_type") == "store"]
    query_turns = [t for t in turns if t.get("turn_type") == "query"]
    story_turns = [t for t in turns if t.get("turn_type") == "story"]

    # Calculate latency metrics
    latencies = [
        t.get("response", {}).get("elapsed_seconds", 0)
        for t in turns
        if not t.get("response", {}).get("error")
    ]

    if latencies:
        latencies.sort()
        n = len(latencies)
        latency_stats = {
            "min": min(latencies),
            "max": max(latencies),
            "mean": sum(latencies) / n,
            "p50": latencies[n // 2],
            "p99": latencies[int(n * 0.99)] if n > 1 else latencies[-1],
        }
    else:
        latency_stats = {"min": 0, "max": 0, "mean": 0, "p50": 0, "p99": 0}

    # Calculate total time
    total_time = sum(
        t.get("response", {}).get("elapsed_seconds", 0) for t in turns
    )

    # Count errors
    error_count = sum(1 for t in turns if t.get("response", {}).get("error"))

    # Calculate recall metrics for query turns
    recall_turns = [t for t in query_turns if t.get("expected_facts")]
    recall_success = sum(1 for t in recall_turns if t.get("recall_success"))
    recall_rate = (recall_success / len(recall_turns) * 100) if recall_turns else 0

    return {
        "total_turns": len(turns),
        "store_turns": len(store_turns),
        "query_turns": len(query_turns),
        "story_turns": len(story_turns),
        "total_time": total_time,
        "error_count": error_count,
        "latency": latency_stats,
        "recall_turns": len(recall_turns),
        "recall_success": recall_success,
        "recall_rate": recall_rate,
    }


def generate_bar_chart_svg(
    data: List[Tuple[str, float, str]],
    width: int = 400,
    height: int = 200,
    title: str = "",
) -> str:
    """Generate an inline SVG bar chart.

    Args:
        data: List of (label, value, color) tuples
        width: SVG width
        height: SVG height
        title: Chart title

    Returns:
        SVG string
    """
    if not data:
        return ""

    max_value = max(d[1] for d in data) if data else 1
    if max_value == 0:
        max_value = 1

    bar_width = 40
    bar_gap = 20
    label_height = 30
    chart_height = height - label_height - 30

    svg_parts = [
        f'<svg viewBox="0 0 {width} {height}" class="chart">',
        f'<text x="{width // 2}" y="20" text-anchor="middle" fill="#888" font-size="12">{title}</text>',
    ]

    x_offset = (width - (len(data) * (bar_width + bar_gap) - bar_gap)) // 2

    for i, (label, value, color) in enumerate(data):
        x = x_offset + i * (bar_width + bar_gap)
        bar_height = (value / max_value) * chart_height if max_value > 0 else 0
        y = 30 + chart_height - bar_height

        # Bar
        svg_parts.append(
            f'<rect x="{x}" y="{y}" width="{bar_width}" height="{bar_height}" '
            f'fill="{color}" rx="4" opacity="0.8"/>'
        )

        # Value label
        svg_parts.append(
            f'<text x="{x + bar_width // 2}" y="{y - 5}" text-anchor="middle" '
            f'fill="#ccc" font-size="11">{value:.0f}ms</text>'
        )

        # X-axis label
        svg_parts.append(
            f'<text x="{x + bar_width // 2}" y="{height - 5}" text-anchor="middle" '
            f'fill="#888" font-size="10">{label}</text>'
        )

    svg_parts.append('</svg>')
    return "\n".join(svg_parts)


def generate_score_distribution_svg(
    scores: List[float],
    width: int = 400,
    height: int = 150,
) -> str:
    """Generate a histogram SVG for score distribution.

    Args:
        scores: List of score values (0-1 range)
        width: SVG width
        height: SVG height

    Returns:
        SVG string
    """
    if not scores:
        return '<p class="no-data">No score data available</p>'

    # Create histogram buckets
    num_buckets = 10
    buckets = [0] * num_buckets

    for score in scores:
        bucket_idx = min(int(score * num_buckets), num_buckets - 1)
        buckets[bucket_idx] += 1

    max_count = max(buckets) if buckets else 1

    bar_width = (width - 40) // num_buckets
    chart_height = height - 40

    svg_parts = [
        f'<svg viewBox="0 0 {width} {height}" class="chart">',
        '<text x="20" y="15" fill="#888" font-size="11">Score Distribution</text>',
    ]

    for i, count in enumerate(buckets):
        x = 20 + i * bar_width
        bar_height = (count / max_count) * chart_height if max_count > 0 else 0
        y = 25 + chart_height - bar_height

        # Color gradient from red to green based on score range
        hue = int(120 * (i / num_buckets))  # 0=red, 120=green
        color = f"hsl({hue}, 70%, 50%)"

        svg_parts.append(
            f'<rect x="{x}" y="{y}" width="{bar_width - 2}" height="{bar_height}" '
            f'fill="{color}" rx="2" opacity="0.8"/>'
        )

        # X-axis label
        if i % 2 == 0:
            svg_parts.append(
                f'<text x="{x + bar_width // 2}" y="{height - 5}" text-anchor="middle" '
                f'fill="#666" font-size="9">{i / num_buckets:.1f}</text>'
            )

    svg_parts.append('</svg>')
    return "\n".join(svg_parts)


def render_turn(turn: Dict[str, Any], index: int) -> str:
    """Render a single turn as HTML with details/summary for expand/collapse."""
    response = turn.get("response", {})
    error = response.get("error")
    elapsed_ms = response.get("elapsed_ms", int(response.get("elapsed_seconds", 0) * 1000))
    status = response.get("status", "N/A")
    turn_type = turn.get("turn_type", "query")
    parsed = turn.get("parsed_response", "(no response)")
    turn_id = turn.get("id", str(turn.get("turn", index)))
    prompt_text = turn.get("prompt", "")

    turn_color = get_turn_color(turn_type)

    # Determine if should be open by default (story collapsed, others open)
    is_story = turn_type == "story"
    open_attr = "" if is_story else " open"

    status_class = "error" if error else "success"
    status_text = f"ERROR: {error}" if error else f"HTTP {status}"

    # Build expected facts section for query turns
    facts_html = ""
    if turn.get("expected_facts"):
        matched = turn.get("facts_matched", [])
        missed = turn.get("facts_missed", [])
        recall_success = turn.get("recall_success", False)

        status_icon = "✓" if recall_success else "✗"
        status_badge = "pass" if recall_success else "fail"

        facts_html = f"""
            <div class="facts-check {status_badge}">
                <span class="status-icon">{status_icon}</span>
                <span class="facts-status">Recall: {len(matched)}/{len(turn['expected_facts'])}</span>
                {f'<span class="facts-matched">Matched: {", ".join(matched)}</span>' if matched else ''}
                {f'<span class="facts-missed">Missed: {", ".join(missed)}</span>' if missed else ''}
            </div>
        """

    # Build semantic match indicator
    semantic_badge = ""
    if turn.get("semantic_match"):
        semantic_badge = '<span class="semantic-badge" title="Semantic match test">🧠</span>'

    # Build score visualization for search results
    scores_html = ""
    body = response.get("body", {})
    if body.get("memories"):
        memories = body["memories"]
        scores = [m.get("score", 0) for m in memories if m.get("score") is not None]
        if scores:
            scores_html = f"""
                <div class="scores-section">
                    <span class="scores-label">Result scores:</span>
                    {"".join(f'<span class="score-badge" style="--score-color: hsl({int(s * 120)}, 70%, 50%)">{s:.3f}</span>' for s in scores[:5])}
                </div>
            """

    return f"""
        <details class="turn turn-{turn_type}"{open_attr}>
            <summary class="turn-summary">
                <span class="turn-header">
                    <span class="turn-id">Turn {turn_id}</span>
                    <span class="turn-type-badge" style="--turn-color: {turn_color}">{turn_type}</span>
                    {semantic_badge}
                </span>
                <span class="turn-meta">
                    <span class="{status_class}">{status_text}</span>
                    <span class="elapsed">{elapsed_ms}ms</span>
                </span>
            </summary>
            <div class="turn-content">
                <div class="prompt-section">
                    <label>Prompt</label>
                    <pre class="prompt-text">{prompt_text}</pre>
                </div>
                <div class="response-section">
                    <label>Response</label>
                    <div class="response-text">{parsed}</div>
                </div>
                {scores_html}
                {facts_html}
            </div>
        </details>
    """


def render_comparison_section(
    stats_a: Dict,
    stats_b: Dict,
    label_a: str = "Profile A",
    label_b: str = "Profile B",
) -> str:
    """Render a side-by-side comparison section."""
    return f"""
        <div class="comparison-grid">
            <div class="profile profile-a">
                <h3 class="profile-title">{label_a}</h3>
                <div class="profile-stats">
                    <div class="stat-row">
                        <span>Total Turns</span>
                        <span>{stats_a.get('total_turns', 0)}</span>
                    </div>
                    <div class="stat-row">
                        <span>Recall Rate</span>
                        <span>{stats_a.get('recall_rate', 0):.1f}%</span>
                    </div>
                    <div class="stat-row">
                        <span>Avg Latency</span>
                        <span>{stats_a.get('latency', {}).get('mean', 0) * 1000:.0f}ms</span>
                    </div>
                    <div class="stat-row">
                        <span>P99 Latency</span>
                        <span>{stats_a.get('latency', {}).get('p99', 0) * 1000:.0f}ms</span>
                    </div>
                </div>
            </div>
            <div class="profile profile-b">
                <h3 class="profile-title">{label_b}</h3>
                <div class="profile-stats">
                    <div class="stat-row">
                        <span>Total Turns</span>
                        <span>{stats_b.get('total_turns', 0)}</span>
                    </div>
                    <div class="stat-row">
                        <span>Recall Rate</span>
                        <span>{stats_b.get('recall_rate', 0):.1f}%</span>
                    </div>
                    <div class="stat-row">
                        <span>Avg Latency</span>
                        <span>{stats_b.get('latency', {}).get('mean', 0) * 1000:.0f}ms</span>
                    </div>
                    <div class="stat-row">
                        <span>P99 Latency</span>
                        <span>{stats_b.get('latency', {}).get('p99', 0) * 1000:.0f}ms</span>
                    </div>
                </div>
            </div>
        </div>
    """


def generate_html_report(
    data: Dict[str, Any],
    env_info: Optional[Dict[str, Any]] = None,
) -> str:
    """Generate complete HTML report from benchmark results.

    Args:
        data: Benchmark results data
        env_info: Optional environment info (server version, config, etc.)

    Returns:
        Complete HTML string
    """
    turns = data.get("turns", [])
    summary = data.get("summary", {})
    stats = calculate_statistics(turns)

    # Extract scores from query turns for visualization
    all_scores = []
    query_latencies = []
    for turn in turns:
        if turn.get("turn_type") == "query":
            response = turn.get("response", {})
            body = response.get("body", {})
            memories = body.get("memories", [])
            for m in memories:
                if m.get("score") is not None:
                    all_scores.append(m["score"])
            if response.get("elapsed_seconds"):
                query_latencies.append(response["elapsed_seconds"] * 1000)

    # Generate latency comparison chart data
    store_latencies = [
        t.get("response", {}).get("elapsed_seconds", 0) * 1000
        for t in turns
        if t.get("turn_type") == "store" and not t.get("response", {}).get("error")
    ]

    latency_chart_svg = generate_bar_chart_svg(
        [
            ("Store", sum(store_latencies) / len(store_latencies) if store_latencies else 0, "#3b82f6"),
            ("Query", sum(query_latencies) / len(query_latencies) if query_latencies else 0, "#a855f7"),
        ],
        title="Average Latency by Type (ms)",
    )

    # Generate score distribution chart
    score_dist_svg = generate_score_distribution_svg(all_scores)

    # Render turns HTML
    turns_html = "\n".join(render_turn(t, i) for i, t in enumerate(turns, 1))

    # Environment info for footer
    timestamp = data.get("timestamp", datetime.now().isoformat())
    server_version = env_info.get("server_version", "unknown") if env_info else "unknown"
    config_params = env_info.get("config", {}) if env_info else {}

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Memorix Benchmark Report - {data.get('scenario', 'Unknown')}</title>
    <style>
        :root {{
            --bg-color: #0a0a0a;
            --card-bg: #141414;
            --card-bg-alt: #1a1a1a;
            --text-color: #e5e5e5;
            --text-muted: #737373;
            --border-color: #262626;
            --accent-color: #3b82f6;
            --success-color: #22c55e;
            --error-color: #ef4444;
            --warning-color: #f59e0b;
            --store-color: #3b82f6;
            --query-color: #a855f7;
            --story-color: #22c55e;
        }}

        * {{
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }}

        body {{
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Inter', sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            line-height: 1.6;
            min-height: 100vh;
        }}

        .container {{
            max-width: 1200px;
            margin: 0 auto;
            padding: 24px;
        }}

        /* Header */
        header {{
            margin-bottom: 32px;
            padding-bottom: 24px;
            border-bottom: 1px solid var(--border-color);
        }}

        h1 {{
            font-size: 28px;
            font-weight: 600;
            margin-bottom: 8px;
        }}

        .scenario-info {{
            color: var(--text-muted);
            font-size: 14px;
        }}

        /* Summary Cards */
        .summary-cards {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
            gap: 16px;
            margin-bottom: 32px;
        }}

        .card {{
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 20px;
        }}

        .card h3 {{
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: var(--text-muted);
            margin-bottom: 8px;
        }}

        .card .value {{
            font-size: 28px;
            font-weight: 600;
        }}

        .card.success .value {{ color: var(--success-color); }}
        .card.error .value {{ color: var(--error-color); }}
        .card.warning .value {{ color: var(--warning-color); }}

        .card .subtext {{
            font-size: 12px;
            color: var(--text-muted);
            margin-top: 4px;
        }}

        /* Turn Type Counts */
        .turn-counts {{
            display: flex;
            gap: 24px;
            margin-top: 16px;
            flex-wrap: wrap;
        }}

        .turn-count {{
            display: flex;
            align-items: center;
            gap: 8px;
        }}

        .turn-count-dot {{
            width: 12px;
            height: 12px;
            border-radius: 50%;
        }}

        .turn-count-dot.store {{ background: var(--store-color); }}
        .turn-count-dot.query {{ background: var(--query-color); }}
        .turn-count-dot.story {{ background: var(--story-color); }}

        /* Metrics Section */
        .metrics-section {{
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 24px;
            margin-bottom: 32px;
        }}

        .metrics-section h2 {{
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 20px;
            color: var(--text-color);
        }}

        .metrics-grid {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 24px;
        }}

        .metric-item {{
            background: var(--card-bg-alt);
            border-radius: 8px;
            padding: 16px;
        }}

        .metric-item h4 {{
            font-size: 13px;
            color: var(--text-muted);
            margin-bottom: 12px;
        }}

        /* Charts */
        .chart {{
            display: block;
            max-width: 100%;
            height: auto;
        }}

        .no-data {{
            color: var(--text-muted);
            font-style: italic;
        }}

        /* Latency Table */
        .latency-table {{
            width: 100%;
            border-collapse: collapse;
        }}

        .latency-table th,
        .latency-table td {{
            padding: 8px 12px;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
        }}

        .latency-table th {{
            color: var(--text-muted);
            font-weight: 500;
            font-size: 12px;
        }}

        .latency-table td {{
            font-variant-numeric: tabular-nums;
        }}

        /* Turn Details */
        .turns-section {{
            margin-bottom: 32px;
        }}

        .turns-section h2 {{
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 16px;
        }}

        .turn {{
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            margin-bottom: 12px;
            overflow: hidden;
        }}

        .turn[open] {{
            border-color: var(--border-color);
        }}

        .turn-summary {{
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px;
            cursor: pointer;
            list-style: none;
        }}

        .turn-summary::-webkit-details-marker {{
            display: none;
        }}

        .turn-summary::before {{
            content: '▶';
            margin-right: 12px;
            color: var(--text-muted);
            transition: transform 0.2s;
        }}

        .turn[open] .turn-summary::before {{
            transform: rotate(90deg);
        }}

        .turn-header {{
            display: flex;
            align-items: center;
            gap: 12px;
        }}

        .turn-id {{
            font-weight: 500;
        }}

        .turn-type-badge {{
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
            text-transform: uppercase;
            background: color-mix(in srgb, var(--turn-color, var(--accent-color)) 20%, transparent);
            color: var(--turn-color, var(--accent-color));
        }}

        .semantic-badge {{
            font-size: 12px;
        }}

        .turn-meta {{
            display: flex;
            align-items: center;
            gap: 16px;
            font-size: 13px;
        }}

        .turn-meta .success {{ color: var(--success-color); }}
        .turn-meta .error {{ color: var(--error-color); }}
        .turn-meta .elapsed {{
            color: var(--text-muted);
            font-variant-numeric: tabular-nums;
        }}

        .turn-content {{
            padding: 0 16px 16px 16px;
            border-top: 1px solid var(--border-color);
        }}

        .prompt-section,
        .response-section {{
            margin-top: 16px;
        }}

        .prompt-section label,
        .response-section label {{
            display: block;
            font-size: 11px;
            text-transform: uppercase;
            color: var(--text-muted);
            margin-bottom: 8px;
        }}

        .prompt-text {{
            background: var(--card-bg-alt);
            padding: 12px;
            border-radius: 6px;
            font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
            font-size: 13px;
            white-space: pre-wrap;
            word-break: break-word;
            overflow-x: auto;
        }}

        .response-text {{
            background: var(--card-bg-alt);
            padding: 12px;
            border-radius: 6px;
            font-size: 13px;
            white-space: pre-wrap;
            word-break: break-word;
        }}

        /* Scores Section */
        .scores-section {{
            margin-top: 12px;
            display: flex;
            align-items: center;
            gap: 8px;
            flex-wrap: wrap;
        }}

        .scores-label {{
            font-size: 11px;
            color: var(--text-muted);
        }}

        .score-badge {{
            display: inline-block;
            padding: 2px 6px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
            background: color-mix(in srgb, var(--score-color, var(--accent-color)) 20%, transparent);
            color: var(--score-color, var(--accent-color));
        }}

        /* Facts Check */
        .facts-check {{
            margin-top: 12px;
            padding: 12px;
            border-radius: 6px;
            display: flex;
            flex-wrap: wrap;
            align-items: center;
            gap: 12px;
        }}

        .facts-check.pass {{
            background: color-mix(in srgb, var(--success-color) 10%, transparent);
            border: 1px solid color-mix(in srgb, var(--success-color) 30%, transparent);
        }}

        .facts-check.fail {{
            background: color-mix(in srgb, var(--error-color) 10%, transparent);
            border: 1px solid color-mix(in srgb, var(--error-color) 30%, transparent);
        }}

        .facts-check .status-icon {{
            font-size: 16px;
        }}

        .facts-check.pass .status-icon {{ color: var(--success-color); }}
        .facts-check.fail .status-icon {{ color: var(--error-color); }}

        .facts-status {{
            font-weight: 500;
        }}

        .facts-matched,
        .facts-missed {{
            font-size: 12px;
            color: var(--text-muted);
        }}

        .facts-missed {{
            color: var(--error-color);
        }}

        /* Footer */
        footer {{
            margin-top: 48px;
            padding-top: 24px;
            border-top: 1px solid var(--border-color);
            color: var(--text-muted);
            font-size: 12px;
        }}

        .footer-grid {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 24px;
        }}

        .footer-item h4 {{
            color: var(--text-color);
            font-weight: 500;
            margin-bottom: 8px;
        }}

        .footer-item p {{
            margin-bottom: 4px;
        }}

        /* Responsive */
        @media (max-width: 768px) {{
            .container {{
                padding: 16px;
            }}

            h1 {{
                font-size: 22px;
            }}

            .summary-cards {{
                grid-template-columns: repeat(2, 1fr);
            }}

            .turn-summary {{
                flex-direction: column;
                align-items: flex-start;
                gap: 8px;
            }}

            .turn-meta {{
                margin-left: 24px;
            }}

            .metrics-grid {{
                grid-template-columns: 1fr;
            }}
        }}

        @media (max-width: 480px) {{
            .summary-cards {{
                grid-template-columns: 1fr;
            }}

            .turn-counts {{
                flex-direction: column;
                gap: 8px;
            }}
        }}
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Memorix Benchmark Report</h1>
            <p class="scenario-info">
                <strong>{data.get('scenario', 'Unknown')}</strong>
                {f' - {data.get("description", "")}' if data.get('description') else ''}
            </p>
            <div class="turn-counts">
                <div class="turn-count">
                    <span class="turn-count-dot store"></span>
                    <span>Store: {stats['store_turns']}</span>
                </div>
                <div class="turn-count">
                    <span class="turn-count-dot query"></span>
                    <span>Query: {stats['query_turns']}</span>
                </div>
                <div class="turn-count">
                    <span class="turn-count-dot story"></span>
                    <span>Story: {stats['story_turns']}</span>
                </div>
            </div>
        </header>

        <!-- Summary Cards -->
        <div class="summary-cards">
            <div class="card">
                <h3>Total Turns</h3>
                <div class="value">{stats['total_turns']}</div>
            </div>
            <div class="card">
                <h3>Total Time</h3>
                <div class="value">{stats['total_time']:.2f}s</div>
            </div>
            <div class="card">
                <h3>Avg Latency</h3>
                <div class="value">{stats['latency']['mean'] * 1000:.0f}ms</div>
                <div class="subtext">P99: {stats['latency']['p99'] * 1000:.0f}ms</div>
            </div>
            <div class="card {('error' if stats['error_count'] > 0 else '')}">
                <h3>Errors</h3>
                <div class="value">{stats['error_count']}</div>
            </div>
            <div class="card success">
                <h3>Recall Rate</h3>
                <div class="value">{stats['recall_rate']:.1f}%</div>
                <div class="subtext">{stats['recall_success']}/{stats['recall_turns']} queries</div>
            </div>
        </div>

        <!-- Metrics Section -->
        <div class="metrics-section">
            <h2>Performance Metrics</h2>
            <div class="metrics-grid">
                <div class="metric-item">
                    <h4>Latency by Operation Type</h4>
                    {latency_chart_svg}
                </div>
                <div class="metric-item">
                    <h4>Latency Distribution</h4>
                    <table class="latency-table">
                        <tr><th>Metric</th><th>Value</th></tr>
                        <tr><td>Min</td><td>{stats['latency']['min'] * 1000:.0f}ms</td></tr>
                        <tr><td>Max</td><td>{stats['latency']['max'] * 1000:.0f}ms</td></tr>
                        <tr><td>Mean</td><td>{stats['latency']['mean'] * 1000:.0f}ms</td></tr>
                        <tr><td>P50</td><td>{stats['latency']['p50'] * 1000:.0f}ms</td></tr>
                        <tr><td>P99</td><td>{stats['latency']['p99'] * 1000:.0f}ms</td></tr>
                    </table>
                </div>
                <div class="metric-item">
                    <h4>Search Score Distribution</h4>
                    {score_dist_svg if all_scores else '<p class="no-data">No search scores available</p>'}
                </div>
            </div>
        </div>

        <!-- Turn Details -->
        <div class="turns-section">
            <h2>Turn Details</h2>
            {turns_html}
        </div>

        <!-- Footer -->
        <footer>
            <div class="footer-grid">
                <div class="footer-item">
                    <h4>Test Environment</h4>
                    <p>Server Version: {server_version}</p>
                    <p>Test Time: {timestamp}</p>
                    <p>API URL: {data.get('api_url', 'N/A')}</p>
                </div>
                <div class="footer-item">
                    <h4>Configuration</h4>
                    <p>Embedding Model: {config_params.get('embedding_model', 'N/A')}</p>
                    <p>Storage Mode: {config_params.get('storage_mode', 'N/A')}</p>
                    <p>Auto Embed: {config_params.get('auto_embed', 'N/A')}</p>
                </div>
            </div>
            <p style="margin-top: 16px;">Generated by Memorix Benchmark Framework</p>
        </footer>
    </div>
</body>
</html>
"""


def generate_report(results_path: str, env_info: Optional[Dict[str, Any]] = None) -> str:
    """Generate HTML report from benchmark results file.

    Args:
        results_path: Path to benchmark-results.json
        env_info: Optional environment info

    Returns:
        HTML string
    """
    with open(results_path) as f:
        data = json.load(f)

    return generate_html_report(data, env_info)


def main():
    parser = argparse.ArgumentParser(
        description="Generate HTML benchmark report"
    )
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
    parser.add_argument(
        "--server-version",
        help="memorix-server version",
        default="unknown",
    )
    parser.add_argument(
        "--embedding-model",
        help="Embedding model used",
        default="text-embedding-3-small",
    )
    args = parser.parse_args()

    if not os.path.exists(args.results_file):
        print(f"ERROR: Results file not found: {args.results_file}", file=sys.stderr)
        sys.exit(1)

    env_info = {
        "server_version": args.server_version,
        "config": {
            "embedding_model": args.embedding_model,
            "storage_mode": "sections",
            "auto_embed": "true",
        },
    }

    html = generate_report(args.results_file, env_info)

    if args.output:
        with open(args.output, "w") as f:
            f.write(html)
        print(f"Report written to {args.output}")
    else:
        print(html)


if __name__ == "__main__":
    main()
