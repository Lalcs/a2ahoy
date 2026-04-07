package presenter

import (
	"fmt"
	"io"

	"github.com/Lalcs/a2ahoy/internal/updater"
)

// PrintUpdateChecking announces that the CLI is about to fetch the latest
// release manifest from GitHub. It is printed before the network call so
// users see immediate feedback.
func PrintUpdateChecking(w io.Writer) {
	fmt.Fprintf(w, "%s Checking latest release on GitHub...\n", styledTag("[update]"))
}

// PrintUpdateDecision shows the result of comparing the running version to
// the latest release. The action is colour-coded according to its severity.
func PrintUpdateDecision(w io.Writer, d updater.Decision) {
	fmt.Fprintf(w, "%s\n", styledHeader("=== Update ==="))
	fmt.Fprintf(w, "%s %s\n", styledLabel("Current:"), d.Current)
	fmt.Fprintf(w, "%s %s\n", styledLabel("Latest: "), d.Latest)
	fmt.Fprintf(w, "%s %s\n", styledLabel("Status: "), styledUpdateAction(d.Action))
}

// PrintUpdateAlreadyLatest is shown after a comparison that resulted in
// ActionUpToDate. It tells the user nothing further will happen.
func PrintUpdateAlreadyLatest(w io.Writer, current string) {
	fmt.Fprintf(w, "%s Already up to date (%s).\n", styledTag("[update]"), current)
}

// PrintUpdateAhead is shown when the running binary is newer than the
// latest published release (e.g. a developer rebuild).
func PrintUpdateAhead(w io.Writer, current, latest string) {
	fmt.Fprintf(w, "%s Current version %s is ahead of latest release %s. Nothing to do.\n",
		styledTag("[update]"), current, latest)
}

// PrintUpdateAvailable is shown for --check-only when an update exists,
// telling the user how to install it.
func PrintUpdateAvailable(w io.Writer, current, latest string) {
	fmt.Fprintf(w, "%s Update available: %s -> %s\n",
		styledTag("[update]"), current, latest)
	fmt.Fprintf(w, "    Run `a2ahoy update` to install.\n")
}

// PrintUpdateDownloading announces the download phase, showing the asset
// name and human-readable size.
func PrintUpdateDownloading(w io.Writer, assetName string, size int64) {
	fmt.Fprintf(w, "%s Downloading %s (%s)...\n",
		styledTag("[update]"), assetName, humanSize(size))
}

// PrintUpdateSuccess is the final success message printed after the binary
// has been replaced. It includes the install path so users can verify the
// outcome.
func PrintUpdateSuccess(w io.Writer, oldVer, newVer, targetPath string) {
	fmt.Fprintf(w, "%s Successfully updated %s -> %s\n",
		styledTag("[update]"), oldVer, newVer)
	fmt.Fprintf(w, "    Installed at: %s\n", targetPath)
}

// styledUpdateAction returns the human-readable status string for an
// update action, with colour applied according to severity.
func styledUpdateAction(a updater.Action) string {
	switch a {
	case updater.ActionUpToDate:
		return greenStyle.Sprint("up to date")
	case updater.ActionUpdate:
		return yellowStyle.Sprint("update available")
	case updater.ActionDevelopment:
		return yellowStyle.Sprint("development build (will install latest)")
	case updater.ActionAhead:
		return yellowStyle.Sprint("ahead of latest")
	case updater.ActionForceReinstall:
		return yellowStyle.Sprint("force reinstall")
	case updater.ActionInvalidLatest:
		return redStyle.Sprint("invalid latest tag")
	default:
		return "unknown"
	}
}

// humanSize formats a byte count using binary prefixes (KiB, MiB). For
// values smaller than 1 KiB it returns plain bytes.
func humanSize(b int64) string {
	const (
		kib = 1024
		mib = 1024 * 1024
	)
	switch {
	case b < kib:
		return fmt.Sprintf("%d B", b)
	case b < mib:
		return fmt.Sprintf("%.1f KiB", float64(b)/kib)
	default:
		return fmt.Sprintf("%.1f MiB", float64(b)/mib)
	}
}
