---
name: memorix-setup
description: "Setup memorix persistent memory with memorix-server. Triggers: set up memorix, install memorix, configure memory."
context: fork
allowed-tools: Bash
---

# memorix Setup for Claude Code

**Persistent memory for Claude Code.** This skill helps you set up memorix with a memorix-server instance.

## Prerequisites

You need a running memorix-server instance. See [server README](https://github.com/devioslang/memorix/tree/main/server) for deployment instructions.

## Setup Steps

### Step 1: Provision a tenant

```bash
# Deploy server (requires a TiDB/MySQL database)
cd server && MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true" go run ./cmd/memorix-server

# Provision a tenant (no auth required)
curl -s -X POST http://localhost:8080/v1alpha1/memorix | jq .
# → { "id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", "claim_url": "..." }
```

Save the returned `id` — this is your `MNEMO_TENANT_ID`.

### Step 2: Configure credentials

Add to `~/.claude/settings.json`:

```json
{
  "env": {
    "MNEMO_API_URL": "http://your-server:8080",
    "MNEMO_TENANT_ID": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  }
}
```

### Step 3: Install plugin

Tell the user to run in Claude Code:
```
/plugin marketplace add devioslang/memorix
/plugin install memorix-memory@memorix
```

### Step 4: Install companion skills

After the plugin is installed, tell the user to also install the following skills to unlock the full memorix experience. Each skill serves a distinct purpose:

| Skill | Install command | What it does |
|-------|----------------|--------------|
| memory-recall | `/skill install memory-recall` | Search past memories on demand |
| memory-store | `/skill install memory-store` | Save important info on demand |
| auto-memory | `/skill install auto-memory` | Auto-record project experiences to MEMORY.md |
| memory (manage) | `/skill install memory-manage` | `/memory` command to view/search MEMORY.md |

All four skills are available from the same marketplace entry:
```
/plugin marketplace add devioslang/memorix
```

Or if installing from `.skill` files directly:
```
/skill install memory-recall.skill
/skill install memory-store.skill
/skill install auto-memory.skill
/skill install memory-manage.skill
```

### Step 5: Restart Claude Code

Tell the user to restart Claude Code to activate the plugin and all skills.

## Verification

After setup, suggest testing:
1. "Remember that this project uses React 18"
2. Start a new session
3. "What UI framework does this project use?"

The agent should recall from memory.

## Summary checklist

Present this checklist to the user at the end of setup:

- [ ] memorix-server is running and accessible
- [ ] Tenant ID provisioned and saved
- [ ] `MNEMO_API_URL` and `MNEMO_TENANT_ID` set in `~/.claude/settings.json`
- [ ] Plugin installed: `memorix-memory`
- [ ] Skill installed: `memory-recall`
- [ ] Skill installed: `memory-store`
- [ ] Skill installed: `auto-memory`
- [ ] Skill installed: `memory-manage`
- [ ] Claude Code restarted
