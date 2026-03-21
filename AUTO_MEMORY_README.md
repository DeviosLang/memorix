# Auto Memory Implementation for Issue #12

## Overview

This implementation adds **Auto Memory 自动积累机制** to memorix, enabling AI agents to automatically accumulate project-specific experiences into a `MEMORY.md` file.

## Components

### 1. Storage Format (MEMORY.md)

A Markdown file organized by sections:

- **📦 Build Commands**: Build, test, and deploy commands
- **🐛 Error Solutions**: Common errors and their solutions
- **🏗️ Architecture Decisions**: Architectural choices and rationale
- **⚙️ User Preferences**: Project-specific preferences
- **📝 Notes**: Miscellaneous observations

### 2. Trigger Detection

Located in `claude-plugin/skills/auto-memory/detect-triggers.py`

Detects four types of triggers:

1. **Build Command Success**: Detects `make build`, `npm test`, etc. with success indicators
2. **Error Resolution**: Detects error patterns followed by solutions
3. **Architecture Decisions**: Detects decision statements with rationale
4. **User Preferences**: Detects preference statements

### 3. Auto Memory Manager

Located in `claude-plugin/skills/auto-memory/auto-memory.sh`

Provides CLI for managing MEMORY.md:

```bash
# Initialize MEMORY.md
auto-memory init

# Read memories
auto-memory read [lines]

# Add entries
auto-memory add-build <cmd> <desc> [category]
auto-memory add-error <error> <solution> [context]
auto-memory add-decision <decision> <rationale> [date]
auto-memory add-preference <pref> [context]

# Search and view
auto-memory search <query>
auto-memory show <section>

# Maintenance
auto-memory archive
auto-memory check
```

### 4. Hooks

Located in `claude-plugin/hooks/`

#### auto-memory-start.sh
- **Hook**: SessionStart
- **Action**: Reads first 200 lines of MEMORY.md and injects as context
- **Priority**: Loads project-level or user-level memory

#### auto-memory-stop.sh
- **Hook**: Stop (async)
- **Action**: Analyzes last 10 messages, detects triggers, updates MEMORY.md
- **Timeout**: 30 seconds

### 5. /memory Command

Located in `claude-plugin/skills/memory-manage/SKILL.md`

User-facing commands:

- `/memory` — Show all memories (first 200 lines)
- `/memory build` — Show build commands
- `/memory errors` — Show error solutions
- `/memory decisions` — Show architecture decisions
- `/memory preferences` — Show user preferences
- `/memory search <query>` — Search memories
- `/memory edit` — Open MEMORY.md in editor
- `/memory clear` — Archive and start fresh

## Features

### ✅ Deduplication

- Exact match: Skips duplicate entries
- Similar match: Appends alternative solutions for errors
- Prevents memory bloat

### ✅ Automatic Accumulation

- No manual intervention required
- Detects valuable experiences automatically
- Records context alongside facts

### ✅ Team Collaboration

- MEMORY.md is Git-trackable
- Team members share accumulated knowledge
- Version-controlled learning

### ✅ Session Integration

- First 200 lines loaded at session start
- Provides instant context for the agent
- Helps avoid repeating past mistakes

## Acceptance Criteria Met

✅ **Agent resolves a build error → MEMORY.md automatically records it**
- The `auto-memory-stop.sh` hook detects error resolution and adds entry

✅ **New session with similar problem → Agent references previous solution**
- `auto-memory-start.sh` loads MEMORY.md into context
- Agent can see past solutions in the first 200 lines

✅ **MEMORY.md is readable, editable, and Git-trackable**
- Plain Markdown format
- Human-readable structure
- Can be manually edited or committed to Git

## Example MEMORY.md

```markdown
# Project Auto Memory

Automatically accumulated project experiences.

---

## 📦 Build Commands

### Build
- Command: `make build` — Builds the Go server binary
- Command: `go build ./cmd/memorix-server` — Direct Go build

### Test
- Command: `make test` — Runs Go unit tests with race detector
- Command: `go test ./...` — Run all tests

---

## 🐛 Error Solutions

- Error: "module not found"
  Solution: Run `go mod tidy` to update dependencies
  Context: Occurred when adding new import

- Error: "permission denied"
  Solution: Run with sudo or fix file permissions
  Context: When accessing /var/log

---

## 🏗️ Architecture Decisions

- Decision: Use chi router for HTTP routing
  Rationale: Lightweight, composable, better performance than gorilla/mux
  Date: 2026-03-21

- Decision: Store memories in TiDB
  Rationale: Native vector support, MySQL compatible, managed service available
  Date: 2026-03-15

---

## ⚙️ User Preferences

- Preference: Always run tests before commit
  Context: Requested in code review

- Preference: Use conventional commits
  Context: Team standard

---

## 📝 Notes

- Note: Server port defaults to 8080, configurable via MNEMO_PORT
- Note: TiDB Cloud free tier includes 25 GiB storage

---

*Auto-maintained by AI agent. Last updated: 2026-03-21*
```

