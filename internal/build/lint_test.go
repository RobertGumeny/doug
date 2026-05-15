package build_test

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/build"
)

func TestRunLint_EmptyCommand_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	err := build.RunLint(dir, "")
	if err == nil {
		t.Fatal("expected error for empty lint command, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error to mention 'empty', got: %v", err)
	}
}

func TestRunLint_WhitespaceOnly_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	err := build.RunLint(dir, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only lint command, got nil")
	}
}

func TestRunLint_ValidCommand_Succeeds(t *testing.T) {
	dir := t.TempDir()
	// "go version" is always available and always exits 0.
	err := build.RunLint(dir, "go version")
	if err != nil {
		t.Errorf("expected RunLint to succeed for 'go version', got: %v", err)
	}
}

func TestRunLint_FailingCommand_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	writeFile(t, dir, "bad.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n")
	// go vet ./... should pass on valid code; use a nonexistent binary to force failure.
	err := build.RunLint(dir, "this-binary-does-not-exist")
	if err == nil {
		t.Fatal("expected error for nonexistent lint binary, got nil")
	}
}

func TestRunLint_MultipleArgs_ParsedCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module testmod\ngo 1.21\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	// go vet ./... is a multi-arg command; it should succeed on valid Go code.
	err := build.RunLint(dir, "go vet ./...")
	if err != nil {
		t.Errorf("expected 'go vet ./...' to pass on valid Go code, got: %v", err)
	}
}
