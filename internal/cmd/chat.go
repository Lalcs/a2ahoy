package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/Lalcs/a2ahoy/internal/chat"
	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/filepart"
	"github.com/spf13/cobra"
)

var (
	flagChatFiles       []string
	flagChatFileURLs    []string
	flagChatOutputModes []string
)

var flagChatSimple bool

var chatCmd = &cobra.Command{
	Use:   "chat <agent-url>",
	Short: "Start an interactive chat REPL with an A2A agent",
	Long: `Starts an interactive, multi-turn chat session with an A2A agent.

By default, chat runs in a rich TUI mode (Bubble Tea) with slash-command
autocomplete, scrollable history, and a status bar showing the current
task and context identifiers. Use --simple for a line-mode REPL
(bufio.Scanner) that is IME-safe and dependency-free; this is also the
mode used automatically when --json is set.

Across turns, contextId and taskId are carried forward automatically so
follow-up messages continue the same conversation.

Slash commands: /new, /get, /cancel, /help, /exit, /quit.`,
	Args: cobra.ExactArgs(1),
	RunE: runChat,
}

func init() {
	chatCmd.Flags().BoolVar(&flagChatSimple, "simple", false,
		"Use a line-mode REPL (bufio.Scanner) instead of the TUI. IME-safe fallback.")
	chatCmd.Flags().StringArrayVar(&flagChatFiles, "file", nil, "Attach a local file to the first turn (repeatable)")
	chatCmd.Flags().StringArrayVar(&flagChatFileURLs, "file-url", nil, "Attach a file by URL to the first turn (repeatable)")
	chatCmd.Flags().StringArrayVar(&flagChatOutputModes, "accepted-output-mode", nil,
		"Accepted output MIME type for every turn (repeatable, e.g. text/plain, application/json)")
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	// Top-level signal context: in simple mode this provides graceful
	// shutdown on SIGINT from outside the per-turn signal window; in
	// TUI mode it is handed to tea.WithContext so the Bubble Tea loop
	// can exit cleanly on SIGINT if the TUI has not yet captured it.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	baseURL := args[0]

	a2aClient, card, err := client.New(ctx, clientOptions(baseURL))
	if err != nil {
		return err
	}
	defer func() { _ = a2aClient.Destroy() }()

	// Load initial file parts from --file / --file-url flags. These
	// are attached to the first chat turn only; subsequent turns are
	// text-only. Per-turn attachment via a /file slash command is a
	// future enhancement.
	initialParts, err := filepart.FileParts(flagChatFiles, flagChatFileURLs)
	if err != nil {
		return err
	}

	// --json is incompatible with the full-screen TUI; fall back to
	// simple mode silently so existing --json pipelines continue to
	// work when users add `chat` to their scripts.
	sendCfg := buildSendConfig(flagChatOutputModes)
	useSimple := flagChatSimple || flagJSON

	if useSimple {
		return chat.RunSimple(ctx, a2aClient, card, baseURL, flagJSON, initialParts, sendCfg)
	}
	return chat.RunTUI(ctx, a2aClient, card, baseURL, initialParts, sendCfg)
}
