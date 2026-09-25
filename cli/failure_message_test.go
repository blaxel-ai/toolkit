package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func buildEvent(kind, timestamp, message string) string {
	value, _ := json.Marshal(map[string]string{"type": "ai.blaxel.controlplane.buildimage." + kind, "time": timestamp, "message": message})
	return string(value)
}

func TestImageBuildFailurePolling(t *testing.T) {
	const reason = "The build environment could not be scheduled in the selected region. Try another region."
	old := buildEvent("failed", "2026-09-18T01:00:00Z", "old failure")
	newest := buildEvent("failed", "2026-09-18T02:00:00Z", reason)
	cases := []struct{ name, status, events, wantStatus, wantMessage string }{
		{"latest unsorted", "FAILED", "[" + newest + "," + old + "]", "failed", reason},
		{"no events", "FAILED", "null", "failed", ""},
		{"malformed events", "FAILED", `{"unexpected":true}`, "failed", ""},
		{"malformed message", "FAILED", `[{"time":"2026-09-18T02:00:00Z","message":42}]`, "failed", ""},
		{"malformed time", "FAILED", "[" + old + "," + buildEvent("failed", "invalid", reason) + "]", "failed", ""},
		{"newer attempt", "FAILED", "[" + old + "," + buildEvent("created", "2026-09-18T03:00:00Z", "new build") + "]", "failed", ""},
		{"new failure empty", "FAILED", "[" + old + "," + buildEvent("failed", "2026-09-18T03:00:00Z", "") + "]", "failed", ""},
		{"still building", "BUILDING", "[" + old + "]", "", ""},
		{"succeeded", "BUILT", "[" + old + "]", "succeeded", ""},
		{"provider detail", "FAILED", "[" + buildEvent("failed", "2026-09-18T03:00:00Z", "operation error DynamoDB: Query") + "]", "failed", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, "/images/sandbox/test-image", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"metadata":{"status":%q,"events":%s}}`, tc.status, tc.events)
			}))
			defer server.Close()
			setupTestClient(t, server.URL)
			status, message, err := getImageBuildStatus("sandbox", "test-image")
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, status)
			require.Equal(t, tc.wantMessage, message)
			require.EqualValues(t, 1, calls.Load(), "reuse the status response")
		})
	}
}

func TestPushWatcherReportsMultilineFailureWithoutLogs(t *testing.T) {
	const reason = "Build failed at Dockerfile:3\nCOPY missing-file /app\nmissing-file: not found"
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/images/sandbox/test-image" {
			polls.Add(1)
			_, _ = fmt.Fprintf(w, `{"metadata":{"status":"FAILED","events":[%s]}}`, buildEvent("failed", time.Now().UTC().Format(time.RFC3339Nano), reason))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	setupTestClient(t, server.URL)
	err := watchBuildLogsNonInteractive("sandbox", "test-image", true, 15*time.Second)
	require.EqualError(t, err, "image build failed: "+reason)
	require.EqualValues(t, 1, polls.Load())
}

func TestResourcePollingRetainsFailureMessage(t *testing.T) {
	const reason = "The build environment is unavailable in this region."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agents/test-agent", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":[%s]}`, buildEvent("failed", "2026-09-18T02:00:00Z", reason))
	}))
	defer server.Close()
	setupTestClient(t, server.URL)
	status, message, err := getResourceStatusDetails("agent", "test-agent")
	require.NoError(t, err)
	require.Equal(t, "FAILED", status)
	require.Equal(t, reason, message)
	require.EqualError(t, failureError("resource deployment failed", message), "resource deployment failed: "+reason)
}

func TestFailureMessageHidesInternalDetails(t *testing.T) {
	for _, message := range []string{"AWS unavailable", "https://internal.example/path", "arn:aws:lambda:x", "gateway 10.0.0.1 unavailable", "failed\x1b[2J", "RequestID: secret", "S3 upload denied", "SQS unavailable", "KMS access denied", "SecretsManager failure", "CloudFront unavailable", "StepFunctions execution failed", "bld-private-workspace.us-was-1.bl.run unreachable", "bucket.s3.us-west-2.amazonaws.com denied"} {
		t.Run(message, func(t *testing.T) {
			require.Empty(t, latestFailureMessage(json.RawMessage("["+buildEvent("failed", "2026-09-18T02:00:00Z", message)+"]"), ""))
		})
	}
	require.EqualError(t, failureError("image build failed", ""), "image build failed")
}

func TestFailureMessagePreservesCustomerDetail(t *testing.T) {
	for _, message := range []string{
		"The build environment could not be scheduled in the selected region. Try another region.",
		"The build command failed with exit code 1.",
		"Cannot find module s3-client.",
	} {
		t.Run(message, func(t *testing.T) {
			require.Equal(t, message, latestFailureMessage(json.RawMessage("["+buildEvent("failed", "2026-09-18T02:00:00Z", message)+"]"), ""))
		})
	}
}

func TestFailureMessagePreservesMultilineControlplaneDetail(t *testing.T) {
	const detail = "Dockerfile:3\n\tCOPY missing-file /app\nerror: missing-file: not found"
	for _, message := range []string{detail, strings.ReplaceAll(detail, "\n", "\r\n")} {
		raw := json.RawMessage("[" + buildEvent("failed", "2026-09-19T02:00:00Z", message) + "]")
		require.Equal(t, detail, latestFailureMessage(raw, ""))
	}
	for _, message := range []string{"error\roverwrite", "error\n\x1b[2J", "error\nAWS unavailable", "error\n\x00hidden"} {
		raw := json.RawMessage("[" + buildEvent("failed", "2026-09-19T02:00:00Z", message) + "]")
		require.Empty(t, latestFailureMessage(raw, ""))
	}
}
