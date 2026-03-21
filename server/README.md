# memorix-server

Go REST API server for memorix — cloud-persistent memory for AI agents.

## Prerequisites

- Go 1.22+ (or Docker)
- MySQL-compatible database: TiDB Serverless, TiDB Self-hosted, or MySQL 8.0+

## Quick Start

### 1. Apply the database schema

```bash
mysql -h <host> -P <port> -u <user> -p < schema.sql
```

### 2. Run with Docker

```bash
docker build -t memorix-server .

docker run -d --name memorix-server -p 8080:8080 \
  -e MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true&tls=true" \
  -e MNEMO_LLM_API_KEY="sk-..." \
  -e MNEMO_LLM_BASE_URL="https://api.openai.com/v1" \
  memorix-server
```

### 3. Run from source

```bash
cd server
export MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true"
export MNEMO_LLM_API_KEY="sk-..."
export MNEMO_LLM_BASE_URL="https://api.openai.com/v1"
go run ./cmd/memorix-server
```

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `MNEMO_DSN` | MySQL DSN. Format: `user:pass@tcp(host:port)/dbname?parseTime=true`. Always quote in shell to avoid issues with `tcp(...)` parentheses. |

### LLM (required for memory writes)

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

### Ingest mode

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_INGEST_MODE` | `smart` | `smart` — LLM extract + reconcile (recommended). `raw` — store as-is, no LLM needed. |

### Embedding (optional — enables vector search)

Without embedding, the server falls back to keyword search only.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_EMBED_API_KEY` | — | API key for embedding provider |
| `MNEMO_EMBED_BASE_URL` | — | Base URL of embedding API (OpenAI-compatible) |
| `MNEMO_EMBED_MODEL` | — | Embedding model name |
| `MNEMO_EMBED_DIMS` | `1536` | Embedding dimensions |
| `MNEMO_EMBED_AUTO_MODEL` | — | TiDB Serverless auto-embed model (e.g. `tidbcloud_free/amazon/titan-embed-text-v2`). Takes priority over client-side embedding. |
| `MNEMO_EMBED_AUTO_DIMS` | `1024` | Dimensions for auto-embed model |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_PORT` | `8080` | HTTP listen port |
| `MNEMO_RATE_LIMIT` | `100` | Requests per second (per tenant) |
| `MNEMO_RATE_BURST` | `200` | Burst size for rate limiter |

### Full-text search

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_FTS_ENABLED` | `false` | Enable FULLTEXT INDEX search. Only set to `true` if your TiDB cluster supports `FTS_MATCH_WORD`. Leave `false` for TiDB Serverless / TiDB Zero. |

### Context Window

Token-budgeted context assembly for agent system prompts.

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

### Memory GC

Automatically removes stale, low-confidence, and over-capacity memories on a configurable schedule.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_GC_ENABLED` | `true` | Enable automatic background GC |
| `MNEMO_GC_INTERVAL` | `24h` | How often GC runs (e.g. `6h`, `24h`) |
| `MNEMO_GC_STALE_THRESHOLD` | `2160h` (90d) | Memories not accessed for this duration become stale candidates |
| `MNEMO_GC_LOW_CONFIDENCE_THRESHOLD` | `0.5` | Memories below this confidence score are GC candidates |
| `MNEMO_GC_MAX_MEMORIES_PER_TENANT` | `10000` | When exceeded, lowest-importance memories are pruned |
| `MNEMO_GC_SNAPSHOT_RETENTION_DAYS` | `30` | Days to keep GC snapshots for recovery audit |
| `MNEMO_GC_BATCH_SIZE` | `100` | Memories processed per GC iteration |

### Rules

Inject hierarchical Markdown rules (organization → user → project → module) into agent system prompts. Module-level rules support YAML frontmatter with `paths:` glob patterns to activate rules for specific file types.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_RULES_ENABLED` | `true` | Enable rules loading and injection |
| `MNEMO_RULES_ORGANIZATION_PATH` | `/etc/agent/rules.md` | Path to organization-level rules file |
| `MNEMO_RULES_USER_PATH` | `~/.agent/rules.md` | Path to user-level rules file |
| `MNEMO_RULES_INJECTION_ENABLED` | `true` | Inject loaded rules into system prompts |
| `MNEMO_RULES_INJECTION_MAX_TOKENS` | `2000` | Maximum tokens for injected rules content |
| `MNEMO_RULES_INJECTION_HEADER` | `## Project Rules\n\n` | Header prepended to the injected rules section |

### Experience Layer (optional — requires external vector store)

Semantic recall over long-term user experiences. Disabled by default; requires a running Qdrant or Chroma instance.

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_EXPERIENCE_ENABLED` | `false` | Enable the experience recall layer |
| `MNEMO_EXPERIENCE_BACKEND` | `qdrant` | Vector store backend: `qdrant` or `chroma` |
| `MNEMO_EXPERIENCE_MAX_PER_USER` | `10000` | Maximum stored experiences per user |
| `MNEMO_QDRANT_URL` | `http://localhost:6333` | Qdrant server URL |
| `MNEMO_QDRANT_API_KEY` | — | Qdrant API key (if auth enabled) |
| `MNEMO_CHROMA_URL` | `http://localhost:8000` | Chroma server URL |
| `MNEMO_CHROMA_DISTANCE` | `cosine` | Chroma distance metric: `cosine`, `l2`, or `ip` |

