package vertexai

import (
	"encoding/json"
	"fmt"

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

// newWireMessage converts an a2a.Message to the Vertex AI wire format.
// Shared by buildSendRequest and buildStreamRequest.
func newWireMessage(msg *a2a.Message) wireMessage {
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
	return wm
}

// hasOutputModes reports whether the request carries non-empty
// AcceptedOutputModes that should be propagated to the wire format.
func hasOutputModes(req *a2a.SendMessageRequest) bool {
	return req.Config != nil && len(req.Config.AcceptedOutputModes) > 0
}

// buildSendRequest converts an a2a.SendMessageRequest into a Vertex AI
// sendRequest for the blocking message:send endpoint. It injects
// blocking: true and propagates AcceptedOutputModes from the request's
// Config when present.
func buildSendRequest(req *a2a.SendMessageRequest) sendRequest {
	cfg := &wireConfig{Blocking: true}
	if hasOutputModes(req) {
		cfg.AcceptedOutputModes = req.Config.AcceptedOutputModes
	}
	return sendRequest{
		Message:       newWireMessage(req.Message),
		Configuration: cfg,
	}
}

// buildStreamRequest converts an a2a.SendMessageRequest into a Vertex AI
// sendRequest for the message:stream endpoint. Streaming is inherently
// non-blocking, so Configuration is only included when AcceptedOutputModes
// are specified.
func buildStreamRequest(req *a2a.SendMessageRequest) sendRequest {
	sr := sendRequest{
		Message: newWireMessage(req.Message),
	}
	if hasOutputModes(req) {
		sr.Configuration = &wireConfig{
			AcceptedOutputModes: req.Config.AcceptedOutputModes,
		}
	}
	return sr
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

// --- Stream event wire types ---
//
// Vertex AI streams StreamResponse proto messages serialized with
// preserving_proto_field_name=False, so snake_case fields are emitted
// as camelCase (e.g. status_update → statusUpdate). Exactly one oneof
// variant is populated per event.

type wireStreamEvent struct {
	Task           *wireTask                `json:"task,omitempty"`
	Msg            *wireMessage             `json:"msg,omitempty"`
	StatusUpdate   *wireStatusUpdateEvent   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *wireArtifactUpdateEvent `json:"artifactUpdate,omitempty"`
}

type wireStatusUpdateEvent struct {
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId"`
	Status    wireStatus     `json:"status"`
	Final     bool           `json:"final,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type wireArtifactUpdateEvent struct {
	TaskID    string         `json:"taskId"`
	ContextID string         `json:"contextId"`
	Artifact  *wireArtifact  `json:"artifact,omitempty"`
	Append    bool           `json:"append,omitempty"`
	LastChunk bool           `json:"lastChunk,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// parseStreamEvent decodes one SSE event payload into an a2a.Event.
// Returns (nil, nil) for empty or unknown oneof variants so callers
// can skip them without treating forward-compatible additions as errors.
func parseStreamEvent(data []byte) (a2a.Event, error) {
	var wse wireStreamEvent
	if err := json.Unmarshal(data, &wse); err != nil {
		return nil, fmt.Errorf("failed to decode stream event: %w", err)
	}
	return toA2AEvent(&wse), nil
}

// toA2AEvent converts a wireStreamEvent oneof into an a2a.Event, or
// returns nil when no variant is populated.
func toA2AEvent(wse *wireStreamEvent) a2a.Event {
	switch {
	case wse.Task != nil:
		return toA2ATask(*wse.Task)
	case wse.Msg != nil:
		return wireMessageToA2A(wse.Msg)
	case wse.StatusUpdate != nil:
		return toA2AStatusUpdate(wse.StatusUpdate)
	case wse.ArtifactUpdate != nil:
		return toA2AArtifactUpdate(wse.ArtifactUpdate)
	}
	return nil
}

func toA2AStatusUpdate(w *wireStatusUpdateEvent) *a2a.TaskStatusUpdateEvent {
	evt := &a2a.TaskStatusUpdateEvent{
		TaskID:    a2a.TaskID(w.TaskID),
		ContextID: w.ContextID,
		Status: a2a.TaskStatus{
			State: a2a.TaskState(w.Status.State),
		},
		Metadata: w.Metadata,
	}
	if w.Status.Message != nil {
		evt.Status.Message = wireMessageToA2A(w.Status.Message)
	}
	return evt
}

func toA2AArtifactUpdate(w *wireArtifactUpdateEvent) *a2a.TaskArtifactUpdateEvent {
	evt := &a2a.TaskArtifactUpdateEvent{
		TaskID:    a2a.TaskID(w.TaskID),
		ContextID: w.ContextID,
		Append:    w.Append,
		LastChunk: w.LastChunk,
		Metadata:  w.Metadata,
	}
	if w.Artifact != nil {
		evt.Artifact = &a2a.Artifact{
			ID:          a2a.ArtifactID(w.Artifact.ArtifactID),
			Parts:       a2a.ContentParts(w.Artifact.Parts),
			Name:        w.Artifact.Name,
			Description: w.Artifact.Description,
		}
	}
	return evt
}
