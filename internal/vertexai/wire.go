package vertexai

import (
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/google/uuid"
)

// --- Request types (Vertex AI Protobuf JSON format) ---

// sendRequest is the top-level request body for Vertex AI message:send.
type sendRequest struct {
	Message       wireMessage `json:"message"`
	Configuration *wireConfig `json:"configuration,omitempty"`
}

// wireMessage is the Vertex AI wire format for a message.
// Key difference from standard A2A: uses "content" instead of "parts".
type wireMessage struct {
	MessageID string      `json:"messageId"`
	Role      string      `json:"role"` // "ROLE_USER" or "ROLE_AGENT"
	Content   []*a2a.Part `json:"content"`
	ContextID string      `json:"contextId,omitempty"`
	TaskID    string      `json:"taskId,omitempty"`
}

// wireConfig holds the Vertex AI send configuration.
type wireConfig struct {
	Blocking            bool     `json:"blocking"`
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
}

// --- Response types ---

// sendResponse is the top-level response from Vertex AI message:send.
type sendResponse struct {
	Task wireTask `json:"task"`
}

// wireTask is the Vertex AI wire format for a task response.
type wireTask struct {
	ID        string         `json:"id"`
	ContextID string         `json:"contextId"`
	Status    wireStatus     `json:"status"`
	Artifacts []wireArtifact `json:"artifacts,omitempty"`
	History   []wireMessage  `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// wireStatus is the Vertex AI wire format for task status.
type wireStatus struct {
	State   string       `json:"state"`
	Message *wireMessage `json:"message,omitempty"`
}

// wireArtifact is the Vertex AI wire format for a task artifact.
// Note: artifacts use "parts" (standard A2A field name), not "content".
type wireArtifact struct {
	ArtifactID  string      `json:"artifactId"`
	Parts       []*a2a.Part `json:"parts,omitempty"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
}

// --- Conversion functions ---

// buildSendRequest converts an a2a.Message into a Vertex AI sendRequest.
// It injects blocking: true by default.
func buildSendRequest(msg *a2a.Message) sendRequest {
	wm := wireMessage{
		MessageID: msg.ID,
		Role:      string(msg.Role),
		Content:   []*a2a.Part(msg.Parts),
		ContextID: msg.ContextID,
		TaskID:    string(msg.TaskID),
	}

	// Assign a message ID if not set.
	if wm.MessageID == "" {
		wm.MessageID = uuid.New().String()
	}

	return sendRequest{
		Message: wm,
		Configuration: &wireConfig{
			Blocking: true,
		},
	}
}

// toA2ATask converts a Vertex AI wireTask response into an a2a.Task.
func toA2ATask(wt wireTask) *a2a.Task {
	task := &a2a.Task{
		ID:        a2a.TaskID(wt.ID),
		ContextID: wt.ContextID,
		Status: a2a.TaskStatus{
			State: a2a.TaskState(wt.Status.State),
		},
		Metadata: wt.Metadata,
	}

	// Convert status message if present.
	if wt.Status.Message != nil {
		task.Status.Message = wireMessageToA2A(wt.Status.Message)
	}

	// Convert history: wireMessage uses "content", map to a2a.Message.Parts.
	for _, wm := range wt.History {
		task.History = append(task.History, wireMessageToA2A(&wm))
	}

	// Convert artifacts: wireArtifact uses "parts" (standard field name).
	for _, wa := range wt.Artifacts {
		task.Artifacts = append(task.Artifacts, &a2a.Artifact{
			ID:          a2a.ArtifactID(wa.ArtifactID),
			Parts:       a2a.ContentParts(wa.Parts),
			Name:        wa.Name,
			Description: wa.Description,
		})
	}

	return task
}

// wireMessageToA2A converts a wireMessage to an a2a.Message.
func wireMessageToA2A(wm *wireMessage) *a2a.Message {
	return &a2a.Message{
		ID:        wm.MessageID,
		Role:      a2a.MessageRole(wm.Role),
		Parts:     a2a.ContentParts(wm.Content),
		ContextID: wm.ContextID,
		TaskID:    a2a.TaskID(wm.TaskID),
	}
}
