package interactive

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---- IsInteractive / fallback ----

func TestIsInteractive_IsCallable(t *testing.T) {
	_ = IsInteractive()
}

func TestIsInteractive_FalseImpliesNewReturnsFallback(t *testing.T) {
	if IsInteractive() {
		t.Skip("skipped: test requires a non-interactive environment")
	}
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	idx, val, err := p.SelectOne("pick", []string{"a", "b"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || val != "b" {
		t.Errorf("want idx=1 val=b; got idx=%d val=%s", idx, val)
	}
}

func TestIsInteractive_TrueImpliesNewReturnsTea(t *testing.T) {
	if !IsInteractive() {
		t.Skip("skipped: test requires an interactive terminal")
	}
	if p := New(); p == nil {
		t.Fatal("New() returned nil in interactive mode")
	}
}

func TestNewWithIO_NonInteractive_ReturnsPrompter(t *testing.T) {
	if p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false); p == nil {
		t.Fatal("expected non-nil Prompter")
	}
}

func TestNewWithIO_NonTTYFallbacksReturnDefaultsAndIgnoreInput(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader("typed input\n"), false)

	idx, val, err := p.SelectOne("Pick", []string{"x", "y", "z"}, 0)
	if err != nil || idx != 0 || val != "x" {
		t.Fatalf("SelectOne want default x without error; got idx=%d val=%q err=%v", idx, val, err)
	}
	ok, err := p.Confirm("OK?", true)
	if err != nil || !ok {
		t.Fatalf("Confirm want true without error; got %v err=%v", ok, err)
	}
	text, err := p.Text("Name?", "default")
	if err != nil || text != "default" {
		t.Fatalf("Text want default without error; got %q err=%v", text, err)
	}
	body, err := p.Compose("Message", "default body")
	if err != nil || body != "default body" {
		t.Fatalf("Compose want default without error; got %q err=%v", body, err)
	}
}

func TestSelectOne_NonTTY_EmptyOptionsReturnsError(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	_, _, err := p.SelectOne("Pick", nil, 0)
	if err == nil {
		t.Fatal("expected error for empty options")
	}
}

func TestSelectOne_NonTTY_DefaultClampedWhenOutOfRange(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	idx, val, err := p.SelectOne("Pick", []string{"x", "y"}, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || val != "x" {
		t.Errorf("want idx=0 val=x; got idx=%d val=%s", idx, val)
	}
}

func TestTextAndCompose_NonTTY_EmptyDefaultsReturnEmpty(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	text, err := p.Text("Name?", "")
	if err != nil || text != "" {
		t.Fatalf("Text want empty default; got %q err=%v", text, err)
	}
	body, err := p.Compose("Write", "")
	if err != nil || body != "" {
		t.Fatalf("Compose want empty default; got %q err=%v", body, err)
	}
}

// ---- selectModel ----

func TestSelectModel_UsesBubblesListWithoutFiltering(t *testing.T) {
	m := newSelectModel("Q", []string{"a", "b", "c"}, 1)
	if m.list.FilteringEnabled() {
		t.Fatal("SelectOne must remain a non-filtering list")
	}
	view := m.View()
	if strings.Contains(view, "Filter") || strings.Contains(view, "/") {
		t.Fatalf("filter/search affordance should not be exposed in view:\n%s", view)
	}
}

func TestSelectModel_NavigateAndSelectWithEnter(t *testing.T) {
	m := newSelectModel("Q", []string{"a", "b", "c"}, 0)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := m2.(selectModel)
	if got.list.Index() != 1 {
		t.Fatalf("want index=1 after down; got %d", got.list.Index())
	}
	m3, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := m3.(selectModel)
	if !selected.done || selected.choice != 1 {
		t.Fatalf("want done choice=1; got done=%v choice=%d", selected.done, selected.choice)
	}
}

func TestSelectModel_NavigateWithJKAndSpace(t *testing.T) {
	m := newSelectModel("Q", []string{"a", "b", "c"}, 1)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	got := m2.(selectModel)
	if got.list.Index() != 0 {
		t.Fatalf("k: want index=0; got %d", got.list.Index())
	}
	m3, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got = m3.(selectModel)
	if got.list.Index() != 1 {
		t.Fatalf("j: want index=1; got %d", got.list.Index())
	}
	m4, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	selected := m4.(selectModel)
	if !selected.done || selected.choice != 1 {
		t.Fatalf("space: want done choice=1; got done=%v choice=%d", selected.done, selected.choice)
	}
}

func TestSelectModel_CancelWithCtrlC_ReturnsDefault(t *testing.T) {
	m := newSelectModel("Q", []string{"a", "b", "c"}, 1)
	m.list.Select(2)
	m.choice = 2
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m2.(selectModel)
	if !got.done || got.choice != 1 {
		t.Errorf("want done=true choice=defaultIdx(1); got done=%v choice=%d", got.done, got.choice)
	}
}

// ---- confirmModel ----

func TestConfirmModel_KeyBehaviors(t *testing.T) {
	tests := []struct {
		name       string
		defaultYes bool
		msg        tea.KeyMsg
		want       bool
	}{
		{"yes", false, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}, true},
		{"upper yes", false, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}, true},
		{"no", true, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, false},
		{"upper no", true, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")}, false},
		{"enter default true", true, tea.KeyMsg{Type: tea.KeyEnter}, true},
		{"enter default false", false, tea.KeyMsg{Type: tea.KeyEnter}, false},
		{"ctrl+c default true", true, tea.KeyMsg{Type: tea.KeyCtrlC}, true},
		{"ctrl+c default false", false, tea.KeyMsg{Type: tea.KeyCtrlC}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newConfirmModel("Q?", tt.defaultYes)
			m2, _ := m.Update(tt.msg)
			got := m2.(confirmModel)
			if !got.done || got.answer != tt.want {
				t.Fatalf("want done answer=%v; got done=%v answer=%v", tt.want, got.done, got.answer)
			}
		})
	}
}

