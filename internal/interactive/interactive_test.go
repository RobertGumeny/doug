package interactive

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// All tests use NewWithIO with isTTY=false so they exercise the fallbackPrompter
// path without requiring a real terminal or a running Bubble Tea program.

// ---- IsInteractive ----

func TestIsInteractive_IsCallable(t *testing.T) {
	// IsInteractive must not panic and must return a bool. The actual value
	// depends on the environment (TTY vs pipe) so we do not assert it here.
	_ = IsInteractive()
}

func TestIsInteractive_FalseImpliesNewReturnsFallback(t *testing.T) {
	if IsInteractive() {
		t.Skip("skipped: test requires a non-interactive environment")
	}
	// When not interactive, New() must return a fallback that silently returns defaults.
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	// Verify deterministic non-interactive behavior: SelectOne returns default.
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
	// When interactive, New() must return the Bubble Tea prompter (non-nil).
	p := New()
	if p == nil {
		t.Fatal("New() returned nil in interactive mode")
	}
}

func TestNewWithIO_NonInteractive_ReturnsPrompter(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	if p == nil {
		t.Fatal("expected non-nil Prompter")
	}
}

// ---- SelectOne ----

func TestSelectOne_NonTTY_ReturnsDefault(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	idx, val, err := p.SelectOne("Pick one", []string{"a", "b", "c"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || val != "b" {
		t.Errorf("want idx=1 val=b; got idx=%d val=%s", idx, val)
	}
}

func TestSelectOne_EmptyOptions_ReturnsError(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	_, _, err := p.SelectOne("Pick", []string{}, 0)
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
	// internal/prompt clamps out-of-range defaultIdx to 0
	if idx != 0 || val != "x" {
		t.Errorf("want idx=0 val=x; got idx=%d val=%s", idx, val)
	}
}

// ---- Confirm ----

func TestConfirm_NonTTY_ReturnsDefaultYes(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got, err := p.Confirm("Continue?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true (defaultYes)")
	}
}

func TestConfirm_NonTTY_ReturnsDefaultNo(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got, err := p.Confirm("Continue?", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false (defaultNo)")
	}
}

// ---- Text ----

func TestText_NonTTY_ReturnsDefault(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got, err := p.Text("Name?", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice" {
		t.Errorf("want alice; got %s", got)
	}
}

func TestText_NonTTY_EmptyDefaultReturnsEmpty(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got, err := p.Text("Name?", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("want empty string; got %q", got)
	}
}

// ---- Compose ----

func TestCompose_NonTTY_ReturnsDefault(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got, err := p.Compose("Write a message", "default text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "default text" {
		t.Errorf("want %q; got %q", "default text", got)
	}
}

func TestCompose_NonTTY_EmptyDefaultReturnsEmpty(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader(""), false)
	got, err := p.Compose("Write a message", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("want empty string; got %q", got)
	}
}

// ---- composeModel unit tests ----

func TestComposeModel_Value_SingleLine(t *testing.T) {
	m := composeModel{
		lines:   []string{"hello world"},
		current: nil,
	}
	if got := m.value(); got != "hello world" {
		t.Errorf("want %q; got %q", "hello world", got)
	}
}

func TestComposeModel_Value_MultiLine(t *testing.T) {
	m := composeModel{
		lines:   []string{"line one", "line two"},
		current: []rune("line three"),
	}
	want := "line one\nline two\nline three"
	if got := m.value(); got != want {
		t.Errorf("want %q; got %q", want, got)
	}
}

func TestComposeModel_Value_Empty(t *testing.T) {
	m := composeModel{}
	if got := m.value(); got != "" {
		t.Errorf("want empty string; got %q", got)
	}
}

// ---- selectModel unit tests ----

func TestSelectModel_NavigateDown(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b", "c"}, cursor: 0, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := m2.(selectModel)
	if got.cursor != 1 {
		t.Errorf("want cursor=1; got %d", got.cursor)
	}
}

func TestSelectModel_NavigateDown_AtBoundary(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b"}, cursor: 1, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := m2.(selectModel)
	if got.cursor != 1 {
		t.Errorf("want cursor clamped to 1; got %d", got.cursor)
	}
}

func TestSelectModel_NavigateUp(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b", "c"}, cursor: 2, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := m2.(selectModel)
	if got.cursor != 1 {
		t.Errorf("want cursor=1; got %d", got.cursor)
	}
}

func TestSelectModel_NavigateUp_AtBoundary(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b"}, cursor: 0, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := m2.(selectModel)
	if got.cursor != 0 {
		t.Errorf("want cursor clamped to 0; got %d", got.cursor)
	}
}

func TestSelectModel_NavigateWithJK(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b", "c"}, cursor: 1, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	got := m2.(selectModel)
	if got.cursor != 0 {
		t.Errorf("k: want cursor=0; got %d", got.cursor)
	}
	m3, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got2 := m3.(selectModel)
	if got2.cursor != 1 {
		t.Errorf("j: want cursor=1; got %d", got2.cursor)
	}
}

