package presenter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestPrintAgentCard_Minimal(t *testing.T) {
	card := &a2a.AgentCard{
		Name:        "TestAgent",
		Description: "A test agent",
		Version:     "1.0.0",
		Capabilities: a2a.AgentCapabilities{
			Streaming:         false,
			PushNotifications: false,
			ExtendedAgentCard: false,
		},
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"=== Agent Card ===",
		"Name:        TestAgent",
		"Description: A test agent",
		"Version:     1.0.0",
		"--- Capabilities ---",
		"Streaming:          false",
		"Push Notifications: false",
		"Extended Card:      false",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintAgentCard_WithProvider(t *testing.T) {
	card := &a2a.AgentCard{
		Name:        "TestAgent",
		Description: "Test",
		Version:     "1.0",
		Provider: &a2a.AgentProvider{
			Org: "TestOrg",
			URL: "https://example.com",
		},
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Provider:    TestOrg (https://example.com)") {
		t.Errorf("missing provider in output:\n%s", got)
	}
}

func TestPrintAgentCard_WithDocumentationURL(t *testing.T) {
	card := &a2a.AgentCard{
		Name:             "TestAgent",
		Description:      "Test",
		Version:          "1.0",
		DocumentationURL: "https://docs.example.com",
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Docs:        https://docs.example.com") {
		t.Errorf("missing docs URL in output:\n%s", got)
	}
}

func TestPrintAgentCard_WithInterfaces(t *testing.T) {
	card := &a2a.AgentCard{
		Name:        "TestAgent",
		Description: "Test",
		Version:     "1.0",
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "https://example.com/a2a",
				ProtocolBinding: a2a.TransportProtocolJSONRPC,
				ProtocolVersion: "1.0",
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"--- Interfaces ---",
		"[JSONRPC]",
		"https://example.com/a2a",
		"(v1.0)",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintAgentCard_WithModes(t *testing.T) {
	card := &a2a.AgentCard{
		Name:               "TestAgent",
		Description:        "Test",
		Version:            "1.0",
		DefaultInputModes:  []string{"text/plain", "application/json"},
		DefaultOutputModes: []string{"text/plain"},
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"--- Default Input Modes ---",
		"text/plain, application/json",
		"--- Default Output Modes ---",
		"text/plain",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintAgentCard_WithSkills(t *testing.T) {
	card := &a2a.AgentCard{
		Name:        "TestAgent",
		Description: "Test",
		Version:     "1.0",
		Skills: []a2a.AgentSkill{
			{
				ID:          "skill-1",
				Name:        "Translate",
				Description: "Translates text",
				Tags:        []string{"nlp", "translation"},
				Examples:    []string{"Translate hello to Japanese", "Translate bonjour to English"},
			},
			{
				ID:   "skill-2",
				Name: "Summarize",
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	checks := []string{
		"--- Skills (2) ---",
		"[1] Translate (id: skill-1)",
		"Description: Translates text",
		"Tags: nlp, translation",
		"Examples:",
		"- Translate hello to Japanese",
		"- Translate bonjour to English",
		"[2] Summarize (id: skill-2)",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintAgentCard_NoProviderNoDocsNoInterfacesNoModesNoSkills(t *testing.T) {
	card := &a2a.AgentCard{
		Name:        "MinimalAgent",
		Description: "Minimal",
		Version:     "0.1",
	}

	var buf bytes.Buffer
	if err := PrintAgentCard(&buf, card); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	notExpected := []string{
		"Provider:",
		"Docs:",
		"--- Interfaces ---",
		"--- Default Input Modes ---",
		"--- Default Output Modes ---",
		"--- Skills",
	}
	for _, notWant := range notExpected {
		if strings.Contains(got, notWant) {
			t.Errorf("unexpected %q in output:\n%s", notWant, got)
		}
	}
}
