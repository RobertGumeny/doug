#!/bin/bash
# handlers/success.sh - Success outcome handler
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

# Note: All core modules are sourced by orchestrator.sh
# We can call their functions directly

handle_success() {
    local task_id="$1"
    local task_type="$2"
    local epic="$3"

    log_success "Task completed: $task_id"

    # If dependencies were added, install them
    if [[ -n "${SESSION_RESULT[dependencies_added]}" ]]; then
        log_info "New dependencies detected: ${SESSION_RESULT[dependencies_added]}"
        install_dependencies || {
            log_error "Failed to install new dependencies"
            return 1
        }
    fi

    # Verify build and tests
    verify_build || {
        log_error "Build verification failed after task completion"
        rollback_changes
        return 1
    }

    verify_tests || {
        log_error "Test verification failed after task completion"
        rollback_changes
        return 1
    }

    # ⭐ Record metrics AFTER verification passes
    record_task_metrics "$task_id" "$TASK_DURATION" "success"

    # Update CHANGELOG if entry provided
    if [[ -n "${SESSION_RESULT[changelog_entry]}" ]]; then
        update_changelog "${SESSION_RESULT[changelog_entry]}" "$task_type"
    fi

    # Mark task as DONE (only for tasks in tasks.yaml)
    if [[ "$task_type" != "bugfix" && "$task_type" != "documentation" && "$task_type" != "manual_review" ]]; then
        update_task_status "$task_id" "DONE"
    fi

    # Commit changes
    local commit_type="feat"
    case "$task_type" in
        bugfix)
            commit_type="fix"
            # Archive bug report
            archive_bug_report "$epic" "$task_id"
            ;;
        documentation)
            commit_type="docs"
            ;;
    esac

    git add -A
    git commit -m "$commit_type: $task_id" || {
        log_error "Failed to commit changes"
        return 1
    }

    log_success "Changes committed"

    # Special handling for bugfix completion
    if [[ "$task_type" == "bugfix" ]]; then
        log_info "Bugfix complete. Resuming original task."
        # The next_task should already point to the interrupted task
        # Just move it to active_task
        local resume_type=$(yq eval '.next_task.type // ""' "$PROJECT_STATE")
        local resume_id=$(yq eval '.next_task.id // ""' "$PROJECT_STATE")

        if [[ -n "$resume_id" ]]; then
            yq eval -i ".active_task.type = \"$resume_type\"" "$PROJECT_STATE"
            yq eval -i ".active_task.id = \"$resume_id\"" "$PROJECT_STATE"
            yq eval -i ".active_task.attempts = 0" "$PROJECT_STATE"

            # Clear next_task (will be set in next iteration)
            yq eval -i '.next_task.type = null' "$PROJECT_STATE"
            yq eval -i '.next_task.id = null' "$PROJECT_STATE"
        fi
        return 0
    fi

    # Special handling for documentation completion
    if [[ "$task_type" == "documentation" ]]; then
        log_success "KB synthesis complete"

        # Mark epic as complete
        local completed_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        yq eval -i ".current_epic.completed_at = \"$completed_at\"" "$PROJECT_STATE"

        return 2  # Signal epic complete
    fi

    # Check if all feature tasks are done
    local remaining_tasks=$(yq eval '.epic.tasks[] | select(.status == "TODO" or .status == "IN_PROGRESS") | .id' "$TASKS_FILE")

    if [[ -z "$remaining_tasks" ]]; then
        # All feature tasks complete - check if KB synthesis needed
        local kb_enabled=$(yq eval '.kb_enabled // true' "$PROJECT_STATE")

        if [[ "$kb_enabled" == "true" ]]; then
            log_info "All feature tasks complete. Triggering KB synthesis."
            yq eval -i '.active_task.type = "documentation"' "$PROJECT_STATE"
            yq eval -i '.active_task.id = "KB_UPDATE"' "$PROJECT_STATE"
            yq eval -i '.active_task.attempts = 0' "$PROJECT_STATE"
            yq eval -i '.next_task.type = null' "$PROJECT_STATE"
            yq eval -i '.next_task.id = null' "$PROJECT_STATE"
            return 0
        else
            log_info "All feature tasks complete. KB synthesis disabled."
            local completed_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
            yq eval -i ".current_epic.completed_at = \"$completed_at\"" "$PROJECT_STATE"
            return 2  # Signal epic complete
        fi
    fi

    # Advance to next task
    advance_to_next_task || {
        log_error "Failed to advance to next task"
        return 1
    }

    return 0
}
