package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
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
	m := newSelectModel(question, options, defaultIdx)
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultIdx, options[defaultIdx], err
	}
	final := result.(selectModel)
	return final.choice, options[final.choice], nil
}

func (p *teaPrompter) Confirm(question string, defaultYes bool) (bool, error) {
	m := newConfirmModel(question, defaultYes)
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultYes, err
	}
	final := result.(confirmModel)
	return final.answer, nil
}

func (p *teaPrompter) Text(question string, defaultVal string) (string, error) {
	m := newTextModel(question, defaultVal)
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultVal, err
	}
	final := result.(textModel)
	val := strings.TrimSpace(final.input.Value())
	if val == "" || final.canceled {
		return defaultVal, nil
	}
	return val, nil
}

func (p *teaPrompter) Compose(header string, defaultVal string) (string, error) {
	m := newComposeModel(header, defaultVal)
	prog := tea.NewProgram(m)
	result, err := prog.Run()
	if err != nil {
		return defaultVal, err
	}
	final := result.(composeModel)
	val := strings.TrimSpace(final.value())
	if val == "" || final.canceled {
		return defaultVal, nil
	}
	return val, nil
}

// ---- Bubble Tea models ----

type selectItem struct {
	label string
}

func (i selectItem) Title() string       { return i.label }
func (i selectItem) Description() string { return "" }
func (i selectItem) FilterValue() string { return i.label }

// selectModel presents a simple cursor-navigable, non-filtering list of options.
type selectModel struct {
	question   string
	list       list.Model
	defaultIdx int
	choice     int
	done       bool
}

func newSelectModel(question string, options []string, defaultIdx int) selectModel {
	items := make([]list.Item, len(options))
	for i, option := range options {
		items[i] = selectItem{label: option}
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 80, len(items)+2)
	l.Title = question
	l.SetFilteringEnabled(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.Select(defaultIdx)

	return selectModel{question: question, list: l, defaultIdx: defaultIdx, choice: defaultIdx}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.choice = m.defaultIdx
			m.done = true
			return m, tea.Quit
		case "enter", " ":
			m.choice = m.list.Index()
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.choice = m.list.Index()
	return m, cmd
}

func (m selectModel) View() string {
	if m.done {
		return ""
	}
	return m.list.View()
}

// confirmModel presents a single yes/no prompt.
type confirmModel struct {
	question   string
	defaultYes bool
	answer     bool
	done       bool
}

func newConfirmModel(question string, defaultYes bool) confirmModel {
	return confirmModel{question: question, defaultYes: defaultYes, answer: defaultYes}
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
		case "enter", "ctrl+c":
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
	input      textinput.Model
	done       bool
	canceled   bool
}

func newTextModel(question string, defaultVal string) textModel {
	input := textinput.New()
	input.Placeholder = defaultVal
	input.Prompt = promptPrefix(question, defaultVal)
	input.Focus()
	return textModel{question: question, defaultVal: defaultVal, input: input}
}

func promptPrefix(question string, defaultVal string) string {
	if defaultVal == "" {
		return question + ": "
	}
	return question + " [" + defaultVal + "]: "
}

func (m textModel) Init() tea.Cmd { return textinput.Blink }

func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case tea.KeyCtrlC:
			m.input.SetValue("")
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case tea.KeySpace:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m textModel) View() string {
	if m.done {
		return ""
	}
	return m.input.View()
}

// composeModel presents a Bubbles textarea prompt.
// Enter submits; Shift+Enter or Ctrl+J inserts a newline; Ctrl+C cancels.
type composeModel struct {
	header   string
	area     textarea.Model
	done     bool
	canceled bool
}

func newComposeModel(header string, defaultVal string) composeModel {
	area := textarea.New()
	area.Placeholder = defaultVal
	area.ShowLineNumbers = false
	area.Prompt = ""
	area.SetWidth(80)
	area.SetHeight(6)
	area.SetValue(defaultVal)
	area.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline"))
	area.Focus()
	return composeModel{header: header, area: area}
}

func (m composeModel) Init() tea.Cmd { return textarea.Blink }

func (m composeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.area.SetWidth(msg.Width)
		}
		if msg.Height > 4 {
			m.area.SetHeight(msg.Height - 4)
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case isShiftEnter(msg):
			m.area.InsertString("\n")
			return m, nil
		case msg.Type == tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case msg.Type == tea.KeyCtrlD:
			m.done = true
			return m, tea.Quit
		case msg.Type == tea.KeyCtrlC:
			m.area.SetValue("")
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case msg.Type == tea.KeySpace:
			var cmd tea.Cmd
			m.area, cmd = m.area.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.area, cmd = m.area.Update(msg)
	return m, cmd
}

func isShiftEnter(msg tea.KeyMsg) bool {
	return msg.String() == "shift+enter"
}

func (m composeModel) View() string {
	if m.done {
		return ""
	}
	var sb strings.Builder
	if m.header != "" {
		sb.WriteString(m.header + "\n")
	}
	sb.WriteString("(Enter submits • Shift+Enter inserts a newline • Ctrl+J inserts a newline • Ctrl+D submits • Ctrl+C cancels)\n\n")
	sb.WriteString(m.area.View())
	return sb.String()
}

func (m composeModel) value() string {
	return m.area.Value()
}
