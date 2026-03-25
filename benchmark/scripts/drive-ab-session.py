#!/usr/bin/env python3
"""
drive-ab-session.py — A/B Memory Recall Benchmark Driver.

Extends the benchmark framework to support A/B testing with expected_facts
validation for measuring memory recall quality.

This script:
1. Reads YAML scenario files with store and query prompts
2. Executes store prompts first (sequentially, with async processing wait)
3. Executes query prompts with expected_facts validation
4. Outputs benchmark-results.json with recall metrics and transcript.md

Usage:
    python3 drive-ab-session.py --api-url http://127.0.0.1:18081 \
        --api-token mnemo_xxx \
        --prompt-file benchmark/prompts/simple-recall.yaml \
        --results-dir benchmark/results/xxx
"""

import argparse
import json
import os
import sys
import time
import urllib.parse
import urllib.request
import urllib.error
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

try:
    import yaml
except ImportError:
    print("ERROR: pyyaml is required. Install with: pip3 install pyyaml", file=sys.stderr)
    sys.exit(1)


def make_request(
    url: str,
    token: str,
    method: str = "GET",
    data: Optional[Dict] = None,
    timeout: int = 30,
) -> Dict[str, Any]:
    """Make an HTTP request to the memorix API."""
    headers = {
        "Authorization": f"Bearer {token}",
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
        with urllib.request.urlopen(req, timeout=timeout) as response:
            response_body = response.read().decode("utf-8")
            elapsed = time.monotonic() - start
            return {
                "status": response.status,
                "body": json.loads(response_body),
                "elapsed_ms": int(elapsed * 1000),
                "elapsed_seconds": round(elapsed, 3),
                "error": None,
            }
    except urllib.error.HTTPError as e:
        elapsed = time.monotonic() - start
        response_body = e.read().decode("utf-8") if e.fp else ""
        return {
            "status": e.code,
            "body": {"error": response_body} if response_body else {},
            "elapsed_ms": int(elapsed * 1000),
            "elapsed_seconds": round(elapsed, 3),
            "error": f"HTTP {e.code}: {response_body[:200]}",
        }
    except urllib.error.URLError as e:
        elapsed = time.monotonic() - start
        return {
            "status": 0,
            "body": {},
            "elapsed_ms": int(elapsed * 1000),
            "elapsed_seconds": round(elapsed, 3),
            "error": str(e.reason),
        }
    except Exception as e:
        elapsed = time.monotonic() - start
        return {
            "status": 0,
            "body": {},
            "elapsed_ms": int(elapsed * 1000),
            "elapsed_seconds": round(elapsed, 3),
            "error": str(e),
        }


def ingest_memory(
    base_url: str,
    token: str,
    content: str,
    tags: List[str],
    timeout: int,
) -> Dict[str, Any]:
    """Ingest content into memorix and return the result."""
    url = f"{base_url}/api/memories"
    data = {
        "content": content,
        "tags": tags,
        "metadata": {
            "source": "benchmark",
            "timestamp": datetime.now(timezone.utc).isoformat(),
        },
    }
    return make_request(url, token, "POST", data, timeout)


def search_memory(
    base_url: str,
    token: str,
    query: str,
    k: int,
    timeout: int,
) -> Dict[str, Any]:
    """Search memories using GET /api/memories?q=..."""
    encoded_query = urllib.parse.quote(query)
    url = f"{base_url}/api/memories?q={encoded_query}&limit={k}"
    return make_request(url, token, "GET", None, timeout)


def check_expected_facts(
    response: Dict[str, Any],
    expected_facts: List[str],
) -> Dict[str, Any]:
    """Check if expected facts are found in the search results."""
    body = response.get("body", {})
    memories = body.get("memories", [])

    # Combine all memory content for fact checking
    combined_content = " ".join(m.get("content", "") for m in memories).lower()

    facts_matched = []
    facts_missed = []

    for fact in expected_facts:
        if fact.lower() in combined_content:
            facts_matched.append(fact)
        else:
            facts_missed.append(fact)

    return {
        "expected_facts": expected_facts,
        "facts_matched": facts_matched,
        "facts_missed": facts_missed,
        "recall_success": len(facts_missed) == 0,
        "memories_found": len(memories),
    }


def parse_prompt_item(p) -> Dict[str, Any]:
    """Parse a prompt item from YAML into a structured dict."""
    if isinstance(p, str):
        # Simple string prompt - treat as query
        return {
            "id": None,
            "text": p,
            "type": "query",
            "tags": [],
            "expected_facts": [],
            "semantic_match": False,
        }

    # Dict prompt with metadata
    return {
        "id": p.get("id"),
        "text": p.get("text", p.get("prompt", "")),
        "type": p.get("type", "query"),
        "tags": p.get("tags", []),
        "expected_facts": p.get("expected_facts", []),
        "semantic_match": p.get("semantic_match", False),
    }


def parse_response(raw: Dict[str, Any]) -> str:
    """Extract a human-readable response from raw output."""
    if raw.get("error"):
        return f"ERROR: {raw['error']}"

    body = raw.get("body", {})
    if not body:
        return "(no response)"

    # Search results from GET /api/memories?q=...
    if "memories" in body:
        memories = body["memories"]
        total = body.get("total", len(memories))
        if isinstance(memories, list):
            parts = [f"Found {total} memories:\n"]
            for i, m in enumerate(memories[:3], 1):
                content = m.get("content", "(no content)")
                score = m.get("score", "-")
                parts.append(f"[{i}] (score: {score})\n{content[:200]}...")
            return "\n\n".join(parts)

    # Store response (async accepted)
    if raw.get("status") in (200, 201, 202):
        return f"Accepted: {body.get('status', 'stored')}"

    return json.dumps(body, indent=2, ensure_ascii=False)


def write_transcript(
    scenario: Dict,
    turns: List[Dict],
    results_dir: str,
    api_url: str,
):
    """Write a human-readable markdown transcript."""
    lines = [
        f"# Benchmark Transcript: {scenario.get('name', 'Unknown')}",
        "",
        f"**Description:** {scenario.get('description', 'N/A')}",
        f"**Date:** {datetime.now(timezone.utc).isoformat()}",
        f"**Server:** {api_url}",
        "",
        "---",
        "",
    ]

    # Calculate recall summary
    query_turns = [t for t in turns if t.get("turn_type") == "query"]
    recall_turns = [t for t in query_turns if t.get("expected_facts")]
    recall_success_count = sum(1 for t in recall_turns if t.get("recall_success"))

    if recall_turns:
        lines.append("## Recall Summary")
        lines.append("")
        lines.append("| Metric | Value |")
        lines.append("|--------|-------|")
        lines.append(f"| Query turns with expected_facts | {len(recall_turns)} |")
        lines.append(f"| Successful recalls | {recall_success_count} |")
        lines.append(f"| Recall rate | {recall_success_count / len(recall_turns) * 100:.1f}% |")
        lines.append("")
        lines.append("---")
        lines.append("")

    for i, turn in enumerate(turns, 1):
        turn_id = turn.get("id", i)
        lines.append(f"## Turn {turn_id}")
        lines.append("")
        lines.append(f"**Type:** {turn.get('turn_type', 'query')}")
        lines.append("")
        lines.append("### Prompt")
        lines.append("")
        prompt_text = turn.get("prompt", "").strip()
        lines.append(f"```")
        lines.append(prompt_text)
        lines.append("```")
        lines.append("")

        resp = turn.get("response", {})
        lines.append("### Response")
        lines.append("")
        lines.append(
            f"*Status: {resp.get('status', 'N/A')} | "
            f"Elapsed: {resp.get('elapsed_ms', 0)}ms*"
        )
        lines.append("")
        lines.append(turn.get("parsed_response", "(no response)"))
        lines.append("")

        # Expected facts check
        expected_facts = turn.get("expected_facts")
        if expected_facts:
            facts_matched = turn.get("facts_matched", [])
            facts_missed = turn.get("facts_missed", [])
            recall_success = turn.get("recall_success", False)

            lines.append("### Expected Facts Check")
            lines.append("")
            status = "PASS" if recall_success else "FAIL"
            lines.append(f"**Status:** {status}")
            lines.append("")
            if facts_matched:
                lines.append(f"- **Matched:** {', '.join(facts_matched)}")
            if facts_missed:
                lines.append(f"- **Missed:** {', '.join(facts_missed)}")
            lines.append("")

        lines.append("---")
        lines.append("")

    path = os.path.join(results_dir, "transcript.md")
    with open(path, "w") as f:
        f.write("\n".join(lines))
    print(f"  Transcript written to {path}")


def write_results_json(
    scenario: Dict,
    turns: List[Dict],
    results_dir: str,
    api_url: str,
):
    """Write structured JSON results."""
    # Calculate summary metrics
    store_turns = [t for t in turns if t.get("turn_type") == "store"]
    query_turns = [t for t in turns if t.get("turn_type") == "query"]
    recall_turns = [t for t in query_turns if t.get("expected_facts")]

    total_time_ms = sum(t.get("response", {}).get("elapsed_ms", 0) for t in turns)
    errors = sum(1 for t in turns if t.get("response", {}).get("error"))
    recall_success_count = sum(1 for t in recall_turns if t.get("recall_success"))

    output = {
        "scenario": scenario.get("name", "unknown"),
        "description": scenario.get("description", ""),
        "api_url": api_url,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "turns": turns,
        "summary": {
            "total_turns": len(turns),
            "store_turns": len(store_turns),
            "query_turns": len(query_turns),
            "total_time_ms": total_time_ms,
            "errors": errors,
            "recall_turns": len(recall_turns),
            "recall_success": recall_success_count,
            "recall_rate": (
                recall_success_count / len(recall_turns) * 100
                if recall_turns else 0
            ),
        },
    }

    path = os.path.join(results_dir, "benchmark-results.json")
    with open(path, "w") as f:
        json.dump(output, f, indent=2, ensure_ascii=False)
    print(f"  Results JSON written to {path}")


def write_metrics(turns: List[Dict], results_dir: str):
    """Write performance metrics."""
    latencies = [
        t.get("response", {}).get("elapsed_seconds", 0)
        for t in turns
        if not t.get("response", {}).get("error")
    ]

    if not latencies:
        return

    latencies.sort()
    n = len(latencies)

    metrics = {
        "latency": {
            "min_ms": int(min(latencies) * 1000),
            "max_ms": int(max(latencies) * 1000),
            "mean_ms": int(sum(latencies) / n * 1000),
            "p50_ms": int(latencies[n // 2] * 1000),
            "p99_ms": int(latencies[int(n * 0.99)] * 1000 if n > 1 else latencies[-1] * 1000),
        },
        "total_requests": len(turns),
        "successful_requests": n,
        "failed_requests": len(turns) - n,
    }

    path = os.path.join(results_dir, "metrics.json")
    with open(path, "w") as f:
        json.dump(metrics, f, indent=2)
    print(f"  Metrics written to {path}")


def main():
    parser = argparse.ArgumentParser(
        description="A/B Memory Recall Benchmark Driver"
    )
    parser.add_argument("--api-url", required=True, help="memorix API base URL")
    parser.add_argument("--api-token", required=True, help="API token")
    parser.add_argument("--prompt-file", required=True, help="YAML prompt file")
    parser.add_argument("--results-dir", required=True, help="Output directory")
    parser.add_argument(
        "--timeout",
        type=int,
        default=600,
        help="Per-prompt timeout (seconds)",
    )
    parser.add_argument(
        "--wait",
        type=int,
        default=3,
        help="Seconds to wait after store operations for async processing",
    )
    args = parser.parse_args()

    # Load scenario
    with open(args.prompt_file) as f:
        scenario = yaml.safe_load(f)

    prompts = scenario.get("prompts", [])
    if not prompts:
        print("ERROR: No prompts found in scenario file.", file=sys.stderr)
        sys.exit(1)

    # Parse prompts
    prompt_items = [parse_prompt_item(p) for p in prompts]

    print(f"  Scenario: {scenario.get('name', 'unknown')}")
    print(f"  Prompts: {len(prompt_items)}")
    print(f"  Timeout: {args.timeout}s")
    print()

    os.makedirs(args.results_dir, exist_ok=True)

    turns = []

    # Separate store and query prompts
    store_items = [p for p in prompt_items if p["type"] == "store"]
    query_items = [p for p in prompt_items if p["type"] == "query"]

    # Execute store prompts
    print(f"  --- Executing {len(store_items)} store operations ---")
    for i, item in enumerate(store_items, 1):
        prompt_text = item["text"]
        tags = item.get("tags", [])
        turn_id = item.get("id", f"store-{i}")

        print(f"  [{turn_id}] Storing: {prompt_text[:60]}...")

        response = ingest_memory(
            args.api_url,
            args.api_token,
            prompt_text,
            tags,
            args.timeout,
        )

        parsed = parse_response(response)

        turn = {
            "id": turn_id,
            "turn": i,
            "prompt": prompt_text,
            "turn_type": "store",
            "response": response,
            "parsed_response": parsed,
            "tags": tags,
        }
        turns.append(turn)

        status = response.get("status", "N/A")
        elapsed = response.get("elapsed_ms", 0)
        error = response.get("error")

        if error:
            print(f"    ERROR: {error}")
        else:
            print(f"    Status: {status} | Elapsed: {elapsed}ms")

    # Wait for async processing
    if store_items:
        print(f"\n  Waiting {args.wait}s for async processing...")
        time.sleep(args.wait)

    # Execute query prompts
    print(f"\n  --- Executing {len(query_items)} query operations ---")
    for i, item in enumerate(query_items, 1):
        prompt_text = item["text"]
        expected_facts = item.get("expected_facts", [])
        turn_id = item.get("id", f"query-{i}")

        print(f"  [{turn_id}] Querying: {prompt_text[:60]}...")

        response = search_memory(
            args.api_url,
            args.api_token,
            prompt_text,
            k=10,
            timeout=args.timeout,
        )

        parsed = parse_response(response)

        turn = {
            "id": turn_id,
            "turn": len(store_items) + i,
            "prompt": prompt_text,
            "turn_type": "query",
            "response": response,
            "parsed_response": parsed,
            "expected_facts": expected_facts,
            "semantic_match": item.get("semantic_match", False),
        }

        # Check expected facts
        if expected_facts:
            fact_check = check_expected_facts(response, expected_facts)
            turn.update(fact_check)

            matched = fact_check["facts_matched"]
            missed = fact_check["facts_missed"]
            success = fact_check["recall_success"]

            status_icon = "PASS" if success else "FAIL"
            print(f"    Status: {response.get('status')} | "
                  f"Elapsed: {response.get('elapsed_ms')}ms | "
                  f"Facts: {status_icon} ({len(matched)}/{len(expected_facts)})")
            if missed:
                print(f"    Missed: {', '.join(missed)}")
        else:
            print(f"    Status: {response.get('status')} | "
                  f"Elapsed: {response.get('elapsed_ms')}ms")

        turns.append(turn)

    # Write outputs
    print()
    write_results_json(scenario, turns, args.results_dir, args.api_url)
    write_transcript(scenario, turns, args.results_dir, args.api_url)
    write_metrics(turns, args.results_dir)

    # Print summary
    query_turns = [t for t in turns if t.get("turn_type") == "query"]
    recall_turns = [t for t in query_turns if t.get("expected_facts")]
    recall_success = sum(1 for t in recall_turns if t.get("recall_success"))
    errors = sum(1 for t in turns if t.get("response", {}).get("error"))

    print()
    print("=" * 60)
    print("  BENCHMARK SUMMARY")
    print("=" * 60)
    print(f"  Scenario: {scenario.get('name', 'unknown')}")
    print(f"  Total turns: {len(turns)}")
    print(f"  Errors: {errors}")
    if recall_turns:
        print(f"  Recall: {recall_success}/{len(recall_turns)} "
              f"({recall_success / len(recall_turns) * 100:.1f}%)")
    print("=" * 60)

    if errors > 0:
        sys.exit(1)


if __name__ == "__main__":
    main()
