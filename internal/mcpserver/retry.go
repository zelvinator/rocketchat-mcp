package mcpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetries     = 2
	baseRetryDelay = 500 * time.Millisecond
	maxRetryDelay  = 5 * time.Second
	maxRetryAfter  = time.Hour
)

// RateLimitError reports that Rocket.Chat rate limited a request.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter <= 0 {
		return "rocketchat: rate limit reached"
	}
	return fmt.Sprintf("rocketchat: rate limit reached; retry after %s", e.RetryAfter.Round(time.Millisecond))
}

// retryTransport retries HTTP 429 responses.
//
// It honors Retry-After in seconds or as an HTTP date. If the requested wait
// is longer than maxDelay, it stops and returns RateLimitError, so an MCP tool
// call does not block for a long time.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	for attempt := 0; ; attempt++ {
		attemptReq := req
		if attempt > 0 {
			var err error
			attemptReq, err = replayRequest(req)
			if err != nil {
				return nil, err
			}
		}

		resp, err := t.base.RoundTrip(attemptReq)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		delay := retryDelay(resp, attempt, t.baseDelay, t.maxDelay)
		drainAndClose(resp.Body)

		if attempt >= t.maxRetries || delay > t.maxDelay {
			return nil, &RateLimitError{RetryAfter: delay}
		}
		if err := sleepContext(ctx, delay); err != nil {
			return nil, err
		}
	}
}

func replayRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil {
		return clone, nil
	}
	if req.GetBody == nil {
		return nil, fmt.Errorf("rocketchat: cannot retry %s %s: request body is not replayable", req.Method, req.URL)
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("rocketchat: recreate %s %s body: %w", req.Method, req.URL, err)
	}
	clone.Body = body
	return clone, nil
}

func retryDelay(resp *http.Response, attempt int, base, max time.Duration) time.Duration {
	if value := strings.TrimSpace(resp.Header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			if seconds <= 0 {
				return 0
			}
			return min(time.Duration(seconds)*time.Second, maxRetryAfter)
		}
		if when, err := http.ParseTime(value); err == nil {
			delay := time.Until(when)
			if delay <= 0 {
				return 0
			}
			return min(delay, maxRetryAfter)
		}
	}

	return min(base<<attempt, max)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	_ = body.Close()
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &retryTransport{
			base:       http.DefaultTransport,
			maxRetries: maxRetries,
			baseDelay:  baseRetryDelay,
			maxDelay:   maxRetryDelay,
		},
	}
}
