package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/khayashi/a2ahoy/internal/presenter"
	"github.com/khayashi/a2ahoy/internal/updater"
	"github.com/khayashi/a2ahoy/internal/version"
	"github.com/spf13/cobra"
)

var (
	flagUpdateCheckOnly bool
	flagUpdateForce     bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Self-update a2ahoy from the latest GitHub release",
	Long: `Fetches the latest release from https://github.com/Lalcs/a2ahoy and
replaces the running binary with the new version if a newer one is available.

Supported platforms: Linux and macOS (amd64, arm64).
Windows users should download new releases manually from the releases page.

Examples:
  a2ahoy update                # check and install if a newer release exists
  a2ahoy update --check-only   # report status without installing
  a2ahoy update --force        # reinstall the latest release unconditionally`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&flagUpdateCheckOnly, "check-only", false,
		"Check for updates without downloading or installing")
	updateCmd.Flags().BoolVar(&flagUpdateForce, "force", false,
		"Reinstall the latest version even if it matches the current version")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := os.Stdout

	// Step 1: ensure the running platform is one we publish binaries for.
	plat, err := updater.CurrentPlatform()
	if err != nil {
		return err
	}

	// Step 2: contact GitHub for the latest release manifest.
	presenter.PrintUpdateChecking(out)
	fetcher := updater.NewGitHubFetcher()
	rel, err := fetcher.FetchLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	// Step 3: compare versions and decide what to do.
	decision := updater.Decide(version.Current(), rel.TagName, flagUpdateForce)
	presenter.PrintUpdateDecision(out, decision)

	// Step 4: branch on the decision.
	switch decision.Action {
	case updater.ActionUpToDate:
		presenter.PrintUpdateAlreadyLatest(out, decision.Current)
		return nil
	case updater.ActionAhead:
		presenter.PrintUpdateAhead(out, decision.Current, decision.Latest)
		return nil
	case updater.ActionInvalidLatest:
		return fmt.Errorf("cannot determine latest version: %s", decision.Reason)
	}

	// Anything left (Update / Development / ForceReinstall) implies an
	// install. --check-only short-circuits before any filesystem mutation.
	if flagUpdateCheckOnly {
		presenter.PrintUpdateAvailable(out, decision.Current, decision.Latest)
		return nil
	}

	// Step 5: locate the platform-specific asset.
	asset, err := rel.FindAssetForPlatform(plat.AssetName())
	if err != nil {
		return err
	}

	// Step 6: prepare and run the install. Prepare validates filesystem
	// access before any download starts so users see permission errors
	// immediately rather than after waiting on a download.
	installer := updater.NewInstaller()
	plan, err := installer.Prepare(asset, rel)
	if err != nil {
		return err
	}

	presenter.PrintUpdateDownloading(out, asset.Name, asset.Size)
	if err := installer.Install(ctx, plan); err != nil {
		return err
	}

	presenter.PrintUpdateSuccess(out, decision.Current, rel.TagName, plan.TargetPath)
	return nil
}
