// Package style defines shared Lipgloss styles for Doug terminal output.
package style

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

// Palette groups the reusable styles Doug uses for terminal output.
type Palette struct {
	InfoBadge    lipgloss.Style
	SuccessBadge lipgloss.Style
	WarningBadge lipgloss.Style
	ErrorBadge   lipgloss.Style
	Section      lipgloss.Style
	Hint         lipgloss.Style
	Selected     lipgloss.Style
	Status       lipgloss.Style
}

// NewPalette returns Doug's shared Lipgloss style palette for w.
// Lipgloss/termenv handle TTY detection and NO_COLOR, so non-TTY and no-color
// environments render stable plain text.
func NewPalette(w io.Writer) Palette {
	r := lipgloss.NewRenderer(w)

	return Palette{
		InfoBadge:    r.NewStyle().Foreground(lipgloss.Color("15")).Bold(true),
		SuccessBadge: r.NewStyle().Foreground(lipgloss.Color("10")),
		WarningBadge: r.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		ErrorBadge:   r.NewStyle().Foreground(lipgloss.Color("9")),
		Section:      r.NewStyle().Foreground(lipgloss.Color("14")),
		Hint:         r.NewStyle().Foreground(lipgloss.Color("8")),
		Selected:     r.NewStyle().Foreground(lipgloss.Color("14")).Bold(true),
		Status:       r.NewStyle().Foreground(lipgloss.Color("12")),
	}
}
