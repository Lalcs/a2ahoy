package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/Lalcs/a2ahoy/internal/chat"
	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/spf13/cobra"
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

	a2aClient, card, err := client.New(ctx, client.Options{
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

	// --json is incompatible with the full-screen TUI; fall back to
	// simple mode silently so existing --json pipelines continue to
	// work when users add `chat` to their scripts.
	useSimple := flagChatSimple || flagJSON

	if useSimple {
		return chat.RunSimple(ctx, a2aClient, card, baseURL, flagJSON)
	}
	return chat.RunTUI(ctx, a2aClient, card, baseURL)
}
