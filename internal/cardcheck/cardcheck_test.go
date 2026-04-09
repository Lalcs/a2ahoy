package cardcheck

import (
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// helper: returns true when result contains an issue with the given code.
// Thin wrapper over containsCode so Result- and []Issue-typed call sites
// share a single implementation.
func hasCode(r Result, code string) bool {
	return containsCode(r.Issues, code)
}

// helper: returns the first issue with the given code, or a zero Issue.
func findByCode(r Result, code string) Issue {
	for _, iss := range r.Issues {
		if iss.Code == code {
			return iss
		}
	}
	return Issue{}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelInfo, "info"},
		{LevelWarning, "warning"},
		{LevelError, "error"},
		{Level(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestResult_Helpers(t *testing.T) {
	r := Result{Issues: []Issue{
		{Level: LevelError, Code: "E1"},
		{Level: LevelWarning, Code: "W1"},
		{Level: LevelWarning, Code: "W2"},
		{Level: LevelInfo, Code: "I1"},
	}}

	if !r.HasIssues() {
		t.Error("HasIssues should be true")
	}
	if !r.HasErrors() {
		t.Error("HasErrors should be true")
	}
	if got, want := r.Count(LevelError), 1; got != want {
		t.Errorf("Count(Error) = %d, want %d", got, want)
	}
	if got, want := r.Count(LevelWarning), 2; got != want {
		t.Errorf("Count(Warning) = %d, want %d", got, want)
	}
	if got, want := r.Count(LevelInfo), 1; got != want {
		t.Errorf("Count(Info) = %d, want %d", got, want)
	}

	warnings := r.ByLevel(LevelWarning)
	if len(warnings) != 2 {
		t.Fatalf("ByLevel(Warning) returned %d items, want 2", len(warnings))
	}
	if warnings[0].Code != "W1" || warnings[1].Code != "W2" {
		t.Errorf("ByLevel order wrong: got %q, %q; want W1, W2", warnings[0].Code, warnings[1].Code)
	}
}

func TestResult_HasErrors_NoErrors(t *testing.T) {
	r := Result{Issues: []Issue{
		{Level: LevelWarning, Code: "W1"},
		{Level: LevelInfo, Code: "I1"},
	}}
	if r.HasErrors() {
		t.Error("HasErrors should be false when no LevelError issues exist")
	}
}

func TestResult_Empty(t *testing.T) {
	var r Result
	if r.HasIssues() {
		t.Error("empty Result should report HasIssues() == false")
	}
	if r.HasErrors() {
		t.Error("empty Result should report HasErrors() == false")
	}
	if got := r.Count(LevelError); got != 0 {
		t.Errorf("Count on empty = %d, want 0", got)
	}
}

func TestRun_NilCard(t *testing.T) {
	r := Run(nil)
	if r.HasIssues() {
		t.Errorf("nil card should yield empty Result, got %d issues", len(r.Issues))
	}
}

func TestRun_HealthyV1Card_NoIssues(t *testing.T) {
	card := &a2a.AgentCard{
		Name:               "HealthyAgent",
		Description:        "A test agent for validation.",
		Version:            "1.0.0",
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []a2a.AgentSkill{
			{
				ID:          "test-skill",
				Name:        "Test Skill",
				Description: "A skill for testing.",
				Tags:        []string{"test"},
			},
		},
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "https://example.com/a2a",
				ProtocolBinding: a2a.TransportProtocolJSONRPC,
				ProtocolVersion: "1.0",
			},
		},
	}
	r := Run(card)
	if r.HasIssues() {
		t.Errorf("healthy v1 card should produce no issues, got %+v", r.Issues)
	}
}

func TestCheckName(t *testing.T) {
	tests := []struct {
		name     string
		card     *a2a.AgentCard
		wantCode bool
	}{
		{"empty name", &a2a.AgentCard{Name: ""}, true},
		{"whitespace only", &a2a.AgentCard{Name: "   "}, true},
		{"valid name", &a2a.AgentCard{Name: "Test"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkName(tt.card)
			if tt.wantCode && (len(issues) == 0 || issues[0].Code != "EMPTY_NAME") {
				t.Errorf("expected EMPTY_NAME, got %+v", issues)
			}
			if !tt.wantCode && len(issues) != 0 {
				t.Errorf("expected no issues, got %+v", issues)
			}
		})
	}
}

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		name     string
		card     *a2a.AgentCard
		wantCode bool
	}{
		{"empty version", &a2a.AgentCard{Version: ""}, true},
		{"whitespace version", &a2a.AgentCard{Version: " "}, true},
		{"valid version", &a2a.AgentCard{Version: "1.0"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkVersion(tt.card)
			if tt.wantCode {
				if len(issues) == 0 || issues[0].Code != "EMPTY_VERSION" {
					t.Errorf("expected EMPTY_VERSION, got %+v", issues)
				}
				if issues[0].Level != LevelWarning {
					t.Errorf("expected Warning level, got %v", issues[0].Level)
				}
			}
			if !tt.wantCode && len(issues) != 0 {
				t.Errorf("expected no issues, got %+v", issues)
			}
		})
	}
}

