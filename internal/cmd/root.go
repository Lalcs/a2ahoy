package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// bearerTokenEnvVar is the environment variable consulted as a fallback for --bearer-token.
const bearerTokenEnvVar = "A2A_BEARER_TOKEN"

var (
	flagGCPAuth     bool
	flagJSON        bool
	flagVertexAI    bool
	flagNoColor     bool
	flagHeaders     []string
	flagBearerToken string
)

var rootCmd = &cobra.Command{
	Use:   "a2ahoy",
	Short: "A2A protocol CLI tool",
	Long:  "a2ahoy is a CLI tool for interacting with A2A (Agent-to-Agent) protocol agents.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if flagNoColor {
			color.NoColor = true
		}
		if flagBearerToken == "" {
			// TrimSpace treats whitespace-only env values as unset.
			flagBearerToken = strings.TrimSpace(os.Getenv(bearerTokenEnvVar))
		}
		if flagGCPAuth && flagVertexAI {
			return fmt.Errorf("--gcp-auth and --vertex-ai cannot be used together")
		}
		if flagBearerToken != "" && flagGCPAuth {
			return fmt.Errorf("--bearer-token and --gcp-auth cannot be used together")
		}
		if flagBearerToken != "" && flagVertexAI {
			return fmt.Errorf("--bearer-token and --vertex-ai cannot be used together")
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagGCPAuth, "gcp-auth", false, "Enable GCP ADC authentication (ID token as Bearer)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON")
	rootCmd.PersistentFlags().BoolVar(&flagVertexAI, "vertex-ai", false, "Treat the URL as a Vertex AI Agent Engine endpoint")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	// StringArrayVar (not StringSliceVar) so values with commas are not split,
	// e.g. --header "Accept=application/json, text/plain".
	rootCmd.PersistentFlags().StringArrayVar(&flagHeaders, "header", nil, "Add a custom HTTP header in KEY=VALUE form (repeatable)")
	rootCmd.PersistentFlags().StringVar(&flagBearerToken, "bearer-token", "", "Bearer token for Authorization header (falls back to A2A_BEARER_TOKEN env var)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
