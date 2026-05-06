package interactive

import (
	"bytes"
	"strings"
	"testing"
)

// All tests use NewWithIO with isTTY=false so they exercise the fallbackPrompter
// path without requiring a real terminal or a running Bubble Tea program.

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

// ---- Interface compliance ----

// Compile-time check: both concrete types satisfy Prompter.
var _ Prompter = (*fallbackPrompter)(nil)
var _ Prompter = (*teaPrompter)(nil)
