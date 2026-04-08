package updater

import (
	"context"
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
