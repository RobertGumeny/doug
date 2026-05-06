package interactive

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// teaPrompter is the Bubble Tea-backed implementation of Prompter. It is used
// when the caller is connected to a real terminal. Each method runs a short-lived
// tea.Program for a single prompt and returns when the user confirms their input.
//
// teaPrompter does not accept injected io.Writer / io.Reader because Bubble Tea
// manages the terminal directly. For testable prompts use NewWithIO with isTTY=false.
type teaPrompter struct{}

func (p *teaPrompter) SelectOne(question string, options []string, defaultIdx int) (int, string, error) {
	if len(options) == 0 {
		return 0, "", fmt.Errorf("interactive.SelectOne: options list must not be empty")
	}
	if defaultIdx < 0 || defaultIdx >= len(options) {
		defaultIdx = 0
	}
	m := selectModel{
		question:   question,
		options:    options,
		cursor:     defaultIdx,
		defaultIdx: defaultIdx,
		choice:     defaultIdx,
	}
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultIdx, options[defaultIdx], err
	}
	final := result.(selectModel)
	return final.choice, options[final.choice], nil
}

func (p *teaPrompter) Confirm(question string, defaultYes bool) (bool, error) {
	m := confirmModel{
		question:   question,
		defaultYes: defaultYes,
		answer:     defaultYes,
	}
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultYes, err
	}
	final := result.(confirmModel)
	return final.answer, nil
}

func (p *teaPrompter) Text(question string, defaultVal string) (string, error) {
	m := textModel{
		question:   question,
		defaultVal: defaultVal,
	}
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultVal, err
	}
	final := result.(textModel)
	val := strings.TrimSpace(string(final.value))
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

// ---- Bubble Tea models ----

// selectModel presents a cursor-navigable list of options.
type selectModel struct {
	question   string
	options    []string
	cursor     int
	defaultIdx int
	choice     int
	done       bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.choice = m.defaultIdx
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.choice = m.cursor
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	if m.done {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(m.question + "\n")
	for i, opt := range m.options {
		if m.cursor == i {
			sb.WriteString("> " + opt + "\n")
		} else {
			sb.WriteString("  " + opt + "\n")
		}
	}
	return sb.String()
}

// confirmModel presents a single yes/no prompt.
type confirmModel struct {
	question   string
	defaultYes bool
	answer     bool
	done       bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.answer = true
			m.done = true
			return m, tea.Quit
		case "n", "N":
			m.answer = false
			m.done = true
			return m, tea.Quit
		case "enter":
			m.answer = m.defaultYes
			m.done = true
			return m, tea.Quit
		case "ctrl+c":
			m.answer = m.defaultYes
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		return ""
	}
	hint := "[Y/n]"
	if !m.defaultYes {
		hint = "[y/N]"
	}
	return fmt.Sprintf("%s %s: ", m.question, hint)
}

// textModel presents a single-line text input.
type textModel struct {
	question   string
	defaultVal string
	value      []rune
	done       bool
}

func (m textModel) Init() tea.Cmd { return nil }

func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case tea.KeyCtrlC:
			m.value = nil
			m.done = true
			return m, tea.Quit
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.value) > 0 {
				m.value = m.value[:len(m.value)-1]
			}
		case tea.KeyRunes:
			m.value = append(m.value, msg.Runes...)
		}
	}
	return m, nil
}

func (m textModel) View() string {
	if m.done {
		return ""
	}
	prompt := m.question
	if m.defaultVal != "" {
		prompt += " [" + m.defaultVal + "]"
	}
	return prompt + ": " + string(m.value) + "_"
}
