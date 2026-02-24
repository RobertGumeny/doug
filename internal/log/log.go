// Package log provides colored terminal output for the doug orchestrator.
// All output uses ANSI escape codes; no external dependencies are required.
package log

import (
	"fmt"
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

// Info prints a white [INFO] message to stdout.
func Info(msg string) {
	fmt.Printf("%s[INFO]%s %s\n", colorWhite, colorReset, msg)
}

// Success prints a green [SUCCESS] message to stdout.
func Success(msg string) {
	fmt.Printf("%s[SUCCESS]%s %s\n", colorGreen, colorReset, msg)
}

// Warning prints a yellow [WARNING] message to stdout.
func Warning(msg string) {
	fmt.Printf("%s[WARNING]%s %s\n", colorYellow, colorReset, msg)
}

// Error prints a red [ERROR] message to stdout.
func Error(msg string) {
	fmt.Printf("%s[ERROR]%s %s\n", colorRed, colorReset, msg)
}

// Fatal prints a red [ERROR] message then exits with status 1.
func Fatal(msg string) {
	Error(msg)
	OsExit(1)
}

// Section prints a cyan unicode box-draw separator with a title,
// matching the visual style of the Bash orchestrator's log_section.
func Section(title string) {
	fmt.Printf("\n%s%s%s\n", colorCyan, sectionLine, colorReset)
	fmt.Printf("%s%s%s\n", colorCyan, title, colorReset)
	fmt.Printf("%s%s%s\n\n", colorCyan, sectionLine, colorReset)
}
