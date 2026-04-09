package cmd

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send <agent-url> <message>",
	Short: "Send a message to an A2A agent",
	Long:  "Sends a message via the SendMessage method and displays the result.",
	Args:  cobra.ExactArgs(2),
	RunE:  runSend,
}

func init() {
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
	defer a2aClient.Destroy()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
	req := &a2a.SendMessageRequest{
		Message: msg,
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
