// Package filepart provides helpers for constructing [a2a.Part] values
// from local files and URLs. It is consumed by the send, stream, and
// chat commands to turn --file / --file-url flags into message parts.
package filepart

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// LoadFileParts reads each local file path, detects its MIME type, and
// returns a slice of [a2a.Part] with Content set to [a2a.Raw].
//
// MIME detection uses [mime.TypeByExtension] first; if the extension is
// unknown it falls back to [http.DetectContentType] (sniffs the first
// 512 bytes). Filename is set to the base name of the path.
func LoadFileParts(paths []string) ([]*a2a.Part, error) {
	parts := make([]*a2a.Part, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", p, err)
		}

		mediaType := detectMIME(p, data)

		part := a2a.NewRawPart(data)
		part.Filename = filepath.Base(p)
		part.MediaType = mediaType
		parts = append(parts, part)
	}
	return parts, nil
}

// URLParts constructs [a2a.Part] values from URL strings. Each URL must
// use an http or https scheme. The MIME type is left empty because the
// receiving agent is expected to resolve it from the URL.
func URLParts(urls []string) ([]*a2a.Part, error) {
	parts := make([]*a2a.Part, 0, len(urls))
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("unsupported URL scheme %q (only http and https are allowed): %s", u.Scheme, raw)
		}

		part := a2a.NewFileURLPart(a2a.URL(raw), "")
		// Extract filename from the last path segment when available.
		if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
			part.Filename = base
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// FileParts loads local files and URL references into a combined
// []*a2a.Part slice (files first, then URLs). This is the building
// block shared by [BuildParts] and callers that need file parts
// without a leading text part (e.g. the chat command).
func FileParts(filePaths, fileURLs []string) ([]*a2a.Part, error) {
	fileParts, err := LoadFileParts(filePaths)
	if err != nil {
		return nil, err
	}
	urlParts, err := URLParts(fileURLs)
	if err != nil {
		return nil, err
	}
	return append(fileParts, urlParts...), nil
}

// BuildParts constructs the full []*a2a.Part slice for a message: a
// text part first, followed by any file parts, then URL parts.
func BuildParts(text string, filePaths, fileURLs []string) ([]*a2a.Part, error) {
	fp, err := FileParts(filePaths, fileURLs)
	if err != nil {
		return nil, err
	}

	parts := make([]*a2a.Part, 0, 1+len(fp))
	parts = append(parts, a2a.NewTextPart(text))
	parts = append(parts, fp...)
	return parts, nil
}

// detectMIME returns a MIME type for the file at the given path.
// It tries the file extension first, then falls back to content sniffing.
func detectMIME(filePath string, data []byte) string {
	if mt := mime.TypeByExtension(filepath.Ext(filePath)); mt != "" {
		return mt
	}
	return http.DetectContentType(data)
}