func TestCheckSupportedInterfacesEmpty(t *testing.T) {
	tests := []struct {
		name     string
		card     *a2a.AgentCard
		wantCode bool
	}{
		{
			name:     "empty interfaces",
			card:     &a2a.AgentCard{SupportedInterfaces: nil},
			wantCode: true,
		},
		{
			name:     "explicit empty slice",
			card:     &a2a.AgentCard{SupportedInterfaces: []*a2a.AgentInterface{}},
			wantCode: true,
		},
		{
			name: "one interface",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{URL: "http://x", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
				},
			},
			wantCode: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkSupportedInterfacesEmpty(tt.card)
			if tt.wantCode {
				if len(issues) == 0 || issues[0].Code != "EMPTY_SUPPORTED_INTERFACES" {
					t.Errorf("expected EMPTY_SUPPORTED_INTERFACES, got %+v", issues)
				}
				if issues[0].Level != LevelError {
					t.Errorf("expected Error level, got %v", issues[0].Level)
				}
			}
			if !tt.wantCode && len(issues) != 0 {
				t.Errorf("expected no issues, got %+v", issues)
			}
		})
	}
}

func TestCheckInterfaces_InvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool // true if INTERFACE_INVALID_URL should be reported
	}{
		{"empty url", "", true},
		{"whitespace url", "   ", true},
		{"relative url", "/a2a", true},
		{"unknown scheme", "ftp://example.com", true},
		{"no host", "http://", true},
		{"http ok", "http://example.com", false},
		{"https ok", "https://example.com/path", false},
		{"grpc ok", "grpc://example.com:50051", false},
		{"grpcs ok", "grpcs://example.com:443", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{
						URL:             tt.url,
						ProtocolBinding: a2a.TransportProtocolJSONRPC,
						ProtocolVersion: "1.0",
					},
				},
			}
			issues := checkInterfaces(card)
			if tt.want && !containsCode(issues, "INTERFACE_INVALID_URL") {
				t.Errorf("expected INTERFACE_INVALID_URL for %q, got %+v", tt.url, issues)
			}
			if !tt.want && containsCode(issues, "INTERFACE_INVALID_URL") {
				t.Errorf("unexpected INTERFACE_INVALID_URL for %q, got %+v", tt.url, issues)
			}
		})
	}
}

func TestCheckInterfaces_UnparseableURL(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "http://example.com/%zz",
				ProtocolBinding: a2a.TransportProtocolJSONRPC,
				ProtocolVersion: "1.0",
			},
		},
	}
	issues := checkInterfaces(card)
	if !containsCode(issues, "INTERFACE_INVALID_URL") {
		t.Errorf("expected INTERFACE_INVALID_URL for unparseable URL, got %+v", issues)
	}
}

