package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/khayashi/a2ahoy/internal/client"
	"github.com/khayashi/a2ahoy/internal/presenter"
	"github.com/spf13/cobra"
)

var streamCmd = &cobra.Command{
	Use:   "stream <agent-url> <message>",
	Short: "Stream a message to an A2A agent via SSE",
	Long:  "Sends a message via the message/stream method and displays events in real-time.",
	Args:  cobra.ExactArgs(2),
	RunE:  runStream,
}

func init() {
	rootCmd.AddCommand(streamCmd)
}

func runStream(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	baseURL := args[0]
	text := args[1]

	a2aClient, _, err := client.New(ctx, client.Options{
		BaseURL:  baseURL,
		GCPAuth:  flagGCPAuth,
		VertexAI: flagVertexAI,
	})
	if err != nil {
		return err
	}
	defer a2aClient.Destroy()

	msg := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))
	req := &a2a.SendMessageRequest{
		Message: msg,
	}

	for event, err := range a2aClient.SendStreamingMessage(ctx, req) {
		if err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(os.Stderr, "\nInterrupted.")
				return nil
			}
			return fmt.Errorf("stream error: %w", err)
		}

		if flagJSON {
			if err := presenter.PrintJSON(os.Stdout, event); err != nil {
				return err
			}
		} else {
			if err := presenter.PrintStreamEvent(os.Stdout, event); err != nil {
				return err
			}
		}
	}

	return nil
}
