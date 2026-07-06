package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpserver "github.com/robertgumeny/doug/internal/mcp"
)

func TestToolsListExposesDescriptionsAndInputSchemas(t *testing.T) {
	resp := handleMCPRequest(rpcRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/list"}, mcpserver.ToolHandler{})
	if resp.Error != nil {
		t.Fatalf("tools/list error: %#v", resp.Error)
	}
	payload, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal tools/list result: %v", err)
	}
	var result struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal tools/list result: %v", err)
	}
	if len(result.Tools) != len(mcpserver.ToolNames()) {
		t.Fatalf("tools/list returned %d tools, want %d", len(result.Tools), len(mcpserver.ToolNames()))
	}
	for _, tool := range result.Tools {
		if strings.TrimSpace(tool.Description) == "" || strings.HasPrefix(tool.Description, "Doug lifecycle tool:") {
			t.Fatalf("tool %q has non-descriptive description %q", tool.Name, tool.Description)
		}
		if got := tool.InputSchema["type"]; got != "object" {
			t.Fatalf("tool %q schema type = %#v, want object", tool.Name, got)
		}
		if _, ok := tool.InputSchema["properties"]; !ok {
			t.Fatalf("tool %q schema missing properties", tool.Name)
		}
		if tool.Name == mcpserver.ToolReconcileLifecycle {
			required, ok := tool.InputSchema["required"].([]any)
			if !ok || len(required) != 1 || required[0] != "mode" {
				t.Fatalf("reconcile_lifecycle required schema = %#v, want mode", tool.InputSchema["required"])
			}
		}
	}
}

func TestRunMCPRejectsInvalidConfigBeforeServing(t *testing.T) {
	dir := t.TempDir()
	dougDir := filepath.Join(dir, ".doug")
	if err := os.MkdirAll(dougDir, 0o755); err != nil {
		t.Fatalf("create .doug: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dougDir, "doug.yaml"), []byte("build_system: rust\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	err := runMCP(dir, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("expected invalid config error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid config") || !strings.Contains(err.Error(), "build_system") {
		t.Fatalf("expected actionable invalid config error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected MCP server not to write frames after config rejection, got %q", out.String())
	}
}
