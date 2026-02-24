#!/bin/bash
# core/git.sh - Git operations
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

ensure_epic_branch() {
    local epic="$1"
    local branch_name=$(yq eval '.current_epic.branch_name // ""' "$PROJECT_STATE")

    # If branch_name not set, generate it
    if [[ -z "$branch_name" ]]; then
        branch_name="feature/$epic"
        yq eval -i ".current_epic.branch_name = \"$branch_name\"" "$PROJECT_STATE"
    fi

    local current_branch=$(git rev-parse --abbrev-ref HEAD)

    if [[ "$current_branch" == "$branch_name" ]]; then
        log_info "Already on branch: $branch_name"
        return 0
    fi

    # Safety check: refuse to work on main/master
    if [[ "$current_branch" == "main" || "$current_branch" == "master" ]]; then
        log_warning "Currently on $current_branch. Switching to epic branch."
    fi

    if git show-ref --verify --quiet "refs/heads/$branch_name"; then
        log_info "Checking out existing branch: $branch_name"
        git checkout "$branch_name" || fatal "Failed to checkout $branch_name"
    else
        log_info "Creating new branch: $branch_name"
        git checkout -b "$branch_name" || fatal "Failed to create branch $branch_name"
    fi
}

rollback_changes() {
    log_warning "Rolling back uncommitted changes..."

    # Safety check: refuse rollback on main/master
    local current_branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$current_branch" == "main" || "$current_branch" == "master" ]]; then
        fatal "Refusing to rollback on $current_branch. Switch to feature branch first."
    fi

    # Preserve state files before rollback
    local temp_dir=$(mktemp -d)
    [[ -f "$PROJECT_STATE" ]] && cp "$PROJECT_STATE" "$temp_dir/project-state.yaml"
    [[ -f "$TASKS_FILE" ]] && cp "$TASKS_FILE" "$temp_dir/tasks.yaml"

    # Reset all files
    git reset --hard HEAD

    # Restore state files
    [[ -f "$temp_dir/project-state.yaml" ]] && cp "$temp_dir/project-state.yaml" "$PROJECT_STATE"
    [[ -f "$temp_dir/tasks.yaml" ]] && cp "$temp_dir/tasks.yaml" "$TASKS_FILE"
    rm -rf "$temp_dir"

    # Clean untracked files, excluding protected paths
    local exclude_args=()
    for path in "${PROTECTED_PATHS[@]}"; do
        exclude_args+=(-e "${path%/}")
    done
    git clean -fd "${exclude_args[@]}"

    log_success "Rollback complete"
}

generate_commit_msg() {
    local type="$1"      # feat | fix | docs | chore
    local task_id="$2"   # EPIC-X-001
    local description="$3" # Optional

    if [[ -n "$description" ]]; then
        echo "$type: $task_id - $description"
    else
        echo "$type: $task_id"
    fi
}
