# OpenCode Plugin for memorix

Persistent memory for [OpenCode](https://opencode.ai) — injects memories into system prompt automatically, with 5 memory tools.

## 🚀 Quick Start (server mode)

```bash
# 1. Provision a tenant
curl -s -X POST http://your-server:8080/v1alpha1/memorix \
  -H "Content-Type: application/json" \
  -d '{}' \
  | jq .
# → { "id": "uuid", "claim_url": "..." }

# 2. Set your memorix-server connection
export MNEMO_API_URL="http://your-server:8080"
export MNEMO_TENANT_ID="uuid"

# 3. Add plugin to opencode.json
echo '{"plugin": ["memorix-opencode"]}' > opencode.json

# 4. Start OpenCode - plugin auto-installs from npm
opencode
```

**That's it!** Your agent now has persistent cloud memory.

---

## How It Works

```
System Prompt Transform → Inject recent memories into system prompt
          ↓
    Agent works normally, can use memory_* tools anytime
```

| Hook / Tool | Trigger | What it does |
|---|---|---|
| `system.transform` | Every chat turn | Injects recent memories into system prompt |
| `memory_store` tool | Agent decides | Store a new memory (with optional key for upsert) |
| `memory_search` tool | Agent decides | Hybrid vector + keyword search (or keyword-only) |
| `memory_get` tool | Agent decides | Retrieve a single memory by ID |
| `memory_update` tool | Agent decides | Update an existing memory |
| `memory_delete` tool | Agent decides | Delete a memory by ID |

## Prerequisites

- [OpenCode](https://opencode.ai) installed
- A running [memorix-server](../server/) instance

## Installation

### Method A: npm plugin (Recommended)

The simplest way — OpenCode auto-installs npm plugins at startup.

Add to your `opencode.json`:

```json
{
  "plugin": ["memorix-opencode"]
}
```

That's it. OpenCode will install `memorix-opencode` from npm automatically on next startup.

### Method B: From source

```bash
git clone https://github.com/devioslang/memorix.git
cd memorix/opencode-plugin
npm install
```

Then register in `opencode.json`:

```json
{
  "plugins": {
    "memorix": {
      "path": "/absolute/path/to/memorix/opencode-plugin"
    }
  }
}
```

### Set environment variables

Connect to a self-hosted memorix-server. Tenant routing uses the tenant ID in the URL path.
All subsequent API calls go to `/v1alpha1/memorix/{tenantID}/memories/...` and require no headers.

```bash
export MNEMO_API_URL="http://your-server:8080"
export MNEMO_TENANT_ID="uuid"
```

### Verify

Start OpenCode in your project. You should see this log line:

```
[memorix] Server mode (memorix-server REST API)
```

If you see `[memorix] No mode configured...`, check your env vars.

## Environment Variables Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `MNEMO_API_URL` | Yes | — | memorix-server base URL |
| `MNEMO_TENANT_ID` | Yes (preferred) | — | Tenant ID for URL routing (`/v1alpha1/memorix/{tenantID}/memories/...`) |
| `MNEMO_API_TOKEN` | No (legacy fallback) | — | Legacy fallback if tenant ID is not set |

## File Structure

```
opencode-plugin/
├── README.md              # This file
├── package.json           # npm package config
├── tsconfig.json          # TypeScript config
└── src/
    ├── index.ts           # Plugin entry point (wiring)
    ├── types.ts           # Config loading, Memory types
    ├── backend.ts         # MemoryBackend interface
    ├── server-backend.ts  # Server mode: memorix-server REST API
    ├── tools.ts           # 5 memory tools (store/search/get/update/delete)
    └── hooks.ts           # system.transform hook (memory injection)
```

## Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `No mode configured` | Missing env vars | Set `MNEMO_API_URL` |
| `Server mode requires...` | Missing tenant ID or legacy token | Set `MNEMO_TENANT_ID` (preferred) or `MNEMO_API_TOKEN` |
| Plugin not loading | Not registered in OpenCode config | Add to `opencode.json` plugins section |
