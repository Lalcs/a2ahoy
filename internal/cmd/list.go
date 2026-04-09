package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

const (
	flagNameListContextID        = "context-id"
	flagNameListStatus           = "status"
	flagNameListPageSize         = "page-size"
	flagNameListPageToken        = "page-token"
	flagNameListIncludeArtifacts = "include-artifacts"
	flagNameListStatusAfter      = "status-after"
)

var listCmd = &cobra.Command{
	Use:   "list <agent-url>",
	Short: "List tasks from an A2A agent",
	Long: `Lists tasks via the ListTasks protocol method.

Filter by context ID, task state, or timestamp. Results are paginated;
use --page-token with the nextPageToken from a prior response to fetch
subsequent pages.`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func init() {
	listCmd.Flags().String(flagNameListContextID, "",
		"Filter tasks by context ID")
	listCmd.Flags().String(flagNameListStatus, "",
		"Filter tasks by state (e.g., TASK_STATE_COMPLETED, TASK_STATE_WORKING)")
	listCmd.Flags().Int(flagNameListPageSize, 0,
		"Maximum number of tasks per page (1-100; omit to use server default)")
	listCmd.Flags().String(flagNameListPageToken, "",
		"Continuation token from a prior response's nextPageToken")
	listCmd.Flags().Int(flagNameHistoryLength, 0,
		"Maximum number of history messages per task (omit to use server default)")
	listCmd.Flags().Bool(flagNameListIncludeArtifacts, false,
		"Include artifacts in the response")
	listCmd.Flags().String(flagNameListStatusAfter, "",
		"Filter tasks updated after this time (RFC3339, e.g., 2026-01-01T00:00:00Z)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer a2aClient.Destroy()

	contextID, _ := cmd.Flags().GetString(flagNameListContextID)
	status, _ := cmd.Flags().GetString(flagNameListStatus)
	pageSize, _ := cmd.Flags().GetInt(flagNameListPageSize)
	pageToken, _ := cmd.Flags().GetString(flagNameListPageToken)
	includeArtifacts, _ := cmd.Flags().GetBool(flagNameListIncludeArtifacts)

	req := &a2a.ListTasksRequest{
		ContextID:        contextID,
		Status:           a2a.TaskState(status),
		PageSize:         pageSize,
		PageToken:        pageToken,
		IncludeArtifacts: includeArtifacts,
	}

	// Pointer fields require Changed() to distinguish "not passed" from
	// "passed as zero value".
	if cmd.Flags().Changed(flagNameHistoryLength) {
		h, _ := cmd.Flags().GetInt(flagNameHistoryLength)
		req.HistoryLength = &h
	}
	if cmd.Flags().Changed(flagNameListStatusAfter) {
		v, _ := cmd.Flags().GetString(flagNameListStatusAfter)
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return fmt.Errorf("invalid --status-after value %q: expected RFC3339 format (e.g., 2026-01-01T00:00:00Z): %w", v, err)
		}
		req.StatusTimestampAfter = &t
	}

	resp, err := a2aClient.ListTasks(ctx, req)
	if err != nil {
		return fmt.Errorf("ListTasks failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, resp)
	}
	return presenter.PrintListTasks(out, resp)
}
