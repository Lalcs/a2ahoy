package chat

import (
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestParseInputLine(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantText  string
		wantSlash bool
		wantName  string
		wantArg   string
	}{
		{"empty", "", "", false, "", ""},
		{"whitespace only", "   \t  ", "", false, "", ""},
		{"plain text", "hello", "hello", false, "", ""},
		{"plain text with spaces", "  hello world  ", "hello world", false, "", ""},
		{"japanese text", "こんにちは", "こんにちは", false, "", ""},
		{"exit", "/exit", "/exit", true, "exit", ""},
		{"exit uppercase", "/EXIT", "/EXIT", true, "exit", ""},
		{"exit mixed case", "/Exit", "/Exit", true, "exit", ""},
		{"quit", "/quit", "/quit", true, "quit", ""},
		{"new", "/new", "/new", true, "new", ""},
		{"help", "/help", "/help", true, "help", ""},
		{"cancel no arg", "/cancel", "/cancel", true, "cancel", ""},
		{"get no arg", "/get", "/get", true, "get", ""},
		{"get with arg", "/get task-123", "/get task-123", true, "get", "task-123"},
		{"get with extra spaces", "/get   task-123   ", "/get   task-123", true, "get", "task-123"},
		{"cancel with arg", "/cancel task-abc", "/cancel task-abc", true, "cancel", "task-abc"},
		{"leading whitespace slash", "  /help", "/help", true, "help", ""},
		{"unknown command", "/unknown foo bar", "/unknown foo bar", true, "unknown", "foo bar"},
		// A bare "/" technically is a slash with an empty name; acceptable — handler will reject.
		{"bare slash", "/", "/", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, isSlash, sc := ParseInputLine(tt.input)
			if text != tt.wantText {
				t.Errorf("text: got %q, want %q", text, tt.wantText)
			}
			if isSlash != tt.wantSlash {
				t.Errorf("isSlash: got %v, want %v", isSlash, tt.wantSlash)
			}
			if sc.Name != tt.wantName {
				t.Errorf("sc.Name: got %q, want %q", sc.Name, tt.wantName)
			}
			if sc.Arg != tt.wantArg {
				t.Errorf("sc.Arg: got %q, want %q", sc.Arg, tt.wantArg)
			}
		})
	}
}

func TestState_Fresh(t *testing.T) {
	var s State
	if !s.IsFresh() {
		t.Error("zero-value State should be fresh")
	}
	if ti := s.TaskInfo(); ti.TaskID != "" || ti.ContextID != "" {
		t.Errorf("fresh State TaskInfo: got %+v, want zero", ti)
	}
}

// stateWith returns a State pre-populated with the given identifiers.
// It exists because State's fields are unexported, so tests cannot use
// a struct literal to seed an established conversation.
func stateWith(taskID a2a.TaskID, contextID string) State {
	var s State
	s.Update(a2a.TaskInfo{TaskID: taskID, ContextID: contextID})
	return s
}

func TestState_Update(t *testing.T) {
	var s State
	s.Update(a2a.TaskInfo{TaskID: "task-1", ContextID: "ctx-1"})
	if s.IsFresh() {
		t.Error("State should not be fresh after Update")
	}
	if s.TaskID() != "task-1" {
		t.Errorf("TaskID: got %q, want %q", s.TaskID(), "task-1")
	}
	if s.ContextID() != "ctx-1" {
		t.Errorf("ContextID: got %q, want %q", s.ContextID(), "ctx-1")
	}
}

func TestState_Update_PartialPreservesExisting(t *testing.T) {
	s := stateWith("task-1", "ctx-1")

	// Update with only TaskID; ContextID should be preserved.
	s.Update(a2a.TaskInfo{TaskID: "task-2"})
	if s.TaskID() != "task-2" {
		t.Errorf("TaskID: got %q, want %q", s.TaskID(), "task-2")
	}
	if s.ContextID() != "ctx-1" {
		t.Errorf("ContextID should be preserved: got %q, want %q", s.ContextID(), "ctx-1")
	}

	// Update with only ContextID; TaskID should be preserved.
	s.Update(a2a.TaskInfo{ContextID: "ctx-2"})
	if s.TaskID() != "task-2" {
		t.Errorf("TaskID should be preserved: got %q, want %q", s.TaskID(), "task-2")
	}
	if s.ContextID() != "ctx-2" {
		t.Errorf("ContextID: got %q, want %q", s.ContextID(), "ctx-2")
	}
}

func TestState_Update_EmptyIgnored(t *testing.T) {
	s := stateWith("task-1", "ctx-1")
	s.Update(a2a.TaskInfo{}) // zero-value should be a no-op
	if s.TaskID() != "task-1" || s.ContextID() != "ctx-1" {
		t.Errorf("empty Update should be no-op, got task=%q ctx=%q", s.TaskID(), s.ContextID())
	}
}

func TestState_Reset(t *testing.T) {
	s := stateWith("task-1", "ctx-1")
	s.Reset()
	if !s.IsFresh() {
		t.Error("Reset should return State to fresh")
	}
}

func TestState_TaskInfoRoundtrip(t *testing.T) {
	s := stateWith("task-x", "ctx-x")
	ti := s.TaskInfo()
	if ti.TaskID != "task-x" || ti.ContextID != "ctx-x" {
		t.Errorf("TaskInfo roundtrip: got %+v", ti)
	}

	// TaskInfo should itself satisfy TaskInfoProvider.
	var _ a2a.TaskInfoProvider = ti
}

