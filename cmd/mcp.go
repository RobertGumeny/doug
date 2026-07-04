package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	mcpserver "github.com/robertgumeny/doug/internal/mcp"
	"github.com/robertgumeny/doug/internal/orchestrator"
)

var mcpCmd = &cobra.Command{
	Use:     "mcp",
	Short:   "Run Doug's local stdio MCP server",
	Long:    "Start Doug's stdio MCP server for editor or agent integrations that need lifecycle tools such as status, reconciliation, task claiming, and completion reporting. The server reads .doug/doug.yaml from the current repo and validates it before serving requests.",
	Example: "  doug mcp",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runMCP(projectRoot, os.Stdin, os.Stdout)
	},
}

func runMCP(projectRoot string, in io.Reader, out io.Writer) error {
	paths := orchestrator.NewPaths(projectRoot)
	cfg, err := config.LoadConfig(paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	h := mcpserver.ToolHandler{ProjectRoot: paths.ProjectRoot, Config: cfg}
	return serveMCP(in, out, h)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func serveMCP(in io.Reader, out io.Writer, handler mcpserver.ToolHandler) error {
	reader := bufio.NewReader(in)
	for {
		payload, err := readMCPFrame(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if writeErr := writeMCPFrame(out, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}}); writeErr != nil {
				return writeErr
			}
			continue
		}
		resp := handleMCPRequest(req, handler)
		if req.ID != nil {
			if err := writeMCPFrame(out, resp); err != nil {
				return err
			}
		}
	}
}

func readMCPFrame(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", value, err)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	payload := make([]byte, contentLength)
	_, err := io.ReadFull(reader, payload)
	return payload, err
}

func writeMCPFrame(out io.Writer, value any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", body.Len()); err != nil {
		return err
	}
	_, err := out.Write(body.Bytes())
	return err
}

func handleMCPRequest(req rpcRequest, handler mcpserver.ToolHandler) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	result, err := dispatchMCP(req, handler)
	if err != nil {
		resp.Error = &rpcError{Code: -32000, Message: err.Error()}
		return resp
	}
	resp.Result = result
	return resp
}

func dispatchMCP(req rpcRequest, handler mcpserver.ToolHandler) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{"protocolVersion": "2024-11-05", "serverInfo": map[string]string{"name": "doug", "version": version}, "capabilities": map[string]any{"tools": map[string]any{}}}, nil
	case "tools/list":
		tools := make([]map[string]any, 0, len(mcpserver.ToolNames()))
		for _, name := range mcpserver.ToolNames() {
			tools = append(tools, map[string]any{"name": name, "description": "Doug lifecycle tool: " + name, "inputSchema": map[string]any{"type": "object"}})
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		return callMCPTool(handler, params.Name, params.Arguments)
	default:
		return nil, fmt.Errorf("unsupported MCP method %q", req.Method)
	}
}

func callMCPTool(handler mcpserver.ToolHandler, name string, args map[string]any) (any, error) {
	switch name {
	case mcpserver.ToolGetStatus:
		return handler.GetStatus()
	case mcpserver.ToolDiagnoseLifecycle:
		return handler.DiagnoseLifecycle()
	case mcpserver.ToolReconcileLifecycle:
		return handler.ReconcileLifecycle(stringArg(args, "mode"))
	case mcpserver.ToolGetNextTask:
		return handler.GetNextTask()
	case mcpserver.ToolReportTaskComplete:
		return handler.ReportTaskComplete(stringArg(args, "task_id"))
	case mcpserver.ToolReportTaskBlocked:
		return handler.ReportTaskBlocked(stringArg(args, "task_id"))
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

func stringArg(args map[string]any, name string) string {
	if args == nil {
		return ""
	}
	value, _ := args[name].(string)
	return value
}
