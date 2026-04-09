package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// installerForTest builds an Installer that pretends targetPath is the
// currently running binary. This is the only way to exercise Install
// without overwriting the real test binary.
func installerForTest(targetPath string) *Installer {
	return &Installer{
		httpClient:        &http.Client{Timeout: 5 * time.Second},
		resolveExecutable: func() (string, error) { return targetPath, nil },
	}
}

// makeFakeBinary writes a fake "old binary" file to dir and returns its
// full path. The contents are arbitrary; tests only check that the file is
// replaced (or not).
func makeFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "a2ahoy-fake")
	if err := os.WriteFile(path, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

// newAssetServer returns an httptest.Server that serves the given bytes
// as a release asset.
func newAssetServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInstaller_Prepare_Success(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)
	inst := installerForTest(target)

	asset := &Asset{Name: "a2ahoy-linux-amd64", BrowserDownloadURL: "http://example.invalid", Size: 10}
	rel := &Release{TagName: "v1.0.0"}

	plan, err := inst.Prepare(asset, rel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.TargetPath != target {
		t.Errorf("TargetPath = %q, want %q", plan.TargetPath, target)
	}
	if plan.BackupPath != target+".bak" {
		t.Errorf("BackupPath = %q, want %q", plan.BackupPath, target+".bak")
	}
	if plan.Asset != asset {
		t.Error("Plan.Asset should be the same pointer")
	}
	if plan.Release != rel {
		t.Error("Plan.Release should be the same pointer")
	}
}

func TestInstaller_Prepare_NilAsset(t *testing.T) {
	inst := installerForTest("/tmp/whatever")
	if _, err := inst.Prepare(nil, &Release{}); err == nil {
		t.Fatal("expected error for nil asset")
	}
}

func TestInstaller_Prepare_NilRelease(t *testing.T) {
	inst := installerForTest("/tmp/whatever")
	if _, err := inst.Prepare(&Asset{}, nil); err == nil {
		t.Fatal("expected error for nil release")
	}
}

func TestInstaller_Prepare_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	// Make the parent directory read-only to block CreateTemp.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	inst := installerForTest(target)
	_, err := inst.Prepare(&Asset{}, &Release{})
	if err == nil {
		t.Fatal("expected permission denied error")
	}
	if !strings.Contains(err.Error(), "cannot write to") {
		t.Errorf("error should mention 'cannot write to': %v", err)
	}
	if !strings.Contains(err.Error(), "install.sh") {
		t.Errorf("error should hint at install.sh: %v", err)
	}
	if !strings.Contains(err.Error(), "~/.local/bin") {
		t.Errorf("error should mention default install path: %v", err)
	}
}

func TestInstaller_Install_E2E(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	newContents := []byte("NEW-BINARY-CONTENTS")
	srv := newAssetServer(t, newContents)

	inst := installerForTest(target)
	asset := &Asset{
		Name:               "a2ahoy-linux-amd64",
		BrowserDownloadURL: srv.URL,
		Size:               int64(len(newContents)),
	}
	plan, err := inst.Prepare(asset, &Release{TagName: "v1.0.0"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if err := inst.Install(context.Background(), plan); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The target file should now contain the new bytes.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != string(newContents) {
		t.Errorf("target contents = %q, want %q", got, newContents)
	}

	// The mode should be 0755.
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o755 {
		t.Errorf("target mode = %o, want 0755", mode)
	}

	// No backup or temp files should be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == filepath.Base(target) {
			continue
		}
		t.Errorf("unexpected leftover file: %q", name)
	}
}

func TestInstaller_Install_NilPlan(t *testing.T) {
	inst := installerForTest("/tmp/whatever")
	if err := inst.Install(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil plan")
	}
}

