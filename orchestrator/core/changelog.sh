#!/bin/bash
# core/changelog.sh - CHANGELOG.md management
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

update_changelog() {
    local changelog_entry="$1"
    local task_type="$2"

    [[ -z "$changelog_entry" ]] && return 0

    log_info "Updating CHANGELOG.md..."

    # Determine section based on task type
    local section="### Added"
    case "$task_type" in
        bugfix)
            section="### Fixed"
            ;;
        feature)
            section="### Added"
            ;;
        documentation)
            section="### Changed"
            ;;
    esac

    # Find the line number of the section under [Unreleased]
    local section_line=$(grep -n "^$section" "$CHANGELOG_FILE" | head -n 1 | cut -d: -f1)

    if [[ -z "$section_line" ]]; then
        log_error "Could not find '$section' in CHANGELOG.md"
        return 1
    fi

    # Check if entry already exists (deduplication)
    if grep -qF "$changelog_entry" "$CHANGELOG_FILE"; then
        log_warning "Entry already exists in CHANGELOG, skipping"
        return 0
    fi

    # Insert the entry after the section header
    local insert_line=$((section_line + 1))
    sed -i "${insert_line}i - $changelog_entry" "$CHANGELOG_FILE"

    log_success "CHANGELOG.md updated"
}
