package cmd

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/filepart"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

var (
	flagSendFiles            []string
	flagSendFileURLs         []string
	flagSendOutputModes      []string
	flagSendAsync            bool
	flagSendReferenceTaskIDs []string
	flagSendExtensions       []string
	flagSendMetadata         []string
	flagSendPushURL          string
	flagSendPushToken        string
)

var sendCmd = &cobra.Command{
	Use:   "send <agent-url> <message>",
	Short: "Send a message to an A2A agent",
	Long:  "Sends a message via the SendMessage method and displays the result.",
	Args:  cobra.ExactArgs(2),
	RunE:  runSend,
}

func init() {
	sendCmd.Flags().StringArrayVar(&flagSendFiles, "file", nil, "Attach a local file (repeatable)")
	sendCmd.Flags().StringArrayVar(&flagSendFileURLs, "file-url", nil, "Attach a file by URL (repeatable)")
	sendCmd.Flags().StringArrayVar(&flagSendOutputModes, "accepted-output-mode", nil,
		"Accepted output MIME type (repeatable, e.g. text/plain, application/json)")
	sendCmd.Flags().BoolVar(&flagSendAsync, "async", false,
		"Return immediately after task creation (sets ReturnImmediately=true)")
	sendCmd.Flags().StringArrayVar(&flagSendReferenceTaskIDs, "reference-task-id", nil,
		"Reference a prior task by ID (repeatable)")
	sendCmd.Flags().Int(flagNameHistoryLength, 0,
		"Maximum number of history messages in the response (omit to use server default)")
	sendCmd.Flags().StringArrayVar(&flagSendExtensions, "extension", nil,
		"Extension URI to declare on the message (repeatable)")
	sendCmd.Flags().StringArrayVar(&flagSendMetadata, "metadata", nil,
		"Request metadata in KEY=VALUE form (repeatable)")
	sendCmd.Flags().StringVar(&flagSendPushURL, "push-url", "",
		"Inline push notification callback URL")
	sendCmd.Flags().StringVar(&flagSendPushToken, "push-token", "",
		"Inline push notification validation token")
	rootCmd.AddCommand(sendCmd)
}

func runSend(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]
	text := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	parts, err := filepart.BuildParts(text, flagSendFiles, flagSendFileURLs)
	if err != nil {
		return err
	}

	msg := a2a.NewMessage(a2a.MessageRoleUser, parts...)
	msg.ReferenceTasks = toTaskIDs(flagSendReferenceTaskIDs)
	msg.Extensions = flagSendExtensions

	metadata, err := parseMetadata(flagSendMetadata)
	if err != nil {
		return err
	}

	req := &a2a.SendMessageRequest{
		Tenant:   flagTenant,
		Message:  msg,
		Config:   buildSendConfig(flagSendOutputModes, flagSendAsync, getHistoryLength(cmd), buildPushConfig(flagSendPushURL, flagSendPushToken)),
		Metadata: metadata,
	}

	result, err := a2aClient.SendMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("SendMessage failed: %w", err)
	}

	out := cmd.OutOrStdout()
	if flagJSON {
		return presenter.PrintJSON(out, result)
	}
	return presenter.PrintSendResult(out, result)
}
