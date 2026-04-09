package client

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "HTTP 400 not retryable",
			err:  fmt.Errorf("HTTP 400: bad request"),
			want: false,
		},
		{
			name: "HTTP 404 not retryable",
			err:  fmt.Errorf("HTTP 404: not found"),
			want: false,
		},
		{
			name: "HTTP 500 retryable",
			err:  fmt.Errorf("HTTP 500: internal server error"),
			want: true,
		},
		{
			name: "HTTP 502 retryable",
			err:  fmt.Errorf("HTTP 502: bad gateway"),
			want: true,
		},
		{
			name: "HTTP 503 retryable",
			err:  fmt.Errorf("HTTP 503: service unavailable"),
			want: true,
		},
		{
			name: "wrapped HTTP 500 (Vertex AI path)",
			err:  fmt.Errorf("send request failed: HTTP 500: oops"),
			want: true,
		},
		{
			name: "url.Error wrapping net error",
			err: &url.Error{
				Op:  "Get",
				URL: "http://example.com",
				Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			},
			want: true,
		},
		{
			name: "net.DNSError",
			err:  &net.DNSError{Err: "no such host", Name: "example.com"},
			want: true,
		},
		{
			name: "wrapped net.DNSError",
			err:  fmt.Errorf("resolve failed: %w", &net.DNSError{Err: "no such host", Name: "example.com"}),
			want: true,
		},
		{
			name: "a2a.ErrInternalError (standard A2A path)",
			err:  a2a.NewError(a2a.ErrInternalError, "something broke"),
			want: true,
		},
		{
			name: "a2a.ErrServerError (standard A2A path)",
			err:  a2a.NewError(a2a.ErrServerError, "server error"),
			want: true,
		},
		{
			name: "wrapped a2a.ErrInternalError",
			err:  fmt.Errorf("call failed: %w", a2a.NewError(a2a.ErrInternalError, "oops")),
			want: true,
		},
		{
			name: "a2a.ErrInvalidRequest not retryable",
			err:  a2a.NewError(a2a.ErrInvalidRequest, "bad request"),
			want: false,
		},
		{
			name: "a2a.ErrTaskNotFound not retryable",
			err:  a2a.NewError(a2a.ErrTaskNotFound, "not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	t.Run("attempt 0 is around 1-1.5s", func(t *testing.T) {
		d := backoff(0)
		if d < time.Second || d > 2*time.Second {
			t.Errorf("backoff(0) = %v, want between 1s and 2s", d)
		}
	})

	t.Run("attempt 1 is around 2-3s", func(t *testing.T) {
		d := backoff(1)
		if d < 2*time.Second || d > 4*time.Second {
			t.Errorf("backoff(1) = %v, want between 2s and 4s", d)
		}
	})

	t.Run("high attempt is capped at 45s", func(t *testing.T) {
		d := backoff(20)
		// 30s base + up to 15s jitter = max 45s
		if d > 45*time.Second {
			t.Errorf("backoff(20) = %v, want <= 45s", d)
		}
		if d < 30*time.Second {
			t.Errorf("backoff(20) = %v, want >= 30s", d)
		}
	})
}

func TestRetry_SucceedsFirstAttempt(t *testing.T) {
	ctx := context.Background()
	calls := 0

	result, err := retry(ctx, 3, func() (string, error) {
		calls++
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetry_SucceedsAfterTransientFailure(t *testing.T) {
	ctx := context.Background()
	calls := 0

	result, err := retry(ctx, 3, func() (string, error) {
		calls++
		if calls < 3 {
			return "", fmt.Errorf("HTTP 503: service unavailable")
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetry_GivesUpAfterMaxRetries(t *testing.T) {
	ctx := context.Background()
	calls := 0

	_, err := retry(ctx, 2, func() (string, error) {
		calls++
		return "", fmt.Errorf("HTTP 500: internal server error")
	})

	if err == nil {
		t.Fatal("expected error but got nil")
	}
	// maxRetries=2 means 3 total attempts (initial + 2 retries)
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetry_DoesNotRetryNonRetryableError(t *testing.T) {
	ctx := context.Background()
	calls := 0

	_, err := retry(ctx, 3, func() (string, error) {
		calls++
		return "", fmt.Errorf("HTTP 400: bad request")
	})

	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry 4xx)", calls)
	}
}

func TestRetry_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := retry(ctx, 10, func() (string, error) {
		calls++
		return "", fmt.Errorf("HTTP 500: internal server error")
	})

	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	// Should have been cancelled before exhausting all 11 attempts
	if calls >= 11 {
		t.Errorf("calls = %d, expected fewer than 11 (context should cancel early)", calls)
	}
}

// mockA2AClient is a minimal mock for testing retryClient.
type mockA2AClient struct {
	sendMessageCalls          int
	sendStreamingMessageCalls int
}

func (m *mockA2AClient) SendMessage(_ context.Context, _ *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	m.sendMessageCalls++
	return nil, nil
}

func (m *mockA2AClient) SendStreamingMessage(_ context.Context, _ *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error] {
	m.sendStreamingMessageCalls++
	return func(yield func(a2a.Event, error) bool) {}
}

func (m *mockA2AClient) GetTask(_ context.Context, _ *a2a.GetTaskRequest) (*a2a.Task, error) {
	return nil, nil
}
func (m *mockA2AClient) CancelTask(_ context.Context, _ *a2a.CancelTaskRequest) (*a2a.Task, error) {
	return nil, nil
}
func (m *mockA2AClient) ListTasks(_ context.Context, _ *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return nil, nil
}
func (m *mockA2AClient) CreateTaskPushConfig(_ context.Context, _ *a2a.CreateTaskPushConfigRequest) (*a2a.TaskPushConfig, error) {
	return nil, nil
}
func (m *mockA2AClient) GetTaskPushConfig(_ context.Context, _ *a2a.GetTaskPushConfigRequest) (*a2a.TaskPushConfig, error) {
	return nil, nil
}
func (m *mockA2AClient) ListTaskPushConfigs(_ context.Context, _ *a2a.ListTaskPushConfigRequest) ([]*a2a.TaskPushConfig, error) {
	return nil, nil
}
func (m *mockA2AClient) DeleteTaskPushConfig(_ context.Context, _ *a2a.DeleteTaskPushConfigRequest) error {
	return nil
}
func (m *mockA2AClient) Destroy() error { return nil }

func TestRetryClient_StreamNotRetried(t *testing.T) {
	mock := &mockA2AClient{}
	rc := &retryClient{inner: mock, maxRetries: 3}

	seq := rc.SendStreamingMessage(context.Background(), &a2a.SendMessageRequest{})
	// Consume the iterator
	for range seq {
	}

	if mock.sendStreamingMessageCalls != 1 {
		t.Errorf("sendStreamingMessageCalls = %d, want 1 (should pass through without retry)", mock.sendStreamingMessageCalls)
	}
}
