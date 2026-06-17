package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("TEST_PI_RPC_MODE"); mode != "" {
		runTestPiRPCSubprocess(mode)
		os.Exit(0)
	}
	if mode := os.Getenv("TEST_PI_INTERACTIVE_MODE"); mode != "" {
		runTestPiInteractiveSubprocess(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runTestPiInteractiveSubprocess(mode string) {
	if path := os.Getenv("TEST_PI_INTERACTIVE_VERIFY_FILE"); path != "" {
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		data, err := json.Marshal(map[string]any{
			"cwd":  cwd,
			"args": os.Args[1:],
		})
		if err != nil {
			os.Exit(1)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			os.Exit(1)
		}
	}

	switch mode {
	case "success":
		os.Exit(0)
	case "failure":
		os.Exit(7)
	case "hang":
		for {
			time.Sleep(100 * time.Millisecond)
		}
	default:
		os.Exit(1)
	}
}

func runTestPiRPCSubprocess(mode string) {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer func() { _ = writer.Flush() }()

	writeLine := func(v any) {
		data, err := json.Marshal(v)
		if err != nil {
			os.Exit(1)
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			os.Exit(1)
		}
		if err := writer.Flush(); err != nil {
			os.Exit(1)
		}
	}

	if !scanner.Scan() {
		os.Exit(1)
	}
	var first map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &first); err != nil {
		os.Exit(1)
	}
	if first["type"] != "get_state" {
		os.Exit(1)
	}
	firstID, _ := first["id"].(string)

	switch mode {
	case "startup_error":
		writeLine(map[string]any{"id": firstID, "type": "response", "success": false, "error": "startup failed"})
		return
	case "scanner_error":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 9*1024*1024) + "\n")
		return
	case "decode_error_then_flood":
		// Emit a malformed startup line so the reader bails on a decode error,
		// then flood stdout past the OS pipe buffer. The reader must drain this
		// remainder on its way out or cmd.Wait() deadlocks against our write.
		_, _ = os.Stdout.WriteString("this is not json\n")
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 1024*1024) + "\n")
		return
	default:
		writeLine(map[string]any{"id": firstID, "type": "response", "success": true, "data": map[string]any{"sessionId": "pi-session-123"}})
	}

	if mode == "startup_only" {
		return
	}
	if mode == "transport_exit" {
		_, _ = os.Stderr.WriteString("provider_transport_failure: websocket connection reset\n")
		os.Exit(7)
	}

	if !scanner.Scan() {
		os.Exit(1)
	}
	var prompt map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &prompt); err != nil {
		os.Exit(1)
	}
	if prompt["type"] != "prompt" {
		os.Exit(1)
	}
	promptID, _ := prompt["id"].(string)

	switch mode {
	case "prompt_error":
		writeLine(map[string]any{"id": promptID, "type": "response", "success": false, "error": "prompt failed"})
	case "prompt_hang":
		for {
			time.Sleep(100 * time.Millisecond)
		}
	case "prompt_close_before_agent_end":
		writeLine(map[string]any{"id": promptID, "type": "response", "success": true, "data": map[string]any{"sessionId": "pi-session-456", "text": "ok"}})
		return
	case "prompt_success", "prompt_with_restrictions":
		writeLine(map[string]any{"id": promptID, "type": "response", "success": true, "data": map[string]any{"sessionId": "pi-session-456", "text": "ok"}})
		writeLine(map[string]any{"id": promptID, "type": "agent_end", "data": map[string]any{"sessionId": "pi-session-456"}})
	case "prompt_with_extension_ui_input":
		writeLine(map[string]any{"id": promptID, "type": "response", "success": true, "data": map[string]any{"sessionId": "pi-session-456"}})
		writeLine(map[string]any{"type": "extension_ui_request", "id": "ui-1", "method": "input", "message": "Continue?", "data": map[string]any{"sessionId": "pi-session-456"}})
		if !scanner.Scan() {
			os.Exit(1)
		}
		writeLine(map[string]any{"id": promptID, "type": "agent_end", "data": map[string]any{"sessionId": "pi-session-456"}})
	default:
		os.Exit(1)
	}
}
