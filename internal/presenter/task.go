package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintSendResult writes a formatted display of a SendMessageResult.
func PrintSendResult(w io.Writer, result a2a.SendMessageResult) error {
	return printSendResultAny(w, result)
}

// printSendResultAny dispatches formatting based on the concrete type of v.
// It is separated from PrintSendResult so that tests can exercise the default
// branch, which is otherwise unreachable through the sealed SendMessageResult
// interface.
func printSendResultAny(w io.Writer, v any) error {
	switch v := v.(type) {
	case *a2a.Task:
		return PrintTask(w, v)
	case *a2a.Message:
		return printMessage(w, v)
	default:
		_, _ = fmt.Fprintf(w, "Unknown result type: %T\n", v)
	}
	return nil
}

// PrintTask writes a formatted display of a Task to w.
func PrintTask(w io.Writer, task *a2a.Task) error {
	_, _ = fmt.Fprintf(w, "%s\n", styledHeader("=== Task ==="))
	_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("ID:       "), task.ID)
	_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("ContextID:"), task.ContextID)
	_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Status:   "), styledTaskState(task.Status.State))

	if task.Status.Timestamp != nil {
		_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Timestamp:"), task.Status.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	}

	// Status message
	if task.Status.Message != nil {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider("--- Status Message ---"))
		printParts(w, task.Status.Message.Parts)
	}

	// History
	if len(task.History) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- History (%d messages) ---", len(task.History))))
		for _, msg := range task.History {
			_, _ = fmt.Fprintf(w, "%s ", styledTag(fmt.Sprintf("[%s]", msg.Role)))
			printParts(w, msg.Parts)
			printReferenceTasks(w, msg.ReferenceTasks)
		}
	}

	// Artifacts
	if len(task.Artifacts) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- Artifacts (%d) ---", len(task.Artifacts))))
		for _, artifact := range task.Artifacts {
			printArtifact(w, artifact)
		}
	}

	return nil
}

func printMessage(w io.Writer, msg *a2a.Message) error {
	_, _ = fmt.Fprintf(w, "%s ", styledTag(fmt.Sprintf("[%s]", msg.Role)))
	printParts(w, msg.Parts)
	printReferenceTasks(w, msg.ReferenceTasks)
	return nil
}

func printReferenceTasks(w io.Writer, refs []a2a.TaskID) {
	if len(refs) == 0 {
		return
	}
	ids := make([]string, len(refs))
	for i, id := range refs {
		ids[i] = string(id)
	}
	_, _ = fmt.Fprintf(w, "  %s %s\n", styledLabel("Reference Tasks:"), strings.Join(ids, ", "))
}

func printArtifact(w io.Writer, artifact *a2a.Artifact) {
	if artifact.Name != "" {
		_, _ = fmt.Fprintf(w, "  %s %s\n", styledLabel("Name:"), artifact.Name)
	}
	if artifact.Description != "" {
		_, _ = fmt.Fprintf(w, "  %s %s\n", styledLabel("Description:"), artifact.Description)
	}
	printParts(w, artifact.Parts)
}

func printParts(w io.Writer, parts a2a.ContentParts) {
	for _, part := range parts {
		switch part.Content.(type) {
		case a2a.Text:
			_, _ = fmt.Fprintf(w, "%s\n", part.Text())
		case a2a.Data:
			_, _ = fmt.Fprintf(w, "%s %v\n", styledTag("[data]"), part.Data())
		case a2a.URL:
			_, _ = fmt.Fprintf(w, "%s %s\n", styledTag("[url]"), part.URL())
		case a2a.Raw:
			_, _ = fmt.Fprintf(w, "%s\n", styledTag(fmt.Sprintf("[raw bytes: %d bytes]", len(part.Raw()))))
		default:
			_, _ = fmt.Fprintf(w, "%s\n", styledTag("[unknown part type]"))
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
			_, _ = fmt.Fprintf(&sb, "[data: %v]", p.Data())
		case a2a.URL:
			_, _ = fmt.Fprintf(&sb, "[url: %s]", p.URL())
		case a2a.Raw:
			_, _ = fmt.Fprintf(&sb, "[raw: %d bytes]", len(p.Raw()))
		}
	}
	return sb.String()
}
