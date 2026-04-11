package cmd

import (
	"context"
	"fmt"

	"github.com/Lalcs/a2ahoy/internal/cardcheck"
	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/presenter"
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

var flagExtended bool

var cardCmd = &cobra.Command{
	Use:   "card <agent-url>",
	Short: "Fetch and display an agent's card",
	Long:  "Fetches the Agent Card from /.well-known/agent-card.json (or Vertex AI /a2a/v1/card) and displays it.",
	Args:  cobra.ExactArgs(1),
	RunE:  runCard,
}

func init() {
	rootCmd.AddCommand(cardCmd)
	cardCmd.Flags().BoolVar(&flagExtended, "extended", false, "Fetch the authenticated extended agent card")
}

func runCard(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baseURL := args[0]

	var card *a2a.AgentCard

	if flagExtended {
		// The extended card requires a full A2A client because
		// GetExtendedAgentCard is an authenticated protocol-level call.
		opts := clientOptions(baseURL)
		a2aClient, _, err := client.New(ctx, opts)
		if err != nil {
			return err
		}
		defer func() { _ = a2aClient.Destroy() }()

		card, err = a2aClient.GetExtendedAgentCard(ctx, &a2a.GetExtendedAgentCardRequest{
			Tenant: flagTenant,
		})
		if err != nil {
			return err
		}
	} else {
		// The card subcommand only needs to display the agent card; creating
		// a full A2A client is unnecessary and would fail on v0.3-only servers
		// that lack supportedInterfaces. ResolveCard handles both v1.0 and v0.3
		// formats and skips client creation entirely.
		// V03RESTMount is intentionally disabled here even though the CLI
		// defaults it on for protocol calls, so `a2ahoy card` keeps showing
		// the raw URLs advertised by the server.
		opts := clientOptions(baseURL)
		opts.V03RESTMount = false
		var err error
		card, err = client.ResolveCard(ctx, opts)
		if err != nil {
			return err
		}
	}

	// Validate the resolved card. Issues are surfaced in the display and
	// ERROR-level issues cause the command to exit non-zero so CI pipelines
	// catch malformed cards.
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
