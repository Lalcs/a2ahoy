package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

func TestParseHeaders_Nil(t *testing.T) {
	got, err := ParseHeaders(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseHeaders_EmptySlice(t *testing.T) {
	got, err := ParseHeaders([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseHeaders_Single(t *testing.T) {
	got, err := ParseHeaders([]string{"X-Foo=bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got[0] != (HeaderEntry{Key: "X-Foo", Value: "bar"}) {
		t.Errorf("entry: got %+v, want {X-Foo bar}", got[0])
	}
}

func TestParseHeaders_Multiple(t *testing.T) {
	got, err := ParseHeaders([]string{"X-A=1", "X-B=2", "X-C=3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []HeaderEntry{
		{Key: "X-A", Value: "1"},
		{Key: "X-B", Value: "2"},
		{Key: "X-C", Value: "3"},
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseHeaders_ValueWithEquals(t *testing.T) {
	got, err := ParseHeaders([]string{"X-Token=foo=bar=baz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0] != (HeaderEntry{Key: "X-Token", Value: "foo=bar=baz"}) {
		t.Errorf("entry: got %+v, want {X-Token foo=bar=baz}", got[0])
	}
}

func TestParseHeaders_EmptyValue(t *testing.T) {
	got, err := ParseHeaders([]string{"X-Empty="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0] != (HeaderEntry{Key: "X-Empty", Value: ""}) {
		t.Errorf("entry: got %+v, want {X-Empty }", got[0])
	}
}

func TestParseHeaders_DuplicateKeyDistinctValues(t *testing.T) {
	// Both entries must be preserved to support HTTP multi-value headers.
	got, err := ParseHeaders([]string{"X-A=1", "X-A=2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got[0].Value != "1" || got[1].Value != "2" {
		t.Errorf("entries: got %+v, want [{X-A 1} {X-A 2}]", got)
	}
}

func TestParseHeaders_NoSeparator(t *testing.T) {
	_, err := ParseHeaders([]string{"foo"})
	if !errors.Is(err, ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
}

func TestParseHeaders_EmptyKey(t *testing.T) {
	_, err := ParseHeaders([]string{"=value"})
	if !errors.Is(err, ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
}

func TestParseHeaders_EmptyString(t *testing.T) {
	_, err := ParseHeaders([]string{""})
	if !errors.Is(err, ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
}

func TestParseHeaders_PartialFailureReturnsNil(t *testing.T) {
	got, err := ParseHeaders([]string{"X-A=1", "bad-entry"})
	if !errors.Is(err, ErrInvalidHeader) {
		t.Errorf("expected ErrInvalidHeader, got: %v", err)
	}
	if got != nil {
		t.Errorf("entries should be nil on error, got: %+v", got)
	}
}

func TestErrInvalidHeader_Wrapping(t *testing.T) {
	_, err := ParseHeaders([]string{"no-equals"})
	if !errors.Is(err, ErrInvalidHeader) {
		t.Errorf("errors.Is(err, ErrInvalidHeader) = false for: %v", err)
	}
}

func TestHeaderInterceptor_Before_MultipleHeaders(t *testing.T) {
	h := NewHeaderInterceptor([]HeaderEntry{
		{Key: "X-Foo", Value: "v1"},
		{Key: "X-Bar", Value: "v2"},
	})
	req := &a2aclient.Request{ServiceParams: make(a2aclient.ServiceParams)}

	ctx, result, err := h.Before(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	if result != nil {
		t.Errorf("result should be nil, got %v", result)
	}

	// ServiceParams normalises keys to lowercase per a2aclient spec.
	if v := req.ServiceParams.Get("x-foo"); len(v) != 1 || v[0] != "v1" {
		t.Errorf("x-foo: got %v, want [v1]", v)
	}
	if v := req.ServiceParams.Get("X-Bar"); len(v) != 1 || v[0] != "v2" {
		t.Errorf("X-Bar (case-insensitive): got %v, want [v2]", v)
	}
}

func TestHeaderInterceptor_Before_LowercaseKeyNormalisation(t *testing.T) {
	entries, err := ParseHeaders([]string{"X-Tenant-ID=tenant-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := NewHeaderInterceptor(entries)

	req := &a2aclient.Request{ServiceParams: make(a2aclient.ServiceParams)}
	if _, _, err := h.Before(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := req.ServiceParams["x-tenant-id"]; !ok {
		t.Errorf("expected lowercased key 'x-tenant-id' in ServiceParams, got keys: %v", keys(req.ServiceParams))
	}
}

func TestHeaderInterceptor_Before_EmptyInterceptor(t *testing.T) {
	h := NewHeaderInterceptor(nil)
	req := &a2aclient.Request{ServiceParams: make(a2aclient.ServiceParams)}

	if _, _, err := h.Before(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.ServiceParams) != 0 {
		t.Errorf("empty interceptor should not mutate ServiceParams, got: %v", req.ServiceParams)
	}
}

// keys returns the map keys for debugging output.
func keys(m a2aclient.ServiceParams) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