func TestState_ResolveTaskID(t *testing.T) {
	tests := []struct {
		name    string
		state   State
		arg     string
		verb    string
		want    a2a.TaskID
		wantErr bool
	}{
		{"explicit arg wins over state", stateWith("task-state", ""), "task-arg", "get", "task-arg", false},
		{"falls back to state", stateWith("task-state", ""), "", "get", "task-state", false},
		{"empty arg and fresh state errors", State{}, "", "cancel", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.state.ResolveTaskID(tt.arg, tt.verb)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err: got %v, want error=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("id: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildChatRequest_Fresh(t *testing.T) {
	var s State
	req := BuildChatRequest(&s, "hello")
	if req == nil || req.Message == nil {
		t.Fatal("BuildChatRequest returned nil request or message")
	}
	if req.Message.TaskID != "" {
		t.Errorf("fresh message TaskID should be empty, got %q", req.Message.TaskID)
	}
	if req.Message.ContextID != "" {
		t.Errorf("fresh message ContextID should be empty, got %q", req.Message.ContextID)
	}
	if req.Message.Role != a2a.MessageRoleUser {
		t.Errorf("message Role: got %q, want %q", req.Message.Role, a2a.MessageRoleUser)
	}
	if len(req.Message.Parts) != 1 {
		t.Fatalf("message Parts: got %d, want 1", len(req.Message.Parts))
	}
	if got := req.Message.Parts[0].Text(); got != "hello" {
		t.Errorf("message text: got %q, want %q", got, "hello")
	}
}

func TestBuildChatRequest_Continuation(t *testing.T) {
	s := stateWith("task-9", "ctx-9")
	req := BuildChatRequest(&s, "follow up")
	if req == nil || req.Message == nil {
		t.Fatal("BuildChatRequest returned nil request or message")
	}
	if req.Message.TaskID != "task-9" {
		t.Errorf("message TaskID: got %q, want %q", req.Message.TaskID, "task-9")
	}
	if req.Message.ContextID != "ctx-9" {
		t.Errorf("message ContextID: got %q, want %q", req.Message.ContextID, "ctx-9")
	}
	if got := req.Message.Parts[0].Text(); got != "follow up" {
		t.Errorf("message text: got %q, want %q", got, "follow up")
	}
}

func TestBuildChatRequest_WithExtraParts(t *testing.T) {
	var s State
	extra1 := a2a.NewRawPart([]byte("file-data"))
	extra1.Filename = "test.png"
	extra2 := a2a.NewFileURLPart("https://example.com/doc.pdf", "")

	req := BuildChatRequest(&s, "analyze", extra1, extra2)
	if req == nil || req.Message == nil {
		t.Fatal("BuildChatRequest returned nil request or message")
	}
	if len(req.Message.Parts) != 3 {
		t.Fatalf("message Parts: got %d, want 3", len(req.Message.Parts))
	}
	// Text part first.
	if got := req.Message.Parts[0].Text(); got != "analyze" {
		t.Errorf("parts[0].Text() = %q, want %q", got, "analyze")
	}
	// File part second.
	if got := req.Message.Parts[1].Filename; got != "test.png" {
		t.Errorf("parts[1].Filename = %q, want %q", got, "test.png")
	}
	// URL part third.
	if got := req.Message.Parts[2].URL(); got != "https://example.com/doc.pdf" {
		t.Errorf("parts[2].URL() = %q, want %q", got, "https://example.com/doc.pdf")
	}
}

func TestBuildChatRequest_ContinuationWithExtraParts(t *testing.T) {
	s := stateWith("task-10", "ctx-10")
	extra := a2a.NewRawPart([]byte("data"))

	req := BuildChatRequest(&s, "more info", extra)
	if req == nil || req.Message == nil {
		t.Fatal("BuildChatRequest returned nil request or message")
	}
	if req.Message.TaskID != "task-10" {
		t.Errorf("message TaskID: got %q, want %q", req.Message.TaskID, "task-10")
	}
	if len(req.Message.Parts) != 2 {
		t.Fatalf("message Parts: got %d, want 2", len(req.Message.Parts))
	}
	if got := req.Message.Parts[0].Text(); got != "more info" {
		t.Errorf("parts[0].Text() = %q, want %q", got, "more info")
	}
}

func TestFilterSuggestions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantOne string // when wantLen == 1, assert the single name
	}{
		{"empty", "", 0, ""},
		{"no slash", "hello", 0, ""},
		{"bare slash matches all", "/", len(AllSuggestions), ""},
		{"slash n matches /new", "/n", 1, "/new"},
		{"slash e matches /exit", "/e", 1, "/exit"},
		{"slash q matches /quit", "/q", 1, "/quit"},
		{"slash h matches /help", "/h", 1, "/help"},
		{"slash c matches /cancel", "/c", 1, "/cancel"},
		{"slash g matches /get", "/g", 1, "/get"},
		{"full match /new", "/new", 1, "/new"},
		{"uppercase", "/EXIT", 1, "/exit"},
		{"no match", "/xyz", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterSuggestions(tt.input)
			if len(got) != tt.wantLen {
				t.Fatalf("len: got %d, want %d (result: %+v)", len(got), tt.wantLen, got)
			}
			if tt.wantLen == 1 && got[0].Name != tt.wantOne {
				t.Errorf("Name: got %q, want %q", got[0].Name, tt.wantOne)
			}
		})
	}
}

func TestAllSuggestions_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range AllSuggestions {
		if seen[s.Name] {
			t.Errorf("duplicate suggestion: %q", s.Name)
		}
		seen[s.Name] = true
	}
}
