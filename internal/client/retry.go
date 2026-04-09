package client

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// retryClient wraps an A2AClient with retry logic for non-streaming operations.
// Streaming (SendStreamingMessage) is passed through without retry because SSE
// streams are not safely re-entrant.
type retryClient struct {
	inner      A2AClient
	maxRetries int
}

// isRetryable returns true for transient errors that warrant a retry.
// HTTP 4xx and application-level A2A errors are NOT retried.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// net.Error covers timeouts, connection refused/reset, DNS failures,
	// and url.Error (which implements net.Error).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// The standard and v0.3 A2A transports return *a2a.Error wrapping
	// sentinel errors. Retry server-side errors (ErrInternalError,
	// ErrServerError) but not client-side ones (ErrInvalidRequest, etc.).
	if errors.Is(err, a2a.ErrInternalError) || errors.Is(err, a2a.ErrServerError) {
		return true
	}

	// The Vertex AI client formats HTTP errors as "HTTP <status>: <body>".
	msg := err.Error()
	if strings.Contains(msg, "HTTP 5") {
		return true
	}

	return false
}

// maxBackoffShift is the largest safe shift for time.Second (int64
// nanoseconds) before overflow. 2^30 * 1e9 ≈ 1.07e18 < math.MaxInt64.
const maxBackoffShift = 30

// backoff returns the sleep duration for attempt i (0-indexed),
// using exponential backoff with jitter. Capped at 30 seconds.
func backoff(attempt int) time.Duration {
	if attempt > maxBackoffShift {
		attempt = maxBackoffShift
	}
	d := time.Second << attempt
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	jitter := time.Duration(rand.Int64N(int64(d / 2)))
	return d + jitter
}

// retry executes fn up to maxRetries+1 times, sleeping between retryable
// failures. The loop respects context cancellation so Ctrl+C terminates
// immediately rather than waiting for the backoff sleep.
func retry[T any](ctx context.Context, maxRetries int, fn func() (T, error)) (T, error) {
	var result T
	var err error
	for attempt := range maxRetries + 1 {
		result, err = fn()
		if err == nil || !isRetryable(err) || attempt == maxRetries {
			return result, err
		}
		timer := time.NewTimer(backoff(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return result, err
}

func (r *retryClient) SendMessage(ctx context.Context, req *a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	return retry(ctx, r.maxRetries, func() (a2a.SendMessageResult, error) {
		return r.inner.SendMessage(ctx, req)
	})
}

// SendStreamingMessage is NOT retried — SSE streams are not safely re-entrant.
func (r *retryClient) SendStreamingMessage(ctx context.Context, req *a2a.SendMessageRequest) iter.Seq2[a2a.Event, error] {
	return r.inner.SendStreamingMessage(ctx, req)
}

func (r *retryClient) GetTask(ctx context.Context, req *a2a.GetTaskRequest) (*a2a.Task, error) {
	return retry(ctx, r.maxRetries, func() (*a2a.Task, error) {
		return r.inner.GetTask(ctx, req)
	})
}

func (r *retryClient) CancelTask(ctx context.Context, req *a2a.CancelTaskRequest) (*a2a.Task, error) {
	return retry(ctx, r.maxRetries, func() (*a2a.Task, error) {
		return r.inner.CancelTask(ctx, req)
	})
}

func (r *retryClient) ListTasks(ctx context.Context, req *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return retry(ctx, r.maxRetries, func() (*a2a.ListTasksResponse, error) {
		return r.inner.ListTasks(ctx, req)
	})
}

func (r *retryClient) CreateTaskPushConfig(ctx context.Context, req *a2a.CreateTaskPushConfigRequest) (*a2a.TaskPushConfig, error) {
	return retry(ctx, r.maxRetries, func() (*a2a.TaskPushConfig, error) {
		return r.inner.CreateTaskPushConfig(ctx, req)
	})
}

func (r *retryClient) GetTaskPushConfig(ctx context.Context, req *a2a.GetTaskPushConfigRequest) (*a2a.TaskPushConfig, error) {
	return retry(ctx, r.maxRetries, func() (*a2a.TaskPushConfig, error) {
		return r.inner.GetTaskPushConfig(ctx, req)
	})
}

func (r *retryClient) ListTaskPushConfigs(ctx context.Context, req *a2a.ListTaskPushConfigRequest) ([]*a2a.TaskPushConfig, error) {
	return retry(ctx, r.maxRetries, func() ([]*a2a.TaskPushConfig, error) {
		return r.inner.ListTaskPushConfigs(ctx, req)
	})
}

func (r *retryClient) DeleteTaskPushConfig(ctx context.Context, req *a2a.DeleteTaskPushConfigRequest) error {
	_, err := retry(ctx, r.maxRetries, func() (struct{}, error) {
		return struct{}{}, r.inner.DeleteTaskPushConfig(ctx, req)
	})
	return err
}

func (r *retryClient) Destroy() error {
	return r.inner.Destroy()
}
