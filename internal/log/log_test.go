package log

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// captureOutput redirects os.Stderr during fn and returns what was written.
func captureOutput(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	if err := w.Close(); err != nil {
		panic(err)
	}
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

func assertNoANSI(t *testing.T, out string) {
	t.Helper()
	if ansiRE.MatchString(out) {
		t.Fatalf("output contains ANSI escape sequences: %q", out)
	}
}

func TestInfo(t *testing.T) {
	out := captureOutput(func() { Info("test message") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("Info output missing [INFO]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Info output missing message: %q", out)
	}
}

func TestSuccess(t *testing.T) {
	out := captureOutput(func() { Success("test message") })
	if !strings.Contains(out, "[SUCCESS]") {
		t.Errorf("Success output missing [SUCCESS]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Success output missing message: %q", out)
	}
}

func TestWarning(t *testing.T) {
	out := captureOutput(func() { Warning("test message") })
	if !strings.Contains(out, "[WARNING]") {
		t.Errorf("Warning output missing [WARNING]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Warning output missing message: %q", out)
	}
}

func TestError(t *testing.T) {
	out := captureOutput(func() { Error("test message") })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("Error output missing [ERROR]: %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("Error output missing message: %q", out)
	}
}

func TestFatal(t *testing.T) {
	var exitCode int
	OsExit = func(code int) { exitCode = code }
	defer func() { OsExit = os.Exit }()

	out := captureOutput(func() { Fatal("fatal message") })

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
	out := captureOutput(func() { Section("My Section") })
	if !strings.Contains(out, "━") {
		t.Errorf("Section output missing box-draw separator: %q", out)
	}
	if !strings.Contains(out, "My Section") {
		t.Errorf("Section output missing title: %q", out)
	}
}

func TestStderrLoggerNonTTYPlainOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := &StderrLogger{w: &buf}

	logger.Info("plain info")
	logger.Success("plain success")
	logger.Warning("plain warning")
	logger.Error("plain error")
	logger.Section("Plain Section")

	want := "[INFO] plain info\n" +
		"[SUCCESS] plain success\n" +
		"[WARNING] plain warning\n" +
		"[ERROR] plain error\n" +
		"\n" + sectionLine + "\n" +
		"Plain Section\n" +
		sectionLine + "\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("unexpected non-TTY output:\nwant %q\n got %q", want, got)
	}
	assertNoANSI(t, buf.String())
}

func TestNoColorPlainOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	out := captureOutput(func() {
		Info("no color info")
		Section("No Color Section")
	})

	want := "[INFO] no color info\n" +
		"\n" + sectionLine + "\n" +
		"No Color Section\n" +
		sectionLine + "\n\n"
	if out != want {
		t.Fatalf("unexpected no-color output:\nwant %q\n got %q", want, out)
	}
	assertNoANSI(t, out)
}
