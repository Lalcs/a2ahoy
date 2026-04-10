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
			cfg := buildSendConfig(tt.modes)
			if cfg != nil {
				t.Errorf("buildSendConfig(%v) = %+v, want nil", tt.modes, cfg)
			}
		})
	}
}

func TestBuildSendConfig_Populated(t *testing.T) {
	modes := []string{"text/plain", "application/json"}
	cfg := buildSendConfig(modes)
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
}