### Tenant connection pool

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_TENANT_POOL_MAX_IDLE` | `5` | Max idle DB connections per tenant |
| `MNEMO_TENANT_POOL_MAX_OPEN` | `10` | Max open DB connections per tenant |
| `MNEMO_TENANT_POOL_IDLE_TIMEOUT` | `10m` | Idle connection timeout |
| `MNEMO_TENANT_POOL_TOTAL_LIMIT` | `200` | Total tenant connection limit |

### TiDB Zero

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_TIDB_ZERO_ENABLED` | `true` | Enable TiDB Zero tenant provisioning |
| `MNEMO_TIDB_ZERO_API_URL` | `https://zero.tidbapi.com/v1alpha1` | TiDB Zero API base URL |

### File uploads

| Variable | Default | Description |
|----------|---------|-------------|
| `MNEMO_UPLOAD_DIR` | `./uploads` | Directory for file storage |
| `MNEMO_WORKER_CONCURRENCY` | `5` | Parallel upload workers |

## API

### Provision a tenant

```bash
curl -X POST http://localhost:8080/v1alpha1/memorix
# → {"id":"<tenantID>","claim_url":"..."}
```

Save the returned `id` — it is used in all memory operations.

### Memory operations

```bash
# Create a memory (smart ingest: LLM extracts + reconciles facts)
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/memories \
  -H "Content-Type: application/json" \
  -H "X-Memorix-Agent-Id: my-agent" \
  -d '{"content":"The server requires LLM config for writes","tags":["infra"]}'

# Search memories (hybrid vector + keyword)
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/memories?q=server&limit=10"

# Get by ID
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/memories/<id>"

# Update with optimistic locking
# If-Match version must match — returns 409 Conflict on mismatch
curl -X PUT http://localhost:8080/v1alpha1/memorix/<tenantID>/memories/<id> \
  -H "Content-Type: application/json" \
  -H "If-Match: 3" \
  -d '{"content":"updated content","tags":["infra"]}'

# Update without version check (last-write-wins)
curl -X PUT http://localhost:8080/v1alpha1/memorix/<tenantID>/memories/<id> \
  -H "Content-Type: application/json" \
  -d '{"content":"updated content","tags":["infra"]}'

# Delete
curl -X DELETE http://localhost:8080/v1alpha1/memorix/<tenantID>/memories/<id>
```

### User Profile Facts

User profile facts are structured long-term facts about users, supporting precise CRUD operations without vector search. Each user can store up to 200 facts, with automatic cleanup of oldest low-confidence facts when capacity is reached.

```bash
# Create a fact
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "category": "personal",
    "key": "name",
    "value": "John Doe",
    "source": "explicit",
    "confidence": 1.0
  }'

# List facts (filter by user_id required)
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts?user_id=user-123"

# List facts by category
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts?user_id=user-123&category=preference"

# Get a single fact
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts/<fact_id>"

# Update a fact
curl -X PUT http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts/<fact_id> \
  -H "Content-Type: application/json" \
  -d '{"value": "Jane Doe"}'

# Delete a fact
curl -X DELETE "http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts/<fact_id>"
```

**Fact Categories:**
- `personal` — Name, role, location, etc.
- `preference` — Language, framework, style preferences
- `goal` — User goals and objectives
- `skill` — Known skills and expertise

**Fact Sources:**
- `explicit` — User explicitly provided (confidence typically 1.0)
- `inferred` — Model inferred from conversation (confidence 0.0–1.0)

### Conversation Summaries

```bash
# Summarize a session
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/summaries \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "session_id": "session-456",
    "messages": [
      {"role": "user", "content": "How do I set up the server?"},
      {"role": "assistant", "content": "Run go run ./cmd/memorix-server with MNEMO_DSN set."}
    ]
  }'

# List summaries for a user
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/summaries?user_id=user-123"
```

### Context Window

```bash
# Assemble a token-budgeted context window
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/context \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "session_id": "session-456",
    "query": "How do I configure embeddings?",
    "max_tokens": 4096
  }'
# → {"context": "...", "total_tokens": 1234, "truncated": false, "layers": {...}}

# Count tokens in a message list
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/context/count \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}'
```

### Memory GC

```bash
# Preview what GC would clean up (dry run)
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/gc/preview

# Trigger GC immediately (real deletion)
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/gc

# List GC run history
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/gc/logs"
```

### Rules

Module-level rule files support YAML frontmatter to restrict activation to specific file paths:

```markdown
---
paths:
  - "*.py"
  - "**/*.py"
name: Python Rules
enabled: true
---

# Python Rules

- Use snake_case for variables
- Follow PEP 8 style guide
```

```bash
# Load rules for active files
curl -X POST http://localhost:8080/v1alpha1/rules/load \
  -H "Content-Type: application/json" \
  -d '{"active_file_paths": ["main.py", "utils/helpers.py"]}'

# Check if any rule files have changed since last load
curl "http://localhost:8080/v1alpha1/rules/changes"

# Inject rules into a system prompt
curl -X POST http://localhost:8080/v1alpha1/rules/inject \
  -H "Content-Type: application/json" \
  -d '{
    "system_instructions": "You are a helpful assistant.",
    "rules": {"merged_content": "# Rules\n\n- Be helpful"},
    "inject_at": "start"
  }'
```

## Build

```bash
# Local binary
make build

# Docker image
make docker-build

# Run with docker (quote MNEMO_DSN to avoid shell parsing tcp(...))
make docker-run MNEMO_DSN="user:pass@tcp(host:4000)/db?parseTime=true&tls=true"
```
