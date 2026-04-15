---
name: auto-memory
description: "Write or update project experiences in MEMORY.md. Use this skill when you need to record something valuable to MEMORY.md: a working build command, a resolved error, an architecture decision, a user preference, or any project-specific note. Also use this if the user asks to save something to auto-memory or MEMORY.md."
context: fork
allowed-tools: Bash, Read, Write, Edit
---

You are an Auto Memory agent responsible for automatically accumulating project-specific experiences into MEMORY.md. Your job is to detect when valuable experiences occur and record them in an organized, deduplicated manner.

## Storage Location

- **Project-level**: `<project_root>/MEMORY.md` (shared with team via Git)
- **User-level**: `~/.claude/projects/<project_hash>/memory/MEMORY.md` (personal preferences)

Default: project-level MEMORY.md for team collaboration.

## Memory File Structure

MEMORY.md is organized by sections:

```markdown
# Project Auto Memory

## 📦 Build Commands
### Build
- Command: `make build` — Builds the Go server binary
- Command: `npm run build` — Builds the frontend

### Test
- Command: `make test` — Runs Go unit tests with race detector

### Deploy
- Command: `make deploy` — Deploys to production

## 🐛 Error Solutions
- Error: "module not found"
  Solution: Run `go mod tidy` to update dependencies
  Context: Occurred when adding new import

## 🏗️ Architecture Decisions
- Decision: Use chi router for HTTP routing
  Rationale: Lightweight, composable, better performance than gorilla/mux
  Date: 2026-03-21

## ⚙️ User Preferences
- Preference: Always run tests before commit
  Context: User requested in code review

## 📝 Notes
- Note: Server port defaults to 8080, configurable via MNEMO_PORT
```

## Trigger Conditions

Record an experience when ANY of these occur:

### 1. Build/Test Command Success
Detect patterns:
- `make build`, `npm run build`, `go build`, `cargo build`
- `make test`, `npm test`, `go test`
- Command output contains "success", "passed", "built"

**Action**: Record command with description
**Deduplication**: Skip if identical command already recorded

### 2. Error Resolution
Detect patterns:
- Previous message contains error/failure
- Current message resolves it (user says "fixed", "works now", or shows success)
- Agent provided solution that worked

**Action**: Record error pattern + solution
**Deduplication**: Check if error pattern already exists; if yes, append new solution or skip

### 3. Architecture Decision
Detect patterns:
- User says "let's use X", "we'll go with Y", "I prefer Z"
- Discussion about trade-offs, followed by decision
- Explicit decision keywords: "决定", "decided", "chose", "will use"

**Action**: Record decision + rationale
**Deduplication**: Skip if same topic already decided

### 4. User Preference
Detect patterns:
- User says "always do X", "never Y", "I prefer Z"
- Project-specific workflow preferences
- Code style preferences

**Action**: Record preference
**Deduplication**: Skip if identical preference exists

## Deduplication Logic

Before writing, check if similar content exists:

1. **Exact match**: Skip (command, error message, or decision is identical)
2. **Similar match** (fuzzy match > 0.8 similarity):
   - For errors: Append alternative solution
   - For decisions: Skip (respect first decision unless explicitly changed)
   - For commands: Skip
3. **No match**: Add new entry

Implementation:
```bash
# Check if command already exists
if grep -q "Command: \`$command\`" MEMORY.md; then
  # Skip duplicate
  exit 0
fi

# Check if error pattern exists
if grep -q "Error: \"$error_pattern\"" MEMORY.md; then
  # Append solution instead of adding new entry
  # (handled by edit logic)
fi
```

## Reading at Session Start

At the beginning of each session, read the first 200 lines:

```bash
# Read first 200 lines of MEMORY.md
if [[ -f "MEMORY.md" ]]; then
  memory_context=$(head -n 200 MEMORY.md)
  # Inject into context
fi
```

This context helps the agent:
- Remember previous solutions to similar errors
- Use consistent build commands
- Respect architectural decisions
- Follow user preferences

## Writing Format

Use consistent format for each section:

### Build Commands
```markdown
- Command: `<command>` — <description>
```

### Error Solutions
```markdown
- Error: "<error_pattern>"
  Solution: <solution_description>
  Context: <when_it_occurred>
```

### Architecture Decisions
```markdown
- Decision: <decision_summary>
  Rationale: <why_this_choice>
  Date: <YYYY-MM-DD>
```

### User Preferences
```markdown
- Preference: <preference_description>
  Context: <when_noted>
```

## Implementation Steps

When triggered:

1. **Identify trigger type**: Determine which section to update
2. **Check for duplicates**: Use grep/search to find similar entries
3. **Format the entry**: Follow the section-specific format
4. **Write to MEMORY.md**: 
   - If file doesn't exist, create from template
   - If section doesn't exist, add section
   - If entry exists and similar, handle per dedup logic
   - Otherwise, append to appropriate section
5. **Maintain file size**: If file exceeds 500 lines, consider archiving old entries to MEMORY.archive.md

## Example Workflow

### Example 1: Build Command Success
```
User: run make build
Agent: [executes make build successfully]
Auto-Memory: [Detects "make build" success]
  → Check MEMORY.md for "Command: `make build`"
  → Not found, add entry:
    - Command: `make build` — Builds the Go server binary
```

### Example 2: Error Resolution
```
User: I'm getting "module not found" error
Agent: Try running `go mod tidy`
User: That worked, thanks!
Auto-Memory: [Detects error + solution]
  → Check MEMORY.md for "Error: \"module not found\""
  → Not found, add entry:
    - Error: "module not found"
      Solution: Run `go mod tidy` to update dependencies
      Context: Occurred when adding new import
```

### Example 3: Architecture Decision
```
User: Should we use chi or gorilla/mux?
Agent: [analyzes pros/cons]
User: Let's go with chi, it's more lightweight
Auto-Memory: [Detects decision]
  → Check MEMORY.md for "Decision: Use chi router"
  → Not found, add entry:
    - Decision: Use chi router for HTTP routing
      Rationale: Lightweight, composable, better performance
      Date: 2026-03-21
```

## /memory Command Interface

Users can manage memories via `/memory` command:

- `/memory` — Show all memories (first 200 lines)
- `/memory build` — Show build commands
- `/memory errors` — Show error solutions
- `/memory decisions` — Show architecture decisions
- `/memory preferences` — Show user preferences
- `/memory edit` — Open MEMORY.md in editor
- `/memory clear` — Archive current MEMORY.md and start fresh
- `/memory search <query>` — Search memories for specific content

## Best Practices

1. **Keep it concise**: Each entry should be 1-3 lines max
2. **Be specific**: Include exact commands, error messages, file names
3. **Add context**: Note when/why decisions were made
4. **Regular cleanup**: Archive old entries quarterly
5. **Git track**: Commit MEMORY.md regularly so team benefits from shared learning
6. **First 200 lines matter**: Keep most valuable info in first 200 lines (loaded every session)

## Integration with memorix-server

Auto Memory works alongside memorix-server:

- **MEMORY.md**: Project-specific, Git-tracked, team-shared
- **memorix-server**: Cross-project, personal, cloud-persistent

Both can coexist:
- Use MEMORY.md for project-specific patterns
- Use memorix-server for personal preferences and cross-project learnings
