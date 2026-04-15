---
name: memorix-setup
description: |
  Setup memorix persistent memory with memorix-server.
  Triggers: "set up memorix", "install memorix plugin", "configure memory plugin",
  "configure openclaw memory", "configure opencode memory",
  "configure claude code memory".
---

# memorix Setup

**Persistent memory for AI agents.** This skill helps you set up memorix with any agent platform.

## Prerequisites

You need a running memorix-server instance. See the [server README](https://github.com/devioslang/memorix/tree/main/server) for deployment instructions.

## Step 0: Get the code

**Option A: Git clone (recommended)**

```bash
git clone https://github.com/DeviosLang/memorix.git
cd memorix
```

**Option B: Download ZIP**

Download and unzip: https://github.com/DeviosLang/memorix/archive/refs/heads/main.zip

```bash
unzip memorix-main.zip
cd memorix-main
```

**Option C: Docker (no code needed)**

If you only want to run the server without building from source, skip to Step 1 and use:

```bash
docker pull ghcr.io/devioslang/memorix-server:latest
docker run -e MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" \
  -p 8080:8080 ghcr.io/devioslang/memorix-server:latest
```

Then skip Step 1 and go directly to Step 2.

---

## Step 1: Deploy memorix-server

```bash
cd memorix/server
MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" go run ./cmd/memorix-server
```

## Step 2: Provision a tenant

```bash
curl -s -X POST http://localhost:8080/v1alpha1/memorix | jq .
# → { "id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", "claim_url": "..." }
```

Save the returned `id` — this is your tenant ID used in all subsequent API calls.

## Step 3: Configure your agent platform

Pick your platform and follow the instructions:

---

#### OpenClaw

Add to `openclaw.json`:

```json
{
  "plugins": {
    "slots": { "memory": "memorix" },
    "entries": {
      "memorix": {
        "enabled": true,
        "config": {
          "apiUrl": "http://localhost:8080",
          "tenantID": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
        }
      }
    }
  }
}
```

Restart OpenClaw. You should see:
```
[memorix] Server mode (tenant-scoped memorix API)
```

---

#### OpenCode

Set environment variables (add to shell profile or `.env`):

```bash
export MNEMO_API_URL="http://localhost:8080"
export MNEMO_TENANT_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

Add to `opencode.json`:
```json
{
  "plugin": ["memorix-opencode"]
}
```

Restart OpenCode. You should see:
```
[memorix] Server mode (memorix-server REST API)
```

---

#### Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "env": {
    "MNEMO_API_URL": "http://localhost:8080",
    "MNEMO_TENANT_ID": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  }
}
```

Install plugin:
```
/plugin marketplace add devioslang/memorix
/plugin install memorix-memory@memorix
```

Restart Claude Code.

---

## Verification

After setup, test memory:

1. Ask your agent: "Remember that the project uses PostgreSQL 15"
2. Start a new session
3. Ask: "What database does this project use?"

The agent should recall the information from memory.

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `No MNEMO_API_URL configured` | Set `MNEMO_API_URL` env var or `apiUrl` in plugin config |
| `MNEMO_TENANT_ID is not set` | Set `MNEMO_TENANT_ID` env var or `tenantID` in plugin config |
| Plugin not loading | Check platform-specific config format |
