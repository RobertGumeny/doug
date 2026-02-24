#!/bin/bash
# handlers/failure.sh - Failure outcome handler
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

# Note: core/git.sh and core/state.sh are sourced by orchestrator.sh
# We can call their functions directly

archive_failure_report() {
    local epic="$1"
    local task_id="$2"

    if [[ ! -f "$LOGS_DIR/ACTIVE_FAILURE.md" ]]; then
        log_warning "No active failure report to archive"
        return 0
    fi

    local archive_dir="$LOGS_DIR/failures/$epic"
    mkdir -p "$archive_dir"

    local archive_file="$archive_dir/failure-${task_id}.md"
    mv "$LOGS_DIR/ACTIVE_FAILURE.md" "$archive_file"

    log_success "Failure report archived: $archive_file"
}

handle_failure() {
    local task_id="$1"
    local task_type="$2"
    local epic="$3"

    log_error "Task failed: $task_id"

    # Rollback changes
    rollback_changes

    # Record metrics for the failed attempt
    record_task_metrics "$task_id" "$TASK_DURATION" "failure"

    local new_attempts=$(yq eval '.active_task.attempts' "$PROJECT_STATE")

    if [[ $new_attempts -ge $MAX_RETRIES ]]; then
        log_error "Max retries ($MAX_RETRIES) reached for task: $task_id"

        # Archive failure report
        archive_failure_report "$epic" "$task_id"

        # Mark task as blocked (only for tasks in tasks.yaml)
        if [[ "$task_type" != "bugfix" && "$task_type" != "documentation" ]]; then
            update_task_status "$task_id" "BLOCKED"
        fi

        # Set active_task to manual_review
        yq eval -i '.active_task.type = "manual_review"' "$PROJECT_STATE"
        yq eval -i '.active_task.id = "'"$task_id"'"' "$PROJECT_STATE"

        fatal "Task blocked after $MAX_RETRIES attempts. Manual review required."
    else
        log_warning "Retry $new_attempts / $MAX_RETRIES for task: $task_id"
    fi

    return 1
}
