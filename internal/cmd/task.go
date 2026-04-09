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

// Flag name constants shared across task subcommands.
const (
	flagNameHistoryLength        = "history-length"
	flagNameListContextID        = "context-id"
	flagNameListStatus           = "status"
	flagNameListPageSize         = "page-size"
	flagNameListPageToken        = "page-token"
	flagNameListIncludeArtifacts = "include-artifacts"
	flagNameListStatusAfter      = "status-after"
)

// taskCmd is the parent command for task-related operations.
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks on an A2A agent",
	Long: `Manage tasks on an A2A agent.

Use the subcommands to retrieve, cancel, or list tasks.`,
}

var taskGetCmd = &cobra.Command{
	Use:   "get <agent-url> <task-id>",
	Short: "Retrieve a task by ID from an A2A agent",
	Long: `Retrieves a task via the GetTask protocol method and displays it.

Use --history-length to limit the number of historical messages returned.`,
	Args: cobra.ExactArgs(2),
	RunE: runTaskGet,
}

var taskCancelCmd = &cobra.Command{
	Use:   "cancel <agent-url> <task-id>",
	Short: "Cancel a task by ID on an A2A agent",
	Long: `Cancels a task via the CancelTask protocol method and displays
the updated task state.

Tasks already in a terminal state (completed, failed, canceled, rejected)
cannot be canceled; the server returns an error in that case.`,
	Args: cobra.ExactArgs(2),
	RunE: runTaskCancel,
}

var taskListCmd = &cobra.Command{
	Use:   "list <agent-url>",
	Short: "List tasks from an A2A agent",
	Long: `Lists tasks via the ListTasks protocol method.

Filter by context ID, task state, or timestamp. Results are paginated;
use --page-token with the nextPageToken from a prior response to fetch
subsequent pages.`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskList,
}

var taskResubscribeCmd = &cobra.Command{
	Use:   "resubscribe <agent-url> <task-id>",
	Short: "Resubscribe to events for an existing task",
	Long: `Resubscribes to a task via the SubscribeToTask protocol method and
streams events in real-time.

This is useful for reconnecting to an in-progress task after a network
disconnection. The server returns events for the current task state.

Press Ctrl+C to disconnect from the event stream.`,
	Args: cobra.ExactArgs(2),
	RunE: runTaskResubscribe,
}

// newResubscribeContext is a test seam — tests override it to exercise the
// SIGINT code path without sending a real OS signal.
var newResubscribeContext = defaultSignalContext

func init() {
	// task get flags
	taskGetCmd.Flags().Int(flagNameHistoryLength, 0,
		"Maximum number of history messages to retrieve (omit to use server default)")

	// task list flags
	taskListCmd.Flags().String(flagNameListContextID, "",
		"Filter tasks by context ID")
	taskListCmd.Flags().String(flagNameListStatus, "",
		"Filter tasks by state (e.g., TASK_STATE_COMPLETED, TASK_STATE_WORKING)")
	taskListCmd.Flags().Int(flagNameListPageSize, 0,
		"Maximum number of tasks per page (1-100; omit to use server default)")
	taskListCmd.Flags().String(flagNameListPageToken, "",
		"Continuation token from a prior response's nextPageToken")
	taskListCmd.Flags().Int(flagNameHistoryLength, 0,
		"Maximum number of history messages per task (omit to use server default)")
	taskListCmd.Flags().Bool(flagNameListIncludeArtifacts, false,
		"Include artifacts in the response")
	taskListCmd.Flags().String(flagNameListStatusAfter, "",
		"Filter tasks updated after this time (RFC3339, e.g., 2026-01-01T00:00:00Z)")

	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskCancelCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskResubscribeCmd)
	rootCmd.AddCommand(taskCmd)
}

func runTaskGet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	req := &a2a.GetTaskRequest{
		ID: a2a.TaskID(taskID),
	}
	// Changed() distinguishes "flag omitted" (use server default) from
	// "explicit --history-length=0" (which still propagates).
	if cmd.Flags().Changed(flagNameHistoryLength) {
		// GetInt cannot fail here: cobra validates the flag type before
		// RunE is invoked, so the value is always a valid int.
		h, _ := cmd.Flags().GetInt(flagNameHistoryLength)
		req.HistoryLength = &h
	}

	task, err := a2aClient.GetTask(ctx, req)
	if err != nil {
		return fmt.Errorf("GetTask failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, task)
	}
	return presenter.PrintTask(out, task)
}

func runTaskCancel(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	req := &a2a.CancelTaskRequest{
		ID: a2a.TaskID(taskID),
	}

	task, err := a2aClient.CancelTask(ctx, req)
	if err != nil {
		return fmt.Errorf("CancelTask failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, task)
	}
	return presenter.PrintTask(out, task)
}

func runTaskList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

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

func runTaskResubscribe(cmd *cobra.Command, args []string) error {
	ctx, cancel := newResubscribeContext()
	defer cancel()

	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	req := &a2a.SubscribeToTaskRequest{
		ID: a2a.TaskID(taskID),
	}

	return consumeEventStream(ctx, cmd, a2aClient.SubscribeToTask(ctx, req), "SubscribeToTask failed")
}
