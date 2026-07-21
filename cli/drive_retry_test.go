package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/require"
)

const retryableWorkloadUnavailableBody = `{
  "error": {
    "code": "WORKLOAD_UNAVAILABLE",
    "message": "The sandbox is temporarily unavailable",
    "origin": "platform",
    "retryable": true,
    "action": "Retry with exponential backoff"
  }
}`

type fakeSandboxRetryClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeSandboxRetryClock) Now() time.Time {
	return c.now
}

func (c *fakeSandboxRetryClock) Sleep(_ context.Context, delay time.Duration) error {
	c.sleeps = append(c.sleeps, delay)
	c.now = c.now.Add(delay)
	return nil
}

func testSandboxRetryPolicy(clock *fakeSandboxRetryClock) sandboxRetryPolicy {
	return sandboxRetryPolicy{
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		TotalBudget:    60 * time.Second,
		HTTPClient:     http.DefaultClient,
		Now:            clock.Now,
		Sleep:          clock.Sleep,
	}
}

func TestSandboxReadRequestRetriesSequentiallyUntilSuccess(t *testing.T) {
	var calls atomic.Int32
	var active atomic.Int32
	var maxActive atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maxActive.Load()
			if current <= previous || maxActive.CompareAndSwap(previous, current) {
				break
			}
		}

		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodGet)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		attempt := calls.Add(1)
		if attempt < 3 {
			w.Header().Set("X-Blaxel-Source", "platform")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(retryableWorkloadUnavailableBody))
			return
		}
		_, _ = w.Write([]byte(`{"mounts":[]}`))
	}))
	defer server.Close()

	clock := &fakeSandboxRetryClock{now: time.Unix(0, 0)}
	firstRetryCalls := 0
	response, err := sandboxReadRequestWithRetryPolicy(
		context.Background(),
		server.URL,
		"/drives/mount",
		"test-token",
		testSandboxRetryPolicy(clock),
		func() { firstRetryCalls++ },
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.False(t, response.PlatformWorkloadUnavailable)
	require.JSONEq(t, `{"mounts":[]}`, string(response.Body))
	require.EqualValues(t, 3, calls.Load())
	require.EqualValues(t, 1, maxActive.Load())
	require.Equal(t, []time.Duration{500 * time.Millisecond, time.Second}, clock.sleeps)
	require.Equal(t, 1, firstRetryCalls)
}

