package config_test

import (
	"testing"

	"github.com/robertgumeny/doug/internal/config"
)

// ---------------------------------------------------------------------------
// ValidateInteractionMode tests
// ---------------------------------------------------------------------------

func TestValidateInteractionMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{name: "empty string is valid (unset)", mode: "", wantErr: false},
		{name: "interactive is valid", mode: config.InteractionModeInteractive, wantErr: false},
		{name: "rpc is valid", mode: config.InteractionModeRPC, wantErr: false},
		{name: "legacy mode is invalid", mode: "legacy", wantErr: true},
		{name: "docker is invalid", mode: "docker", wantErr: true},
		{name: "arbitrary string is invalid", mode: "some-mode", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateInteractionMode(tt.mode)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateInteractionMode(%q) = nil, want error", tt.mode)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateInteractionMode(%q) = %v, want nil", tt.mode, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidatePhaseInteractionMode tests
// ---------------------------------------------------------------------------

func TestValidatePhaseInteractionMode(t *testing.T) {
	tests := []struct {
		name    string
		phase   string
		mode    string
		wantErr bool
	}{
		{name: "valid rpc for runtime", phase: "runtime", mode: "rpc", wantErr: false},
		{name: "valid interactive for planning", phase: "planning", mode: "interactive", wantErr: false},
		{name: "empty is valid", phase: "runtime", mode: "", wantErr: false},
		{name: "invalid mode names phase in error", phase: "runtime", mode: "docker", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidatePhaseInteractionMode(tt.phase, tt.mode)
			if tt.wantErr && err == nil {
				t.Errorf("ValidatePhaseInteractionMode(%q, %q) = nil, want error", tt.phase, tt.mode)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePhaseInteractionMode(%q, %q) = %v, want nil", tt.phase, tt.mode, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultInteractionModeForPhase tests
// ---------------------------------------------------------------------------

func TestDefaultInteractionModeForPhase(t *testing.T) {
	tests := []struct {
		phase string
		want  string
	}{
		{"planning", config.InteractionModeInteractive},
		{"runtime", config.InteractionModeRPC},
		{"scaffold", config.InteractionModeRPC},
		{"research", config.InteractionModeRPC},
		{"post_epic_review", config.InteractionModeRPC},
		{"post_epic_kb", config.InteractionModeRPC},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			got := config.DefaultInteractionModeForPhase(tt.phase)
			if got != tt.want {
				t.Errorf("DefaultInteractionModeForPhase(%q) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Interaction mode constant values
// ---------------------------------------------------------------------------

func TestInteractionModeConstants(t *testing.T) {
	if config.InteractionModeInteractive != "interactive" {
		t.Errorf("InteractionModeInteractive = %q, want %q", config.InteractionModeInteractive, "interactive")
	}
	if config.InteractionModeRPC != "rpc" {
		t.Errorf("InteractionModeRPC = %q, want %q", config.InteractionModeRPC, "rpc")
	}
}
