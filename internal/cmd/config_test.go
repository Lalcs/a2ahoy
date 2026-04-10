package cmd

import (
	"testing"
)

func TestBuildSendConfig_Nil(t *testing.T) {
	tests := []struct {
		name  string
		modes []string
	}{
		{"nil slice", nil},
		{"empty slice", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildSendConfig(tt.modes, false)
			if cfg != nil {
				t.Errorf("buildSendConfig(%v, false) = %+v, want nil", tt.modes, cfg)
			}
		})
	}
}

func TestBuildSendConfig_Populated(t *testing.T) {
	modes := []string{"text/plain", "application/json"}
	cfg := buildSendConfig(modes, false)
	if cfg == nil {
		t.Fatal("buildSendConfig returned nil, want non-nil")
	}
	if len(cfg.AcceptedOutputModes) != 2 {
		t.Fatalf("AcceptedOutputModes length: got %d, want 2", len(cfg.AcceptedOutputModes))
	}
	if cfg.AcceptedOutputModes[0] != "text/plain" {
		t.Errorf("AcceptedOutputModes[0]: got %q, want %q", cfg.AcceptedOutputModes[0], "text/plain")
	}
	if cfg.AcceptedOutputModes[1] != "application/json" {
		t.Errorf("AcceptedOutputModes[1]: got %q, want %q", cfg.AcceptedOutputModes[1], "application/json")
	}
	if cfg.ReturnImmediately {
		t.Error("ReturnImmediately: got true, want false")
	}
}

func TestBuildSendConfig_ReturnImmediately(t *testing.T) {
	cfg := buildSendConfig(nil, true)
	if cfg == nil {
		t.Fatal("buildSendConfig(nil, true) returned nil, want non-nil")
	}
	if !cfg.ReturnImmediately {
		t.Error("ReturnImmediately: got false, want true")
	}
	if len(cfg.AcceptedOutputModes) != 0 {
		t.Errorf("AcceptedOutputModes should be empty, got %v", cfg.AcceptedOutputModes)
	}
}

func TestBuildSendConfig_BothFlags(t *testing.T) {
	modes := []string{"text/plain"}
	cfg := buildSendConfig(modes, true)
	if cfg == nil {
		t.Fatal("buildSendConfig returned nil, want non-nil")
	}
	if !cfg.ReturnImmediately {
		t.Error("ReturnImmediately: got false, want true")
	}
	if len(cfg.AcceptedOutputModes) != 1 {
		t.Fatalf("AcceptedOutputModes length: got %d, want 1", len(cfg.AcceptedOutputModes))
	}
	if cfg.AcceptedOutputModes[0] != "text/plain" {
		t.Errorf("AcceptedOutputModes[0]: got %q, want %q", cfg.AcceptedOutputModes[0], "text/plain")
	}
}
