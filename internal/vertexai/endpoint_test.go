package vertexai

import (
	"testing"
)

func TestParseEndpoint_NormalizesV1ToV1beta1(t *testing.T) {
	ep, err := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1/projects/my-project/locations/us-central1/reasoningEngines/12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/my-project/locations/us-central1/reasoningEngines/12345"
	if ep.base != want {
		t.Errorf("base URL mismatch:\n  got:  %s\n  want: %s", ep.base, want)
	}
}

func TestParseEndpoint_V1beta1AlreadyNormalized(t *testing.T) {
	input := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/my-project/locations/us-central1/reasoningEngines/12345"
	ep, err := ParseEndpoint(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ep.base != input {
		t.Errorf("base URL should be unchanged:\n  got:  %s\n  want: %s", ep.base, input)
	}
}

func TestParseEndpoint_StripsTrailingSlash(t *testing.T) {
	ep, err := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/my-project/locations/us-central1/reasoningEngines/12345/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/my-project/locations/us-central1/reasoningEngines/12345"
	if ep.base != want {
		t.Errorf("trailing slash not stripped:\n  got:  %s\n  want: %s", ep.base, want)
	}
}

func TestParseEndpoint_StripsTrailingQuery(t *testing.T) {
	ep, err := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123:query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123"
	if ep.base != want {
		t.Errorf(":query not stripped:\n  got:  %s\n  want: %s", ep.base, want)
	}
}

func TestParseEndpoint_StripsTrailingQueryWithSlash(t *testing.T) {
	ep, err := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123:query/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123"
	if ep.base != want {
		t.Errorf(":query/ not stripped:\n  got:  %s\n  want: %s", ep.base, want)
	}
}

func TestParseEndpoint_EmptyURL(t *testing.T) {
	_, err := ParseEndpoint("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestParseEndpoint_MissingScheme(t *testing.T) {
	_, err := ParseEndpoint("us-central1-aiplatform.googleapis.com/v1/projects/p/locations/l/reasoningEngines/123")
	if err == nil {
		t.Fatal("expected error for URL without scheme")
	}
}

func TestEndpoint_CardURL(t *testing.T) {
	ep, _ := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123")
	got := ep.CardURL()
	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123/a2a/v1/card"
	if got != want {
		t.Errorf("CardURL:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestEndpoint_SendURL(t *testing.T) {
	ep, _ := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123")
	got := ep.SendURL()
	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123/a2a/v1/message:send"
	if got != want {
		t.Errorf("SendURL:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestEndpoint_StreamURL(t *testing.T) {
	ep, _ := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123")
	got := ep.StreamURL()
	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123/a2a/v1/message:stream"
	if got != want {
		t.Errorf("StreamURL:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestEndpoint_TaskURL(t *testing.T) {
	ep, _ := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123")
	got := ep.TaskURL("task-abc")
	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123/a2a/v1/tasks/task-abc"
	if got != want {
		t.Errorf("TaskURL:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestEndpoint_CancelTaskURL(t *testing.T) {
	ep, _ := ParseEndpoint("https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123")
	got := ep.CancelTaskURL("task-abc")
	want := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/p/locations/l/reasoningEngines/123/a2a/v1/tasks/task-abc:cancel"
	if got != want {
		t.Errorf("CancelTaskURL:\n  got:  %s\n  want: %s", got, want)
	}
}
