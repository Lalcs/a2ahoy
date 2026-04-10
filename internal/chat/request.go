package chat

import (
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// BuildChatRequest constructs a SendMessageRequest for the given text
// and optional extra parts (e.g. file attachments). It chooses between
// a fresh a2a.NewMessage (when the state has no active task/context)
// and a2a.NewMessageForTask (when continuing an existing conversation).
// The resulting request is ready to hand to A2AClient.SendMessage or
// A2AClient.SendStreamingMessage.
//
// cfg, when non-nil, is attached as the request's Configuration so the
// agent knows which output MIME types the client accepts. Pass nil to
// omit the configuration key from the JSON-RPC request (the default
// when --accepted-output-mode is not specified).
//
// The text part is always placed first so agents see the user's
// instruction before any attached data. The variadic extraParts
// parameter preserves backward compatibility — existing callers that
// pass no extra parts continue to work identically.
func BuildChatRequest(state *State, text string, cfg *a2a.SendMessageConfig, extraParts ...*a2a.Part) *a2a.SendMessageRequest {
	parts := make([]*a2a.Part, 0, 1+len(extraParts))
	parts = append(parts, a2a.NewTextPart(text))
	parts = append(parts, extraParts...)

	var msg *a2a.Message
	if state.IsFresh() {
		msg = a2a.NewMessage(a2a.MessageRoleUser, parts...)
	} else {
		msg = a2a.NewMessageForTask(a2a.MessageRoleUser, state.TaskInfo(), parts...)
	}
	return &a2a.SendMessageRequest{Message: msg, Config: cfg}
}
