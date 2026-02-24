#!/bin/bash
# handlers/epic.sh - Epic completion handler
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

# Note: core/metrics.sh is sourced by orchestrator.sh
# We can call its functions directly

handle_epic_complete() {
    local epic="$1"

    log_success "Epic complete: $epic"

    # Print metrics summary
    print_epic_summary "$epic"

    # Commit all epic changes (KB updates, session logs, YAML state, code changes)
    log_info "Committing all epic changes..."
    git add -A
    git commit -m "chore(epic): finalize $epic

- Knowledge base updates
- Session logs
- State and task tracking
- All code changes from epic" || {
        log_error "Failed to commit epic changes"
        return 1
    }

    log_success "All epic changes committed"

    echo ""
    echo "=============================================="
    echo "  EPIC COMPLETE: $epic"
    echo "=============================================="
    echo ""
    echo "Branch: $(yq eval '.current_epic.branch_name' "$PROJECT_STATE")"
    echo "Review changes and merge when ready"
    echo ""

    return 0
}