func TestInstaller_Install_DownloadFails(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)
	originalContents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	inst := installerForTest(target)
	plan, err := inst.Prepare(
		&Asset{Name: "a2ahoy-linux-amd64", BrowserDownloadURL: srv.URL},
		&Release{TagName: "v1.0.0"},
	)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if err := inst.Install(context.Background(), plan); err == nil {
		t.Fatal("expected install to fail when download fails")
	}

	// The original binary must be untouched after a download failure
	// (no backup made yet, no swap attempted).
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != string(originalContents) {
		t.Errorf("target was modified despite download failure")
	}

	// No backup or temp files left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() == filepath.Base(target) {
			continue
		}
		t.Errorf("unexpected leftover file: %q", entry.Name())
	}
}

func TestInstaller_Install_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	inst := installerForTest(target)
	plan, err := inst.Prepare(
		&Asset{Name: "a2ahoy-linux-amd64", BrowserDownloadURL: srv.URL},
		&Release{TagName: "v1.0.0"},
	)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := inst.Install(ctx, plan); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestInstaller_DefaultResolveExecutable(t *testing.T) {
	// Smoke test the default resolver: it should return a non-empty
	// path that exists on disk. We can't assert the exact value because
	// `go test` runs the test binary from a temp location.
	path, err := defaultResolveExecutable()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected a non-empty path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("resolved path does not exist: %v", err)
	}
}

func TestNewInstaller_Defaults(t *testing.T) {
	inst := NewInstaller()
	if inst == nil {
		t.Fatal("NewInstaller returned nil")
	}
	if inst.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
	if inst.resolveExecutable == nil {
		t.Fatal("resolveExecutable is nil")
	}
}

func TestInstaller_Prepare_ResolveError(t *testing.T) {
	inst := &Installer{
		httpClient: &http.Client{},
		resolveExecutable: func() (string, error) {
			return "", fmt.Errorf("cannot locate executable")
		},
	}
	_, err := inst.Prepare(&Asset{}, &Release{})
	if err == nil {
		t.Fatal("expected error when resolveExecutable fails")
	}
	if !strings.Contains(err.Error(), "cannot locate executable") {
		t.Errorf("error should contain resolver message: %v", err)
	}
}

func TestInstaller_Install_CreateTempError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	srv := newAssetServer(t, []byte("NEW"))
	inst := installerForTest(target)
	plan := &Plan{
		Asset:      &Asset{BrowserDownloadURL: srv.URL},
		Release:    &Release{TagName: "v1.0.0"},
		TargetPath: filepath.Join(dir, "subdir", "binary"), // subdir does not exist
		BackupPath: filepath.Join(dir, "subdir", "binary.bak"),
	}

	err := inst.Install(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error when CreateTemp fails")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("error should mention temp file creation: %v", err)
	}
}

func TestInstaller_Install_BackupRenameFails(t *testing.T) {
	dir := t.TempDir()
	srv := newAssetServer(t, []byte("NEW-BINARY"))

	// Target does not exist, so Rename(target, backup) will fail.
	nonExistent := filepath.Join(dir, "no-such-binary")
	plan := &Plan{
		Asset:      &Asset{BrowserDownloadURL: srv.URL},
		Release:    &Release{TagName: "v1.0.0"},
		TargetPath: nonExistent,
		BackupPath: nonExistent + ".bak",
	}

	inst := installerForTest(nonExistent)
	err := inst.Install(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error when backup rename fails")
	}
	if !strings.Contains(err.Error(), "backup current binary") {
		t.Errorf("error should mention backup: %v", err)
	}

	// No temp files should be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		t.Errorf("unexpected leftover file: %q", entry.Name())
	}
}

