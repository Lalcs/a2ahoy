package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Lalcs/a2ahoy/internal/client"
	"github.com/Lalcs/a2ahoy/internal/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// bearerTokenEnvVar is the environment variable consulted as a fallback for --bearer-token.
const bearerTokenEnvVar = "A2A_BEARER_TOKEN"

var (
	flagGCPAuth      bool
	flagJSON         bool
	flagVertexAI     bool
	flagV03RESTMount bool
	flagNoColor      bool
	flagHeaders      []string
	flagBearerToken  string
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
	// Wire the build-time version into Cobra so that `--version` works and
	// the version string is visible at the top of `a2ahoy --help`. Long is
	// rebuilt here (rather than overriding the template) so subcommand help
	// output is unaffected — only the root command's help shows the version.
	rootCmd.Version = version.Current()
	rootCmd.SetVersionTemplate("a2ahoy version {{.Version}}\n")
	rootCmd.Long = fmt.Sprintf(
		"a2ahoy %s is a CLI tool for interacting with A2A (Agent-to-Agent) protocol agents.",
		version.Current(),
	)

	rootCmd.PersistentFlags().BoolVar(&flagGCPAuth, "gcp-auth", false, "Enable GCP ADC authentication (ID token as Bearer)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON")
	rootCmd.PersistentFlags().BoolVar(&flagVertexAI, "vertex-ai", false, "Treat the URL as a Vertex AI Agent Engine endpoint")
	rootCmd.PersistentFlags().BoolVar(&flagV03RESTMount, "v03-rest-mount", false, "Apply A2A v0.3 REST /v1 mount-point prefix workaround (for Python a2a-sdk / ADK / Vertex AI servers)")
	rootCmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	// StringArrayVar (not StringSliceVar) so values with commas are not split,
	// e.g. --header "Accept=application/json, text/plain".
	rootCmd.PersistentFlags().StringArrayVar(&flagHeaders, "header", nil, "Add a custom HTTP header in KEY=VALUE form (repeatable)")
	rootCmd.PersistentFlags().StringVar(&flagBearerToken, "bearer-token", "", "Bearer token for Authorization header (falls back to A2A_BEARER_TOKEN env var)")
}

// clientOptions builds a client.Options from the global persistent flags and
// the given base URL. All commands that call client.New share this builder so
// new flags are wired in exactly one place.
func clientOptions(baseURL string) client.Options {
	return client.Options{
		BaseURL:      baseURL,
		GCPAuth:      flagGCPAuth,
		VertexAI:     flagVertexAI,
		V03RESTMount: flagV03RESTMount,
		Headers:      flagHeaders,
		BearerToken:  flagBearerToken,
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
