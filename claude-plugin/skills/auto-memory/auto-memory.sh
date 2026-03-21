#!/usr/bin/env bash
# auto-memory.sh — Manage project-level auto memory (MEMORY.md)
# This script handles reading, writing, and managing the MEMORY.md file

set -euo pipefail

# Configuration
MEMORY_FILE="${MEMORY_FILE:-MEMORY.md}"
MAX_LINES="${MAX_LINES:-200}"
MAX_FILE_SIZE="${MAX_FILE_SIZE:-500}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Initialize MEMORY.md from template if it doesn't exist
init_memory_file() {
  if [[ ! -f "$MEMORY_FILE" ]]; then
    cat > "$MEMORY_FILE" << 'EOF'
# Project Auto Memory

Automatically accumulated project experiences. This file is Git-tracked and shared with the team.

---

## 📦 Build Commands

Commands used to build, test, and deploy the project.

### Build

### Test

### Deploy

---

## 🐛 Error Solutions

Common errors encountered and their solutions.

---

## 🏗️ Architecture Decisions

Important architectural choices made during development.

---

## ⚙️ User Preferences

Project-specific preferences and configurations.

---

## 📝 Notes

Additional notes and observations.

---

*Auto-maintained by AI agent. Last updated: $(date +%Y-%m-%d)*
EOF
    echo -e "${GREEN}Created MEMORY.md from template${NC}"
  fi
}

# Read first N lines of MEMORY.md (default 200)
read_memory() {
  local lines="${1:-$MAX_LINES}"
  if [[ -f "$MEMORY_FILE" ]]; then
    head -n "$lines" "$MEMORY_FILE"
  else
    echo "No MEMORY.md found. Run 'auto-memory init' to create one."
    exit 1
  fi
}

# Check if entry already exists (deduplication)
entry_exists() {
  local pattern="$1"
  grep -q "$pattern" "$MEMORY_FILE" 2>/dev/null
}

# Add a build command
add_build_command() {
  local command="$1"
  local description="$2"
  local category="${3:-Build}" # Build, Test, or Deploy
  
  init_memory_file
  
  # Check for duplicate
  if entry_exists "Command: \`$command\`"; then
    echo -e "${YELLOW}Command already exists: $command${NC}"
    return 0
  fi
  
  # Find the appropriate section
  local section="### $category"
  if ! grep -q "^$section" "$MEMORY_FILE"; then
    # Add section if it doesn't exist
    case "$category" in
      Build) sed -i "/## 📦 Build Commands/,/---/ { /^$/a ### Build\n }" "$MEMORY_FILE" ;;
      Test) sed -i "/## 📦 Build Commands/,/---/ { /^$/a ### Test\n }" "$MEMORY_FILE" ;;
      Deploy) sed -i "/## 📦 Build Commands/,/---/ { /^$/a ### Deploy\n }" "$MEMORY_FILE" ;;
    esac
  fi
  
  # Add command under appropriate subsection
  local entry="- Command: \`$command\` — $description"
  
  # Use awk to insert after the appropriate ### header
  awk -v section="$section" -v entry="$entry" '
    $0 == section { print; print entry; next }
    { print }
  ' "$MEMORY_FILE" > "${MEMORY_FILE}.tmp" && mv "${MEMORY_FILE}.tmp" "$MEMORY_FILE"
  
  echo -e "${GREEN}Added build command: $command${NC}"
  update_timestamp
}