// ---- textModel ----

func TestTextModel_UsesBubblesTextInputForEditing(t *testing.T) {
	m := newTextModel("Name?", "")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	got := m2.(textModel)
	if got.input.Value() != "hi" {
		t.Fatalf("want hi; got %q", got.input.Value())
	}
	m3, _ := got.Update(tea.KeyMsg{Type: tea.KeySpace})
	got = m3.(textModel)
	if got.input.Value() != "hi " {
		t.Fatalf("want trailing space; got %q", got.input.Value())
	}
	m4, _ := got.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got = m4.(textModel)
	if got.input.Value() != "hi" {
		t.Fatalf("want backspace to remove space; got %q", got.input.Value())
	}
}

func TestTextModel_EnterAndCtrlCDefaultSemantics(t *testing.T) {
	m := newTextModel("Name?", "alice")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(textModel)
	if !got.done || got.canceled || got.input.Value() != "" {
		t.Fatalf("enter should submit empty input for default fallback; got done=%v canceled=%v value=%q", got.done, got.canceled, got.input.Value())
	}

	m = newTextModel("Name?", "alice")
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bob")})
	m3, _ := m2.(textModel).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got = m3.(textModel)
	if !got.done || !got.canceled || got.input.Value() != "" {
		t.Fatalf("ctrl+c should cancel to default; got done=%v canceled=%v value=%q", got.done, got.canceled, got.input.Value())
	}
}

// ---- composeModel ----

func TestComposeModel_UsesBubblesTextareaForValueAndEditing(t *testing.T) {
	m := newComposeModel("H", "")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	got := m2.(composeModel)
	if got.value() != "hello" {
		t.Fatalf("want hello; got %q", got.value())
	}
	m3, _ := got.Update(tea.KeyMsg{Type: tea.KeySpace})
	got = m3.(composeModel)
	if got.value() != "hello " {
		t.Fatalf("want trailing space; got %q", got.value())
	}
	m4, _ := got.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got = m4.(composeModel)
	if got.value() != "hello" {
		t.Fatalf("want backspace to remove space; got %q", got.value())
	}
}

func TestComposeModel_Update_EnterSubmitsAndCtrlDSubmits(t *testing.T) {
	m := newComposeModel("H", "")
	m.area.SetValue("line one")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(composeModel)
	if !got.done || got.value() != "line one" {
		t.Fatalf("enter should submit current value; got done=%v value=%q", got.done, got.value())
	}

	m = newComposeModel("H", "")
	m.area.SetValue("line one\nline two")
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got = m2.(composeModel)
	if !got.done || got.value() != "line one\nline two" {
		t.Fatalf("ctrl+d should submit current value; got done=%v value=%q", got.done, got.value())
	}
}

func TestComposeModel_Update_ShiftEnterAndCtrlJInsertNewline(t *testing.T) {
	m := newComposeModel("H", "")
	m.area.SetValue("line one")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("shift+enter")})
	got := m2.(composeModel)
	if got.done || got.value() != "line one\n" {
		t.Fatalf("shift+enter should insert newline without submit; got done=%v value=%q", got.done, got.value())
	}

	m = newComposeModel("H", "")
	m.area.SetValue("line one")
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	got = m2.(composeModel)
	if got.done || got.value() != "line one\n" {
		t.Fatalf("ctrl+j should insert newline without submit; got done=%v value=%q", got.done, got.value())
	}
}

func TestComposeModel_Update_CtrlCCancelsToDefault(t *testing.T) {
	m := newComposeModel("H", "default")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" typed")})
	m3, _ := m2.(composeModel).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m3.(composeModel)
	if !got.done || !got.canceled || got.value() != "" {
		t.Fatalf("ctrl+c should clear and mark canceled; got done=%v canceled=%v value=%q", got.done, got.canceled, got.value())
	}
}

func TestComposeModel_Update_WindowSizeWrapsLongInput(t *testing.T) {
	m := newComposeModel("H", "abcdefghijklmnopqrstuvwxyz")
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 20})
	got := m2.(composeModel)
	view := got.View()
	if !strings.Contains(view, "H") || !strings.Contains(view, "abcdefghijklmnop") {
		t.Fatalf("expected header and textarea content in view:\n%s", view)
	}
}

func TestComposeModel_View_ShowsSubmitAndNewlineHints(t *testing.T) {
	m := newComposeModel("H", "text")
	view := m.View()
	for _, want := range []string{"Enter submits", "Shift+Enter inserts a newline", "Ctrl+J inserts a newline", "Ctrl+D submits", "Ctrl+C cancels"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected hint %q in view:\n%s", want, view)
		}
	}
}

// Compile-time check: both concrete types satisfy Prompter.
var _ Prompter = (*fallbackPrompter)(nil)
var _ Prompter = (*teaPrompter)(nil)
