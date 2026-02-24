#!/bin/bash
# handlers/bug.sh - Bug outcome handler
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

# Note: core/git.sh and core/state.sh are sourced by orchestrator.sh
# We can call their functions directly

generate_bug_id() {
    local original_task="$1"
    echo "BUG-${original_task}"
}

archive_bug_report() {
    local epic="$1"
    local task_id="$2"

    if [[ ! -f "$LOGS_DIR/ACTIVE_BUG.md" ]]; then
        log_warning "No active bug report to archive"
        return 0
    fi

    local archive_dir="$LOGS_DIR/bugs/$epic"
    mkdir -p "$archive_dir"

    local archive_file="$archive_dir/bug-${task_id}.md"
    mv "$LOGS_DIR/ACTIVE_BUG.md" "$archive_file"

    log_success "Bug report archived: $archive_file"
}

handle_bug() {
    local task_id="$1"
    local epic="$2"
    local task_type="$3"

    # Prevent nested bugs
    if [[ "$task_type" == "bugfix" ]]; then
        log_error "Bug discovered during bugfix task. This indicates a critical issue."
        log_error "ACTIVE_BUG.md contains details of the nested bug."

        fatal "Nested bug detected. Manual review required."
    fi

    log_warning "Blocking bug discovered during: $task_id"

    # Rollback any uncommitted changes from the failed task
    rollback_changes

    # Record metrics for the bug attempt
    record_task_metrics "$task_id" "$TASK_DURATION" "bug"

    # Generate bug ID
    local bug_id=$(generate_bug_id "$task_id")

    # Leave the original task as IN_PROGRESS (it's interrupted, not complete)
    # Don't change its status

    # Set active_task to bugfix
    yq eval -i ".active_task.type = \"bugfix\"" "$PROJECT_STATE"
    yq eval -i ".active_task.id = \"$bug_id\"" "$PROJECT_STATE"
    yq eval -i ".active_task.attempts = 0" "$PROJECT_STATE"

    # Set next_task to the interrupted task (will resume after bugfix)
    # Synthetic tasks (bugfix, documentation) are not in tasks.yaml — use the
    # in-memory task_type value directly to avoid an empty string.
    local interrupted_type
    if [[ "$task_type" == "bugfix" || "$task_type" == "documentation" ]]; then
        interrupted_type="$task_type"
    else
        interrupted_type=$(yq eval ".epic.tasks[] | select(.id == \"$task_id\") | .type" "$TASKS_FILE")
    fi
    yq eval -i ".next_task.type = \"$interrupted_type\"" "$PROJECT_STATE"
    yq eval -i ".next_task.id = \"$task_id\"" "$PROJECT_STATE"

    log_info "Next task set to bugfix: $bug_id"
    log_info "Will resume task: $task_id after bugfix complete"

    return 0
}