func TestCheckInterfaces_EmptyProtocolVersion(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "https://example.com",
				ProtocolBinding: a2a.TransportProtocolJSONRPC,
				ProtocolVersion: "",
			},
		},
	}
	issues := checkInterfaces(card)
	if !containsCode(issues, "INTERFACE_EMPTY_PROTOCOL_VERSION") {
		t.Errorf("expected INTERFACE_EMPTY_PROTOCOL_VERSION, got %+v", issues)
	}
}

func TestCheckInterfaces_UnknownProtocolBinding(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "https://example.com",
				ProtocolBinding: a2a.TransportProtocol("SOAP"),
				ProtocolVersion: "1.0",
			},
		},
	}
	issues := checkInterfaces(card)
	if !containsCode(issues, "INTERFACE_UNKNOWN_PROTOCOL_BINDING") {
		t.Errorf("expected INTERFACE_UNKNOWN_PROTOCOL_BINDING, got %+v", issues)
	}
}

func TestCheckInterfaces_NilEntrySkipped(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			nil,
			{URL: "https://example.com", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
		},
	}
	// Should not panic, and the nil entry should not produce any issues.
	issues := checkInterfaces(card)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

// TestCheckV03HTTPJSONMissingV1 mirrors TestApplyV03RESTMountPrefix in
// internal/client/client_test.go — the predicate in checkV03HTTPJSONMissingV1
// and applyV03RESTMountPrefix must stay in sync. Drift is detected by this test.
func TestCheckV03HTTPJSONMissingV1(t *testing.T) {
	tests := []struct {
		name        string
		card        *a2a.AgentCard
		wantWarning bool
	}{
		{
			name: "HTTP+JSON v0.3 with trailing slash → warn",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999/",
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: "0.3.0",
				}},
			},
			wantWarning: true,
		},
		{
			name: "HTTP+JSON v0.3 without trailing slash → warn",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999",
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: "0.3.0",
				}},
			},
			wantWarning: true,
		},
		{
			name: "HTTP+JSON v0.3 already ending in /v1 → ok",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999/v1",
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: "0.3.0",
				}},
			},
			wantWarning: false,
		},
		{
			name: "HTTP+JSON v0.3 ending in /v1/ → ok",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999/v1/",
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: "0.3.0",
				}},
			},
			wantWarning: false,
		},
		{
			name: "HTTP+JSON v0.3 short version → warn",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999/",
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: "0.3",
				}},
			},
			wantWarning: true,
		},
		{
			name: "JSONRPC v0.3 → untouched",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999/",
					ProtocolBinding: a2a.TransportProtocolJSONRPC,
					ProtocolVersion: "0.3.0",
				}},
			},
			wantWarning: false,
		},
		{
			name: "HTTP+JSON v1.0 → untouched",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{{
					URL:             "http://localhost:9999/",
					ProtocolBinding: a2a.TransportProtocolHTTPJSON,
					ProtocolVersion: "1.0",
				}},
			},
			wantWarning: false,
		},
		{
			name: "nil interface entry skipped",
			card: &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{nil},
			},
			wantWarning: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkV03HTTPJSONMissingV1(tt.card)
			got := containsCode(issues, "V03_HTTPJSON_MISSING_V1")
			if got != tt.wantWarning {
				t.Errorf("got warning=%v, want %v; issues=%+v", got, tt.wantWarning, issues)
			}
			// When warning fires, the hint should be populated.
			if tt.wantWarning && len(issues) > 0 {
				if issues[0].Hint == "" {
					t.Error("expected non-empty Hint when warning fires")
				}
				if !strings.Contains(issues[0].Hint, "/v1") {
					t.Errorf("expected Hint to mention \"/v1\", got %q", issues[0].Hint)
				}
			}
		})
	}
}

func TestCheckV03HTTPJSONMissingV1_MixedInterfaces(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{URL: "http://a/", ProtocolBinding: a2a.TransportProtocolHTTPJSON, ProtocolVersion: "0.3.0"},
			{URL: "http://b/", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "0.3.0"},
			{URL: "http://c/", ProtocolBinding: a2a.TransportProtocolHTTPJSON, ProtocolVersion: "1.0"},
		},
	}
	issues := checkV03HTTPJSONMissingV1(card)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue (only interface 0 matches), got %d: %+v", len(issues), issues)
	}
	if issues[0].Field != "supportedInterfaces[0].url" {
		t.Errorf("expected field=supportedInterfaces[0].url, got %q", issues[0].Field)
	}
}

