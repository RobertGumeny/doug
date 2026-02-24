#!/bin/bash
# core/project.sh - Build system abstraction
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh

detect_project_type() {
    # Detect based on manifest files
    [[ -f "$PROJECT_ROOT/package.json" ]] && echo "npm" && return 0
    [[ -f "$PROJECT_ROOT/go.mod" ]] && echo "go" && return 0
    [[ -f "$PROJECT_ROOT/pyproject.toml" ]] || [[ -f "$PROJECT_ROOT/requirements.txt" ]] || \
        [[ -f "$PROJECT_ROOT/setup.py" ]] && echo "python" && return 0

    log_error "Cannot detect project type"
    return 1
}

get_install_cmd() {
    local project_type=$(detect_project_type)
    case "$project_type" in
        npm)    echo "npx --no-install npm install" ;;
        go)     echo "go mod download" ;;
        python) log_warning "Python install command not configured"; echo ":" ;;
        *)      log_warning "Unknown project type: no install command"; echo ":" ;;
    esac
}

get_build_cmd() {
    local project_type=$(detect_project_type)
    case "$project_type" in
        npm)    echo "npx --no-install npm run build" ;;
        go)     echo "go build ./..." ;;
        python) log_warning "Python build command not configured"; echo ":" ;;
        *)      log_warning "Unknown project type: no build command"; echo ":" ;;
    esac
}

get_test_cmd() {
    local project_type=$(detect_project_type)
    case "$project_type" in
        npm)    echo "npx --no-install npm run test" ;;
        go)     echo "go test ./..." ;;
        python) log_warning "Python test command not configured"; echo ":" ;;
        *)      log_warning "Unknown project type: no test command"; echo ":" ;;
    esac
}

is_project_initialized() {
    local project_type=$(detect_project_type)
    case "$project_type" in
        npm)    [[ -d "$PROJECT_ROOT/node_modules" ]] ;;
        go)     [[ -f "$PROJECT_ROOT/go.mod" ]] ;;
        python) return 0 ;;
        *)      return 0 ;; # Assume initialized if unknown
    esac
}
