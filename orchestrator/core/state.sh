#!/bin/bash
# core/state.sh - Project state management (COMPLEX MODULE)
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

# Note: All other core modules are sourced by orchestrator.sh
# We can call their functions directly

bootstrap_from_tasks() {
    log_info "Checking if state needs bootstrapping from tasks.yaml..."

    # Only bootstrap if current_epic.id is empty
    local epic_id=$(yq eval '.current_epic.id // ""' "$PROJECT_STATE")

    if [[ -n "$epic_id" ]]; then
        log_info "Epic already initialized: $epic_id"
        return 0
    fi

    log_info "Bootstrapping state from tasks.yaml (first run)..."

    # Read epic metadata from tasks.yaml
    local tasks_epic_id=$(yq eval '.epic.id // ""' "$TASKS_FILE")
    local tasks_epic_name=$(yq eval '.epic.name // ""' "$TASKS_FILE")

    [[ -z "$tasks_epic_id" ]] && fatal "tasks.yaml missing epic.id"
    [[ -z "$tasks_epic_name" ]] && fatal "tasks.yaml missing epic.name"

    # Generate branch name
    local branch_name="feature/$tasks_epic_id"

    # Get current timestamp
    local started_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Populate current_epic
    yq eval -i ".current_epic.id = \"$tasks_epic_id\"" "$PROJECT_STATE"
    yq eval -i ".current_epic.name = \"$tasks_epic_name\"" "$PROJECT_STATE"
    yq eval -i ".current_epic.branch_name = \"$branch_name\"" "$PROJECT_STATE"
    yq eval -i ".current_epic.started_at = \"$started_at\"" "$PROJECT_STATE"

    # Find first task (should be TODO)
    local first_task_id=$(yq eval '.epic.tasks[0].id // ""' "$TASKS_FILE")
    local first_task_type=$(yq eval '.epic.tasks[0].type // ""' "$TASKS_FILE")

    [[ -z "$first_task_id" ]] && fatal "tasks.yaml has no tasks"
    [[ -z "$first_task_type" ]] && fatal "First task missing type"

    # Find second task (if exists)
    local second_task_id=$(yq eval '.epic.tasks[1].id // ""' "$TASKS_FILE")
    local second_task_type=$(yq eval '.epic.tasks[1].type // ""' "$TASKS_FILE")

    # Populate active_task (first task)
    yq eval -i ".active_task.type = \"$first_task_type\"" "$PROJECT_STATE"
    yq eval -i ".active_task.id = \"$first_task_id\"" "$PROJECT_STATE"
    yq eval -i ".active_task.attempts = 0" "$PROJECT_STATE"

    # Populate next_task (second task, if exists)
    if [[ -n "$second_task_id" ]]; then
        yq eval -i ".next_task.type = \"$second_task_type\"" "$PROJECT_STATE"
        yq eval -i ".next_task.id = \"$second_task_id\"" "$PROJECT_STATE"
    else
        # Only one task in epic
        yq eval -i ".next_task.type = null" "$PROJECT_STATE"
        yq eval -i ".next_task.id = null" "$PROJECT_STATE"
    fi

    log_success "Bootstrapped state:"
    log_info "  Epic: $tasks_epic_id - $tasks_epic_name"
    log_info "  Branch: $branch_name"
    log_info "  Active task: $first_task_id ($first_task_type)"
    [[ -n "$second_task_id" ]] && log_info "  Next task: $second_task_id ($second_task_type)"
}

validate_yaml_structure() {
    log_info "Validating YAML configuration..."

    # Project state checks
    local epic_id=$(yq eval '.current_epic.id // ""' "$PROJECT_STATE")
    [[ -z "$epic_id" ]] && fatal "project-state.yaml: missing current_epic.id (run bootstrap_from_tasks first)"

    local active_task_type=$(yq eval '.active_task.type // ""' "$PROJECT_STATE")
    [[ -z "$active_task_type" ]] && fatal "project-state.yaml: missing active_task.type"

    local active_task_id=$(yq eval '.active_task.id // ""' "$PROJECT_STATE")
    [[ -z "$active_task_id" ]] && fatal "project-state.yaml: missing active_task.id"

    # Tasks.yaml checks
    local tasks_epic=$(yq eval '.epic.id // ""' "$TASKS_FILE")
    [[ -z "$tasks_epic" ]] && fatal "tasks.yaml: missing epic.id"

    # Verify epic IDs match
    [[ "$epic_id" != "$tasks_epic" ]] && \
        fatal "Epic mismatch: project-state has '$epic_id', tasks.yaml has '$tasks_epic'"

    # Validate task statuses
    local invalid_tasks=$(yq eval '.epic.tasks[] | select(.status != "TODO" and .status != "IN_PROGRESS" and .status != "DONE" and .status != "BLOCKED") | .id' "$TASKS_FILE")
    [[ -n "$invalid_tasks" ]] && fatal "Invalid task statuses found: $invalid_tasks"

    log_success "YAML validation passed"
}

