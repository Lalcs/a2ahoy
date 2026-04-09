package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Installer downloads release assets and replaces the running binary on
// Linux and macOS. The implementation relies on POSIX semantics: the
// running process keeps its inode reference open, so renaming a new file
// over the executable path is safe.
type Installer struct {
	httpClient *http.Client

	// resolveExecutable is the function used to determine the path of the
	// running binary. Tests inject a fake to target a temp file instead.
	resolveExecutable func() (string, error)
}

// NewInstaller returns an Installer with a 5-minute HTTP timeout, matching
// the existing Vertex AI client's timeout for long-running downloads.
func NewInstaller() *Installer {
	return &Installer{
		httpClient:        &http.Client{Timeout: 5 * time.Minute},
		resolveExecutable: defaultResolveExecutable,
	}
}

// NewInstallerForTest returns an Installer whose resolveExecutable always
// returns the supplied path. This allows callers (e.g., cmd package tests)
// to redirect the install target to a temporary file instead of the
// running binary. Production code should use NewInstaller.
func NewInstallerForTest(executablePath string) *Installer {
	return &Installer{
		httpClient:        &http.Client{Timeout: 5 * time.Minute},
		resolveExecutable: func() (string, error) { return executablePath, nil },
	}
}

// defaultResolveExecutable returns the path of the running binary with
// symlinks resolved to their real targets. This is required when the
// binary is installed via Homebrew or a similar tool that exposes the
// command through a wrapper symlink.
func defaultResolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		// Fall back to the unresolved path: better than failing.
		return exe, nil
	}
	return resolved, nil
}

// Plan describes a planned binary replacement. It is constructed by
// Prepare and consumed by Install. Holding the data on a value type makes
// the operation easier to log and inspect from callers.
type Plan struct {
	Asset      *Asset
	Release    *Release
	TargetPath string
	BackupPath string
}

// Prepare validates that the running binary can be replaced. It returns a
// Plan ready to be passed to Install, or an error explaining why the
// update cannot proceed (typically a missing write permission on the
// install directory).
//
// Prepare must not modify the filesystem destructively: it should be safe
// to call from --check-only flows in the future.
func (i *Installer) Prepare(asset *Asset, rel *Release) (*Plan, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is required")
	}
	if rel == nil {
		return nil, fmt.Errorf("release is required")
	}

	target, err := i.resolveExecutable()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(target)
	if err := checkDirWritable(dir); err != nil {
		return nil, err
	}

	return &Plan{
		Asset:      asset,
		Release:    rel,
		TargetPath: target,
		BackupPath: target + ".bak",
	}, nil
}

// checkDirWritable verifies that the process can create files in dir by
// actually creating and removing a tiny probe file. We avoid os.Stat-based
// checks because filesystem permissions on macOS and Linux can be subtler
// than the mode bits suggest (ACLs, mount options, etc.).
func checkDirWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".a2ahoy-write-check-*")
	if err != nil {
		return fmt.Errorf(
			"cannot write to %s: %w\nhint: reinstall via install.sh "+
				"(installs to ~/.local/bin without sudo by default)",
			dir, err)
	}
	probeName := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probeName)
	return nil
}

// Install downloads the planned asset and atomically replaces the running
// binary. On any error after the backup has been created, Install attempts
// to restore the backup so the user is left with a working binary.
func (i *Installer) Install(ctx context.Context, plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}

	dir := filepath.Dir(plan.TargetPath)

	// Step 1: create the temp file in the same directory as the target
	// to guarantee that the final os.Rename does not cross filesystems
	// (which would fail with EXDEV on Linux).
	tmpFile, err := os.CreateTemp(dir, ".a2ahoy-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Sentinel: tmpPath is cleared once the rename succeeds. The deferred
	// cleanup removes the temp file in every other path.
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	// Step 2: download the asset into the temp file.
	if err := i.downloadAsset(ctx, plan.Asset, tmpFile); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Step 3: ensure the new binary is executable.
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	// Step 4: move the existing binary aside as a backup.
	if err := os.Rename(plan.TargetPath, plan.BackupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	backupRestored := false
	defer func() {
		// If we failed before the swap completed, the deferred restore
		// in the error branch already moved the backup back. Otherwise
		// remove the leftover backup as a best-effort cleanup.
		if backupRestored {
			return
		}
		_ = os.Remove(plan.BackupPath)
	}()

	// Step 5: atomically place the new binary at the target path.
	if err := os.Rename(tmpPath, plan.TargetPath); err != nil {
		// Rollback: restore the backup so the user keeps a working
		// binary. We deliberately ignore the rollback error because
		// the original error is more useful to surface.
		if restoreErr := os.Rename(plan.BackupPath, plan.TargetPath); restoreErr == nil {
			backupRestored = true
		}
		return fmt.Errorf("install new binary (backup restored): %w", err)
	}

	// The temp file has been consumed by the successful rename, so the
	// deferred cleanup must not try to remove it again.
	tmpPath = ""
	return nil
}

// downloadAsset streams the asset's bytes into dst, returning a wrapped
// error on any failure. The HTTP request inherits the supplied context so
// the download can be cancelled cooperatively.
func (i *Installer) downloadAsset(ctx context.Context, asset *Asset, dst io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d for %s", resp.StatusCode, asset.BrowserDownloadURL)
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("download body: %w", err)
	}
	return nil
}
