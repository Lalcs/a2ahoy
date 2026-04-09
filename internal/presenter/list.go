package presenter

import (
	"fmt"
	"io"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintListTasks writes a formatted display of a ListTasksResponse.
func PrintListTasks(w io.Writer, resp *a2a.ListTasksResponse) error {
	if len(resp.Tasks) == 0 {
		fmt.Fprintln(w, "No tasks found.")
		return nil
	}

	fmt.Fprintf(w, "%s\n", styledHeader(fmt.Sprintf("=== Tasks (%d of %d total) ===", len(resp.Tasks), resp.TotalSize)))

	for _, task := range resp.Tasks {
		fmt.Fprintf(w, "  %s %-20s %s %s  %s\n",
			styledLabel("ID:"), task.ID,
			styledLabel("Context:"), task.ContextID,
			styledTaskState(task.Status.State))
	}

	if resp.NextPageToken != "" {
		fmt.Fprintf(w, "\n%s %s\n",
			styledLabel("Next page:"),
			resp.NextPageToken)
	}

	return nil
}
