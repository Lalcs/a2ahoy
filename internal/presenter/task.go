package presenter

import (
	"fmt"
	"io"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintSendResult writes a formatted display of a SendMessageResult.
func PrintSendResult(w io.Writer, result a2a.SendMessageResult) error {
	switch v := result.(type) {
	case *a2a.Task:
		return printTask(w, v)
	case *a2a.Message:
		return printMessage(w, v)
	default:
		fmt.Fprintf(w, "Unknown result type: %T\n", result)
	}
	return nil
}

func printTask(w io.Writer, task *a2a.Task) error {
	fmt.Fprintf(w, "=== Task ===\n")
	fmt.Fprintf(w, "ID:        %s\n", task.ID)
	fmt.Fprintf(w, "ContextID: %s\n", task.ContextID)
	fmt.Fprintf(w, "Status:    %s\n", task.Status.State)

	if task.Status.Timestamp != nil {
		fmt.Fprintf(w, "Timestamp: %s\n", task.Status.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	}

	// Status message
	if task.Status.Message != nil {
		fmt.Fprintf(w, "\n--- Status Message ---\n")
		printParts(w, task.Status.Message.Parts)
	}

	// History
	if len(task.History) > 0 {
		fmt.Fprintf(w, "\n--- History (%d messages) ---\n", len(task.History))
		for _, msg := range task.History {
			fmt.Fprintf(w, "[%s] ", msg.Role)
			printParts(w, msg.Parts)
		}
	}

	// Artifacts
	if len(task.Artifacts) > 0 {
		fmt.Fprintf(w, "\n--- Artifacts (%d) ---\n", len(task.Artifacts))
		for _, artifact := range task.Artifacts {
			printArtifact(w, artifact)
		}
	}

	return nil
}

func printMessage(w io.Writer, msg *a2a.Message) error {
	fmt.Fprintf(w, "[%s] ", msg.Role)
	printParts(w, msg.Parts)
	return nil
}

func printArtifact(w io.Writer, artifact *a2a.Artifact) {
	if artifact.Name != "" {
		fmt.Fprintf(w, "  Name: %s\n", artifact.Name)
	}
	if artifact.Description != "" {
		fmt.Fprintf(w, "  Description: %s\n", artifact.Description)
	}
	printParts(w, artifact.Parts)
}

func printParts(w io.Writer, parts a2a.ContentParts) {
	for _, part := range parts {
		switch part.Content.(type) {
		case a2a.Text:
			fmt.Fprintf(w, "%s\n", part.Text())
		case a2a.Data:
			fmt.Fprintf(w, "[data] %v\n", part.Data())
		case a2a.URL:
			fmt.Fprintf(w, "[url] %s\n", part.URL())
		case a2a.Raw:
			fmt.Fprintf(w, "[raw bytes: %d bytes]\n", len(part.Raw()))
		default:
			fmt.Fprintf(w, "[unknown part type]\n")
		}
	}
}
