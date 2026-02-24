#!/bin/bash
# core/metrics.sh - Task and epic metrics tracking
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

ensure_metrics_structure() {
    # Initialize metrics block if missing
    local has_metrics=$(yq eval 'has("metrics")' "$PROJECT_STATE")

    if [[ "$has_metrics" != "true" ]]; then
        log_info "Initializing metrics structure in project-state.yaml"
        yq eval -i '.metrics = {
            "total_tasks_completed": 0,
            "total_duration_seconds": 0,
            "tasks": []
        }' "$PROJECT_STATE"
    else
        # Migration: Remove token-related fields if they exist
        local has_tokens=$(yq eval '.metrics | has("total_estimated_tokens")' "$PROJECT_STATE")
        local has_cost=$(yq eval '.metrics | has("total_estimated_token_cost")' "$PROJECT_STATE")

        if [[ "$has_tokens" == "true" || "$has_cost" == "true" ]]; then
            log_info "Migrating metrics structure (removing token tracking)"
            yq eval -i 'del(.metrics.total_estimated_tokens)' "$PROJECT_STATE"
            yq eval -i 'del(.metrics.total_estimated_token_cost)' "$PROJECT_STATE"

            # Clean up task-level token fields
            yq eval -i '.metrics.tasks |= map(del(.estimated_tokens) | del(.estimated_token_cost))' "$PROJECT_STATE"
        fi
    fi
}

record_task_metrics() {
    local task_id="$1"
    local duration_seconds="$2"
    local outcome="$3"

    # Default to 0 if not provided
    duration_seconds=${duration_seconds:-0}
    outcome=${outcome:-"unknown"}

    log_info "Recording metrics: $task_id ($outcome, ${duration_seconds}s)"

    ensure_metrics_structure

    local completed_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Append to metrics.tasks array
    if ! yq eval -i ".metrics.tasks += [{
        \"task_id\": \"$task_id\",
        \"outcome\": \"$outcome\",
        \"duration_seconds\": $duration_seconds,
        \"completed_at\": \"$completed_at\"
    }]" "$PROJECT_STATE"; then
        log_warning "Failed to record metrics (yq error), continuing anyway"
        return 0  # Don't fail the task
    fi

    # Update totals
    update_metric_totals
}

update_metric_totals() {
    local total_tasks=$(yq eval '.metrics.tasks | length' "$PROJECT_STATE")
    local total_duration=$(yq eval '.metrics.tasks | map(.duration_seconds) | (if length > 0 then add else 0 end)' "$PROJECT_STATE")

    yq eval -i ".metrics.total_tasks_completed = $total_tasks" "$PROJECT_STATE"
    yq eval -i ".metrics.total_duration_seconds = $total_duration" "$PROJECT_STATE"
}

print_epic_summary() {
    local epic_id="$1"

    ensure_metrics_structure

    local total_tasks=$(yq eval '.metrics.total_tasks_completed' "$PROJECT_STATE")
    local total_duration=$(yq eval '.metrics.total_duration_seconds' "$PROJECT_STATE")

    # Convert seconds to human readable
    local hours=$((total_duration / 3600))
    local minutes=$(((total_duration % 3600) / 60))
    local seconds=$((total_duration % 60))

    # Calculate averages
    local avg_duration=0
    if [[ $total_tasks -gt 0 ]]; then
        avg_duration=$((total_duration / total_tasks))
    fi

    echo ""
    echo "═══════════════════════════════════════"
    echo "  EPIC METRICS: $epic_id"
    echo "═══════════════════════════════════════"
    echo "  Tasks Completed: $total_tasks"
    echo "  Total Time: ${hours}h ${minutes}m ${seconds}s"
    echo "  Avg Time/Task: ${avg_duration}s"
    echo "═══════════════════════════════════════"
    echo ""
}