func TestRun_Ordering(t *testing.T) {
	// Card designed to trigger: 1 error (empty name), multiple warnings
	// (empty version, empty description, empty modes, empty skills,
	// v0.3 /v1 missing).
	card := &a2a.AgentCard{
		Name:    "", // EMPTY_NAME → error
		Version: "", // EMPTY_VERSION → warning
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "http://localhost:9999",
				ProtocolBinding: a2a.TransportProtocolHTTPJSON,
				ProtocolVersion: "0.3.0",
			},
		},
	}
	r := Run(card)

	// The first issue must be the error.
	if len(r.Issues) == 0 || r.Issues[0].Level != LevelError {
		t.Fatalf("expected first issue to be an error, got %+v", r.Issues)
	}

	// All errors must come before any warnings.
	sawWarning := false
	for _, iss := range r.Issues {
		switch iss.Level {
		case LevelWarning, LevelInfo:
			sawWarning = true
		case LevelError:
			if sawWarning {
				t.Errorf("error %q appeared after warning; ordering violation", iss.Code)
			}
		}
	}

	// Specific codes should be present (original + newly added).
	for _, want := range []string{
		"EMPTY_NAME",
		"EMPTY_VERSION",
		"EMPTY_DESCRIPTION",
		"EMPTY_DEFAULT_INPUT_MODES",
		"EMPTY_DEFAULT_OUTPUT_MODES",
		"EMPTY_SKILLS",
		"V03_HTTPJSON_MISSING_V1",
	} {
		if !hasCode(r, want) {
			t.Errorf("expected issue code %q in Run output", want)
		}
	}
}

func TestRun_V03HTTPJSONCard_Regression(t *testing.T) {
	// Reproduce a typical Python a2a-sdk 0.3.x card that triggers the
	// /v1 warning. This is the precise scenario the user originally
	// asked about. All other required fields are populated so the only
	// issue is the V03 /v1 warning.
	card := &a2a.AgentCard{
		Name:               "adk-agent",
		Description:        "An ADK agent",
		Version:            "0.1.0",
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []a2a.AgentSkill{
			{ID: "echo", Name: "Echo", Description: "Echoes the input", Tags: []string{"echo"}},
		},
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "http://localhost:9999",
				ProtocolBinding: a2a.TransportProtocolHTTPJSON,
				ProtocolVersion: "0.3.0",
			},
		},
	}
	r := Run(card)
	iss := findByCode(r, "V03_HTTPJSON_MISSING_V1")
	if iss.Code == "" {
		t.Fatal("expected V03_HTTPJSON_MISSING_V1 warning")
	}
	if iss.Level != LevelWarning {
		t.Errorf("expected Warning level, got %v", iss.Level)
	}
	if iss.Field != "supportedInterfaces[0].url" {
		t.Errorf("expected field=supportedInterfaces[0].url, got %q", iss.Field)
	}
	// Only the V03 warning should be present; no other issues expected.
	if len(r.Issues) != 1 {
		t.Errorf("expected exactly 1 issue (V03_HTTPJSON_MISSING_V1), got %d: %+v", len(r.Issues), r.Issues)
	}
}

// containsCode reports whether a slice of issues (not a Result) contains
// the given code. Used for single-check tests.
func containsCode(issues []Issue, code string) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Priority B tests
// -----------------------------------------------------------------------------

