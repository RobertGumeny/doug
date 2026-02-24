#!/bin/bash
# core/dependencies.sh - Dependency and environment management
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

# Note: core/project.sh and core/verification.sh are sourced by orchestrator.sh
# We can call their functions directly

check_dependencies() {
    log_info "Checking system dependencies..."

    command -v yq &> /dev/null     || fatal "yq is required but not installed."
    command -v claude &> /dev/null || fatal "claude CLI is required but not installed."
    command -v git &> /dev/null    || fatal "git is required but not installed."

    # Check for project-specific tools
    local project_type=$(detect_project_type)
    case "$project_type" in
        npm)
            command -v npm &> /dev/null || fatal "npm is required but not installed."
            ;;
        go)
            command -v go &> /dev/null || fatal "go is required but not installed."
            ;;
        python)
            command -v python &> /dev/null || command -v python3 &> /dev/null || \
                fatal "python is required but not installed."
            ;;
    esac

    [[ -f "$PROJECT_STATE" ]] || fatal "project-state.yaml not found."
    [[ -f "$TASKS_FILE" ]]    || fatal "tasks.yaml not found."

    # Ensure log directory structure
    mkdir -p "$LOGS_DIR/sessions" "$LOGS_DIR/bugs" "$LOGS_DIR/failures"

    # Ensure CHANGELOG.md exists
    if [[ ! -f "$CHANGELOG_FILE" ]]; then
        log_info "Creating CHANGELOG.md with Keep a Changelog format"
        cat > "$CHANGELOG_FILE" << 'EOF'
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Removed

EOF
    fi

    log_success "Dependencies verified"
}

install_dependencies() {
    log_info "Installing project dependencies..."

    local install_cmd=$(get_install_cmd)
    (cd "$PROJECT_ROOT" && eval "$install_cmd") || return 1

    log_success "Dependencies installed"
}

ensure_project_ready() {
    log_info "Initializing project environment..."

    # Install dependencies
    install_dependencies || fatal "Failed to install dependencies."

    # Only verify if project is initialized (skip for new projects)
    if is_project_initialized; then
        verify_build || {
            log_error "Pre-flight build failed."
            log_info "If this is a new project, ensure build scripts are configured."
            fatal "Fix compilation errors before continuing."
        }

        verify_tests || {
            log_error "Pre-flight tests failed."
            log_info "If this is a new project, ensure test scripts are configured."
            fatal "Fix test failures before continuing."
        }
    else
        log_warning "Project not fully initialized. Skipping build/test checks."
    fi

    log_success "Project environment ready"
}
