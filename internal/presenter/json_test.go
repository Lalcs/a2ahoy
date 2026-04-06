package presenter

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPrintJSON_Struct(t *testing.T) {
	var buf bytes.Buffer
	input := struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}{Name: "Alice", Age: 30}

	if err := PrintJSON(&buf, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"name": "Alice"`) {
		t.Errorf("expected name field, got:\n%s", got)
	}
	if !strings.Contains(got, `"age": 30`) {
		t.Errorf("expected age field, got:\n%s", got)
	}
}

func TestPrintJSON_Nil(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintJSON(&buf, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != "null" {
		t.Errorf("got %q, want %q", got, "null")
	}
}

func TestPrintJSON_Indented(t *testing.T) {
	var buf bytes.Buffer
	input := map[string]string{"key": "value"}

	if err := PrintJSON(&buf, input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "  ") {
		t.Error("expected indented output")
	}
}

func TestPrintJSON_MarshalError(t *testing.T) {
	var buf bytes.Buffer
	err := PrintJSON(&buf, make(chan int))
	if err == nil {
		t.Fatal("expected error for non-marshalable type")
	}
	if !strings.Contains(err.Error(), "failed to encode JSON") {
		t.Errorf("unexpected error message: %v", err)
	}
}

type errWriter struct{}

func (w *errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

func TestPrintJSON_WriteError(t *testing.T) {
	err := PrintJSON(&errWriter{}, map[string]string{"key": "value"})
	if err == nil {
		t.Fatal("expected error for failing writer")
	}
}
