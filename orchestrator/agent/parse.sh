#!/bin/bash
# agent/parse.sh - Session result parsing
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

declare -gA SESSION_RESULT=()

parse_session_result() {
    local epic="$1"
    local task_id="$2"
    local attempt="$3"

    local session_file="$LOGS_DIR/sessions/$epic/session-${task_id}_attempt-${attempt}.md"

    if [[ ! -f "$session_file" ]]; then
        log_error "Session result not found: $session_file"
        return 1
    fi

    # Extract YAML frontmatter (between first and second ---)
    local frontmatter
    frontmatter=$(awk '
        BEGIN { in_fm=0 }
        /^---[[:space:]]*$/ {
            if (in_fm) exit
            in_fm=1
            next
        }
        in_fm { print }
    ' "$session_file")

    if [[ -z "$frontmatter" ]]; then
        log_error "No frontmatter found in session result"
        return 1
    fi

    # Parse fields into SESSION_RESULT associative array
    SESSION_RESULT[outcome]=$(yq -r '.outcome // ""' <<< "$frontmatter")
    SESSION_RESULT[changelog_entry]=$(yq -r '.changelog_entry // ""' <<< "$frontmatter")
    SESSION_RESULT[dependencies_added]=$(yq -r '.dependencies_added // ""' <<< "$frontmatter")

    if [[ -z "${SESSION_RESULT[outcome]}" ]]; then
        log_error "Frontmatter parsed but outcome missing"
        return 1
    fi

    # Validate outcome
    case "${SESSION_RESULT[outcome]}" in
        SUCCESS|BUG|FAILURE|EPIC_COMPLETE)
            log_info "Valid outcome: ${SESSION_RESULT[outcome]}"
            ;;
        *)
            log_error "Invalid outcome: ${SESSION_RESULT[outcome]}"
            return 1
            ;;
    esac

    return 0
}
