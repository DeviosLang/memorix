#!/usr/bin/env bash
# auto-memory-start.sh — Load AUTO MEMORY at session start
# Reads first 200 lines of MEMORY.md and injects as additionalContext
# Hook: SessionStart (sync, timeout: 10s)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")/skills/auto-memory"

# Check if auto-memory is enabled (default: yes)
if [[ "${AUTO_MEMORY_ENABLED:-yes}" != "yes" ]]; then
  exit 0
fi

# Determine MEMORY.md location
# Priority: project-level > user-level
MEMORY_FILE=""

# Check for project-level MEMORY.md
if [[ -f "MEMORY.md" ]]; then
  MEMORY_FILE="MEMORY.md"
elif [[ -n "${CLAUDE_PROJECT_DIR:-}" && -f "${CLAUDE_PROJECT_DIR}/MEMORY.md" ]]; then
  MEMORY_FILE="${CLAUDE_PROJECT_DIR}/MEMORY.md"
# Check for user-level memory
elif [[ -f "${HOME}/.claude/projects/memory/MEMORY.md" ]]; then
  MEMORY_FILE="${HOME}/.claude/projects/memory/MEMORY.md"
fi

# If no MEMORY.md found, exit silently
if [[ -z "$MEMORY_FILE" ]]; then
  exit 0
fi

# Read first 200 lines
memory_content=$(head -n 200 "$MEMORY_FILE" 2>/dev/null)

if [[ -z "$memory_content" ]]; then
  exit 0
fi

# Format as context
context=$(cat << EOF
[auto-memory] Project experiences loaded from $MEMORY_FILE:

$memory_content
EOF
)

# Return additionalContext to inject into Claude's context
MNEMO_CONTEXT="$context" python3 -c "
import json, os
output = {
    'hookSpecificOutput': {
        'hookEventName': 'SessionStart',
        'additionalContext': os.environ['MNEMO_CONTEXT']
    }
}
print(json.dumps(output))
"
