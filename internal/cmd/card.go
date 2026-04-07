package cmd

import (
	"context"
	"os"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/spf13/cobra"
)

var cardCmd = &cobra.Command{
	Use:   "card <agent-url>",
	Short: "Fetch and display an agent's card",
	Long:  "Fetches the Agent Card from /.well-known/agent-card.json (or Vertex AI /a2a/v1/card) and displays it.",
	Args:  cobra.ExactArgs(1),
	RunE:  runCard,
}

func init() {
	rootCmd.AddCommand(cardCmd)
}

func runCard(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]

	// The card subcommand only needs to display the agent card; creating
	// a full A2A client is unnecessary and would fail on v0.3-only servers
	// that lack supportedInterfaces. ResolveCard handles both v1.0 and v0.3
	// formats and skips client creation entirely.
	card, err := client.ResolveCard(ctx, client.Options{
		BaseURL:  baseURL,
		GCPAuth:  flagGCPAuth,
		VertexAI: flagVertexAI,
		Headers:  flagHeaders,
	})
	if err != nil {
		return err
	}

	if flagJSON {
		return presenter.PrintJSON(os.Stdout, card)
	}
	return presenter.PrintAgentCard(os.Stdout, card)
}