func TestRetryableWorkloadUnavailableAcceptsCanonicalProvenanceFallbacks(t *testing.T) {
	tests := []struct {
		name   string
		header string
		body   string
		want   bool
	}{
		{
			name:   "canonical response header",
			header: "platform",
			body:   `{"error":{"code":"WORKLOAD_UNAVAILABLE","retryable":true}}`,
			want:   true,
		},
		{
			name: "typed envelope origin",
			body: retryableWorkloadUnavailableBody,
			want: true,
		},
		{
			name: "no platform provenance",
			body: `{"error":{"code":"WORKLOAD_UNAVAILABLE","retryable":true}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
			}
			if tt.header != "" {
				resp.Header.Set("X-Blaxel-Source", tt.header)
			}

			require.Equal(t, tt.want, isRetryableWorkloadUnavailable(resp, []byte(tt.body)))
		})
	}
}

func TestSandboxReadRequestDoesNotRetryUntrustedOrNonRetryableErrors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		header string
		status int
	}{
		{
			name: "non-retryable gateway error",
			body: `{"error":{"code":"WORKLOAD_NOT_FOUND","message":"missing","origin":"platform","retryable":false}}`,
		},
		{
			name: "workload-owned lookalike",
			body: `{"error":{"code":"WORKLOAD_UNAVAILABLE","message":"not platform owned","retryable":true}}`,
		},
		{
			name:   "platform header with wrong code",
			body:   `{"error":{"code":"BAD_REQUEST","message":"bad request","retryable":true}}`,
			header: "platform",
		},
		{
			name:   "retryable envelope with wrong status",
			body:   retryableWorkloadUnavailableBody,
			header: "platform",
			status: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if tt.header != "" {
					w.Header().Set("X-Blaxel-Source", tt.header)
				}
				status := tt.status
				if status == 0 {
					status = http.StatusNotFound
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			clock := &fakeSandboxRetryClock{now: time.Unix(0, 0)}
			response, err := sandboxReadRequestWithRetryPolicy(
				context.Background(), server.URL, "/drives/mount", "token", testSandboxRetryPolicy(clock), nil,
			)

			require.NoError(t, err)
			wantStatus := tt.status
			if wantStatus == 0 {
				wantStatus = http.StatusNotFound
			}
			require.Equal(t, wantStatus, response.StatusCode)
			require.Equal(t, 1, calls)
			require.Empty(t, clock.sleeps)
			require.True(t, core.IsExpectedCLIError(classifySandboxReadAPIError(response, "list mounted drives")))
		})
	}
}

func TestSandboxReadRequestStopsAtRetryBudget(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("X-Blaxel-Source", "platform")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(retryableWorkloadUnavailableBody))
	}))
	defer server.Close()

	clock := &fakeSandboxRetryClock{now: time.Unix(0, 0)}
	response, err := sandboxReadRequestWithRetryPolicy(
		context.Background(), server.URL, "/drives/mount", "token", testSandboxRetryPolicy(clock), nil,
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.True(t, response.PlatformWorkloadUnavailable)
	require.Equal(t, 8, calls)
	require.Equal(t, []time.Duration{
		500 * time.Millisecond,
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		28*time.Second + 500*time.Millisecond,
	}, clock.sleeps)
	require.Equal(t, 60*time.Second, clock.Now().Sub(time.Unix(0, 0)))

	finalErr := classifySandboxReadAPIError(response, "list mounted drives")
	require.ErrorContains(t, finalErr, "HTTP 404, WORKLOAD_UNAVAILABLE")
	require.ErrorContains(t, finalErr, "temporarily unavailable")
	require.ErrorContains(t, finalErr, "Retry with exponential backoff")
	require.False(t, core.IsExpectedCLIError(finalErr))
}

func TestSandboxReadRequestHonorsCancellationDuringBackoff(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("X-Blaxel-Source", "platform")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(retryableWorkloadUnavailableBody))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	clock := &fakeSandboxRetryClock{now: time.Unix(0, 0)}
	policy := testSandboxRetryPolicy(clock)
	policy.Sleep = sleepWithContext
	response, err := sandboxReadRequestWithRetryPolicy(
		ctx,
		server.URL,
		"/drives/mount",
		"token",
		policy,
		cancel,
	)

	require.Nil(t, response)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestSandboxReadRequestDoesNotRetryTransportErrors(t *testing.T) {
	transportErr := errors.New("connection reset")
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, transportErr
	})}

	clock := &fakeSandboxRetryClock{now: time.Unix(0, 0)}
	policy := testSandboxRetryPolicy(clock)
	policy.HTTPClient = client
	response, err := sandboxReadRequestWithRetryPolicy(
		context.Background(), "https://sandbox.example", "/drives/mount", "token", policy, nil,
	)

	require.Nil(t, response)
	require.ErrorIs(t, err, transportErr)
	require.EqualValues(t, 1, calls.Load())
	require.Empty(t, clock.sleeps)
}

type closeTrackingBody struct {
	io.Reader
	closed *atomic.Bool
	err    error
}

func (b *closeTrackingBody) Close() error {
	b.closed.Store(true)
	return b.err
}

func TestSandboxReadRequestClosesEveryResponse(t *testing.T) {
	firstClosed := &atomic.Bool{}
	secondClosed := &atomic.Bool{}
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, http.MethodGet, req.Method)
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{"X-Blaxel-Source": []string{"platform"}},
				Body: &closeTrackingBody{
					Reader: strings.NewReader(retryableWorkloadUnavailableBody),
					closed: firstClosed,
				},
				Request: req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: &closeTrackingBody{
				Reader: strings.NewReader(`{"mounts":[]}`),
				closed: secondClosed,
				err:    errors.New("close failed after complete read"),
			},
			Request: req,
		}, nil
	})}

	clock := &fakeSandboxRetryClock{now: time.Unix(0, 0)}
	policy := testSandboxRetryPolicy(clock)
	policy.HTTPClient = client
	response, err := sandboxReadRequestWithRetryPolicy(
		context.Background(), "https://sandbox.example", "/drives/mount", "token", policy, nil,
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.True(t, firstClosed.Load())
	require.True(t, secondClosed.Load())
	require.Equal(t, 2, calls)
}

func TestNewSandboxAPIErrorSupportsLegacyAndTypedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "legacy", body: `{"error":"not found"}`, want: "HTTP 404): not found"},
		{name: "typed", body: retryableWorkloadUnavailableBody, want: "HTTP 404, WORKLOAD_UNAVAILABLE"},
		{name: "unstructured", body: "upstream unavailable", want: "HTTP 404): upstream unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newSandboxAPIError([]byte(tt.body), http.StatusNotFound, "list mounted drives")
			require.ErrorContains(t, err, tt.want)
		})
	}
}
