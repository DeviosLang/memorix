#!/usr/bin/env bash
# auto-memory-stop.sh — Auto Memory trigger on session stop
# Analyzes the conversation and updates MEMORY.md if triggers detected
# Hook: Stop (async, timeout: 30s)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")/skills/auto-memory"

source "${SCRIPT_DIR}/common.sh"

read_stdin

# Check if auto-memory is enabled (default: yes)
if [[ "${AUTO_MEMORY_ENABLED:-yes}" != "yes" ]]; then
  exit 0
fi

# Check if detect-triggers.py exists
if [[ ! -f "${SKILL_DIR}/detect-triggers.py" ]]; then
  echo "Warning: detect-triggers.py not found" >&2
  exit 0
fi

# Extract transcript from hook input
transcript=$(echo "$HOOK_INPUT" | python3 -c "
import json, sys
data = json.load(sys.stdin)
transcript = data.get('transcript', [])
# Only process last 10 messages to avoid processing too much
recent = transcript[-10:] if len(transcript) > 10 else transcript
print(json.dumps({'messages': recent}))
" 2>/dev/null)

if [[ -z "$transcript" ]]; then
  exit 0
fi

# Detect triggers
triggers_json=$(echo "$transcript" | python3 "${SKILL_DIR}/detect-triggers.py" 2>/dev/null)

if [[ -z "$triggers_json" ]]; then
  exit 0
fi

# Parse triggers and update MEMORY.md
echo "$triggers_json" | python3 -c "
import json
import sys
import subprocess
import os

data = json.load(sys.stdin)
triggers = data.get('triggers', [])

if not triggers:
    sys.exit(0)

# Get the auto-memory script path
skill_dir = os.environ.get('SKILL_DIR', '')
auto_memory_script = os.path.join(skill_dir, 'auto-memory.sh')

if not os.path.exists(auto_memory_script):
    print(f'Warning: {auto_memory_script} not found', file=sys.stderr)
    sys.exit(1)

# Process each trigger
for trigger in triggers:
    trigger_type = trigger.get('type', '')
    trigger_data = trigger.get('data', {})
    
    try:
        if trigger_type == 'build_command':
            cmd = trigger_data.get('command', '')
            category = trigger_data.get('category', 'Build')
            desc = trigger_data.get('description', '')
            subprocess.run([auto_memory_script, 'add-build', cmd, desc, category], 
                          check=False, capture_output=True)
        
        elif trigger_type == 'error_solution':
            error = trigger_data.get('error', '')
            solution = trigger_data.get('solution', 'See conversation')
            context = trigger_data.get('context', '')
            subprocess.run([auto_memory_script, 'add-error', error, solution, context],
                          check=False, capture_output=True)
        
        elif trigger_type == 'decision':
            decision = trigger_data.get('decision', '')
            rationale = trigger_data.get('rationale', '')
            date = trigger_data.get('date', '')
            subprocess.run([auto_memory_script, 'add-decision', decision, rationale, date],
                          check=False, capture_output=True)
        
        elif trigger_type == 'preference':
            preference = trigger_data.get('preference', '')
            context = trigger_data.get('context', '')
            subprocess.run([auto_memory_script, 'add-preference', preference, context],
                          check=False, capture_output=True)
    
    except Exception as e:
        print(f'Error processing trigger: {e}', file=sys.stderr)

# Report number of triggers processed
print(f'[auto-memory] Processed {len(triggers)} trigger(s)')
" 2>&1 || true
