// Package status provides shared TTY-gated live status output for long agent runs.
package status

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultInterruptHint = "Ctrl-C to interrupt"
	defaultWaitingText   = "waiting for agent activity"
	maxActivityRunes     = 96
)

var ansiControlRE = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// LineLogger is the narrow non-TTY logging interface used by Indicator.
type LineLogger interface {
	Info(msg string)
}

// Options configures an Indicator.
type Options struct {
	TaskID        string
	Delay         time.Duration
	Writer        io.Writer
	TTY           bool
	Logger        LineLogger
	InterruptHint string
	WaitingText   string
}

// Indicator renders a single live status line on TTYs and preserves
// line-oriented heartbeat logs on non-TTY outputs.
type Indicator struct {
	taskID        string
	delay         time.Duration
	writer        io.Writer
	tty           bool
	logger        LineLogger
	interruptHint string
	waitingText   string
	visible       bool
}

// New returns a reusable live status indicator. A nil writer disables TTY
// rendering; a nil logger disables non-TTY heartbeat logs.
func New(opts Options) *Indicator {
	hint := strings.TrimSpace(opts.InterruptHint)
	if hint == "" {
		hint = defaultInterruptHint
	}
	waiting := strings.TrimSpace(opts.WaitingText)
	if waiting == "" {
		waiting = defaultWaitingText
	}
	return &Indicator{
		taskID:        opts.TaskID,
		delay:         opts.Delay,
		writer:        opts.Writer,
		tty:           opts.TTY && opts.Writer != nil,
		logger:        opts.Logger,
		interruptHint: hint,
		waitingText:   waiting,
	}
}

// Heartbeat records progress at elapsed. TTY indicators render only after the
// configured delay; non-TTY indicators emit one ordinary line per heartbeat.
func (i *Indicator) Heartbeat(elapsed time.Duration, activity string) {
	displayElapsed := elapsed.Round(time.Second)
	activity = SanitizeActivity(activity, i.waitingText)
	if !i.tty {
		if i.logger != nil {
			i.logger.Info(fmt.Sprintf("[%s] +%s — %s", i.taskID, displayElapsed, activity))
		}
		return
	}
	if elapsed < i.delay {
		return
	}
	i.visible = true
	_, _ = fmt.Fprintf(i.writer, "\r⏳ [%s] +%s — %s (%s)", i.taskID, displayElapsed, activity, i.interruptHint)
}

// Finish clears any visible TTY status line. It is best-effort terminal output.
func (i *Indicator) Finish() {
	if !i.tty || !i.visible {
		return
	}
	_, _ = fmt.Fprint(i.writer, "\r\033[2K")
	i.visible = false
}

// SanitizeActivity normalizes activity to one bounded, printable line with ANSI
// terminal control sequences removed. Empty activity falls back to fallback.
func SanitizeActivity(activity, fallback string) string {
	activity = ansiControlRE.ReplaceAllString(activity, "")
	activity = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, activity)
	activity = strings.Join(strings.Fields(activity), " ")
	if activity == "" {
		activity = strings.TrimSpace(fallback)
	}
	if activity == "" {
		activity = defaultWaitingText
	}
	if utf8.RuneCountInString(activity) <= maxActivityRunes {
		return activity
	}
	runes := []rune(activity)
	return string(runes[:maxActivityRunes-3]) + "..."
}
