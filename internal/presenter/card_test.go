package presenter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Lalcs/a2ahoy/internal/cardcheck"
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

// -----------------------------------------------------------------------------
// PrintValidation / PrintValidationSummary tests
// -----------------------------------------------------------------------------

func TestPrintValidation_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintValidation(&buf, cardcheck.Result{})
	// Critical: must write nothing so minimal-card display stays clean.
	if got := buf.String(); got != "" {
		t.Errorf("expected empty output for empty result, got %q", got)
	}
}

func TestPrintValidation_SingleWarning(t *testing.T) {
	result := cardcheck.Result{Issues: []cardcheck.Issue{
		{
			Level:   cardcheck.LevelWarning,
			Code:    "V03_HTTPJSON_MISSING_V1",
			Message: "HTTP+JSON interface advertises A2A v0.3 but URL lacks /v1.",
			Field:   "supportedInterfaces[0].url",
			Hint:    "append \"/v1\" to the URL.",
		},
	}}

	var buf bytes.Buffer
	PrintValidation(&buf, result)

	got := buf.String()
	checks := []string{
		"--- Validation (1 warning) ---",
		"[WARN]",
		"V03_HTTPJSON_MISSING_V1",
		"HTTP+JSON interface advertises A2A v0.3 but URL lacks /v1.",
		"field: supportedInterfaces[0].url",
		"hint:  append \"/v1\" to the URL.",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestPrintValidation_MultipleLevels(t *testing.T) {
	result := cardcheck.Result{Issues: []cardcheck.Issue{
		{Level: cardcheck.LevelError, Code: "EMPTY_NAME", Message: "no name", Field: "name"},
		{Level: cardcheck.LevelError, Code: "EMPTY_SUPPORTED_INTERFACES", Message: "no interfaces", Field: "supportedInterfaces"},
		{Level: cardcheck.LevelWarning, Code: "EMPTY_VERSION", Message: "no version", Field: "version"},
		{Level: cardcheck.LevelInfo, Code: "PROTOCOL_VERSION_UNRECOGNIZED", Message: "custom version", Field: "supportedInterfaces[0].protocolVersion"},
	}}

	var buf bytes.Buffer
	PrintValidation(&buf, result)

	got := buf.String()
	// Header should include pluralized counts in Error → Warning → Info order.
	if !strings.Contains(got, "--- Validation (2 errors, 1 warning, 1 info) ---") {
		t.Errorf("expected header with \"2 errors, 1 warning, 1 info\" in output:\n%s", got)
	}

	// All level tags must be present.
	for _, tag := range []string{"[ERROR]", "[WARN]", "[INFO]"} {
		if !strings.Contains(got, tag) {
			t.Errorf("missing %q in output:\n%s", tag, got)
		}
	}

	// Errors must appear before warnings before infos (check positional
	// order of the representative codes).
	posError := strings.Index(got, "EMPTY_NAME")
	posWarn := strings.Index(got, "EMPTY_VERSION")
	posInfo := strings.Index(got, "PROTOCOL_VERSION_UNRECOGNIZED")
	if !(posError < posWarn && posWarn < posInfo) {
		t.Errorf("issue ordering wrong: error=%d warn=%d info=%d\n%s", posError, posWarn, posInfo, got)
	}
}

func TestPrintValidation_IssueWithoutFieldOrHint(t *testing.T) {
	result := cardcheck.Result{Issues: []cardcheck.Issue{
		{
			Level:   cardcheck.LevelWarning,
			Code:    "SOMETHING",
			Message: "a message with no field and no hint",
		},
	}}

	var buf bytes.Buffer
	PrintValidation(&buf, result)

	got := buf.String()
	if !strings.Contains(got, "SOMETHING") {
		t.Errorf("missing code in output:\n%s", got)
	}
	if !strings.Contains(got, "a message with no field and no hint") {
		t.Errorf("missing message in output:\n%s", got)
	}
	// Field and hint labels should NOT appear when those fields are empty.
	if strings.Contains(got, "field:") {
		t.Errorf("unexpected field label in output:\n%s", got)
	}
	if strings.Contains(got, "hint:") {
		t.Errorf("unexpected hint label in output:\n%s", got)
	}
}

func TestPrintValidationSummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintValidationSummary(&buf, cardcheck.Result{})
	if got := buf.String(); got != "" {
		t.Errorf("expected empty output for empty result, got %q", got)
	}
}

func TestPrintValidationSummary_WithIssues(t *testing.T) {
	result := cardcheck.Result{Issues: []cardcheck.Issue{
		{Level: cardcheck.LevelError, Code: "EMPTY_NAME", Field: "name"},
		{Level: cardcheck.LevelWarning, Code: "V03_HTTPJSON_MISSING_V1", Field: "supportedInterfaces[0].url"},
	}}

	var buf bytes.Buffer
	PrintValidationSummary(&buf, result)
	got := buf.String()

	checks := []string{
		"a2ahoy card: error: EMPTY_NAME name",
		"a2ahoy card: warning: V03_HTTPJSON_MISSING_V1 supportedInterfaces[0].url",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in summary output:\n%s", want, got)
		}
	}
	// Each issue should be on its own line.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestPrintValidationSummary_IssueWithoutField(t *testing.T) {
	result := cardcheck.Result{Issues: []cardcheck.Issue{
		{Level: cardcheck.LevelWarning, Code: "SOMETHING"},
	}}

	var buf bytes.Buffer
	PrintValidationSummary(&buf, result)
	got := buf.String()
	// When field is empty, the summary should show "-" as a placeholder.
	if !strings.Contains(got, "a2ahoy card: warning: SOMETHING -") {
		t.Errorf("missing placeholder field in summary: %q", got)
	}
}

func TestStyledIssueLevel(t *testing.T) {
	// color.NoColor is true via TestMain so we get plain text.
	// Compare on trimmed tokens so trivial alignment-padding changes
	// do not break the test — padding is layout, not contract.
	tests := []struct {
		level cardcheck.Level
		want  string
	}{
		{cardcheck.LevelError, "[ERROR]"},
		{cardcheck.LevelWarning, "[WARN]"},
		{cardcheck.LevelInfo, "[INFO]"},
		{cardcheck.Level(99), "[?]"},
	}
	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			got := strings.TrimSpace(styledIssueLevel(tt.level))
			if got != tt.want {
				t.Errorf("styledIssueLevel(%v) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}
