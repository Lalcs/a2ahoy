package cmd

import (
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
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
			cfg := buildSendConfig(tt.modes, false, nil, nil)
			if cfg != nil {
				t.Errorf("buildSendConfig(%v, false, nil, nil) = %+v, want nil", tt.modes, cfg)
			}
		})
	}
}

func TestBuildSendConfig_Populated(t *testing.T) {
	modes := []string{"text/plain", "application/json"}
	cfg := buildSendConfig(modes, false, nil, nil)
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
	cfg := buildSendConfig(nil, true, nil, nil)
	if cfg == nil {
		t.Fatal("buildSendConfig(nil, true, nil, nil) returned nil, want non-nil")
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
	cfg := buildSendConfig(modes, true, nil, nil)
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

func TestBuildSendConfig_HistoryLength(t *testing.T) {
	h := 10
	cfg := buildSendConfig(nil, false, &h, nil)
	if cfg == nil {
		t.Fatal("buildSendConfig with historyLength returned nil, want non-nil")
	}
	if cfg.HistoryLength == nil || *cfg.HistoryLength != 10 {
		t.Errorf("HistoryLength: got %v, want 10", cfg.HistoryLength)
	}
}

func TestBuildSendConfig_HistoryLengthZero(t *testing.T) {
	h := 0
	cfg := buildSendConfig(nil, false, &h, nil)
	if cfg == nil {
		t.Fatal("buildSendConfig with historyLength=0 returned nil, want non-nil")
	}
	if cfg.HistoryLength == nil || *cfg.HistoryLength != 0 {
		t.Errorf("HistoryLength: got %v, want 0", cfg.HistoryLength)
	}
}

func TestBuildSendConfig_PushConfig(t *testing.T) {
	pc := &a2a.PushConfig{URL: "https://example.com/callback", Token: "tok"}
	cfg := buildSendConfig(nil, false, nil, pc)
	if cfg == nil {
		t.Fatal("buildSendConfig with pushConfig returned nil, want non-nil")
	}
	if cfg.PushConfig == nil {
		t.Fatal("PushConfig: got nil, want non-nil")
	}
	if cfg.PushConfig.URL != "https://example.com/callback" {
		t.Errorf("PushConfig.URL: got %q, want %q", cfg.PushConfig.URL, "https://example.com/callback")
	}
	if cfg.PushConfig.Token != "tok" {
		t.Errorf("PushConfig.Token: got %q, want %q", cfg.PushConfig.Token, "tok")
	}
}

func TestBuildPushConfig_Empty(t *testing.T) {
	if pc := buildPushConfig("", ""); pc != nil {
		t.Errorf("buildPushConfig(\"\", \"\") = %+v, want nil", pc)
	}
}

func TestBuildPushConfig_WithURL(t *testing.T) {
	pc := buildPushConfig("https://example.com/push", "my-token")
	if pc == nil {
		t.Fatal("buildPushConfig returned nil, want non-nil")
	}
	if pc.URL != "https://example.com/push" {
		t.Errorf("URL: got %q, want %q", pc.URL, "https://example.com/push")
	}
	if pc.Token != "my-token" {
		t.Errorf("Token: got %q, want %q", pc.Token, "my-token")
	}
}

func TestParseMetadata_Nil(t *testing.T) {
	m, err := parseMetadata(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Errorf("parseMetadata(nil) = %v, want nil", m)
	}
}

func TestParseMetadata_Valid(t *testing.T) {
	m, err := parseMetadata([]string{"key1=value1", "key2=value2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("length: got %d, want 2", len(m))
	}
	if m["key1"] != "value1" {
		t.Errorf("key1: got %v, want %q", m["key1"], "value1")
	}
	if m["key2"] != "value2" {
		t.Errorf("key2: got %v, want %q", m["key2"], "value2")
	}
}

func TestParseMetadata_ValueWithEquals(t *testing.T) {
	m, err := parseMetadata([]string{"key=val=ue"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["key"] != "val=ue" {
		t.Errorf("key: got %v, want %q", m["key"], "val=ue")
	}
}

func TestParseMetadata_Invalid(t *testing.T) {
	_, err := parseMetadata([]string{"no-equals"})
	if err == nil {
		t.Error("parseMetadata(\"no-equals\") should return error")
	}
}

func TestToTaskIDs_Nil(t *testing.T) {
	ids := toTaskIDs(nil)
	if ids != nil {
		t.Errorf("toTaskIDs(nil) = %v, want nil", ids)
	}
}

func TestToTaskIDs_Empty(t *testing.T) {
	ids := toTaskIDs([]string{})
	if ids != nil {
		t.Errorf("toTaskIDs([]) = %v, want nil", ids)
	}
}

func TestToTaskIDs_Populated(t *testing.T) {
	ids := toTaskIDs([]string{"id-1", "id-2"})
	if len(ids) != 2 {
		t.Fatalf("length: got %d, want 2", len(ids))
	}
	if ids[0] != "id-1" {
		t.Errorf("ids[0]: got %q, want %q", ids[0], "id-1")
	}
}
