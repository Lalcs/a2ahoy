package cmd

import (
	"context"
	"os"

	"github.com/khayashi/a2ahoy/internal/client"
	"github.com/khayashi/a2ahoy/internal/presenter"
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

	a2aClient, card, err := client.New(ctx, client.Options{
		BaseURL:  baseURL,
		GCPAuth:  flagGCPAuth,
		VertexAI: flagVertexAI,
	})
	if err != nil {
		return err
	}
	defer a2aClient.Destroy()

	if flagJSON {
		return presenter.PrintJSON(os.Stdout, card)
	}
	return presenter.PrintAgentCard(os.Stdout, card)
}
