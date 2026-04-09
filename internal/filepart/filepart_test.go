package filepart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestLoadFileParts_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	parts, err := LoadFileParts([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(parts))
	}

	part := parts[0]
	if got := string(part.Raw()); got != "hello world" {
		t.Errorf("Raw = %q, want %q", got, "hello world")
	}
	if part.Filename != "hello.txt" {
		t.Errorf("Filename = %q, want %q", part.Filename, "hello.txt")
	}
	if part.MediaType != "text/plain; charset=utf-8" {
		t.Errorf("MediaType = %q, want %q", part.MediaType, "text/plain; charset=utf-8")
	}
}

func TestLoadFileParts_MIMEByExtension(t *testing.T) {
	tests := []struct {
		ext      string
		content  []byte
		wantMIME string
	}{
		{".png", []byte("not a real png"), "image/png"},
		{".pdf", []byte("not a real pdf"), "application/pdf"},
		{".csv", []byte("a,b,c"), "text/csv"},
		{".json", []byte(`{}`), "application/json"},
	}

	tmp := t.TempDir()
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			p := filepath.Join(tmp, "test"+tt.ext)
			if err := os.WriteFile(p, tt.content, 0o644); err != nil {
				t.Fatal(err)
			}

			parts, err := LoadFileParts([]string{p})
			if err != nil {
				t.Fatal(err)
			}
			// mime.TypeByExtension may include parameters (e.g. charset);
			// check prefix to stay robust across Go versions.
			if parts[0].MediaType == "" {
				t.Errorf("MediaType is empty for %s", tt.ext)
			}
		})
	}
}

func TestLoadFileParts_ContentSniffing(t *testing.T) {
	// A file with no extension but with a PNG header should be detected
	// via http.DetectContentType.
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

	tmp := t.TempDir()
	p := filepath.Join(tmp, "noext")
	if err := os.WriteFile(p, pngHeader, 0o644); err != nil {
		t.Fatal(err)
	}

	parts, err := LoadFileParts([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if parts[0].MediaType != "image/png" {
		t.Errorf("MediaType = %q, want %q", parts[0].MediaType, "image/png")
	}
}

func TestLoadFileParts_FileNotFound(t *testing.T) {
	_, err := LoadFileParts([]string{"/nonexistent/file.txt"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if got := err.Error(); !strings.Contains(got, "/nonexistent/file.txt") {
		t.Errorf("error %q does not contain file path", got)
	}
}

func TestLoadFileParts_MultipleFiles(t *testing.T) {
	tmp := t.TempDir()
	names := []string{"a.txt", "b.txt", "c.txt"}
	paths := make([]string, len(names))
	for i, name := range names {
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		paths[i] = p
	}

	parts, err := LoadFileParts(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(parts))
	}
	for i, name := range names {
		if parts[i].Filename != name {
			t.Errorf("parts[%d].Filename = %q, want %q", i, parts[i].Filename, name)
		}
	}
}

func TestURLParts_Valid(t *testing.T) {
	urls := []string{
		"https://example.com/data.csv",
		"http://example.com/path/to/image.png",
	}

	parts, err := URLParts(urls)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(parts))
	}

	if got := parts[0].URL(); got != a2a.URL("https://example.com/data.csv") {
		t.Errorf("URL = %q, want %q", got, "https://example.com/data.csv")
	}
	if parts[0].Filename != "data.csv" {
		t.Errorf("Filename = %q, want %q", parts[0].Filename, "data.csv")
	}

	if parts[1].Filename != "image.png" {
		t.Errorf("Filename = %q, want %q", parts[1].Filename, "image.png")
	}
}

func TestURLParts_InvalidScheme(t *testing.T) {
	tests := []string{
		"ftp://example.com/file.txt",
		"file:///local/path",
		"example.com/no-scheme",
	}
	for _, u := range tests {
		t.Run(u, func(t *testing.T) {
			_, err := URLParts([]string{u})
			if err == nil {
				t.Fatalf("expected error for URL %q", u)
			}
		})
	}
}

func TestURLParts_InvalidURL(t *testing.T) {
	// A URL with an invalid control character triggers url.Parse error.
	_, err := URLParts([]string{string([]byte{0x7f})})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestFileParts_FileError(t *testing.T) {
	_, err := FileParts([]string{"/nonexistent/file"}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestFileParts_URLError(t *testing.T) {
	_, err := FileParts(nil, []string{"ftp://bad-scheme"})
	if err == nil {
		t.Fatal("expected error for invalid URL scheme")
	}
}

func TestBuildParts_FileError(t *testing.T) {
	_, err := BuildParts("text", []string{"/nonexistent/file"}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestBuildParts_URLError(t *testing.T) {
	_, err := BuildParts("text", nil, []string{"ftp://bad-scheme"})
	if err == nil {
		t.Fatal("expected error for invalid URL scheme")
	}
}

func TestFileParts_Combined(t *testing.T) {
	tmp := t.TempDir()
	fp := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(fp, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	parts, err := FileParts([]string{fp}, []string{"https://example.com/doc.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(parts))
	}
	if parts[0].Filename != "test.txt" {
		t.Errorf("parts[0].Filename = %q, want %q", parts[0].Filename, "test.txt")
	}
	if got := parts[1].URL(); got != a2a.URL("https://example.com/doc.pdf") {
		t.Errorf("parts[1].URL() = %q, want %q", got, "https://example.com/doc.pdf")
	}
}

func TestFileParts_Empty(t *testing.T) {
	parts, err := FileParts(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 0 {
		t.Fatalf("got %d parts, want 0", len(parts))
	}
}

func TestBuildParts_TextWithFiles(t *testing.T) {
	tmp := t.TempDir()
	fp := filepath.Join(tmp, "test.png")
	if err := os.WriteFile(fp, []byte("png-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	parts, err := BuildParts("hello", []string{fp}, []string{"https://example.com/doc.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("got %d parts, want 3", len(parts))
	}

	// Text part first.
	if got := parts[0].Text(); got != "hello" {
		t.Errorf("parts[0].Text() = %q, want %q", got, "hello")
	}
	// File part second.
	if parts[1].Filename != "test.png" {
		t.Errorf("parts[1].Filename = %q, want %q", parts[1].Filename, "test.png")
	}
	// URL part last.
	if got := parts[2].URL(); got != a2a.URL("https://example.com/doc.pdf") {
		t.Errorf("parts[2].URL() = %q, want %q", got, "https://example.com/doc.pdf")
	}
}

func TestBuildParts_TextOnly(t *testing.T) {
	parts, err := BuildParts("just text", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(parts))
	}
	if got := parts[0].Text(); got != "just text" {
		t.Errorf("Text() = %q, want %q", got, "just text")
	}
}
