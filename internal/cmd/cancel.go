package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

var cancelCmd = &cobra.Command{
	Use:   "cancel <agent-url> <task-id>",
	Short: "Cancel a task by ID on an A2A agent",
	Long: `Cancels a task via the tasks/cancel (CancelTask) protocol method
and displays the updated task state.

Tasks already in a terminal state (completed, failed, canceled, rejected)
cannot be canceled; the server returns an error in that case.`,
	Args: cobra.ExactArgs(2),
	RunE: runCancel,
}

func init() {
	rootCmd.AddCommand(cancelCmd)
}

func runCancel(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, client.Options{
		BaseURL:     baseURL,
		GCPAuth:     flagGCPAuth,
		VertexAI:    flagVertexAI,
		Headers:     flagHeaders,
		BearerToken: flagBearerToken,
	})
	if err != nil {
		return err
	}
	defer a2aClient.Destroy()

	req := &a2a.CancelTaskRequest{
		ID: a2a.TaskID(taskID),
	}

	task, err := a2aClient.CancelTask(ctx, req)
	if err != nil {
		return fmt.Errorf("tasks/cancel failed: %w", err)
	}

	if flagJSON {
		return presenter.PrintJSON(os.Stdout, task)
	}
	return presenter.PrintTask(os.Stdout, task)
}
