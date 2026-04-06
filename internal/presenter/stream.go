package presenter

import (
	"fmt"
	"io"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintStreamEvent writes a formatted display of a streaming event.
func PrintStreamEvent(w io.Writer, event a2a.Event) error {
	switch v := event.(type) {
	case *a2a.Task:
		fmt.Fprintf(w, "[task] id=%s status=%s\n", v.ID, v.Status.State)

	case *a2a.Message:
		fmt.Fprintf(w, "[%s] ", v.Role)
		printParts(w, v.Parts)

	case *a2a.TaskStatusUpdateEvent:
		fmt.Fprintf(w, "[status] %s", v.Status.State)
		if v.Status.Message != nil {
			fmt.Fprintf(w, " - ")
			printParts(w, v.Status.Message.Parts)
		} else {
			fmt.Fprintln(w)
		}

	case *a2a.TaskArtifactUpdateEvent:
		if v.Artifact != nil {
			if !v.Append && v.Artifact.Name != "" {
				fmt.Fprintf(w, "[artifact] %s\n", v.Artifact.Name)
			}
			printParts(w, v.Artifact.Parts)
		}

	default:
		fmt.Fprintf(w, "[unknown event] %T\n", event)
	}

	return nil
}