func TestCheckStreamingCapabilityHasTransport(t *testing.T) {
	tests := []struct {
		name string
		card *a2a.AgentCard
		want bool
	}{
		{
			name: "streaming true, no interfaces",
			card: &a2a.AgentCard{
				Capabilities: a2a.AgentCapabilities{Streaming: true},
			},
			want: true,
		},
		{
			name: "streaming true, has interfaces",
			card: &a2a.AgentCard{
				Capabilities: a2a.AgentCapabilities{Streaming: true},
				SupportedInterfaces: []*a2a.AgentInterface{
					{URL: "http://x", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
				},
			},
			want: false,
		},
		{
			name: "streaming false, no interfaces",
			card: &a2a.AgentCard{
				Capabilities: a2a.AgentCapabilities{Streaming: false},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkStreamingCapabilityHasTransport(tt.card)
			got := containsCode(issues, "STREAMING_NO_COMPATIBLE_TRANSPORT")
			if got != tt.want {
				t.Errorf("got %v, want %v; issues=%+v", got, tt.want, issues)
			}
		})
	}
}

func TestCheckDuplicateInterfaceURLBinding(t *testing.T) {
	tests := []struct {
		name     string
		ifaces   []*a2a.AgentInterface
		wantDups int
	}{
		{
			name: "no duplicates",
			ifaces: []*a2a.AgentInterface{
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
				{URL: "http://b", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
			},
			wantDups: 0,
		},
		{
			name: "same url, same binding → duplicate",
			ifaces: []*a2a.AgentInterface{
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "0.3.0"},
			},
			wantDups: 1,
		},
		{
			name: "same url, different binding → ok",
			ifaces: []*a2a.AgentInterface{
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolHTTPJSON, ProtocolVersion: "1.0"},
			},
			wantDups: 0,
		},
		{
			name: "triple duplicate",
			ifaces: []*a2a.AgentInterface{
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "0.3.0"},
				{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "2.0"},
			},
			wantDups: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &a2a.AgentCard{SupportedInterfaces: tt.ifaces}
			issues := checkDuplicateInterfaceURLBinding(card)
			if len(issues) != tt.wantDups {
				t.Errorf("got %d issues, want %d: %+v", len(issues), tt.wantDups, issues)
			}
		})
	}
}

func TestCheckSkills_DuplicateID(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "translate", Name: "Translate"},
			{ID: "summarize", Name: "Summarize"},
			{ID: "translate", Name: "Translate v2"},
		},
	}
	issues := checkSkills(card)
	if !containsCode(issues, "SKILL_DUPLICATE_ID") {
		t.Errorf("expected SKILL_DUPLICATE_ID, got %+v", issues)
	}
}

func TestCheckSkills_EmptyName(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "translate", Name: ""},
		},
	}
	issues := checkSkills(card)
	if !containsCode(issues, "SKILL_EMPTY_NAME") {
		t.Errorf("expected SKILL_EMPTY_NAME, got %+v", issues)
	}
}

func TestCheckSkills_WhitespaceName(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "translate", Name: "   "},
		},
	}
	issues := checkSkills(card)
	if !containsCode(issues, "SKILL_EMPTY_NAME") {
		t.Errorf("expected SKILL_EMPTY_NAME for whitespace-only name, got %+v", issues)
	}
}

func TestCheckSkills_HealthySkill_NoIssue(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "translate", Name: "Translate", Description: "Translates text"},
		},
	}
	issues := checkSkills(card)
	if len(issues) != 0 {
		t.Errorf("expected no issues for healthy skill, got %+v", issues)
	}
}

func TestCheckDuplicateInterfaceURLBinding_NilEntry(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			nil,
			{URL: "http://a", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
		},
	}
	issues := checkDuplicateInterfaceURLBinding(card)
	if len(issues) != 0 {
		t.Errorf("expected no issues when nil entry is present, got %+v", issues)
	}
}

func TestCheckProtocolVersionRecognized_NilEntry(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			nil,
			{URL: "http://x", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: "1.0"},
		},
	}
	issues := checkProtocolVersionRecognized(card)
	if len(issues) != 0 {
		t.Errorf("expected no issues when nil entry is present, got %+v", issues)
	}
}

