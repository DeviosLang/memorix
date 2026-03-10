# memorix CLI

Command-line tool for testing memorix-server REST API endpoints.

## Installation

```bash
cd cli
go build -o memorix .

# Optionally install to $GOPATH/bin
go install .
```

## Configuration

Set environment variables for convenience:

```bash
export MNEMO_API_URL="http://localhost:8080"
export MNEMO_TENANT_ID="your-tenant-id"
export MNEMO_AGENT_ID="cli-agent"
```

Or use flags:

```bash
memorix -u http://localhost:8080 -t your-tenant-id -a my-agent <command>
```

## Commands

### Provision a new tenant

```bash
memorix provision
# Returns: {"id": "uuid", "claim_url": "..."}
```

### Memory Operations

```bash
# Create a memory
memorix memory create "Project uses PostgreSQL 15" --tags "tech-stack,database"

# Search memories
memorix memory search -q "database" --limit 10
memorix memory search --tags "tech-stack" --state "active"

# Get a specific memory
memorix memory get <memory-id>

# Update a memory
memorix memory update <memory-id> -c "Updated content" --tags "new-tag"

# Delete a memory
memorix memory delete <memory-id>

# Bulk create from JSON file
memorix memory bulk ./memories.json

# Ingest conversation messages
memorix memory ingest ./messages.json --session-id "session-001"

# Get bootstrap memories for agent startup
memorix memory bootstrap --limit 20
```

### Task Operations (File Uploads)

```bash
# Upload a memory file
memorix task create ./memory.json --file-type memory

# Upload a session file
memorix task create ./sessions/session-001.json --file-type session --session-id session-001

# List all tasks
memorix task list

# Get task status
memorix task get <task-id>
```

### Tenant Operations

```bash
# Get tenant info
memorix tenant info
```

## File Formats

### Bulk Create JSON

```json
[
  {"content": "First memory", "tags": ["tag1"]},
  {"content": "Second memory", "tags": ["tag2"]}
]
```

### Ingest Messages JSON

```json
[
  {"role": "user", "content": "What is React?"},
  {"role": "assistant", "content": "React is a JavaScript library..."}
]
```

## Examples

```bash
# Full workflow example
memorix provision
# → {"id": "abc123..."}

export MNEMO_TENANT_ID="abc123..."

# Create some memories
memorix memory create "The project uses React 18 for the frontend" --tags "tech-stack,frontend"
memorix memory create "PostgreSQL 15 is the primary database" --tags "tech-stack,database"
memorix memory create "API runs on port 8080" --tags "config"

# Search for tech stack info
memorix memory search -q "tech stack"

# Upload existing session files
memorix task create ./sessions/session-001.json --file-type session --session-id session-001

# Check upload status
memorix task list
```

## Global Flags

| Flag | Short | Env Var | Default | Description |
|------|-------|---------|---------|-------------|
| `--api-url` | `-u` | `MNEMO_API_URL` | `http://localhost:8080` | memorix-server API URL |
| `--tenant-id` | `-t` | `MNEMO_TENANT_ID` | - | Tenant ID |
| `--agent-id` | `-a` | `MNEMO_AGENT_ID` | `cli-agent` | Agent ID |
| `--timeout` | - | - | `30s` | Request timeout |
