package cmd

import (
	"context"
	"fmt"
	"iter"
	"os"
	"os/signal"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/filepart"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

var (
	flagStreamFiles            []string
	flagStreamFileURLs         []string
	flagStreamOutputModes      []string
	flagStreamReferenceTaskIDs []string
	flagStreamExtensions       []string
	flagStreamMetadata         []string
)

var streamCmd = &cobra.Command{
	Use:   "stream <agent-url> <message>",
	Short: "Stream a message to an A2A agent via SSE",
	Long:  "Sends a message via the SendStreamingMessage method and displays events in real-time.",
	Args:  cobra.ExactArgs(2),
	RunE:  runStream,
}

// defaultSignalContext returns a context that is cancelled on SIGINT.
// Used as the default value for newStreamContext and newResubscribeContext.
func defaultSignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

// newStreamContext is a test seam — tests override it to exercise the
// SIGINT code path without sending a real OS signal.
var newStreamContext = defaultSignalContext

func init() {
	streamCmd.Flags().StringArrayVar(&flagStreamFiles, "file", nil, "Attach a local file (repeatable)")
	streamCmd.Flags().StringArrayVar(&flagStreamFileURLs, "file-url", nil, "Attach a file by URL (repeatable)")
	streamCmd.Flags().StringArrayVar(&flagStreamOutputModes, "accepted-output-mode", nil,
		"Accepted output MIME type (repeatable, e.g. text/plain, application/json)")
	streamCmd.Flags().StringArrayVar(&flagStreamReferenceTaskIDs, "reference-task-id", nil,
		"Reference a prior task by ID (repeatable)")
	streamCmd.Flags().Int(flagNameHistoryLength, 0,
		"Maximum number of history messages in the response (omit to use server default)")
	streamCmd.Flags().StringArrayVar(&flagStreamExtensions, "extension", nil,
		"Extension URI to declare on the message (repeatable)")
	streamCmd.Flags().StringArrayVar(&flagStreamMetadata, "metadata", nil,
		"Request metadata in KEY=VALUE form (repeatable)")
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
	defer func() { _ = a2aClient.Destroy() }()

	parts, err := filepart.BuildParts(text, flagStreamFiles, flagStreamFileURLs)
	if err != nil {
		return err
	}

	msg := a2a.NewMessage(a2a.MessageRoleUser, parts...)
	msg.ReferenceTasks = toTaskIDs(flagStreamReferenceTaskIDs)
	msg.Extensions = flagStreamExtensions

	metadata, err := parseMetadata(flagStreamMetadata)
	if err != nil {
		return err
	}

	req := &a2a.SendMessageRequest{
		Tenant:   flagTenant,
		Message:  msg,
		Config:   buildSendConfig(flagStreamOutputModes, false, getHistoryLength(cmd), nil),
		Metadata: metadata,
	}

	return consumeEventStream(ctx, cmd, a2aClient.SendStreamingMessage(ctx, req), "stream error")
}

// consumeEventStream reads events from an SSE iterator and prints them.
// It handles JSON/human-readable output dispatch and SIGINT detection.
// Shared by runStream and runTaskResubscribe.
func consumeEventStream(ctx context.Context, cmd *cobra.Command, events iter.Seq2[a2a.Event, error], errLabel string) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	for event, err := range events {
		if err != nil {
			if ctx.Err() != nil {
				_, _ = fmt.Fprintln(errOut, "\nInterrupted.")
				return nil
			}
			return fmt.Errorf("%s: %w", errLabel, err)
		}

		if flagJSON {
			if err := presenter.PrintJSON(out, event); err != nil {
				return err
			}
		} else {
			_ = presenter.PrintStreamEvent(out, event)
		}
	}

	return nil
}