validate_state_sync() {
    log_info "Verifying synchronization between project state and task manifest..."

    # Check if the tasks file actually starts with an 'epic' object
    local has_epic=$(yq eval 'has("epic")' "$TASKS_FILE")
    if [[ "$has_epic" != "true" ]]; then
        fatal "Structure Error: $TASKS_FILE does not contain a root 'epic' object."
    fi

    # Check if the TASK_ID from project-state exists in the tasks.yaml list
    # (Only check for non-special tasks: not bugfix, not documentation, not manual_review)
    if [[ "$TASK_TYPE" != "bugfix" && "$TASK_TYPE" != "documentation" && "$TASK_TYPE" != "manual_review" ]]; then
        local task_exists=$(yq eval ".epic.tasks[] | select(.id == \"$TASK_ID\") | .id" "$TASKS_FILE")

        if [[ -z "$task_exists" ]]; then
            log_warning "Task Mismatch: $TASK_ID not found in $TASKS_FILE."

            # Attempt auto-recovery by finding the first TODO or IN_PROGRESS
            local next_task=$(find_next_active_task)

            if [[ -n "$next_task" ]]; then
                log_info "Auto-correcting state: Redirecting to first available task ($next_task)."
                local next_type=$(yq eval ".epic.tasks[] | select(.id == \"$next_task\") | .type" "$TASKS_FILE")
                yq eval -i ".active_task.type = \"$next_type\"" "$PROJECT_STATE"
                yq eval -i ".active_task.id = \"$next_task\"" "$PROJECT_STATE"
                yq eval -i ".active_task.attempts = 0" "$PROJECT_STATE"
                # Re-run read_task_info to refresh local variables
                read_task_info
            else
                fatal "No 'TODO' or 'IN_PROGRESS' tasks found in $TASKS_FILE. Is the epic already complete?"
            fi
        fi
    fi

    log_success "State synchronization verified."
}

read_task_info() {
    # Read the ACTIVE task (what we're working on now)
    TASK_TYPE=$(yq eval '.active_task.type // ""' "$PROJECT_STATE")
    TASK_ID=$(yq eval '.active_task.id // ""' "$PROJECT_STATE")
    TASK_ATTEMPTS=$(yq eval '.active_task.attempts // 0' "$PROJECT_STATE")
    CURRENT_EPIC=$(yq eval '.current_epic.id // ""' "$PROJECT_STATE")

    if [[ -z "$TASK_TYPE" || -z "$TASK_ID" ]]; then
        fatal "Invalid or missing active_task in project-state.yaml"
    fi
}

initialize_task_pointers() {
    log_info "Initializing task pointers..."

    # Find first IN_PROGRESS or TODO task
    local active_id=$(yq eval '.epic.tasks[] | select(.status == "IN_PROGRESS") | .id' "$TASKS_FILE" | head -n 1)

    if [[ -z "$active_id" ]]; then
        # No IN_PROGRESS, find first TODO
        active_id=$(yq eval '.epic.tasks[] | select(.status == "TODO") | .id' "$TASKS_FILE" | head -n 1)

        if [[ -z "$active_id" ]]; then
            # No TODO tasks either - check if KB needed
            local kb_enabled=$(yq eval '.kb_enabled // true' "$PROJECT_STATE")
            local active_task_type=$(yq eval '.active_task.type' "$PROJECT_STATE")

            if [[ "$kb_enabled" == "true" && "$active_task_type" != "documentation" ]]; then
                log_info "No TODO tasks remaining. Setting up KB synthesis."
                yq eval -i '.active_task.type = "documentation"' "$PROJECT_STATE"
                yq eval -i '.active_task.id = "KB_UPDATE"' "$PROJECT_STATE"
                yq eval -i '.active_task.attempts = 0' "$PROJECT_STATE"
                yq eval -i '.next_task.type = null' "$PROJECT_STATE"
                yq eval -i '.next_task.id = null' "$PROJECT_STATE"
                return 0
            fi

            log_info "No active tasks found and KB already complete."
            return 0
        fi
    fi

    # Set active_task
    local active_type=$(yq eval ".epic.tasks[] | select(.id == \"$active_id\") | .type" "$TASKS_FILE")
    yq eval -i ".active_task.type = \"$active_type\"" "$PROJECT_STATE"
    yq eval -i ".active_task.id = \"$active_id\"" "$PROJECT_STATE"
    yq eval -i ".active_task.attempts = 0" "$PROJECT_STATE"

    # Find next_task (first TODO after active that isn't active itself)
    local next_id=$(yq eval ".epic.tasks[] | select(.status == \"TODO\" and .id != \"$active_id\") | .id" "$TASKS_FILE" | head -n 1)

    if [[ -n "$next_id" ]]; then
        local next_type=$(yq eval ".epic.tasks[] | select(.id == \"$next_id\") | .type" "$TASKS_FILE")
        yq eval -i ".next_task.type = \"$next_type\"" "$PROJECT_STATE"
        yq eval -i ".next_task.id = \"$next_id\"" "$PROJECT_STATE"
        log_info "Task pointers initialized: active=$active_id, next=$next_id"
    else
        yq eval -i '.next_task.type = null' "$PROJECT_STATE"
        yq eval -i '.next_task.id = null' "$PROJECT_STATE"
        log_info "Task pointers initialized: active=$active_id, next=(none - last task)"
    fi
}