func TestCheckProtocolVersionRecognized(t *testing.T) {
	tests := []struct {
		name    string
		version a2a.ProtocolVersion
		want    bool
	}{
		{"v1.0 recognized", "1.0", false},
		{"v1.1.2 recognized", "1.1.2", false},
		{"v0.3 recognized", "0.3", false},
		{"v0.3.0 recognized", "0.3.0", false},
		{"v2.0 unrecognized", "2.0", true},
		{"v0.2 unrecognized", "0.2", true},
		{"garbage unrecognized", "foo", true},
		{"empty skipped (reported elsewhere)", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := &a2a.AgentCard{
				SupportedInterfaces: []*a2a.AgentInterface{
					{URL: "http://x", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: tt.version},
				},
			}
			issues := checkProtocolVersionRecognized(card)
			got := containsCode(issues, "PROTOCOL_VERSION_UNRECOGNIZED")
			if got != tt.want {
				t.Errorf("got %v, want %v; issues=%+v", got, tt.want, issues)
			}
			if tt.want && len(issues) > 0 && issues[0].Level != LevelInfo {
				t.Errorf("expected LevelInfo, got %v", issues[0].Level)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// New check tests (A2AHOY-20)
// -----------------------------------------------------------------------------

func TestCheckRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		card      *a2a.AgentCard
		wantCodes []string
	}{
		{
			name:      "all empty",
			card:      &a2a.AgentCard{},
			wantCodes: []string{"EMPTY_DESCRIPTION", "EMPTY_DEFAULT_INPUT_MODES", "EMPTY_DEFAULT_OUTPUT_MODES"},
		},
		{
			name: "description empty only",
			card: &a2a.AgentCard{
				Description:        "",
				DefaultInputModes:  []string{"text/plain"},
				DefaultOutputModes: []string{"text/plain"},
			},
			wantCodes: []string{"EMPTY_DESCRIPTION"},
		},
		{
			name: "whitespace description",
			card: &a2a.AgentCard{
				Description:        "   ",
				DefaultInputModes:  []string{"text/plain"},
				DefaultOutputModes: []string{"text/plain"},
			},
			wantCodes: []string{"EMPTY_DESCRIPTION"},
		},
		{
			name: "defaultInputModes empty only",
			card: &a2a.AgentCard{
				Description:        "ok",
				DefaultInputModes:  nil,
				DefaultOutputModes: []string{"text/plain"},
			},
			wantCodes: []string{"EMPTY_DEFAULT_INPUT_MODES"},
		},
		{
			name: "defaultOutputModes empty only",
			card: &a2a.AgentCard{
				Description:        "ok",
				DefaultInputModes:  []string{"text/plain"},
				DefaultOutputModes: []string{},
			},
			wantCodes: []string{"EMPTY_DEFAULT_OUTPUT_MODES"},
		},
		{
			name: "all populated",
			card: &a2a.AgentCard{
				Description:        "A valid description",
				DefaultInputModes:  []string{"text/plain"},
				DefaultOutputModes: []string{"text/plain"},
			},
			wantCodes: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkRequiredFields(tt.card)
			for _, code := range tt.wantCodes {
				if !containsCode(issues, code) {
					t.Errorf("expected %s, got %+v", code, issues)
				}
			}
			if tt.wantCodes == nil && len(issues) != 0 {
				t.Errorf("expected no issues, got %+v", issues)
			}
		})
	}
}

func TestCheckSkillsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		card     *a2a.AgentCard
		wantCode bool
	}{
		{"nil skills", &a2a.AgentCard{Skills: nil}, true},
		{"empty slice", &a2a.AgentCard{Skills: []a2a.AgentSkill{}}, true},
		{
			"one skill present",
			&a2a.AgentCard{Skills: []a2a.AgentSkill{{ID: "s1", Name: "S1", Description: "d"}}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkSkillsEmpty(tt.card)
			got := containsCode(issues, "EMPTY_SKILLS")
			if got != tt.wantCode {
				t.Errorf("got %v, want %v; issues=%+v", got, tt.wantCode, issues)
			}
			if tt.wantCode && len(issues) > 0 && issues[0].Level != LevelWarning {
				t.Errorf("expected Warning level, got %v", issues[0].Level)
			}
		})
	}
}

