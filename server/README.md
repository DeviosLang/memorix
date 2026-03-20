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

### LLM (required for memory write)

Memory writes use a two-phase smart pipeline: the LLM extracts atomic facts from content, then reconciles them against existing memories to prevent duplicates. **Without LLM config, all write requests will fail.**

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
| `MNEMO_INGEST_MODE` | `smart` | `smart` — LLM extract + reconcile. `raw` — store as-is, no LLM needed (used by the `/ingest` endpoint). |

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
# Create a memory
curl -X POST http://localhost:8080/v1alpha1/memorix/<tenantID>/memories \
  -H "Content-Type: application/json" \
  -H "X-Memorix-Agent-Id: my-agent" \
  -d '{"content":"The server requires LLM config for writes","tags":["infra"]}'

# Search memories
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/memories?q=server&limit=10"

# Get by ID
curl "http://localhost:8080/v1alpha1/memorix/<tenantID>/memories/<id>"

# Update
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
curl -X DELETE http://localhost:8080/v1alpha1/memorix/<tenantID>/user-profile/facts/<fact_id>"
```

**Fact Categories:**
- `personal` - Name, role, location, etc.
- `preference` - Language, framework, style preferences
- `goal` - User goals and objectives
- `skill` - Known skills and expertise

**Fact Sources:**
- `explicit` - User explicitly provided (confidence typically 1.0)
- `inferred` - Model inferred from conversation (confidence 0.0-1.0)

## Build

```bash
# Local binary
make build

# Docker image
make docker-build

# Run with docker (quote MNEMO_DSN to avoid shell parsing tcp(...))
make docker-run MNEMO_DSN="user:pass@tcp(host:4000)/db?parseTime=true&tls=true"
```
