#!/usr/bin/env bash
# common.sh — Shared helpers for memorix hooks.
# Sourced by all hook scripts.
#
# Requires MNEMO_API_URL + MNEMO_TENANT_ID to connect to memorix-server.

set -euo pipefail

memorix_check_env() {
  if [[ -z "${MNEMO_API_URL:-}" ]]; then
    echo '{"error":"MNEMO_API_URL is not set"}' >&2
    return 1
  fi
  if [[ -z "${MNEMO_TENANT_ID:-}" ]]; then
    echo '{"error":"MNEMO_TENANT_ID is not set"}' >&2
    return 1
  fi
}

# Tenant-scoped base path.
memorix_base() {
  echo "${MNEMO_API_URL}/v1alpha1/memorix/${MNEMO_TENANT_ID}"
}

# memorix_server_get <path> — GET request to memorix-server (tenant-scoped).
memorix_server_get() {
  local path="$1"
  curl -sf --max-time 8 \
    -H "Content-Type: application/json" \
    -H "X-Memorix-Agent-Id: ${MNEMO_AGENT_ID:-claude-code}" \
    "$(memorix_base)${path}"
}

# memorix_server_post <path> <json_body> — POST request to memorix-server (tenant-scoped).
memorix_server_post() {
  local path="$1"
  local body="$2"
  curl -sf --max-time 8 \
    -H "Content-Type: application/json" \
    -H "X-Memorix-Agent-Id: ${MNEMO_AGENT_ID:-claude-code}" \
    -d "${body}" \
    "$(memorix_base)${path}"
}

# ─── Public helpers ─────────────────────────────────────────────────

# memorix_get_memories [limit] — Fetch recent memories.
memorix_get_memories() {
  local limit="${1:-20}"
  memorix_server_get "/memories?limit=${limit}"
}

# memorix_post_memory <json_body> — Store a memory.
memorix_post_memory() {
  local body="$1"
  memorix_server_post "/memories" "$body"
}

# memorix_search <query> [limit] — Search memories.
memorix_search() {
  local query="$1"
  local limit="${2:-10}"
  local encoded_q
  encoded_q=$(printf '%s' "$query" | python3 -c "import sys,urllib.parse; print(urllib.parse.quote(sys.stdin.read().strip()))" 2>/dev/null || echo "$query")
  memorix_server_get "/memories?q=${encoded_q}&limit=${limit}"
}

# read_stdin — Read stdin (hook input JSON) into $HOOK_INPUT.
read_stdin() {
  local input=""
  IFS= read -r -d '' -t 2 input 2>/dev/null || true
  HOOK_INPUT="${input:-{}}"
}