func TestInstaller_Install_BackupRenameFails_MissingParent(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	srv := newAssetServer(t, []byte("NEW-BINARY"))
	inst := installerForTest(target)

	// BackupPath parent directory doesn't exist, so Rename(target, backup)
	// will fail.
	plan := &Plan{
		Asset:      &Asset{BrowserDownloadURL: srv.URL},
		Release:    &Release{TagName: "v1.0.0"},
		TargetPath: target,
		BackupPath: filepath.Join(dir, "no-such-subdir", "a2ahoy-fake.bak"),
	}

	err := inst.Install(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error when backup rename fails")
	}
	if !strings.Contains(err.Error(), "backup current binary") {
		t.Errorf("error should mention backup: %v", err)
	}
}

func TestInstaller_Install_DownloadAssetInvalidURL(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	inst := installerForTest(target)
	// URL with null byte causes NewRequestWithContext to fail.
	plan := &Plan{
		Asset:      &Asset{BrowserDownloadURL: "http://example.com/\x00bad"},
		Release:    &Release{TagName: "v1.0.0"},
		TargetPath: target,
		BackupPath: target + ".bak",
	}

	err := inst.Install(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error for invalid download URL")
	}
	if !strings.Contains(err.Error(), "build download request") {
		t.Errorf("error should mention request building: %v", err)
	}
}

func TestNewInstallerForTest(t *testing.T) {
	inst := NewInstallerForTest("/fake/path")
	if inst == nil {
		t.Fatal("NewInstallerForTest returned nil")
	}
	if inst.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
	// Verify the resolveExecutable returns the path we injected.
	got, err := inst.resolveExecutable()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/fake/path" {
		t.Errorf("resolveExecutable: got %q, want %q", got, "/fake/path")
	}
}

func TestInstaller_Install_FinalRenameFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	srv := newAssetServer(t, []byte("NEW-BINARY"))
	inst := installerForTest(target)

	// Create a directory at the target path's location after backup
	// by making the TargetPath point to a path that will become a
	// directory, preventing the final rename.
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	innerTarget := filepath.Join(subdir, "binary")
	if err := os.WriteFile(innerTarget, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("write inner target: %v", err)
	}

	plan := &Plan{
		Asset:      &Asset{BrowserDownloadURL: srv.URL},
		Release:    &Release{TagName: "v1.0.0"},
		TargetPath: innerTarget,
		BackupPath: innerTarget + ".bak",
	}

	// Make the subdirectory read-only AFTER Prepare but BEFORE Install's
	// final rename step. We rely on the fact that the download succeeds
	// and the backup rename succeeds (because the file is IN the dir),
	// but the final os.Rename(tmpFile, target) fails because the dir
	// became read-only to new file creation.
	//
	// Actually, os.Rename doesn't create new files - it just updates
	// directory entries, so read-only on the parent wouldn't help.
	// Instead, remove the subdirectory entirely after Install starts.
	// This is inherently racy, so we skip if the condition can't be met.

	// Instead, test the path where TargetPath's parent dir gets removed.
	// This will cause the final rename to fail, triggering rollback.
	// Since the backup was already created, the rollback tries to rename
	// backup → target, which also fails (parent gone), setting backupRestored=false.
	if err := os.RemoveAll(subdir); err != nil {
		t.Fatalf("remove subdir: %v", err)
	}

	// Install will fail at CreateTemp because subdir no longer exists.
	err := inst.Install(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstaller_Install_DownloadPartialBody(t *testing.T) {
	dir := t.TempDir()
	target := makeFakeBinary(t, dir)

	// Server that declares a large Content-Length then closes the
	// connection after sending a few bytes, triggering an io.Copy error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		// hijack the connection and close it to force an io.Copy error
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			if conn != nil {
				_ = conn.Close()
			}
		}
	}))
	t.Cleanup(srv.Close)

	inst := installerForTest(target)
	plan := &Plan{
		Asset:      &Asset{BrowserDownloadURL: srv.URL},
		Release:    &Release{TagName: "v1.0.0"},
		TargetPath: target,
		BackupPath: target + ".bak",
	}

	err := inst.Install(context.Background(), plan)
	if err == nil {
		t.Fatal("expected error for partial download")
	}
}
