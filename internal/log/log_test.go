package log_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/log"
)

// captureOutput redirects os.Stdout during fn and returns what was written.
func captureOutput(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

func TestInfo(t *testing.T) {
	out := captureOutput(func() { log.Info("test message") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("Info output missing [INFO]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Info output missing message: %q", out)
	}
}

func TestSuccess(t *testing.T) {
	out := captureOutput(func() { log.Success("test message") })
	if !strings.Contains(out, "[SUCCESS]") {
		t.Errorf("Success output missing [SUCCESS]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Success output missing message: %q", out)
	}
}

func TestWarning(t *testing.T) {
	out := captureOutput(func() { log.Warning("test message") })
	if !strings.Contains(out, "[WARNING]") {
		t.Errorf("Warning output missing [WARNING]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Warning output missing message: %q", out)
	}
}

func TestError(t *testing.T) {
	out := captureOutput(func() { log.Error("test message") })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("Error output missing [ERROR]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Error output missing message: %q", out)
	}
}

func TestFatal(t *testing.T) {
	var exitCode int
	log.OsExit = func(code int) { exitCode = code }
	defer func() { log.OsExit = os.Exit }()

	out := captureOutput(func() { log.Fatal("fatal message") })

	if exitCode != 1 {
		t.Errorf("Fatal did not call exit with code 1, got: %d", exitCode)
	}
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("Fatal output missing [ERROR]: %q", out)
	}
	if !strings.Contains(out, "fatal message") {
		t.Errorf("Fatal output missing message: %q", out)
	}
}

func TestSection(t *testing.T) {
	out := captureOutput(func() { log.Section("My Section") })
	if !strings.Contains(out, "━") {
		t.Errorf("Section output missing box-draw separator: %q", out)
	}
	if !strings.Contains(out, "My Section") {
		t.Errorf("Section output missing title: %q", out)
	}
}
