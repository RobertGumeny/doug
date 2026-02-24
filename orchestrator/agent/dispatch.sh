#!/bin/bash
# agent/dispatch.sh - Claude agent invocation
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

create_session_file() {
    local epic="$1"
    local task_id="$2"
    local attempt="$3"

    local session_file="$LOGS_DIR/sessions/$epic/session-${task_id}_attempt-${attempt}.md"
    local template="$LOGS_DIR/sessions/SESSION_RESULTS_TEMPLATE.md"

    if [[ ! -f "$template" ]]; then
        fatal "Session template not found: $template"
    fi

    cp "$template" "$session_file"
    sed -i "s/^task_id: \"\"/task_id: \"${task_id}\"/" "$session_file"

    log_info "Created session file: $session_file"
    echo "$session_file"
}

run_agent() {
    local task_type="$1"
    local task_id="$2"
    local attempts="${3:-1}"  # Optional: current attempt number
    local session_file="$4"  # Pre-created session file path

    log_info "Dispatching agent for: $task_type / $task_id (attempt $attempts)"

    # Get skill name from config (with fallback to defaults)
    local skill_name=$(get_skill_for_task_type "$task_type")

    # Handle special cases
    if [[ "$skill_name" == "manual-review" ]]; then
        log_warning "Manual review required. Human intervention needed."
        exit 0
    fi

    if [[ -z "$skill_name" ]]; then
        fatal "Unknown task type: $task_type (no mapping in skills-config.yaml)"
    fi

    # Construct agent prompt based on task type
    local agent_prompt=""
    local additional_context=""

    # Add context for bugfix tasks
    if [[ "$task_type" == "bugfix" ]]; then
        additional_context=" Follow all instructions in CLAUDE.md and information in ACTIVE_BUG.md."
    else
        additional_context=" Follow all instructions in CLAUDE.md."
    fi

    # Include session file path so agent never needs to compute it
    local session_context=""
    if [[ -n "$session_file" ]]; then
        session_context=" Your session summary is pre-created at: ${session_file}"
    fi

    agent_prompt="Activate the ${skill_name} skill for task ${task_id}.${additional_context}${session_context}"

    # Record start time for metrics
    export TASK_START_TIME=$(date +%s)

    # Dispatch Claude from PROJECT_ROOT to ensure correct settings are loaded
    (cd "$PROJECT_ROOT" && claude -p "$agent_prompt") 2>&1
}

get_skill_for_task_type() {
    local task_type="$1"
    local config_file="$PROJECT_ROOT/.claude/skills-config.yaml"

    # Try to read from config file if it exists
    if [[ -f "$config_file" ]]; then
        local skill_name=$(yq eval ".skill_mappings.${task_type} // \"\"" "$config_file" 2>/dev/null)

        # If found in config, return it
        if [[ -n "$skill_name" ]]; then
            echo "$skill_name"
            return 0
        fi
    fi

    # Fallback to hardcoded defaults for backward compatibility
    case "$task_type" in
        feature)
            echo "implement-feature"
            ;;
        bugfix)
            echo "implement-bugfix"
            ;;
        documentation)
            echo "implement-documentation"
            ;;
        manual_review)
            echo "manual-review"
            ;;
        *)
            # Return empty string for unknown types
            echo ""
            ;;
    esac
}
