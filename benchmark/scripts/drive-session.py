#!/usr/bin/env python3
"""
drive-session.py — Parallel prompt driver for memorix benchmarks.

Sends prompts to memorix server, captures responses, and produces
structured results + a human-readable transcript.
"""

import argparse
import json
import os
import sys
import time
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor, as_completed
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
                "elapsed_seconds": round(elapsed, 3),
                "error": None,
            }
    except urllib.error.HTTPError as e:
        elapsed = time.monotonic() - start
        response_body = e.read().decode("utf-8") if e.fp else ""
        return {
            "status": e.code,
            "body": {},
            "elapsed_seconds": round(elapsed, 3),
            "error": f"HTTP {e.code}: {response_body}",
        }
    except urllib.error.URLError as e:
        elapsed = time.monotonic() - start
        return {
            "status": 0,
            "body": {},
            "elapsed_seconds": round(elapsed, 3),
            "error": str(e.reason),
        }
    except Exception as e:
        elapsed = time.monotonic() - start
        return {
            "status": 0,
            "body": {},
            "elapsed_seconds": round(elapsed, 3),
            "error": str(e),
        }


def ingest_memory(
    base_url: str,
    token: str,
    content: str,
    timeout: int,
) -> Dict[str, Any]:
    """Ingest content into memorix and return the result."""
    url = f"{base_url}/api/memories"
    data = {
        "content": content,
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
    """Search memories and return results."""
    url = f"{base_url}/api/memories/search"
    data = {
        "query": query,
        "k": k,
    }
    return make_request(url, token, "POST", data, timeout)


def list_memories(
    base_url: str,
    token: str,
    timeout: int,
) -> Dict[str, Any]:
    """List all memories."""
    url = f"{base_url}/api/memories"
    return make_request(url, token, "GET", None, timeout)


def send_prompt(
    base_url: str,
    token: str,
    prompt: str,
    timeout: int,
    turn_type: str = "query",
) -> Dict[str, Any]:
    """
    Send a prompt to memorix and capture the response.

    For 'store' type prompts, we ingest the content.
    For 'query' type prompts, we search for relevant memories.
    """
    start = time.monotonic()

    if turn_type == "store":
        result = ingest_memory(base_url, token, prompt, timeout)
    else:
        # Default to search/query
        result = search_memory(base_url, token, prompt, k=5, timeout=timeout)

    result["prompt"] = prompt
    result["turn_type"] = turn_type
    return result


def parse_response(raw: Dict[str, Any]) -> str:
    """Extract a human-readable response from raw output."""
    if raw.get("error"):
        return f"ERROR: {raw['error']}"

    body = raw.get("body", {})
    if not body:
        return "(no response)"

    # Try to extract meaningful content
    if "results" in body:
        # Search results
        results = body["results"]
        if isinstance(results, list):
            parts = []
            for i, r in enumerate(results[:3], 1):  # Top 3 results
                content = r.get("content", "(no content)")
                score = r.get("score", 0)
                parts.append(f"[{i}] (score: {score:.3f})\n{content[:200]}...")
            return "\n\n".join(parts)

    if "id" in body:
        # Ingest response
        memory_id = body.get("id", "unknown")
        return f"Stored memory: {memory_id}"

    if "memories" in body:
        # List response
        memories = body["memories"]
        return f"Found {len(memories)} memories"

    return json.dumps(body, indent=2, ensure_ascii=False)


def write_transcript(scenario: Dict, turns: List[Dict], results_dir: str):
    """Write a human-readable markdown transcript."""
    lines = [
        f"# Benchmark Transcript: {scenario['name']}",
        "",
        f"**Description:** {scenario.get('description', 'N/A')}",
        f"**Date:** {datetime.now(timezone.utc).isoformat()}",
        f"**Server:** {scenario.get('api_url', 'N/A')}",
        "",
        "---",
        "",
    ]

    for i, turn in enumerate(turns, 1):
        lines.append(f"## Turn {i}")
        lines.append("")
        lines.append(f"**Type:** {turn.get('turn_type', 'query')}")
        lines.append("")
        lines.append("### Prompt")
        lines.append("")
        lines.append(f"```\n{turn['prompt'].strip()}\n```")
        lines.append("")
        lines.append("### Response")
        lines.append("")
        resp = turn["response"]
        lines.append(
            f"*Status: {resp.get('status', 'N/A')} | "
            f"Elapsed: {resp.get('elapsed_seconds', 0)}s*"
        )
        lines.append("")
        lines.append(turn["parsed_response"])
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
    output = {
        "scenario": scenario["name"],
        "description": scenario.get("description", ""),
        "api_url": api_url,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "turns": turns,
        "summary": {
            "total_turns": len(turns),
            "total_time_seconds": sum(
                t.get("response", {}).get("elapsed_seconds", 0) for t in turns
            ),
            "errors": sum(
                1 for t in turns if t.get("response", {}).get("error")
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
            "min": round(min(latencies), 3),
            "max": round(max(latencies), 3),
            "mean": round(sum(latencies) / n, 3),
            "p50": round(latencies[n // 2], 3),
            "p99": round(latencies[int(n * 0.99)] if n > 1 else latencies[-1], 3),
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
    parser = argparse.ArgumentParser(description="Drive benchmark sessions")
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
        "--concurrency",
        type=int,
        default=1,
        help="Number of concurrent requests",
    )
    args = parser.parse_args()

    # Load scenario
    with open(args.prompt_file) as f:
        scenario = yaml.safe_load(f)

    prompts = scenario.get("prompts", [])
    if not prompts:
        print("ERROR: No prompts found in scenario file.", file=sys.stderr)
        sys.exit(1)

    # Support both string and dict prompts
    prompt_items = []
    for p in prompts:
        if isinstance(p, str):
            prompt_items.append({"text": p, "type": "query"})
        elif isinstance(p, dict):
            prompt_items.append({
                "text": p.get("text", p.get("prompt", "")),
                "type": p.get("type", "query"),
            })

    print(f"  Scenario: {scenario['name']}")
    print(f"  Prompts: {len(prompt_items)}")
    print(f"  Timeout: {args.timeout}s")
    print(f"  Concurrency: {args.concurrency}")
    print()

    os.makedirs(args.results_dir, exist_ok=True)

    turns = []

    for i, item in enumerate(prompt_items, 1):
        prompt_text = item["text"]
        turn_type = item["type"]

        print(f"  --- Turn {i}/{len(prompt_items)} ({turn_type}) ---")
        print(
            f"  Prompt: {prompt_text[:80]}{'...' if len(prompt_text) > 80 else ''}"
        )

        # Send request
        response = send_prompt(
            args.api_url,
            args.api_token,
            prompt_text,
            args.timeout,
            turn_type,
        )

        # Parse response
        parsed = parse_response(response)

        turn = {
            "turn": i,
            "prompt": prompt_text,
            "turn_type": turn_type,
            "response": response,
            "parsed_response": parsed,
        }
        turns.append(turn)

        status = response.get("status", "N/A")
        elapsed = response.get("elapsed_seconds", 0)
        error = response.get("error")

        if error:
            print(f"  ERROR: {error}")
        else:
            print(f"  Status: {status} | Elapsed: {elapsed}s")
        print()

    # Write outputs
    write_results_json(scenario, turns, args.results_dir, args.api_url)
    write_transcript(scenario, turns, args.results_dir)
    write_metrics(turns, args.results_dir)

    print()
    print(f"  Done. {len(turns)} turns completed.")


if __name__ == "__main__":
    main()
