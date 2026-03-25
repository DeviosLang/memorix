<p align="center">
  <img src="assets/logo.png" alt="memorix" width="180" />
</p>

<h1 align="center">memorix</h1>

<p align="center">
  <strong>Persistent Memory for AI Agents.</strong><br/>
  Your agents forget everything between sessions. memorix fixes that.
</p>

<p align="center">
  <a href="https://tidbcloud.com"><img src="https://img.shields.io/badge/Powered%20by-TiDB%20Starter-E60C0C?style=flat&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTEuOTk4NCAxLjk5OTAyTDMuNzE4NzUgNy40OTkwMkwzLjcxODc1IDE3TDExLjk5NjQgMjIuNUwyMC4yODE0IDE3VjcuNDk5MDJMMTEuOTk4NCAxLjk5OTAyWiIgZmlsbD0id2hpdGUiLz48L3N2Zz4=" alt="Powered by TiDB Starter"></a>
  <a href="https://goreportcard.com/report/github.com/devioslang/memorix/server"><img src="https://goreportcard.com/badge/github.com/devioslang/memorix/server" alt="Go Report Card"></a>
  <a href="https://github.com/devioslang/memorix/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue.svg" alt="License"></a>
  <a href="https://github.com/devioslang/memorix"><img src="https://img.shields.io/github/stars/devioslang/memorix?style=social" alt="Stars"></a>
</p>

---

## 🚀 Quick Start

> **Want the fastest path?** See the [TiDB Serverless Quickstart](docs/quickstart-tidb.md) — deploy with 4 environment variables, no separate embedding service needed, estimated 15 minutes to a fully working setup.
>
> **Deploying to Kubernetes?** See the [K8s Deployment Guide](docs/deployment-k8s.md) — includes Deployment, Service, Ingress, HPA, and ready-to-use manifests in `deploy/k8s/`.

**Server-based memory via memorix-server.**

```bash
# 1. Deploy memorix-server (Go 1.22+ required)
cd server && MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" \
  MNEMO_LLM_API_KEY="sk-..." \
  MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
  go run ./cmd/memorix-server
```

**2. Install plugin for your agent (pick one):**

| Platform | Install |
|----------|---------|
| **Claude Code** | `/plugin marketplace add devioslang/memorix` then `/plugin install memorix-memory@memorix` |
| **OpenCode** | Add `"plugin": ["memorix-opencode"]` to `opencode.json` |
| **OpenClaw** | Add `memorix` to `openclaw.json` plugins (see [openclaw-plugin/README](openclaw-plugin/README.md)) |

```bash
# 3. Provision a tenant and set credentials
curl -s -X POST localhost:8080/v1alpha1/memorix
# → {"id":"...", "claim_url":"..."}

export MNEMO_API_URL="http://localhost:8080"
export MNEMO_TENANT_ID="..."
```

All agents pointing at the same tenant ID share one memory pool.

---

## The Problem

AI coding agents — Claude Code, OpenCode, OpenClaw, and others — often maintain separate local memory files. The result:

- 🧠 **Amnesia** — Agent forgets everything when a session ends
- 🏝️ **Silos** — One agent can't access what another learned yesterday
- 📁 **Local files** — Memory is tied to a single machine, lost when you switch devices
- 🚫 **No team sharing** — Your teammate's agent can't benefit from your agent's discoveries

