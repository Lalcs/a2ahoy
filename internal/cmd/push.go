package cmd

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

// Flag name constants for push subcommands.
const (
	flagNamePushURL             = "url"
	flagNamePushID              = "push-id"
	flagNamePushToken           = "token"
	flagNamePushAuthScheme      = "auth-scheme"
	flagNamePushAuthCredentials = "auth-credentials"
	flagNamePushPageSize        = "page-size"
	flagNamePushPageToken       = "page-token"
)

// pushCmd is the parent command for push notification configuration operations.
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Manage push notification configurations",
	Long: `Manage push notification configurations for A2A tasks.

Push notifications allow agents to send task status updates to a
callback URL. Use the subcommands to create, retrieve, list, or
delete push notification configurations.`,
}

var pushSetCmd = &cobra.Command{
	Use:   "set <agent-url> <task-id>",
	Short: "Create a push notification configuration",
	Long: `Creates a push notification configuration for a task via
CreateTaskPushNotificationConfig. The agent will POST task status
updates to the specified callback URL.`,
	Args: cobra.ExactArgs(2),
	RunE: runPushSet,
}

var pushGetCmd = &cobra.Command{
	Use:   "get <agent-url> <task-id> <config-id>",
	Short: "Retrieve a push notification configuration",
	Long: `Retrieves a specific push notification configuration by ID via
GetTaskPushNotificationConfig.`,
	Args: cobra.ExactArgs(3),
	RunE: runPushGet,
}

var pushListCmd = &cobra.Command{
	Use:   "list <agent-url> <task-id>",
	Short: "List push notification configurations",
	Long: `Lists all push notification configurations for a task via
ListTaskPushNotificationConfigs. Results are paginated; use
--page-token with the token from a prior response to fetch
subsequent pages.`,
	Args: cobra.ExactArgs(2),
	RunE: runPushList,
}

var pushDeleteCmd = &cobra.Command{
	Use:   "delete <agent-url> <task-id> <config-id>",
	Short: "Delete a push notification configuration",
	Long: `Deletes a specific push notification configuration by ID via
DeleteTaskPushNotificationConfig.`,
	Args: cobra.ExactArgs(3),
	RunE: runPushDelete,
}

func init() {
	// push set flags
	pushSetCmd.Flags().String(flagNamePushURL, "",
		"Callback URL where the agent should send push notifications (required)")
	_ = pushSetCmd.MarkFlagRequired(flagNamePushURL)
	pushSetCmd.Flags().String(flagNamePushID, "",
		"Optional unique ID for the push notification configuration")
	pushSetCmd.Flags().String(flagNamePushToken, "",
		"Optional validation token for incoming push notifications")
	pushSetCmd.Flags().String(flagNamePushAuthScheme, "",
		"Authentication scheme for the callback URL (e.g., Bearer, Basic)")
	pushSetCmd.Flags().String(flagNamePushAuthCredentials, "",
		"Authentication credentials for the callback URL")

	// push list flags
	pushListCmd.Flags().Int(flagNamePushPageSize, 0,
		"Maximum number of configs per page (omit to use server default)")
	pushListCmd.Flags().String(flagNamePushPageToken, "",
		"Continuation token from a prior response")

	pushCmd.AddCommand(pushSetCmd)
	pushCmd.AddCommand(pushGetCmd)
	pushCmd.AddCommand(pushListCmd)
	pushCmd.AddCommand(pushDeleteCmd)
	rootCmd.AddCommand(pushCmd)
}

func runPushSet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	callbackURL, _ := cmd.Flags().GetString(flagNamePushURL)
	pushID, _ := cmd.Flags().GetString(flagNamePushID)
	token, _ := cmd.Flags().GetString(flagNamePushToken)
	authScheme, _ := cmd.Flags().GetString(flagNamePushAuthScheme)
	authCredentials, _ := cmd.Flags().GetString(flagNamePushAuthCredentials)

	pushConfig := a2a.PushConfig{
		URL:   callbackURL,
		ID:    pushID,
		Token: token,
	}
	if cmd.Flags().Changed(flagNamePushAuthCredentials) && !cmd.Flags().Changed(flagNamePushAuthScheme) {
		return fmt.Errorf("--auth-credentials requires --auth-scheme")
	}
	if cmd.Flags().Changed(flagNamePushAuthScheme) {
		pushConfig.Auth = &a2a.PushAuthInfo{
			Scheme:      authScheme,
			Credentials: authCredentials,
		}
	}

	req := &a2a.CreateTaskPushConfigRequest{
		TaskID: a2a.TaskID(taskID),
		Config: pushConfig,
	}

	result, err := a2aClient.CreateTaskPushConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("CreateTaskPushConfig failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, result)
	}
	return presenter.PrintTaskPushConfig(out, result)
}

func runPushGet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]
	configID := args[2]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	req := &a2a.GetTaskPushConfigRequest{
		TaskID: a2a.TaskID(taskID),
		ID:     configID,
	}

	result, err := a2aClient.GetTaskPushConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("GetTaskPushConfig failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, result)
	}
	return presenter.PrintTaskPushConfig(out, result)
}

func runPushList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	pageSize, _ := cmd.Flags().GetInt(flagNamePushPageSize)
	pageToken, _ := cmd.Flags().GetString(flagNamePushPageToken)

	req := &a2a.ListTaskPushConfigRequest{
		TaskID:    a2a.TaskID(taskID),
		PageSize:  pageSize,
		PageToken: pageToken,
	}

	result, err := a2aClient.ListTaskPushConfigs(ctx, req)
	if err != nil {
		return fmt.Errorf("ListTaskPushConfigs failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, result)
	}
	return presenter.PrintTaskPushConfigs(out, result)
}

func runPushDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	taskID := args[1]
	configID := args[2]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	req := &a2a.DeleteTaskPushConfigRequest{
		TaskID: a2a.TaskID(taskID),
		ID:     configID,
	}

	if err := a2aClient.DeleteTaskPushConfig(ctx, req); err != nil {
		return fmt.Errorf("DeleteTaskPushConfig failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, struct{}{})
	}
	return presenter.PrintTaskPushConfigDeleted(out, taskID, configID)
}
