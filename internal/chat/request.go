package chat

import (
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// BuildChatRequest constructs a SendMessageRequest for the given text.
// It chooses between a fresh a2a.NewMessage (when the state has no
// active task/context) and a2a.NewMessageForTask (when continuing an
// existing conversation). The resulting request is ready to hand to
// A2AClient.SendMessage or A2AClient.SendStreamingMessage.
//
// This function is pure so callers (TUI and simple mode alike) build
// requests identically.
func BuildChatRequest(state *State, text string) *a2a.SendMessageRequest {
	var msg *a2a.Message
	if state.IsFresh() {
		msg = a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
	} else {
		msg = a2a.NewMessageForTask(a2a.MessageRoleUser, state.TaskInfo(), a2a.NewTextPart(text))
	}
	return &a2a.SendMessageRequest{Message: msg}
}
