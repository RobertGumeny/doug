#!/bin/bash
# core/verification.sh - Build and test verification
# Sourced by agent_loop.sh - depends on: lib/config.sh, lib/logging.sh, core/project.sh

verify_build() {
    log_info "Verifying build..."

    local build_cmd=$(get_build_cmd)
    local build_output=$(mktemp)

    if (cd "$PROJECT_ROOT" && eval "$build_cmd") > "$build_output" 2>&1; then
        rm -f "$build_output"
        log_success "Build passed"
        return 0
    else
        log_error "Build failed. Last 50 lines:"
        echo "----------------------------------------"
        tail -n 50 "$build_output"
        echo "----------------------------------------"
        rm -f "$build_output"
        return 1
    fi
}

verify_tests() {
    log_info "Running tests..."

    local test_cmd=$(get_test_cmd)

    # Skip tests if not configured (: is the no-op fallback)
    if [[ "$test_cmd" == ":" ]]; then
        log_warning "No test command configured, skipping tests"
        return 0
    fi

    # For npm projects, check if test script exists using npm's native detection
    local project_type=$(detect_project_type)
    if [[ "$project_type" == "npm" ]]; then
        # Use npm run to check if test script is defined
        if ! (cd "$PROJECT_ROOT" && npm run test --dry-run &>/dev/null); then
            log_warning "No test script configured in package.json, skipping tests"
            return 0
        fi
    fi

    local test_output=$(mktemp)

    # Run the test command and capture output
    if (cd "$PROJECT_ROOT" && eval "$test_cmd") > "$test_output" 2>&1; then
        # Check if this is the null test indicator (no real tests configured)
        if grep -q "NO_TESTS_CONFIGURED" "$test_output"; then
            log_warning "Test infrastructure not yet configured (NO_TESTS_CONFIGURED), skipping tests"
            rm -f "$test_output"
            return 0
        fi
        # Real tests passed
        rm -f "$test_output"
        log_success "Tests passed"
        return 0
    else
        # Command failed - check if it's just the null indicator
        if grep -q "NO_TESTS_CONFIGURED" "$test_output"; then
            log_warning "Test infrastructure not yet configured (NO_TESTS_CONFIGURED), skipping tests"
            rm -f "$test_output"
            return 0
        fi
        # Real test failure
        log_error "Tests failed. Last 50 lines:"
        echo "----------------------------------------"
        tail -n 50 "$test_output"
        echo "----------------------------------------"
        rm -f "$test_output"
        return 1
    fi
}
