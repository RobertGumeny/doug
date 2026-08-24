package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/lifecycle"
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

// writeMCPServeFixtures lays down the minimum lifecycle state a tools/call needs
// so the round-trip test exercises a real handler rather than an error path.
func writeMCPServeFixtures(t *testing.T, root string) {
	t.Helper()
	paths := lifecycle.DefaultPaths(root)
	if err := os.MkdirAll(filepath.Dir(paths.StatePath), 0o755); err != nil {
		t.Fatalf("create .doug: %v", err)
	}
	state := `current_epic:
    id: EPIC-MCP
    name: MCP Epic
    branch_name: feature/EPIC-MCP
    started_at: "2026-01-01T00:00:00Z"
    completed_at: null
active_task:
    type: feature
    id: ""
next_task:
    type: feature
    id: TASK-1
`
	tasks := `epic:
    id: EPIC-MCP
    name: MCP Epic
    tasks:
        - id: TASK-1
          type: feature
          status: TODO
          description: Do the thing
          acceptance_criteria:
            - Status works
`
	if err := os.WriteFile(paths.StatePath, []byte(state), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if err := os.WriteFile(paths.TasksPath, []byte(tasks), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
}

// TestServeMCPRoundTripsNewlineDelimitedJSON drives serveMCP over the transport
// a real MCP client speaks. It is the regression guard for the framing defect:
// the server previously expected LSP Content-Length headers, so a conforming
// client's NDJSON was consumed as unrecognized headers and the server answered
// nothing and exited cleanly. Asserting through handleMCPRequest instead of
// serveMCP is what let that reach main, so this test must stay at this level.
func TestServeMCPRoundTripsNewlineDelimitedJSON(t *testing.T) {
	root := t.TempDir()
	writeMCPServeFixtures(t, root)

	// The last message deliberately omits its trailing newline: a client may
	// close the stream straight after writing, and that message must still be
	// answered rather than dropped at EOF.
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_status","arguments":{}}}`,
	}, "\n") + "\n" + `{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}`

	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(in), &out, mcpserver.ToolHandler{ProjectRoot: root}); err != nil {
		t.Fatalf("serveMCP: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("server produced no output; the transport is not answering a conforming client")
	}
	if strings.Contains(out.String(), "Content-Length") {
		t.Fatalf("response used LSP header framing, want newline-delimited JSON: %q", out.String())
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	// Four responses, not five: the notification carries no id and must be
	// answered with silence.
	if len(lines) != 4 {
		t.Fatalf("got %d response lines, want 4 (notification must not be answered): %q", len(lines), out.String())
	}

	var gotIDs []float64
	results := map[float64]json.RawMessage{}
	for i, line := range lines {
		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      float64         `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   *rpcError       `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response %d is not valid JSON: %v (%q)", i, err, line)
		}
		if resp.JSONRPC != "2.0" {
			t.Errorf("response %d jsonrpc = %q, want 2.0", i, resp.JSONRPC)
		}
		if resp.Error != nil {
			t.Errorf("response %d returned error: %#v", i, resp.Error)
		}
		if resp.Result == nil {
			t.Errorf("response %d has no result", i)
		}
		gotIDs = append(gotIDs, resp.ID)
		results[resp.ID] = resp.Result
	}
	for i, want := range []float64{1, 2, 3, 4} {
		if gotIDs[i] != want {
			t.Errorf("response %d id = %v, want %v", i, gotIDs[i], want)
		}
	}

	// Envelope assertions above cannot tell a correct result from an empty one.
	// The tools/call result must carry MCP content blocks, and those blocks must
	// carry the handler's actual data: a bare domain struct decodes as a valid
	// response yet renders as no output at all in a conforming client.
	status := decodeToolCallText(t, results[3])
	if !strings.Contains(status, "EPIC-MCP") {
		t.Errorf("tools/call content does not carry handler data, got %q", status)
	}
}

// decodeToolCallText unwraps a CallToolResult and returns its text content,
// failing the test if the result is not in MCP's content-block shape.
func decodeToolCallText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("tools/call result is not a CallToolResult: %v (%s)", err, raw)
	}
	if len(result.Content) == 0 {
		t.Fatalf("tools/call result has no content blocks; a client renders this as an empty response: %s", raw)
	}
	if result.Content[0].Type != "text" {
		t.Fatalf("content block type = %q, want text", result.Content[0].Type)
	}
	return result.Content[0].Text
}

// TestToolCallReportsHandlerFailureAsErrorContent covers the error half of the
// contract: a handler that fails must come back as readable isError content the
// model can act on, not as a JSON-RPC error that reads as a dead protocol.
func TestToolCallReportsHandlerFailureAsErrorContent(t *testing.T) {
	// No fixtures: the handler cannot load lifecycle state and must fail.
	root := t.TempDir()

	in := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_status","arguments":{}}}` + "\n"
	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(in), &out, mcpserver.ToolHandler{ProjectRoot: root}); err != nil {
		t.Fatalf("serveMCP: %v", err)
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v (%q)", err, out.String())
	}
	if resp.Error != nil {
		t.Fatalf("handler failure surfaced as a JSON-RPC error, want isError content: %#v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result is not a CallToolResult: %v (%s)", err, resp.Result)
	}
	if !result.IsError {
		t.Errorf("isError = false, want true for a failed handler: %s", resp.Result)
	}
	if len(result.Content) == 0 || result.Content[0].Text == "" {
		t.Errorf("error result carries no message for the model to act on: %s", resp.Result)
	}
}

// TestToolCallRejectsUnknownToolAsProtocolError pins the other side of the
// split: an unserved tool name is a protocol fault, so it stays a JSON-RPC
// error rather than becoming a result the model might treat as real data.
func TestToolCallRejectsUnknownToolAsProtocolError(t *testing.T) {
	in := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"no_such_tool","arguments":{}}}` + "\n"
	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(in), &out, mcpserver.ToolHandler{}); err != nil {
		t.Fatalf("serveMCP: %v", err)
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v (%q)", err, out.String())
	}
	if resp.Error == nil {
		t.Fatalf("unknown tool returned a result, want a JSON-RPC error: %s", resp.Result)
	}
	if !strings.Contains(resp.Error.Message, "no_such_tool") {
		t.Errorf("error message %q does not name the offending tool", resp.Error.Message)
	}
}

func TestServeMCPSkipsBlankLinesBetweenMessages(t *testing.T) {
	in := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n\n\n"
	var out bytes.Buffer
	if err := serveMCP(strings.NewReader(in), &out, mcpserver.ToolHandler{}); err != nil {
		t.Fatalf("serveMCP: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d response lines, want 1: %q", len(lines), out.String())
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