find_next_active_task() {
    # Find first task with status TODO or IN_PROGRESS
    yq eval '.epic.tasks[] | select(.status == "TODO" or .status == "IN_PROGRESS") | .id' "$TASKS_FILE" | head -n 1
}

# Checks for the explicit terminal state [v0.2.3]
is_epic_complete() {
    local active_type=$(yq eval '.active_task.type // ""' "$PROJECT_STATE")
    [[ "$active_type" == "epic-complete" ]]
}

# Logic to determine if we should move to KB synthesis or if we are totally done
needs_kb_synthesis() {
    local remaining_tasks=$(yq eval '.epic.tasks[] | select(.status == "TODO" or .status == "IN_PROGRESS") | .id' "$TASKS_FILE")

    # If tasks remain, we definitely aren't done
    [[ -n "$remaining_tasks" ]] && return 1

    local kb_enabled=$(yq eval '.kb_enabled // true' "$PROJECT_STATE")
    local active_task_type=$(yq eval '.active_task.type' "$PROJECT_STATE")

    # If backlog is empty but KB is enabled and not yet started/finished
    if [[ "$kb_enabled" == "true" && "$active_task_type" != "documentation" ]]; then
        return 0 # Yes, needs synthesis
    fi

    return 1 # No, either KB is disabled or already in progress/done
}

# Centralizes early exit logic
handle_early_exit() {
    if is_epic_complete; then
        log_success "Epic $CURRENT_EPIC already complete."
        echo ""
        echo "To start a new epic:"
        echo "  1. Update tasks.yaml with new epic"
        echo "  2. Reset current_epic.id in project-state.yaml"
        echo "  3. Run: $0 $MAX_ITERATIONS"
        echo ""
        exit 0
    fi

    # Check if we should quit because there's no feature work AND no KB work
    if ! needs_kb_synthesis; then
        local remaining_tasks=$(yq eval '.epic.tasks[] | select(.status == "TODO" or .status == "IN_PROGRESS") | .id' "$TASKS_FILE")
        if [[ -z "$remaining_tasks" ]]; then
             log_success "Epic $CURRENT_EPIC already complete. No open tasks remaining."
             exit 0
        fi
    fi
}

update_task_status() {
    local task_id="$1"
    local status="$2"

    log_info "Updating task $task_id status to $status"

    # Update in tasks.yaml using yq
    yq eval -i "(.epic.tasks[] | select(.id == \"$task_id\") | .status) = \"$status\"" "$TASKS_FILE"
}

increment_attempts() {
    local current=$(yq eval '.active_task.attempts' "$PROJECT_STATE")
    local new_attempts=$((current + 1))
    yq eval -i ".active_task.attempts = $new_attempts" "$PROJECT_STATE"
    log_info "Incremented attempts to $new_attempts"
}

advance_to_next_task() {
    log_info "Advancing to next task..."

    # Read next_task from project state
    local next_type=$(yq eval '.next_task.type // ""' "$PROJECT_STATE")
    local next_id=$(yq eval '.next_task.id // ""' "$PROJECT_STATE")

    if [[ -z "$next_id" ]]; then
        # No next task set - check if there are more TODO tasks
        local next_available=$(yq eval '.epic.tasks[] | select(.status == "TODO") | .id' "$TASKS_FILE" | head -n 1)

        if [[ -z "$next_available" ]]; then
            # No more TODO tasks
            log_info "No more tasks in epic"
            return 1
        fi

        next_id="$next_available"
        next_type=$(yq eval ".epic.tasks[] | select(.id == \"$next_id\") | .type" "$TASKS_FILE")
    fi

    # Move next_task to active_task
    yq eval -i ".active_task.type = \"$next_type\"" "$PROJECT_STATE"
    yq eval -i ".active_task.id = \"$next_id\"" "$PROJECT_STATE"
    yq eval -i ".active_task.attempts = 0" "$PROJECT_STATE"

    # Find the new next_task (first TODO after the new active)
    local new_next=$(yq eval ".epic.tasks[] | select(.status == \"TODO\" and .id != \"$next_id\") | .id" "$TASKS_FILE" | head -n 1)

    if [[ -n "$new_next" ]]; then
        local new_next_type=$(yq eval ".epic.tasks[] | select(.id == \"$new_next\") | .type" "$TASKS_FILE")
        yq eval -i ".next_task.type = \"$new_next_type\"" "$PROJECT_STATE"
        yq eval -i ".next_task.id = \"$new_next\"" "$PROJECT_STATE"
    else
        yq eval -i '.next_task.type = null' "$PROJECT_STATE"
        yq eval -i '.next_task.id = null' "$PROJECT_STATE"
    fi

    log_success "Advanced to task: $next_id"
    return 0
}