# Add an error solution
add_error_solution() {
  local error_pattern="$1"
  local solution="$2"
  local context="${3:-}"
  
  init_memory_file
  
  # Check if error already exists
  if entry_exists "Error: \"$error_pattern\""; then
    echo -e "${YELLOW}Error already documented: $error_pattern${NC}"
    # Could append alternative solution here
    return 0
  fi
  
  # Add error solution under ## 🐛 Error Solutions
  local entry="- Error: \"$error_pattern\""
  entry+=$'\n'"  Solution: $solution"
  if [[ -n "$context" ]]; then
    entry+=$'\n'"  Context: $context"
  fi
  
  # Find Error Solutions section and append
  awk -v entry="$entry" '
    /^## 🐛 Error Solutions/ { in_section=1 }
    in_section && /^## / && !/^## 🐛 Error Solutions/ { print entry; print ""; in_section=0 }
    { print }
    END { if (in_section) print entry }
  ' "$MEMORY_FILE" > "${MEMORY_FILE}.tmp" && mv "${MEMORY_FILE}.tmp" "$MEMORY_FILE"
  
  echo -e "${GREEN}Added error solution for: $error_pattern${NC}"
  update_timestamp
}

# Add an architecture decision
add_architecture_decision() {
  local decision="$1"
  local rationale="$2"
  local date="${3:-$(date +%Y-%m-%d)}"
  
  init_memory_file
  
  # Check if similar decision exists
  if entry_exists "Decision: $decision"; then
    echo -e "${YELLOW}Decision already documented: $decision${NC}"
    return 0
  fi
  
  # Add decision under ## 🏗️ Architecture Decisions
  local entry="- Decision: $decision"
  entry+=$'\n'"  Rationale: $rationale"
  entry+=$'\n'"  Date: $date"
  
  awk -v entry="$entry" '
    /^## 🏗️ Architecture Decisions/ { in_section=1 }
    in_section && /^## / && !/^## 🏗️ Architecture Decisions/ { print entry; print ""; in_section=0 }
    { print }
    END { if (in_section) print entry }
  ' "$MEMORY_FILE" > "${MEMORY_FILE}.tmp" && mv "${MEMORY_FILE}.tmp" "$MEMORY_FILE"
  
  echo -e "${GREEN}Added architecture decision: $decision${NC}"
  update_timestamp
}

# Add a user preference
add_user_preference() {
  local preference="$1"
  local context="${2:-}"
  
  init_memory_file
  
  # Check for duplicate
  if entry_exists "Preference: $preference"; then
    echo -e "${YELLOW}Preference already documented: $preference${NC}"
    return 0
  fi
  
  # Add preference under ## ⚙️ User Preferences
  local entry="- Preference: $preference"
  if [[ -n "$context" ]]; then
    entry+=$'\n'"  Context: $context"
  fi
  
  awk -v entry="$entry" '
    /^## ⚙️ User Preferences/ { in_section=1 }
    in_section && /^## / && !/^## ⚙️ User Preferences/ { print entry; print ""; in_section=0 }
    { print }
    END { if (in_section) print entry }
  ' "$MEMORY_FILE" > "${MEMORY_FILE}.tmp" && mv "${MEMORY_FILE}.tmp" "$MEMORY_FILE"
  
  echo -e "${GREEN}Added user preference: $preference${NC}"
  update_timestamp
}

# Add a note
add_note() {
  local note="$1"
  
  init_memory_file
  
  # Add note under ## 📝 Notes
  local entry="- Note: $note"
  
  awk -v entry="$entry" '
    /^## 📝 Notes/ { in_section=1 }
    in_section && /^## / && !/^## 📝 Notes/ { print entry; print ""; in_section=0 }
    { print }
    END { if (in_section) print entry }
  ' "$MEMORY_FILE" > "${MEMORY_FILE}.tmp" && mv "${MEMORY_FILE}.tmp" "$MEMORY_FILE"
  
  echo -e "${GREEN}Added note: $note${NC}"
  update_timestamp
}

# Search memories
search_memory() {
  local query="$1"
  echo "Searching for: $query"
  echo "---"
  grep -i -A 2 -B 1 "$query" "$MEMORY_FILE" 2>/dev/null || echo "No results found."
}

