package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/khayashi/a2ahoy/internal/auth"
	"github.com/khayashi/a2ahoy/internal/presenter"
	"github.com/khayashi/a2ahoy/internal/vertexai"
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

	if flagVertexAI {
		return runCardVertexAI(ctx, baseURL)
	}
	return runCardStandard(ctx, baseURL)
}

func runCardVertexAI(ctx context.Context, baseURL string) error {
	endpoint, err := vertexai.ParseEndpoint(baseURL)
	if err != nil {
		return fmt.Errorf("invalid Vertex AI endpoint: %w", err)
	}

	interceptor, err := auth.NewGCPAccessTokenInterceptor(ctx)
	if err != nil {
		return fmt.Errorf("GCP access token auth setup failed: %w", err)
	}

	vc := vertexai.NewClient(endpoint, interceptor.GetToken)
	card, err := vc.FetchCard(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch Vertex AI agent card: %w", err)
	}

	if flagJSON {
		return presenter.PrintJSON(os.Stdout, card)
	}
	return presenter.PrintAgentCard(os.Stdout, card)
}

func runCardStandard(ctx context.Context, baseURL string) error {
	var resolveOpts []agentcard.ResolveOption

	if flagGCPAuth {
		interceptor, err := auth.NewGCPAuthInterceptor(ctx, baseURL)
		if err != nil {
			return fmt.Errorf("GCP auth setup failed: %w", err)
		}
		token, err := interceptor.GetToken()
		if err != nil {
			return fmt.Errorf("failed to obtain GCP ID token: %w", err)
		}
		resolveOpts = append(resolveOpts, agentcard.WithRequestHeader("Authorization", "Bearer "+token))
	}

	card, err := agentcard.DefaultResolver.Resolve(ctx, baseURL, resolveOpts...)
	if err != nil {
		return fmt.Errorf("failed to fetch agent card: %w", err)
	}

	if flagJSON {
		return presenter.PrintJSON(os.Stdout, card)
	}
	return presenter.PrintAgentCard(os.Stdout, card)
}
