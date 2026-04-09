package cmd

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

// flagNameHistoryLength is shared between the flag definition and the
// Changed() lookup so the two never drift apart silently.
const flagNameHistoryLength = "history-length"

var getCmd = &cobra.Command{
	Use:   "get <agent-url> <task-id>",
	Short: "Retrieve a task by ID from an A2A agent",
	Long: `Retrieves a task via the tasks/get (GetTask) protocol method and displays it.

Use --history-length to limit the number of historical messages returned.`,
	Args: cobra.ExactArgs(2),
	RunE: runGet,
}

func init() {
	getCmd.Flags().Int(flagNameHistoryLength, 0,
		"Maximum number of history messages to retrieve (omit to use server default)")
	rootCmd.AddCommand(getCmd)
}

func runGet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, client.Options{
		BaseURL:      baseURL,
		GCPAuth:      flagGCPAuth,
		VertexAI:     flagVertexAI,
		V03RESTMount: flagV03RESTMount,
		Headers:      flagHeaders,
		BearerToken:  flagBearerToken,
	})
	if err != nil {
		return err
	}
	defer a2aClient.Destroy()

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
		return fmt.Errorf("tasks/get failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, task)
	}
	return presenter.PrintTask(out, task)
}