## Testing the Implementation

### Manual Test

1. Initialize a MEMORY.md:
```bash
cd /root/.worktrees/memorix/mem-24
./claude-plugin/skills/auto-memory/auto-memory.sh init
```

2. Add a build command:
```bash
./claude-plugin/skills/auto-memory/auto-memory.sh add-build "make build" "Builds Go server" Build
```

3. Add an error solution:
```bash
./claude-plugin/skills/auto-memory/auto-memory.sh add-error "module not found" "Run go mod tidy" "When adding imports"
```

4. View the memory:
```bash
./claude-plugin/skills/auto-memory/auto-memory.sh read
```

### Integration Test

The hooks will automatically trigger in Claude Code:
1. Session starts → `auto-memory-start.sh` loads MEMORY.md
2. Agent works and resolves issues → Triggers detected
3. Session ends → `auto-memory-stop.sh` updates MEMORY.md

## Configuration

### Environment Variables

- `AUTO_MEMORY_ENABLED`: Enable/disable auto-memory (default: `yes`)
- `MEMORY_FILE`: Custom MEMORY.md path (default: `MEMORY.md`)
- `MAX_LINES`: Lines to read at session start (default: `200`)
- `MAX_FILE_SIZE`: Max lines before warning (default: `500`)

### Disable Auto Memory

To disable automatic accumulation:

```bash
export AUTO_MEMORY_ENABLED=no
```

Or in `~/.claude/settings.json`:
```json
{
  "env": {
    "AUTO_MEMORY_ENABLED": "no"
  }
}
```

## Integration with memorix-server

Auto Memory and memorix-server complement each other:

| Feature | MEMORY.md | memorix-server |
|---------|-----------|----------------|
| **Scope** | Project-specific | Cross-project |
| **Storage** | Git-tracked file | Cloud database |
| **Sharing** | Team via Git | Personal/team via API |
| **Content** | Build commands, errors, decisions | Personal facts, cross-project patterns |
| **Access** | File read/write | REST API |

**Recommended workflow**:
1. Use **MEMORY.md** for project-specific learnings (shared with team)
2. Use **memorix-server** for personal preferences (cross-project)
3. Both systems work together seamlessly

## Future Enhancements

Potential improvements for future iterations:

1. **LLM-powered summarization**: Compress old memories periodically
2. **Semantic search**: Use embeddings to find relevant memories
3. **Memory expiration**: Auto-archive entries older than N days
4. **Conflict resolution**: Handle contradictory memories
5. **Memory importance scoring**: Prioritize most valuable entries

## Files Modified/Created

### Created
- `claude-plugin/skills/auto-memory/SKILL.md` — Skill documentation
- `claude-plugin/skills/auto-memory/auto-memory.sh` — Memory manager CLI
- `claude-plugin/skills/auto-memory/detect-triggers.py` — Trigger detection
- `claude-plugin/hooks/auto-memory-start.sh` — Session start hook
- `claude-plugin/hooks/auto-memory-stop.sh` — Session stop hook
- `claude-plugin/skills/memory-manage/SKILL.md` — /memory command skill
- `AUTO_MEMORY_TEMPLATE.md` — Template for MEMORY.md

### Modified
- `claude-plugin/hooks/hooks.json` — Added auto-memory hooks

## Verification

Run these commands to verify the implementation:

```bash
# Check files exist
ls -la claude-plugin/skills/auto-memory/
ls -la claude-plugin/hooks/auto-memory-*.sh
ls -la claude-plugin/skills/memory-manage/

# Test auto-memory CLI
./claude-plugin/skills/auto-memory/auto-memory.sh help
./claude-plugin/skills/auto-memory/auto-memory.sh init
./claude-plugin/skills/auto-memory/auto-memory.sh read

# Test trigger detection
echo '{"messages": [{"role": "user", "content": "run make build"}]}' | \
  python3 claude-plugin/skills/auto-memory/detect-triggers.py

# Verify hooks are registered
cat claude-plugin/hooks/hooks.json | grep -A2 auto-memory
```

## Conclusion

This implementation fully satisfies Issue #12 requirements:
- ✅ Auto Memory storage format (Markdown, topic-partitioned)
- ✅ Write trigger detection (build success, error resolution, decisions, preferences)
- ✅ Deduplication logic
- ✅ Session start reads first 200 lines
- ✅ /memory command interface

The system is ready for production use and provides a solid foundation for automatic knowledge accumulation in AI agents.
