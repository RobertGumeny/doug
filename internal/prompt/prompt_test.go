package prompt

import (
	"bytes"
	"strings"
	"testing"
)

// ---- SelectOne ----

func TestSelectOne_NonTTY_ReturnsDefault(t *testing.T) {
	idx, val, err := SelectOne(new(bytes.Buffer), strings.NewReader(""), false, "Pick one", []string{"a", "b", "c"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || val != "b" {
		t.Errorf("want idx=1 val=b; got idx=%d val=%s", idx, val)
	}
}

func TestSelectOne_TTY_ValidChoice(t *testing.T) {
	var out bytes.Buffer
	idx, val, err := SelectOne(&out, strings.NewReader("2\n"), true, "Pick one", []string{"go", "npm", "pnpm"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 || val != "npm" {
		t.Errorf("want idx=1 val=npm; got idx=%d val=%s", idx, val)
	}
	if !strings.Contains(out.String(), "Pick one") {
		t.Error("expected question text in output")
	}
}

func TestSelectOne_TTY_EmptyInputReturnsDefault(t *testing.T) {
	idx, val, err := SelectOne(new(bytes.Buffer), strings.NewReader("\n"), true, "Pick", []string{"x", "y"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || val != "x" {
		t.Errorf("want default; got idx=%d val=%s", idx, val)
	}
}

func TestSelectOne_TTY_OutOfRangeReturnsDefault(t *testing.T) {
	idx, val, err := SelectOne(new(bytes.Buffer), strings.NewReader("99\n"), true, "Pick", []string{"x", "y"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 0 || val != "x" {
		t.Errorf("want default; got idx=%d val=%s", idx, val)
	}
}

func TestSelectOne_EmptyOptions_ReturnsError(t *testing.T) {
	_, _, err := SelectOne(new(bytes.Buffer), strings.NewReader(""), true, "Pick", []string{}, 0)
	if err == nil {
		t.Fatal("expected error for empty options")
	}
}

// ---- Confirm ----

func TestConfirm_NonTTY_ReturnsDefault(t *testing.T) {
	got, err := Confirm(new(bytes.Buffer), strings.NewReader(""), false, "Continue?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true (defaultYes)")
	}
}

func TestConfirm_TTY_Yes(t *testing.T) {
	for _, input := range []string{"y\n", "yes\n", "Y\n", "YES\n"} {
		got, err := Confirm(new(bytes.Buffer), strings.NewReader(input), true, "Continue?", false)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if !got {
			t.Errorf("input %q: expected true", input)
		}
	}
}

func TestConfirm_TTY_No(t *testing.T) {
	for _, input := range []string{"n\n", "no\n", "N\n", "NO\n"} {
		got, err := Confirm(new(bytes.Buffer), strings.NewReader(input), true, "Continue?", true)
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", input, err)
		}
		if got {
			t.Errorf("input %q: expected false", input)
		}
	}
}

func TestConfirm_TTY_EmptyInputReturnsDefault(t *testing.T) {
	got, err := Confirm(new(bytes.Buffer), strings.NewReader("\n"), true, "Continue?", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected defaultYes=true")
	}
}

func TestConfirm_TTY_HintDisplayed(t *testing.T) {
	var out bytes.Buffer
	_, _ = Confirm(&out, strings.NewReader("\n"), true, "Really?", true)
	if !strings.Contains(out.String(), "[Y/n]") {
		t.Errorf("expected [Y/n] hint in output, got: %s", out.String())
	}

	out.Reset()
	_, _ = Confirm(&out, strings.NewReader("\n"), true, "Really?", false)
	if !strings.Contains(out.String(), "[y/N]") {
		t.Errorf("expected [y/N] hint in output, got: %s", out.String())
	}
}

// ---- Text ----

func TestText_NonTTY_ReturnsDefault(t *testing.T) {
	got, err := Text(new(bytes.Buffer), strings.NewReader(""), false, "Name?", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice" {
		t.Errorf("want alice; got %s", got)
	}
}

func TestText_TTY_UserInput(t *testing.T) {
	got, err := Text(new(bytes.Buffer), strings.NewReader("bob\n"), true, "Name?", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "bob" {
		t.Errorf("want bob; got %s", got)
	}
}

func TestText_TTY_EmptyInputReturnsDefault(t *testing.T) {
	got, err := Text(new(bytes.Buffer), strings.NewReader("\n"), true, "Name?", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice" {
		t.Errorf("want alice; got %s", got)
	}
}

func TestText_TTY_DefaultShownInPrompt(t *testing.T) {
	var out bytes.Buffer
	_, _ = Text(&out, strings.NewReader("\n"), true, "Name?", "alice")
	if !strings.Contains(out.String(), "[alice]") {
		t.Errorf("expected default value in prompt, got: %s", out.String())
	}
}

func TestText_TTY_NoDefaultNobrackets(t *testing.T) {
	var out bytes.Buffer
	_, _ = Text(&out, strings.NewReader("\n"), true, "Name?", "")
	if strings.Contains(out.String(), "[") {
		t.Errorf("no brackets expected when no default, got: %s", out.String())
	}
}
