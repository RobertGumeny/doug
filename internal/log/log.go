// Package log provides styled terminal output for the doug orchestrator.
package log

import (
	"fmt"
	"io"
	"os"

	"github.com/robertgumeny/doug/internal/style"
)

// sectionLine is the unicode box-draw separator matching the Bash orchestrator.
const sectionLine = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

// OsExit is the function called by Fatal to terminate the process.
// It is a package-level variable so tests can replace it without process-exit overhead.
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

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeLog(w io.Writer, badge func(style.Palette) string, msg string) {
	writef(w, "%s %s\n", badge(style.NewPalette(w)), msg)
}

func writeSection(w io.Writer, title string) {
	section := style.NewPalette(w).Section
	writef(w, "\n%s\n", section.Render(sectionLine))
	writef(w, "%s\n", section.Render(title))
	writef(w, "%s\n\n", section.Render(sectionLine))
}

func (l *StderrLogger) Info(msg string) {
	writeLog(l.w, func(p style.Palette) string { return p.InfoBadge.Render("[INFO]") }, msg)
}

func (l *StderrLogger) Success(msg string) {
	writeLog(l.w, func(p style.Palette) string { return p.SuccessBadge.Render("[SUCCESS]") }, msg)
}

func (l *StderrLogger) Warning(msg string) {
	writeLog(l.w, func(p style.Palette) string { return p.WarningBadge.Render("[WARNING]") }, msg)
}

func (l *StderrLogger) Error(msg string) {
	writeLog(l.w, func(p style.Palette) string { return p.ErrorBadge.Render("[ERROR]") }, msg)
}

func (l *StderrLogger) Fatal(msg string) {
	l.Error(msg)
	OsExit(1)
}

func (l *StderrLogger) Section(title string) {
	writeSection(l.w, title)
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

// Info prints an [INFO] message to stderr.
func Info(msg string) {
	writeLog(os.Stderr, func(p style.Palette) string { return p.InfoBadge.Render("[INFO]") }, msg)
}

// Success prints a [SUCCESS] message to stderr.
func Success(msg string) {
	writeLog(os.Stderr, func(p style.Palette) string { return p.SuccessBadge.Render("[SUCCESS]") }, msg)
}

// Warning prints a [WARNING] message to stderr.
func Warning(msg string) {
	writeLog(os.Stderr, func(p style.Palette) string { return p.WarningBadge.Render("[WARNING]") }, msg)
}

// Error prints an [ERROR] message to stderr.
func Error(msg string) {
	writeLog(os.Stderr, func(p style.Palette) string { return p.ErrorBadge.Render("[ERROR]") }, msg)
}

// Fatal prints a red [ERROR] message then exits with status 1.
func Fatal(msg string) {
	Error(msg)
	OsExit(1)
}

// Section prints a unicode box-draw separator with a title,
// matching the visual style of the Bash orchestrator's log_section.
func Section(title string) {
	writeSection(os.Stderr, title)
}