**memorix** gives every agent a shared, cloud-persistent memory with hybrid vector + keyword search — powered by [TiDB Starter](https://tidbcloud.com).

## Why TiDB Starter?

memorix uses [TiDB Starter](https://tidbcloud.com) (formerly TiDB Serverless) as the backing store for memorix-server:

| Feature | What it means for you |
|---|---|
| **Free tier** | 25 GiB storage, 250M Request Units/month — enough for most individual and small team use |
| **TiDB Cloud Zero** | Instant database provisioning via API — no signup required for first 30 days |
| **Native VECTOR type** | Hybrid search (vector + keyword) without a separate vector database |
| **Auto-embedding (`EMBED_TEXT`)** | TiDB generates embeddings server-side — no OpenAI key needed for semantic search |
| **Zero ops** | No servers to manage, no scaling to worry about, automatic backups |
| **MySQL compatible** | Migrate to self-hosted TiDB or MySQL anytime |

This architecture keeps agent plugins **stateless** — all state lives in memorix-server, backed by TiDB.

## Supported Agents

memorix provides native plugins for major AI coding agent platforms:

| Platform | Plugin | How It Works | Install Guide |
|---|---|---|---|
| **Claude Code** | Hooks + Skills | Auto-loads memories on session start, auto-saves on stop | [`claude-plugin/README.md`](claude-plugin/README.md) |
| **OpenCode** | Plugin SDK | `system.transform` injects memories, `session.idle` auto-captures | [`opencode-plugin/README.md`](opencode-plugin/README.md) |
| **OpenClaw** | Memory Plugin | Replaces built-in memory slot (`kind: "memory"`), framework manages lifecycle | [`openclaw-plugin/README.md`](openclaw-plugin/README.md) |
| **Any HTTP client** | REST API | `curl` to memorix-server | [API Reference](#api-reference) |

All plugins expose the same 5 tools: `memory_store`, `memory_search`, `memory_get`, `memory_update`, `memory_delete`.

> **🤖 For AI Agents**: Use the Quick Start above to deploy memorix-server and provision a tenant ID, then follow the platform-specific README for configuration details.

## Stateless Agents, Cloud Memory

A key design principle: **agent plugins carry zero state.** All memory lives in memorix-server, backed by TiDB/MySQL. This means:

- **Agent plugins stay stateless** — deploy any number of agent instances freely; they all share the same memory pool via memorix-server
- **Switch machines freely** — your agent's memory follows you, not your laptop
- **Multi-agent collaboration** — Claude Code, OpenCode, OpenClaw, and any HTTP client share memories when pointed at the same server
- **Centralized control** — rate limits and audit live in one place

## 🧠 Auto Memory (Project-Level)

**NEW in memorix**: Automatic accumulation of project-specific experiences!

In addition to cloud-persistent memory via memorix-server, memorix now includes **Auto Memory** — a project-level memory system that automatically accumulates build commands, error solutions, architecture decisions, and user preferences.

### How It Works

```
Session Start → Load first 200 lines of MEMORY.md
     ↓
Agent Works → Detects valuable experiences:
  • Build/test commands (success)
  • Errors resolved
  • Architecture decisions
  • User preferences
     ↓
Session End → Auto-update MEMORY.md
```

### MEMORY.md

A Git-tracked Markdown file in your project root:

```markdown
# Project Auto Memory

## 📦 Build Commands
- Command: `make build` — Builds Go server
- Command: `make test` — Runs unit tests

## 🐛 Error Solutions
- Error: "module not found"
  Solution: Run `go mod tidy`

## 🏗️ Architecture Decisions
- Decision: Use chi router
  Rationale: Lightweight, composable
  Date: 2026-03-21

## ⚙️ User Preferences
- Preference: Always run tests before commit
```

### Features

✅ **Automatic accumulation** — No manual intervention needed
✅ **Deduplication** — Same command/error recorded only once
✅ **Team collaboration** — Commit MEMORY.md to share with team
✅ **Session integration** — First 200 lines loaded every session
✅ **/memory command** — View, search, and manage memories

### Usage

```bash
# View all memories
/memory

# View specific section
/memory build
/memory errors

# Search memories
/memory search "router"

# Edit manually
/memory edit
```

### Auto Memory vs memorix-server

| Feature | Auto Memory (MEMORY.md) | memorix-server |
|---------|-------------------------|----------------|
| **Scope** | Project-specific | Cross-project |
| **Storage** | Git-tracked file | Cloud database |
| **Sharing** | Team via Git | Personal/team via API |
| **Content** | Build commands, errors, decisions | Personal facts, patterns |
| **Best for** | Team collaboration | Personal preferences |

**Use both**: Auto Memory for project-specific knowledge (shared via Git), memorix-server for personal preferences (cloud-persistent).

## API Reference

Agent identity: `X-Memorix-Agent-Id` header (optional, used for attribution).

### Tenant Provisioning

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix` | Provision tenant (no auth). Returns `{ "id", "claim_url" }`. |

### Memory CRUD

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix/{tenantID}/memories` | Unified write: `{"content",...}` for direct create, or `{"messages",...}` for smart ingest pipeline (LLM extract + reconcile). |
| `GET` | `/v1alpha1/memorix/{tenantID}/memories` | Search/list. Params: `?q=`, `?tags=`, `?source=`, `?key=`, `?limit=`, `?offset=`. Hybrid vector + keyword when embedding is configured. |
| `GET` | `/v1alpha1/memorix/{tenantID}/memories/{id}` | Get single memory by ID. |
| `PUT` | `/v1alpha1/memorix/{tenantID}/memories/{id}` | Update memory. Supports `If-Match: <version>` header for optimistic locking — returns `409 Conflict` if version does not match. |
| `DELETE` | `/v1alpha1/memorix/{tenantID}/memories/{id}` | Delete memory. |

### User Profile Facts

Structured long-term facts about users (name, preferences, skills, goals). Stored with `(user_id, category, key)` unique constraint, supporting precise CRUD without vector search.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix/{tenantID}/user-profile/facts` | Create a fact. Fields: `user_id`, `category`, `key`, `value`, `source`, `confidence`. |
| `GET` | `/v1alpha1/memorix/{tenantID}/user-profile/facts` | List facts. Requires `?user_id=`. Optional `?category=`. |
| `GET` | `/v1alpha1/memorix/{tenantID}/user-profile/facts/{id}` | Get single fact. |
| `PUT` | `/v1alpha1/memorix/{tenantID}/user-profile/facts/{id}` | Update fact value/confidence. |
| `DELETE` | `/v1alpha1/memorix/{tenantID}/user-profile/facts/{id}` | Delete fact. |
| `POST` | `/v1alpha1/memorix/{tenantID}/user-profile/extract` | LLM-extract facts from messages and upsert. |
| `POST` | `/v1alpha1/memorix/{tenantID}/user-profile/reconcile` | Batch reconcile facts (dedup + LWW merge). |
| `GET` | `/v1alpha1/memorix/{tenantID}/user-profile/reconcile/audit` | List reconciliation audit log. |
| `GET` | `/v1alpha1/memorix/{tenantID}/user-profile/reconcile/audit/{fact_id}` | Audit log for a specific fact. |

### Conversation Summaries

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix/{tenantID}/summaries` | Summarize a conversation session. Fields: `user_id`, `session_id`, `messages`. |
| `GET` | `/v1alpha1/memorix/{tenantID}/summaries` | List summaries. Params: `?user_id=`, `?session_id=`. |
| `GET` | `/v1alpha1/memorix/{tenantID}/summaries/{id}` | Get single summary. |
| `DELETE` | `/v1alpha1/memorix/{tenantID}/summaries/{id}` | Delete summary. |

### Context Window

Assemble and manage token-budgeted context windows for injection into agent system prompts.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix/{tenantID}/context` | Assemble a context window. Fields: `user_id`, `session_id`, `query`, `max_tokens`. Returns layered context (system + memory + summary + session). |
| `POST` | `/v1alpha1/memorix/{tenantID}/context/truncate` | Truncate a message list to fit within a token budget. |
| `POST` | `/v1alpha1/memorix/{tenantID}/context/count` | Count tokens in a message list. |
| `POST` | `/v1alpha1/memorix/{tenantID}/context/build` | Build a full context object with elastic budget layers. |

### Memory GC

Garbage-collect stale, low-confidence, and over-capacity memories. GC also runs automatically on a configurable interval (default: every 24 hours).

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix/{tenantID}/gc` | Trigger GC immediately. Optional `{"dry_run": true}` to preview without deleting. |
| `POST` | `/v1alpha1/memorix/{tenantID}/gc/preview` | Preview what GC would delete (always dry-run). |
| `GET` | `/v1alpha1/memorix/{tenantID}/gc/logs` | List GC run history. |
| `GET` | `/v1alpha1/memorix/{tenantID}/gc/snapshots/{id}` | Get a GC snapshot for recovery audit. |

### File Imports

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/memorix/{tenantID}/imports` | Upload a file for async memory ingestion. |
| `GET` | `/v1alpha1/memorix/{tenantID}/imports` | List import tasks. |
| `GET` | `/v1alpha1/memorix/{tenantID}/imports/{id}` | Get import task status. |

### Rules API

Inject hierarchical Markdown rules (organization → user → project → module) into agent system prompts.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1alpha1/rules/load` | Load applicable rules for a set of active file paths. Returns merged rule content and change records. |
| `GET` | `/v1alpha1/rules/changes` | Check whether any rule files have changed since last load. |
| `GET` | `/v1alpha1/rules/status` | Get rules loader status and cached file list. |
| `POST` | `/v1alpha1/rules/inject` | Inject loaded rules into a system prompt string. Supports `start`, `end`, and `replace_marker` injection modes. |

### User-Centric API

Per-user memory access without tenant scoping (used by agent hooks).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/memory/{user_id}/facts` | List all facts for a user. |
| `DELETE` | `/api/memory/{user_id}/facts/{fact_id}` | Delete a specific fact. |
| `GET` | `/api/memory/{user_id}/summaries` | List conversation summaries for a user. |
| `GET` | `/api/memory/{user_id}/experiences` | Semantic search over user's experience store (requires vector store). |
| `POST` | `/api/memory/{user_id}/gc` | Trigger GC for a specific user. |
| `GET` | `/api/memory/{user_id}/stats` | Get memory statistics for a user. |
| `GET` | `/api/memory/{user_id}/overview` | Get a full memory overview (facts + summaries + stats). |

## Self-Hosting

### Environment Variables

#### Required

| Variable | Description |
|----------|-------------|
| `MNEMO_DSN` | Database connection string. Format: `user:pass@tcp(host:port)/dbname?parseTime=true`. Always quote in shell to avoid issues with `tcp(...)` parentheses. |

#### LLM (required for memory writes)

Memory writes use a two-phase smart pipeline: the LLM extracts atomic facts from content, then reconciles them against existing memories to prevent duplicates. **Without LLM config, all write requests will fail** (unless `MNEMO_INGEST_MODE=raw`).

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_LLM_API_KEY` | — | API key for the LLM provider |
| `MNEMO_LLM_BASE_URL` | — | Base URL of the LLM API (OpenAI-compatible) |
| `MNEMO_LLM_MODEL` | `gpt-4o-mini` | Model name to use |
| `MNEMO_LLM_TEMPERATURE` | `0.1` | Sampling temperature |

**Provider examples:**

```bash
# OpenAI
MNEMO_LLM_BASE_URL=https://api.openai.com/v1
MNEMO_LLM_MODEL=gpt-4o-mini

# Ollama (local)
MNEMO_LLM_BASE_URL=http://localhost:11434/v1
MNEMO_LLM_MODEL=llama3.2
MNEMO_LLM_API_KEY=ollama   # placeholder required

# DeepSeek
MNEMO_LLM_BASE_URL=https://api.deepseek.com/v1
MNEMO_LLM_MODEL=deepseek-chat
```

#### Ingest Mode

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_INGEST_MODE` | `smart` | `smart` — LLM extract + reconcile (recommended). `raw` — store as-is, no LLM needed. |

#### Embedding (optional — enables vector search)

Without embedding config, the server falls back to keyword search only.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_EMBED_API_KEY` | — | API key for embedding provider |
| `MNEMO_EMBED_BASE_URL` | — | Base URL of embedding API (OpenAI-compatible) |
| `MNEMO_EMBED_MODEL` | — | Embedding model name |
| `MNEMO_EMBED_DIMS` | `1536` | Embedding dimensions |
| `MNEMO_EMBED_AUTO_MODEL` | — | TiDB Serverless auto-embed model (e.g. `tidbcloud_free/amazon/titan-embed-text-v2`). Takes priority over client-side embedding. |
| `MNEMO_EMBED_AUTO_DIMS` | `1024` | Dimensions for auto-embed model |

#### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_PORT` | `8080` | HTTP listen port |
| `MNEMO_RATE_LIMIT` | `100` | Requests per second (per tenant) |
| `MNEMO_RATE_BURST` | `200` | Burst size |
| `MNEMO_FTS_ENABLED` | `false` | Enable FULLTEXT INDEX search. Only set `true` if your TiDB cluster supports `FTS_MATCH_WORD`. Leave `false` for TiDB Serverless / TiDB Zero. |

#### Context Window

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_MAX_CONTEXT_TOKENS` | `8192` | Maximum tokens in an assembled context window |
| `MNEMO_TOKENIZER_TYPE` | `tiktoken` | Token counter: `tiktoken` or `estimate` |
| `MNEMO_TOKENIZER_MODEL` | `gpt-4` | Model used for tiktoken encoding selection |
| `MNEMO_SYSTEM_PROMPT_RESERVED_TOKENS` | `500` | Token budget reserved for system prompt layer |
| `MNEMO_MEMORY_RESERVED_TOKENS` | `2000` | Token budget reserved for memory injection |
| `MNEMO_METADATA_RESERVED_TOKENS` | `200` | Token budget reserved for session metadata |
| `MNEMO_USER_MEMORY_BUDGET_MIN` | `500` | Minimum tokens for user memory layer |
| `MNEMO_USER_MEMORY_BUDGET_MAX` | `1500` | Maximum tokens for user memory layer |
| `MNEMO_SUMMARY_BUDGET_MIN` | `300` | Minimum tokens for conversation summary layer |
| `MNEMO_SUMMARY_BUDGET_MAX` | `800` | Maximum tokens for conversation summary layer |

#### Memory GC

Automatically runs on a configurable interval to remove stale, low-confidence, and over-capacity memories.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_GC_ENABLED` | `true` | Enable automatic background GC |
| `MNEMO_GC_INTERVAL` | `24h` | How often GC runs (e.g. `6h`, `24h`) |
| `MNEMO_GC_STALE_THRESHOLD` | `2160h` (90d) | Memories not accessed for this duration become stale candidates |
| `MNEMO_GC_LOW_CONFIDENCE_THRESHOLD` | `0.5` | Memories below this confidence score are GC candidates |
| `MNEMO_GC_MAX_MEMORIES_PER_TENANT` | `10000` | When exceeded, lowest-importance memories are pruned |
| `MNEMO_GC_SNAPSHOT_RETENTION_DAYS` | `30` | Days to keep GC snapshots for recovery audit |
| `MNEMO_GC_BATCH_SIZE` | `100` | Memories processed per GC iteration |

#### Rules

Inject hierarchical Markdown rules into agent system prompts. Rules are loaded from organization → user → project → module level, with later levels taking precedence.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_RULES_ENABLED` | `true` | Enable the rules loading and injection system |
| `MNEMO_RULES_ORGANIZATION_PATH` | `/etc/agent/rules.md` | Path to organization-level rules file |
| `MNEMO_RULES_USER_PATH` | `~/.agent/rules.md` | Path to user-level rules file |
| `MNEMO_RULES_INJECTION_ENABLED` | `true` | Inject loaded rules into system prompts |
| `MNEMO_RULES_INJECTION_MAX_TOKENS` | `2000` | Maximum tokens for rules content in system prompt |
| `MNEMO_RULES_INJECTION_HEADER` | `## Project Rules\n\n` | Header prepended to injected rules section |

#### Experience Layer (optional — requires external vector store)

Semantic recall over long-term user experiences. Requires an external vector store (Qdrant or Chroma).

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_EXPERIENCE_ENABLED` | `false` | Enable the experience recall layer |
| `MNEMO_EXPERIENCE_BACKEND` | `qdrant` | Vector store backend: `qdrant` or `chroma` |
| `MNEMO_EXPERIENCE_MAX_PER_USER` | `10000` | Maximum stored experiences per user |
| `MNEMO_QDRANT_URL` | `http://localhost:6333` | Qdrant server URL |
| `MNEMO_QDRANT_API_KEY` | — | Qdrant API key (if auth enabled) |
| `MNEMO_CHROMA_URL` | `http://localhost:8000` | Chroma server URL |
| `MNEMO_CHROMA_DISTANCE` | `cosine` | Chroma distance metric: `cosine`, `l2`, or `ip` |

#### TiDB Zero

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_TIDB_ZERO_ENABLED` | `true` | Enable TiDB Zero tenant provisioning |
| `MNEMO_TIDB_ZERO_API_URL` | `https://zero.tidbapi.com/v1alpha1` | TiDB Zero API base URL |

#### Tenant Connection Pool

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_TENANT_POOL_MAX_IDLE` | `5` | Max idle DB connections per tenant |
| `MNEMO_TENANT_POOL_MAX_OPEN` | `10` | Max open DB connections per tenant |
| `MNEMO_TENANT_POOL_IDLE_TIMEOUT` | `10m` | Idle connection timeout |
| `MNEMO_TENANT_POOL_TOTAL_LIMIT` | `200` | Total tenant connection limit |

#### File Uploads

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_UPLOAD_DIR` | `./uploads` | Directory for file storage |
| `MNEMO_WORKER_CONCURRENCY` | `5` | Parallel upload workers |

### Build & Run

```bash
cd server
go build -o memorix-server ./cmd/memorix-server
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" \
  MNEMO_LLM_API_KEY="sk-..." \
  MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
  ./memorix-server
```

### Docker

```bash
docker build -t memorix-server ./server
docker run \
  -e MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true&tls=true" \
  -e MNEMO_LLM_API_KEY="sk-..." \
  -e MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
  -p 8080:8080 memorix-server
```

## Project Structure

```
memorix/
├── server/                     # Go API server (Go 1.22+)
│   ├── cmd/memorix-server/       # Entry point, DI wiring, graceful shutdown
│   ├── internal/
│   │   ├── config/             # Env var config loading
│   │   ├── domain/             # Core types, errors, token generation
│   │   ├── embed/              # Embedding provider (OpenAI/Ollama/any)
│   │   ├── handler/            # HTTP handlers + chi router
│   │   ├── middleware/         # Tenant resolution + rate limiter
│   │   ├── repository/         # Interface + TiDB SQL implementation
│   │   ├── service/            # Business logic (upsert, LWW, hybrid search, GC, rules)
│   │   ├── tokenizer/          # Token counting (tiktoken + estimate backends)
│   │   └── vectorstore/        # Vector store abstraction (Qdrant, Chroma)
│   ├── schema.sql
│   └── Dockerfile
│
├── opencode-plugin/            # OpenCode agent plugin (TypeScript)
│   └── src/                    # Plugin SDK tools + hooks + server backend
│
├── openclaw-plugin/            # OpenClaw agent plugin (TypeScript)
│   ├── index.ts                # Tool registration
│   └── server-backend.ts       # Server: fetch → memorix API
│
├── claude-plugin/              # Claude Code plugin (Hooks + Skills)
│   ├── hooks/                  # Lifecycle hooks (bash + curl)
│   └── skills/                 # memory-recall + memory-store + memorix-setup
│
├── skills/                     # Shared skills (OpenClaw ClawHub format)
│   └── memorix-setup/           # Setup skill
│
└── docs/DESIGN.md              # Full design document
```

## Roadmap

| Phase | What | Status |
|-------|------|--------|
| **Phase 1** | Core server + CRUD + hybrid search (vector + keyword + FTS) + upsert + LWW + plugins (Claude Code, OpenCode, OpenClaw) | ✅ Done |
| **Phase 1.5** | User profile facts, conversation summaries, context window assembly, memory GC, rules injection, smart ingest pipeline (LLM extract + reconcile), optimistic locking (`If-Match` / `409`) | ✅ Done |
| **Phase 2** | Memory tiers (`working/short/long/reference`), salience scoring, auto promote/demote, archive-first forgetting | 🔜 Planned |
| **Phase 3** | Knowledge base migration (tenant-to-tenant + `memorix migrate` CLI), LLM-assisted conflict merge, auto-tagging | 📋 Planned |
| **Phase 4** | Rule normalization (repeated patterns → executable rules, auditable + reversible), Skill/MCP recommendation mining from memory signals | 📋 Planned |
| **Phase 5** | Web dashboard, bulk import/export, CLI wizard | 📋 Planned |

Vector Clock CRDT was deferred and removed from the roadmap.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

[Apache-2.0](LICENSE)

---

<p align="center">
  <a href="https://tidbcloud.com"><img src="assets/tidb-logo.png" alt="TiDB Starter" height="36" /></a>
  <br/>
  <sub>Built with <a href="https://tidbcloud.com">TiDB Starter</a> — zero-ops database with native vector search.</sub>
</p>
