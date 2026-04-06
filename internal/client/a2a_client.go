package client

import (
	"context"
	"iter"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// A2AClient defines the common interface for A2A protocol clients.
// Both the standard a2aclient.Client and the Vertex AI client satisfy
// this interface, allowing commands to be transport-agnostic.
type A2AClient interface {
	// SendMessage sends a message and returns the result (Task or Message).
	SendMessage(ctx context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error)

	// SendStreamingMessage sends a message and streams events via SSE.
	SendStreamingMessage(ctx context.Context, req *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error]

	// Destroy releases any resources held by the client.
	Destroy() error
}