# Show specific section
show_section() {
  local section="$1"
  case "$section" in
    build|commands)
      sed -n '/## 📦 Build Commands/,/^## /p' "$MEMORY_FILE" | head -n -1
      ;;
    errors)
      sed -n '/## 🐛 Error Solutions/,/^## /p' "$MEMORY_FILE" | head -n -1
      ;;
    decisions|architecture)
      sed -n '/## 🏗️ Architecture Decisions/,/^## /p' "$MEMORY_FILE" | head -n -1
      ;;
    preferences)
      sed -n '/## ⚙️ User Preferences/,/^## /p' "$MEMORY_FILE" | head -n -1
      ;;
    notes)
      sed -n '/## 📝 Notes/,/^## /p' "$MEMORY_FILE" | head -n -1
      ;;
    *)
      echo "Unknown section: $section"
      echo "Available: build, errors, decisions, preferences, notes"
      exit 1
      ;;
  esac
}

# Archive old memories
archive_memories() {
  local archive_file="${MEMORY_FILE%.md}.archive.md"
  
  if [[ -f "$MEMORY_FILE" ]]; then
    cp "$MEMORY_FILE" "$archive_file"
    echo -e "${GREEN}Archived current memories to $archive_file${NC}"
    init_memory_file
    echo -e "${GREEN}Started fresh MEMORY.md${NC}"
  fi
}

# Update timestamp at bottom of file
update_timestamp() {
  if grep -q "Last updated:" "$MEMORY_FILE"; then
    sed -i "s/Last updated: .*/Last updated: $(date +%Y-%m-%d)/" "$MEMORY_FILE"
  fi
}

# Check file size and warn if too large
check_file_size() {
  local lines
  lines=$(wc -l < "$MEMORY_FILE" 2>/dev/null || echo "0")
  
  if [[ "$lines" -gt "$MAX_FILE_SIZE" ]]; then
    echo -e "${YELLOW}Warning: MEMORY.md has $lines lines (max: $MAX_FILE_SIZE)${NC}"
    echo "Consider archiving old entries: auto-memory archive"
  fi
}

# Main CLI
main() {
  local command="${1:-help}"
  shift || true
  
  case "$command" in
    init)
      init_memory_file
      ;;
    read)
      read_memory "$@"
      ;;
    add-build)
      add_build_command "$@"
      ;;
    add-error)
      add_error_solution "$@"
      ;;
    add-decision)
      add_architecture_decision "$@"
      ;;
    add-preference)
      add_user_preference "$@"
      ;;
    add-note)
      add_note "$@"
      ;;
    search)
      search_memory "$@"
      ;;
    show)
      show_section "$@"
      ;;
    archive)
      archive_memories
      ;;
    check)
      check_file_size
      ;;
    help|--help|-h)
      cat << EOF
Auto Memory Manager - Manage project-level auto memory (MEMORY.md)

Usage: auto-memory <command> [arguments]

Commands:
  init                     Initialize MEMORY.md from template
  read [lines]             Read first N lines (default: $MAX_LINES)
  
  add-build <cmd> <desc> [category]
                           Add build command (category: Build|Test|Deploy)
  add-error <error> <solution> [context]
                           Add error solution
  add-decision <decision> <rationale> [date]
                           Add architecture decision
  add-preference <pref> [context]
                           Add user preference
  add-note <note>          Add a note
  
  search <query>           Search memories
  show <section>           Show specific section
                           (build, errors, decisions, preferences, notes)
  
  archive                  Archive current memories and start fresh
  check                    Check file size and warn if too large

Examples:
  auto-memory init
  auto-memory read 100
  auto-memory add-build "make build" "Builds Go server" Build
  auto-memory add-error "module not found" "Run go mod tidy" "When adding new import"
  auto-memory add-decision "Use chi router" "Lightweight, composable"
  auto-memory search "router"
  auto-memory show errors
EOF
      ;;
    *)
      echo "Unknown command: $command"
      echo "Run 'auto-memory help' for usage."
      exit 1
      ;;
  esac
}

# Run if called directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
