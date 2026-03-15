// Package log provides colored terminal output for the doug orchestrator.
// All output uses ANSI escape codes; no external dependencies are required.
package log

import (
	"fmt"
	"io"
	"os"
)

// ANSI escape codes for terminal colors.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorCyan   = "\033[0;36m"
	colorWhite  = "\033[1;37m"
)

// sectionLine is the unicode box-draw separator matching the Bash orchestrator.
const sectionLine = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

// OsExit is the function called by Fatal to terminate the process.
// It is a package-level variable so tests can replace it without subprocess overhead.
var OsExit = os.Exit

// Logger is the interface for structured terminal output used throughout the
// orchestrator. It matches the set of package-level functions in this package.
type Logger interface {
	Info(msg string)
	Success(msg string)
	Warning(msg string)
	Error(msg string)
	Fatal(msg string)
	Section(title string)
}

// StderrLogger writes colored log output to an io.Writer (typically os.Stderr).
type StderrLogger struct {
	w io.Writer
}

// New returns a Logger that writes colored output to os.Stderr.
func New() *StderrLogger {
	return &StderrLogger{w: os.Stderr}
}

// Discard returns a Logger that silently discards all output.
// Useful in tests where log noise is not desired.
func Discard() Logger {
	return &discardLogger{}
}

func (l *StderrLogger) Info(msg string) {
	fmt.Fprintf(l.w, "%s[INFO]%s %s\n", colorWhite, colorReset, msg)
}

func (l *StderrLogger) Success(msg string) {
	fmt.Fprintf(l.w, "%s[SUCCESS]%s %s\n", colorGreen, colorReset, msg)
}

func (l *StderrLogger) Warning(msg string) {
	fmt.Fprintf(l.w, "%s[WARNING]%s %s\n", colorYellow, colorReset, msg)
}

func (l *StderrLogger) Error(msg string) {
	fmt.Fprintf(l.w, "%s[ERROR]%s %s\n", colorRed, colorReset, msg)
}

func (l *StderrLogger) Fatal(msg string) {
	l.Error(msg)
	OsExit(1)
}

func (l *StderrLogger) Section(title string) {
	fmt.Fprintf(l.w, "\n%s%s%s\n", colorCyan, sectionLine, colorReset)
	fmt.Fprintf(l.w, "%s%s%s\n", colorCyan, title, colorReset)
	fmt.Fprintf(l.w, "%s%s%s\n\n", colorCyan, sectionLine, colorReset)
}

// discardLogger silently discards all log output.
type discardLogger struct{}

func (d *discardLogger) Info(msg string)      {}
func (d *discardLogger) Success(msg string)   {}
func (d *discardLogger) Warning(msg string)   {}
func (d *discardLogger) Error(msg string)     {}
func (d *discardLogger) Fatal(msg string)     { OsExit(1) }
func (d *discardLogger) Section(title string) {}

// Package-level functions write to os.Stderr. They are retained for use in
// cmd/ and other callers that do not yet hold a Logger instance.

// Info prints a white [INFO] message to stderr.
func Info(msg string) {
	fmt.Fprintf(os.Stderr, "%s[INFO]%s %s\n", colorWhite, colorReset, msg)
}

// Success prints a green [SUCCESS] message to stderr.
func Success(msg string) {
	fmt.Fprintf(os.Stderr, "%s[SUCCESS]%s %s\n", colorGreen, colorReset, msg)
}

// Warning prints a yellow [WARNING] message to stderr.
func Warning(msg string) {
	fmt.Fprintf(os.Stderr, "%s[WARNING]%s %s\n", colorYellow, colorReset, msg)
}

// Error prints a red [ERROR] message to stderr.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", colorRed, colorReset, msg)
}

// Fatal prints a red [ERROR] message then exits with status 1.
func Fatal(msg string) {
	Error(msg)
	OsExit(1)
}

// Section prints a cyan unicode box-draw separator with a title,
// matching the visual style of the Bash orchestrator's log_section.
func Section(title string) {
	fmt.Fprintf(os.Stderr, "\n%s%s%s\n", colorCyan, sectionLine, colorReset)
	fmt.Fprintf(os.Stderr, "%s%s%s\n", colorCyan, title, colorReset)
	fmt.Fprintf(os.Stderr, "%s%s%s\n\n", colorCyan, sectionLine, colorReset)
}
