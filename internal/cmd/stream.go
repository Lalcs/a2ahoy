package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/filepart"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

var (
	flagStreamFiles    []string
	flagStreamFileURLs []string
)

var streamCmd = &cobra.Command{
	Use:   "stream <agent-url> <message>",
	Short: "Stream a message to an A2A agent via SSE",
	Long:  "Sends a message via the SendStreamingMessage method and displays events in real-time.",
	Args:  cobra.ExactArgs(2),
	RunE:  runStream,
}

// newStreamContext creates the cancellable context used by runStream.
// Tests override this to inject a pre-cancelled or manually-cancellable
// context so the SIGINT path can be exercised without sending a real OS
// signal.
var newStreamContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func init() {
	streamCmd.Flags().StringArrayVar(&flagStreamFiles, "file", nil, "Attach a local file (repeatable)")
	streamCmd.Flags().StringArrayVar(&flagStreamFileURLs, "file-url", nil, "Attach a file by URL (repeatable)")
	rootCmd.AddCommand(streamCmd)
}

func runStream(cmd *cobra.Command, args []string) error {
	ctx, cancel := newStreamContext()
	defer cancel()

	baseURL := args[0]
	text := args[1]

	a2aClient, _, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer a2aClient.Destroy()

	parts, err := filepart.BuildParts(text, flagStreamFiles, flagStreamFileURLs)
	if err != nil {
		return err
	}

	msg := a2a.NewMessage(a2a.MessageRoleUser, parts...)
	req := &a2a.SendMessageRequest{
		Message: msg,
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	for event, err := range a2aClient.SendStreamingMessage(ctx, req) {
		if err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(errOut, "\nInterrupted.")
				return nil
			}
			return fmt.Errorf("stream error: %w", err)
		}

		if flagJSON {
			if err := presenter.PrintJSON(out, event); err != nil {
				return err
			}
		} else {
			// PrintStreamEvent uses fmt.Fprintf internally and always
			// returns nil, so the returned error is intentionally discarded.
			_ = presenter.PrintStreamEvent(out, event)
		}
	}

	return nil
}