func TestCheckInterfaces_EmptyProtocolBinding(t *testing.T) {
	card := &a2a.AgentCard{
		SupportedInterfaces: []*a2a.AgentInterface{
			{
				URL:             "https://example.com",
				ProtocolBinding: "",
				ProtocolVersion: "1.0",
			},
		},
	}
	issues := checkInterfaces(card)
	if !containsCode(issues, "INTERFACE_EMPTY_PROTOCOL_BINDING") {
		t.Errorf("expected INTERFACE_EMPTY_PROTOCOL_BINDING, got %+v", issues)
	}
	// Should NOT also fire INTERFACE_UNKNOWN_PROTOCOL_BINDING (mutually exclusive).
	if containsCode(issues, "INTERFACE_UNKNOWN_PROTOCOL_BINDING") {
		t.Errorf("unexpected INTERFACE_UNKNOWN_PROTOCOL_BINDING when binding is empty")
	}
}

func TestCheckSkills_EmptyID(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "", Name: "NoID", Description: "desc"},
		},
	}
	issues := checkSkills(card)
	if !containsCode(issues, "SKILL_EMPTY_ID") {
		t.Errorf("expected SKILL_EMPTY_ID, got %+v", issues)
	}
}

func TestCheckSkills_EmptyDescription(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "s1", Name: "S1", Description: ""},
		},
	}
	issues := checkSkills(card)
	if !containsCode(issues, "SKILL_EMPTY_DESCRIPTION") {
		t.Errorf("expected SKILL_EMPTY_DESCRIPTION, got %+v", issues)
	}
}

func TestCheckSkills_EmptyName_NoID(t *testing.T) {
	// SKILL_EMPTY_NAME should fire even when ID is also empty.
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "", Name: "", Description: "desc"},
		},
	}
	issues := checkSkills(card)
	if !containsCode(issues, "SKILL_EMPTY_NAME") {
		t.Errorf("expected SKILL_EMPTY_NAME even without ID, got %+v", issues)
	}
	if !containsCode(issues, "SKILL_EMPTY_ID") {
		t.Errorf("expected SKILL_EMPTY_ID, got %+v", issues)
	}
}

func TestCheckProvider(t *testing.T) {
	tests := []struct {
		name      string
		card      *a2a.AgentCard
		wantCodes []string
	}{
		{
			name:      "nil provider",
			card:      &a2a.AgentCard{},
			wantCodes: nil,
		},
		{
			name: "both populated",
			card: &a2a.AgentCard{
				Provider: &a2a.AgentProvider{Org: "Acme", URL: "https://acme.com"},
			},
			wantCodes: nil,
		},
		{
			name: "empty organization",
			card: &a2a.AgentCard{
				Provider: &a2a.AgentProvider{Org: "", URL: "https://acme.com"},
			},
			wantCodes: []string{"PROVIDER_EMPTY_ORGANIZATION"},
		},
		{
			name: "empty url",
			card: &a2a.AgentCard{
				Provider: &a2a.AgentProvider{Org: "Acme", URL: ""},
			},
			wantCodes: []string{"PROVIDER_EMPTY_URL"},
		},
		{
			name: "both empty",
			card: &a2a.AgentCard{
				Provider: &a2a.AgentProvider{Org: "", URL: ""},
			},
			wantCodes: []string{"PROVIDER_EMPTY_ORGANIZATION", "PROVIDER_EMPTY_URL"},
		},
		{
			name: "whitespace organization",
			card: &a2a.AgentCard{
				Provider: &a2a.AgentProvider{Org: "   ", URL: "https://acme.com"},
			},
			wantCodes: []string{"PROVIDER_EMPTY_ORGANIZATION"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkProvider(tt.card)
			for _, code := range tt.wantCodes {
				if !containsCode(issues, code) {
					t.Errorf("expected %s, got %+v", code, issues)
				}
			}
			if tt.wantCodes == nil && len(issues) != 0 {
				t.Errorf("expected no issues, got %+v", issues)
			}
		})
	}
}
