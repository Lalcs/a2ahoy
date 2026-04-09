package cmd

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/cardcheck"
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
		BaseURL:     baseURL,
		GCPAuth:     flagGCPAuth,
		VertexAI:    flagVertexAI,
		Headers:     flagHeaders,
		BearerToken: flagBearerToken,
	})
	if err != nil {
		return err
	}

	// Validate the resolved card. Issues are surfaced in the display and
	// (as of this change) ERROR-level issues cause the command to exit
	// non-zero so CI pipelines catch malformed cards.
	result := cardcheck.Run(card)

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	if flagJSON {
		if err := presenter.PrintJSON(out, card); err != nil {
			return err
		}
		// stdout stays pure JSON for scripts; validation summary goes to
		// stderr so pipelines like `a2ahoy card --json | jq` keep working.
		presenter.PrintValidationSummary(errOut, result)
	} else {
		// PrintAgentCard uses fmt.Fprintf internally and always returns
		// nil, so the returned error is intentionally discarded.
		_ = presenter.PrintAgentCard(out, card)
		presenter.PrintValidation(out, result)
	}

	// Breaking change (user-approved): fail the command when the card has
	// any validation errors so downstream commands (send/stream/get/cancel)
	// do not silently fail against a malformed card.
	if n := result.Count(cardcheck.LevelError); n > 0 {
		return fmt.Errorf("agent card validation failed: %d error(s)", n)
	}
	return nil
}