func TestSelectModel_SelectWithEnter(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b", "c"}, cursor: 2, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(selectModel)
	if !got.done || got.choice != 2 {
		t.Errorf("want done=true choice=2; got done=%v choice=%d", got.done, got.choice)
	}
}

func TestSelectModel_SelectWithSpace(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b"}, cursor: 1, defaultIdx: 0, choice: 0}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	got := m2.(selectModel)
	if !got.done || got.choice != 1 {
		t.Errorf("want done=true choice=1; got done=%v choice=%d", got.done, got.choice)
	}
}

func TestSelectModel_CancelWithCtrlC_ReturnsDefault(t *testing.T) {
	m := selectModel{question: "Q", options: []string{"a", "b", "c"}, cursor: 2, defaultIdx: 1, choice: 2}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m2.(selectModel)
	if !got.done || got.choice != 1 {
		t.Errorf("want done=true choice=defaultIdx(1); got done=%v choice=%d", got.done, got.choice)
	}
}

// ---- confirmModel unit tests ----

func TestConfirmModel_YesKey(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: false}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got := m2.(confirmModel)
	if !got.done || !got.answer {
		t.Errorf("want done=true answer=true; got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_UpperYesKey(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: false}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
	got := m2.(confirmModel)
	if !got.done || !got.answer {
		t.Errorf("want done=true answer=true; got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_NoKey(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: true}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := m2.(confirmModel)
	if !got.done || got.answer {
		t.Errorf("want done=true answer=false; got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_UpperNoKey(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: true}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	got := m2.(confirmModel)
	if !got.done || got.answer {
		t.Errorf("want done=true answer=false; got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_EnterUsesDefault_True(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: true, answer: true}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(confirmModel)
	if !got.done || !got.answer {
		t.Errorf("want done=true answer=true; got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_EnterUsesDefault_False(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: false, answer: false}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(confirmModel)
	if !got.done || got.answer {
		t.Errorf("want done=true answer=false; got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_CancelWithCtrlC_ReturnsDefault(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: true, answer: true}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m2.(confirmModel)
	if !got.done || !got.answer {
		t.Errorf("want done=true answer=defaultYes(true); got done=%v answer=%v", got.done, got.answer)
	}
}

func TestConfirmModel_CancelWithCtrlC_DefaultFalse(t *testing.T) {
	m := confirmModel{question: "Q?", defaultYes: false, answer: false}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m2.(confirmModel)
	if !got.done || got.answer {
		t.Errorf("want done=true answer=defaultYes(false); got done=%v answer=%v", got.done, got.answer)
	}
}

// ---- textModel unit tests ----

func TestTextModel_RuneInput(t *testing.T) {
	m := textModel{question: "Name?", defaultVal: ""}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	got := m2.(textModel)
	if string(got.value) != "hi" {
		t.Errorf("want value=hi; got %q", string(got.value))
	}
	if got.done {
		t.Error("should not be done after rune input")
	}
}

func TestTextModel_SpaceKeyInput(t *testing.T) {
	m := textModel{question: "Name?", value: []rune("hello")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := m2.(textModel)
	if string(got.value) != "hello " {
		t.Errorf("want value=%q; got %q", "hello ", string(got.value))
	}
}

func TestTextModel_Backspace(t *testing.T) {
	m := textModel{question: "Name?", value: []rune("abc")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := m2.(textModel)
	if string(got.value) != "ab" {
		t.Errorf("want value=ab; got %q", string(got.value))
	}
}

func TestTextModel_Backspace_EmptyValue(t *testing.T) {
	m := textModel{question: "Name?", value: nil}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := m2.(textModel)
	if len(got.value) != 0 {
		t.Errorf("want empty value after backspace on empty; got %q", string(got.value))
	}
}

func TestTextModel_EnterFinalizes(t *testing.T) {
	m := textModel{question: "Name?", value: []rune("alice")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(textModel)
	if !got.done {
		t.Error("want done=true after enter")
	}
	if string(got.value) != "alice" {
		t.Errorf("want value=alice; got %q", string(got.value))
	}
}

func TestTextModel_CancelWithCtrlC_ClearsValue(t *testing.T) {
	m := textModel{question: "Name?", value: []rune("alice")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m2.(textModel)
	if !got.done {
		t.Error("want done=true after ctrl+c")
	}
	if len(got.value) != 0 {
		t.Errorf("want value cleared; got %q", string(got.value))
	}
}

// ---- composeModel Update tests ----

func TestComposeModel_Update_RuneInput(t *testing.T) {
	m := composeModel{header: "H"}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	got := m2.(composeModel)
	if string(got.current) != "hello" {
		t.Errorf("want current=hello; got %q", string(got.current))
	}
	if got.done {
		t.Error("should not be done after rune input")
	}
}

func TestComposeModel_Update_SpaceKeyInput(t *testing.T) {
	m := composeModel{current: []rune("hello")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := m2.(composeModel)
	if string(got.current) != "hello " {
		t.Errorf("want current=%q; got %q", "hello ", string(got.current))
	}
}

func TestComposeModel_Update_Enter_Submits(t *testing.T) {
	m := composeModel{header: "H", current: []rune("line one")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m2.(composeModel)
	if !got.done {
		t.Fatal("want done=true after enter")
	}
	if got.value() != "line one" {
		t.Errorf("want current line preserved for submission; got %q", got.value())
	}
}

func TestComposeModel_Update_ShiftEnter_InsertsNewline(t *testing.T) {
	m := composeModel{header: "H", current: []rune("line one")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("shift+enter")})
	got := m2.(composeModel)
	if got.done {
		t.Fatal("should not submit on shift+enter")
	}
	if len(got.lines) != 1 || got.lines[0] != "line one" {
		t.Errorf("want lines=[line one]; got %v", got.lines)
	}
	if len(got.current) != 0 {
		t.Errorf("want current cleared; got %q", string(got.current))
	}
}

func TestComposeModel_Update_Backspace(t *testing.T) {
	m := composeModel{current: []rune("abc")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := m2.(composeModel)
	if string(got.current) != "ab" {
		t.Errorf("want current=ab; got %q", string(got.current))
	}
}

func TestComposeModel_Update_Backspace_EmptyCurrent_JoinsPreviousLine(t *testing.T) {
	m := composeModel{lines: []string{"prev"}, current: nil}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := m2.(composeModel)
	if len(got.lines) != 0 {
		t.Errorf("want previous line removed from lines; got %v", got.lines)
	}
	if string(got.current) != "prev" {
		t.Errorf("want current to contain previous line; got %q", string(got.current))
	}
}

func TestComposeModel_Update_CtrlD_Finalizes(t *testing.T) {
	m := composeModel{header: "H", lines: []string{"line one"}, current: []rune("line two")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got := m2.(composeModel)
	if !got.done {
		t.Error("want done=true after ctrl+d")
	}
	if v := got.value(); v != "line one\nline two" {
		t.Errorf("want %q; got %q", "line one\nline two", v)
	}
}

func TestComposeModel_Update_WindowSizeWrapsLongInput(t *testing.T) {
	m := composeModel{header: "H", current: []rune("abcdefghijklmnopqrstuvwxyz")}
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 20})
	got := m2.(composeModel)
	view := got.View()
	for _, want := range []string{"abcdefghijklmnopqrst\n", "uvwxyz_\n"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected wrapped segment %q in view:\n%s", want, view)
		}
	}
}

func TestComposeModel_View_ShowsSubmitAndNewlineHints(t *testing.T) {
	m := composeModel{header: "H", current: []rune("text")}
	view := m.View()
	for _, want := range []string{"Enter submits", "Shift+Enter inserts a newline", "Ctrl+C cancels"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected hint %q in view:\n%s", want, view)
		}
	}
}

func TestComposeModel_Update_CtrlC_ClearsAllContent(t *testing.T) {
	m := composeModel{header: "H", lines: []string{"line one"}, current: []rune("partial")}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := m2.(composeModel)
	if !got.done {
		t.Error("want done=true after ctrl+c")
	}
	if len(got.lines) != 0 || len(got.current) != 0 {
		t.Errorf("want lines and current cleared; got lines=%v current=%q", got.lines, string(got.current))
	}
	if got.value() != "" {
		t.Errorf("want empty value after cancel; got %q", got.value())
	}
}

// ---- Non-interactive error reporting ----

func TestSelectOne_NonTTY_IgnoresInputReader(t *testing.T) {
	// Even when the reader has content, non-interactive mode ignores it and returns default.
	p := NewWithIO(new(bytes.Buffer), strings.NewReader("2\n"), false)
	idx, val, err := p.SelectOne("Pick", []string{"x", "y", "z"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || val != "x" {
		t.Errorf("want idx=0 val=x (default); got idx=%d val=%s", idx, val)
	}
}

func TestConfirm_NonTTY_IgnoresInputReader(t *testing.T) {
	// Even when the reader has "n", non-interactive mode returns the default.
	p := NewWithIO(new(bytes.Buffer), strings.NewReader("n\n"), false)
	got, err := p.Confirm("OK?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("want true (default); non-interactive should ignore reader input")
	}
}

func TestText_NonTTY_IgnoresInputReader(t *testing.T) {
	p := NewWithIO(new(bytes.Buffer), strings.NewReader("typed value\n"), false)
	got, err := p.Text("Name?", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "default" {
		t.Errorf("want default; non-interactive should ignore reader input; got %q", got)
	}
}

// ---- Interface compliance ----

// Compile-time check: both concrete types satisfy Prompter.
var _ Prompter = (*fallbackPrompter)(nil)
var _ Prompter = (*teaPrompter)(nil)
