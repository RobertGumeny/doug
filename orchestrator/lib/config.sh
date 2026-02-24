#!/bin/bash
# lib/config.sh - Global configuration and constants

# Detect project root
# The orchestrator framework lives in root/orchestrator/, so we need to find the project root
if [[ -f "project-state.yaml" ]]; then
    # Already in project root (invoked as: cd /myproject && ./agent_loop)
    PROJECT_ROOT="$(pwd)"
elif [[ -f "../project-state.yaml" ]]; then
    # Invoked from orchestrator subdirectory (invoked as: cd /myproject/orchestrator && ./some_script.sh)
    PROJECT_ROOT="$(cd .. && pwd)"
else
    echo "ERROR: Cannot find project-state.yaml. Orchestrator must be run from project root or orchestrator subdirectory."
    exit 1
fi

# File paths (absolute, based on project root)
PROJECT_STATE="$PROJECT_ROOT/project-state.yaml"
TASKS_FILE="$PROJECT_ROOT/tasks.yaml"
LOGS_DIR="$PROJECT_ROOT/logs"
CHANGELOG_FILE="$PROJECT_ROOT/CHANGELOG.md"

# Limits
MAX_RETRIES=5
# NOTE: MAX_ITERATIONS is set by orchestrator.sh from command-line args

# Protected paths (survive git rollback)
PROTECTED_PATHS=(
    "logs/"
    "docs/kb/"
    ".env"
    "*.backup"
    "project-state.yaml"
    "tasks.yaml"
)

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'
