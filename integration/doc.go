// Package integration contains the end-to-end smoke tests for the doug
// orchestrator. Tests in this package exercise the full orchestration loop
// with a real git repository and a mock agent.
//
// These tests are opt-in and excluded from the default unit-test pass.
// Run with: go test -tags=integration ./integration -v -timeout 120s
package integration
