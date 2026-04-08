package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintSendResult writes a formatted display of a SendMessageResult.
func PrintSendResult(w io.Writer, result a2a.SendMessageResult) error {
	switch v := result.(type) {
	case *a2a.Task:
		return PrintTask(w, v)
	case *a2a.Message:
		return printMessage(w, v)
	default:
		fmt.Fprintf(w, "Unknown result type: %T\n", result)
	}
	return nil
}

// PrintTask writes a formatted display of a Task to w.
func PrintTask(w io.Writer, task *a2a.Task) error {
	fmt.Fprintf(w, "%s\n", styledHeader("=== Task ==="))
	fmt.Fprintf(w, "%s %s\n", styledLabel("ID:       "), task.ID)
	fmt.Fprintf(w, "%s %s\n", styledLabel("ContextID:"), task.ContextID)
	fmt.Fprintf(w, "%s %s\n", styledLabel("Status:   "), styledTaskState(task.Status.State))

	if task.Status.Timestamp != nil {
		fmt.Fprintf(w, "%s %s\n", styledLabel("Timestamp:"), task.Status.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	}

	// Status message
	if task.Status.Message != nil {
		fmt.Fprintf(w, "\n%s\n", styledDivider("--- Status Message ---"))
		printParts(w, task.Status.Message.Parts)
	}

	// History
	if len(task.History) > 0 {
		fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- History (%d messages) ---", len(task.History))))
		for _, msg := range task.History {
			fmt.Fprintf(w, "%s ", styledTag(fmt.Sprintf("[%s]", msg.Role)))
			printParts(w, msg.Parts)
		}
	}

	// Artifacts
	if len(task.Artifacts) > 0 {
		fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- Artifacts (%d) ---", len(task.Artifacts))))
		for _, artifact := range task.Artifacts {
			printArtifact(w, artifact)
		}
	}

	return nil
}

func printMessage(w io.Writer, msg *a2a.Message) error {
	fmt.Fprintf(w, "%s ", styledTag(fmt.Sprintf("[%s]", msg.Role)))
	printParts(w, msg.Parts)
	return nil
}

func printArtifact(w io.Writer, artifact *a2a.Artifact) {
	if artifact.Name != "" {
		fmt.Fprintf(w, "  %s %s\n", styledLabel("Name:"), artifact.Name)
	}
	if artifact.Description != "" {
		fmt.Fprintf(w, "  %s %s\n", styledLabel("Description:"), artifact.Description)
	}
	printParts(w, artifact.Parts)
}

func printParts(w io.Writer, parts a2a.ContentParts) {
	for _, part := range parts {
		switch part.Content.(type) {
		case a2a.Text:
			fmt.Fprintf(w, "%s\n", part.Text())
		case a2a.Data:
			fmt.Fprintf(w, "%s %v\n", styledTag("[data]"), part.Data())
		case a2a.URL:
			fmt.Fprintf(w, "%s %s\n", styledTag("[url]"), part.URL())
		case a2a.Raw:
			fmt.Fprintf(w, "%s\n", styledTag(fmt.Sprintf("[raw bytes: %d bytes]", len(part.Raw()))))
		default:
			fmt.Fprintf(w, "%s\n", styledTag("[unknown part type]"))
		}
	}
}

// TextFromParts concatenates the textual payload of a ContentParts slice
// into a single string. Non-text parts are summarised inline so callers
// that only need a flat text snapshot — like the chat TUI viewport —
// can reuse the same extraction logic without writing to an io.Writer.
//
// The output is plain (no ANSI styling) so it is safe to feed into
// further rendering layers like lipgloss.
func TextFromParts(parts a2a.ContentParts) string {
	var sb strings.Builder
	for _, p := range parts {
		if p == nil {
			continue
		}
		switch p.Content.(type) {
		case a2a.Text:
			sb.WriteString(p.Text())
		case a2a.Data:
			fmt.Fprintf(&sb, "[data: %v]", p.Data())
		case a2a.URL:
			fmt.Fprintf(&sb, "[url: %s]", p.URL())
		case a2a.Raw:
			fmt.Fprintf(&sb, "[raw: %d bytes]", len(p.Raw()))
		}
	}
	return sb.String()
}
