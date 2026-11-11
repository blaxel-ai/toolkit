package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createSandboxAgainst creates a sandbox against a stub API with the CLI's
// client middleware and returns the error and the number of requests made.
func createSandboxAgainst(t *testing.T, maxRetries int, handler http.HandlerFunc) (error, int32) {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client := blaxel.NewClient(
		option.WithBaseURL(server.URL),
		option.WithAPIKey("test"),
		option.WithMaxRetries(maxRetries),
		option.WithMiddleware(core.NoRetryOnQuotaExceeded),
	)
	_, err := client.Sandboxes.New(context.Background(), blaxel.SandboxNewParams{
		Sandbox: blaxel.SandboxParam{Metadata: blaxel.MetadataParam{Name: "sbx"}},
	})
	return err, atomic.LoadInt32(&calls)
}

func TestSandboxCreateQuotaExceededIsNotRetried(t *testing.T) {
	err, calls := createSandboxAgainst(t, 2, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"QUOTA_EXCEEDED","message":"Sandbox quota exceeded: 10/10"}`))
	})
	require.Error(t, err)
	assert.Equal(t, int32(1), calls, "QUOTA_EXCEEDED must not be retried")
	assert.Equal(t,
		"Sandbox quota exceeded: 10/10 (HTTP 429 QUOTA_EXCEEDED): not retryable: delete unused sandboxes or volumes, or raise the quota: https://docs.blaxel.ai/Security/Quotas",
		extractErrorMessage(err))
}

func TestSandboxCreateRateLimitIsRetriedWithHint(t *testing.T) {
	err, calls := createSandboxAgainst(t, 1, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After-Ms", "1")
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"WORKSPACE_RATE_LIMIT_EXCEEDED","message":"Too many concurrent sandbox creations"}`))
	})
	require.Error(t, err)
	assert.Equal(t, int32(2), calls, "rate limits stay retryable")
	assert.Equal(t,
		"Too many concurrent sandbox creations (HTTP 429 WORKSPACE_RATE_LIMIT_EXCEEDED): retry in 7s",
		extractErrorMessage(err))
}

func TestSandboxCreateRateLimitWithoutRetryAfter(t *testing.T) {
	err, _ := createSandboxAgainst(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"WORKSPACE_RATE_LIMIT_EXCEEDED","message":"Slow down","status":429}}`))
	})
	require.Error(t, err)
	assert.Equal(t, "Slow down (HTTP 429 WORKSPACE_RATE_LIMIT_EXCEEDED): retry in a few seconds", extractErrorMessage(err))
}

func TestSandboxCreateQuotaHeaderFromServerWins(t *testing.T) {
	_, calls := createSandboxAgainst(t, 1, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-should-retry", "true")
		w.Header().Set("Retry-After-Ms", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"QUOTA_EXCEEDED","message":"quota"}`))
	})
	assert.Equal(t, int32(2), calls, "an explicit x-should-retry from the server is respected")
}

func TestSandboxCreateEmptyForbidden(t *testing.T) {
	err, _ := createSandboxAgainst(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	require.Error(t, err)
	msg := extractErrorMessage(err)
	assert.Contains(t, msg, "403 Forbidden")
	assert.Contains(t, msg, "(HTTP 403, empty body): blocked before the API (edge rate limit or firewall), not a quota")
}

func TestExtractErrorMessageWithoutCodeIsUnchanged(t *testing.T) {
	err, _ := createSandboxAgainst(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"bad spec"}`))
	})
	require.Error(t, err)
	assert.Equal(t, "bad spec", extractErrorMessage(err))
}
