package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const badRunDeployMessage = `#5 [2/2] RUN echo BAD_RUN >&2; exit 42
#5 0.064 BAD_RUN
#5 ERROR: process "/bin/sh -c echo BAD_RUN >&2; exit 42" did not complete successfully: exit code: 42
Dockerfile:2
--------------------
   1 | FROM alpine:3.22
   2 | >>> RUN echo BAD_RUN >&2; exit 42
--------------------
error: failed to solve: process "/bin/sh -c echo BAD_RUN >&2; exit 42" did not complete successfully: exit code: 42`

// A failing command line holding a URL is infrastructure-looking; the rest of
// the build log must still be reported.
const urlRunDeployMessage = `#5 [2/2] RUN wget -q https://example.invalid/x
#5 ERROR: process "/bin/sh -c wget -q https://example.invalid/x" did not complete successfully: exit code: 4
Dockerfile:2
--------------------
   1 | FROM alpine:3.22
   2 | >>> RUN wget -q https://example.invalid/x
--------------------
error: failed to solve: process "/bin/sh -c wget -q https://example.invalid/x" did not complete successfully: exit code: 4`

const missingBaseDeployMessage = `#2 ERROR: docker.io/library/alpine:missing-tag: not found
Dockerfile:1
--------------------
   1 | >>> FROM alpine:missing-tag
--------------------
error: failed to solve: alpine:missing-tag: not found`

func deployTestClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	original, workspace := core.GetClient(), core.GetWorkspace()
	server := httptest.NewServer(handler)
	client := blaxel.NewClient(option.WithBaseURL(server.URL+"/"), option.WithAPIKey("test"), option.WithMaxRetries(0))
	core.SetClient(&client)
	core.SetWorkspace("test-workspace")
	core.RegisterResourceOperations(context.Background())
	t.Cleanup(func() {
		core.SetClient(original)
		core.SetWorkspace(workspace)
		core.RegisterResourceOperations(context.Background())
		server.Close()
	})
}

func deployEventJSON(t *testing.T, eventType, revision, message string, at time.Time) json.RawMessage {
	t.Helper()
	data, err := json.Marshal([]failureEvent{{Type: "ai.blaxel.controlplane." + eventType, Revision: revision, Status: "FAILED", Message: message, Time: at.UTC().Format(time.RFC3339Nano)}})
	require.NoError(t, err)
	return data
}

func captureDeployStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	os.Stdout = file
	defer func() { os.Stdout = original; _ = file.Close() }()
	fn()
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(data)
}

func TestDeployFailureDiagnostics(t *testing.T) {
	cases := []struct{ name, eventType, message, logs, code, cause, step string }{
		{"failed RUN", "buildimage.failed", badRunDeployMessage, "", "BUILD_FAILED", "Dockerfile RUN failed with exit code 42", "Dockerfile:2 RUN echo BAD_RUN >&2; exit 42"},
		{"URL in the failing command", "buildimage.failed", urlRunDeployMessage, "", "BUILD_FAILED", "Dockerfile RUN failed with exit code 4", "Dockerfile:2 RUN"},
		{"missing base", "buildimage.failed", missingBaseDeployMessage, "", "BUILD_FAILED", "docker.io/library/alpine:missing-tag: not found", "Dockerfile:1 FROM alpine:missing-tag"},
		{"generic rollout with exit evidence", "deployment.failed", "Deployment has failed", "STARTUP_EXIT\nApplication exited with 0x2a00 (exit code: 42)", "ROLLOUT_FAILED", noSpecificCause, ""},
		{"generic rollout empty logs", "deployment.failed", "Deployment has failed", "", "ROLLOUT_FAILED", noSpecificCause, ""},
		// The step must survive an evidence tail cut off by the 20 line bound.
		{"failed RUN with long output", "buildimage.failed", badRunDeployMessage + "\n" + strings.Repeat("later evidence\n", 100), "", "BUILD_FAILED", "Dockerfile RUN failed with exit code 42", "Dockerfile:2 RUN echo BAD_RUN >&2; exit 42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logReads atomic.Int32
			deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/observability/logs" {
					logReads.Add(1)
					require.Equal(t, "agents", r.URL.Query().Get("resourceType"))
					require.Equal(t, "test", r.URL.Query().Get("workloadIds"))
					require.Equal(t, "1000", r.URL.Query().Get("limit"))
					_, _ = fmt.Fprintf(w, `{"test":{"logs":[{"timestamp":%q,"message":%q}]},"other-tenant":{"logs":[{"message":"must not appear"}]}}`, time.Now().UTC().Format(time.RFC3339Nano), tc.logs)
					return
				}
				_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":%s}`, deployEventJSON(t, tc.eventType, "r1", tc.message, time.Now()))
			})
			d := Deployment{cwd: t.TempDir(), name: "test", observations: []deployObservation{{kind: "agent", name: "test", started: time.Now().Add(-time.Minute)}}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := d.waitForRollouts(ctx, time.Millisecond)
			require.Error(t, err)
			require.True(t, core.IsExpectedCLIError(err))
			diagnostic := d.observations[0].diagnostics
			require.Equal(t, tc.code, diagnostic.Code)
			require.Equal(t, tc.cause, diagnostic.Cause)
			require.Equal(t, tc.step, diagnostic.Step)
			require.LessOrEqual(t, len(diagnostic.LogTail), 20)
			if tc.code == "BUILD_FAILED" {
				require.Zero(t, logReads.Load(), "failure event is primary build evidence")
				require.Equal(t, "resource.events", diagnostic.Source)
				require.Equal(t, []string{"bl", "deploy", "--yes", "--wait", "-w", "test-workspace"}, diagnostic.Next.Command)
			} else {
				require.EqualValues(t, 1, logReads.Load())
				require.Equal(t, []string{"bl", "logs", "agent", "test", "-w", "test-workspace", "--period", "30m", "--utc"}, diagnostic.Next.Command)
				if tc.logs != "" {
					require.Contains(t, strings.Join(diagnostic.LogTail, "\n"), "exit code: 42")
				}
			}
			pretty := captureStderr(t, func() { require.True(t, d.printFailureDiagnostics()) })
			require.Contains(t, pretty, "cause: "+tc.cause)
			require.NotContains(t, pretty, "https://", "infrastructure details never reach the terminal")
			require.Equal(t, 1, strings.Count(pretty, "next:"))
			if tc.step != "" {
				require.Contains(t, pretty, "step: "+tc.step)
			}
		})
	}
}

func TestDeployRuntimeDiagnosticReadFailureDoesNotMaskFailure(t *testing.T) {
	var reads atomic.Int32
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	o := deployObservation{kind: "agent", name: "test", started: time.Now(), rollout: resourceRollout{Status: "FAILED"}}
	diagnostic := (&Deployment{}).failureDiagnostics(o, "")
	require.Equal(t, "ROLLOUT_FAILED", diagnostic.Code)
	require.Equal(t, noSpecificCause, diagnostic.Cause)
	require.EqualValues(t, 1, reads.Load(), "no retry or pagination")
}

func TestDeployWaitReportsEachResourceAndPreservesTimeout(t *testing.T) {
	core.SetConfigType("agent")
	started := time.Now().Add(-time.Minute)
	var extraReads atomic.Int32
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path != "/agents/extra":
			_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":%s}`, deployEventJSON(t, "buildimage.failed", "r1", badRunDeployMessage, time.Now()))
		case extraReads.Add(1) == 1:
			_, _ = w.Write([]byte(`{"status":"DEPLOYING"}`))
		default:
			<-r.Context().Done() // a hung API read must not extend --timeout
		}
	})
	d := Deployment{name: "test", cwd: t.TempDir(), observations: []deployObservation{{kind: "agent", name: "test", started: started}, {kind: "agent", name: "extra", started: started}}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	begin := time.Now()
	err := d.waitForRollouts(ctx, time.Millisecond)
	require.Error(t, err)
	require.Less(t, time.Since(begin), time.Second)
	require.Equal(t, "BUILD_FAILED", d.observations[0].diagnostics.Code)
	require.Equal(t, "DEPLOY_TIMEOUT", d.observations[1].diagnostics.Code)
	require.Equal(t, "monitor", d.observations[1].diagnostics.Phase)
	var result struct {
		Success   bool `json:"success"`
		Resources []struct {
			Status      string            `json:"status"`
			Diagnostics deployDiagnostics `json:"diagnostics"`
		} `json:"resources"`
	}
	output := captureDeployStdout(t, func() { d.printStructuredOutput("json", time.Now(), true, err) })
	decoder := json.NewDecoder(strings.NewReader(output))
	require.NoError(t, decoder.Decode(&result))
	require.ErrorIs(t, decoder.Decode(new(any)), io.EOF, "only the JSON document on stdout")
	require.False(t, result.Success)
	require.Len(t, result.Resources, 2)
	require.Equal(t, "FAILED", result.Resources[0].Status)
	require.Equal(t, "DEPLOYING", result.Resources[1].Status, "a timeout keeps the last observed status")
	require.Equal(t, "DEPLOY_TIMEOUT", result.Resources[1].Diagnostics.Code)
	var yamlResult struct {
		Resources []struct{ Diagnostics deployDiagnostics } `yaml:"resources"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(captureDeployStdout(t, func() { d.printStructuredOutput("yaml", time.Now(), true, err) })), &yamlResult))
	require.Equal(t, "BUILD_FAILED", yamlResult.Resources[0].Diagnostics.Code)
}

// A failure is this deploy's when it is new since the apply, whatever this
// machine's clock says.
func TestDeployFailureIsJudgedByEventsNotTheClock(t *testing.T) {
	event := failureEvent{Type: "ai.blaxel.controlplane.buildimage.failed", Status: "FAILED", Time: time.Now().UTC().Format(time.RFC3339Nano), Message: "boom"}
	raw, err := json.Marshal([]failureEvent{event})
	require.NoError(t, err)
	o := deployObservation{started: time.Now().Add(time.Hour), rollout: resourceRollout{Events: raw}, baselineEvents: map[failureEvent]struct{}{}}
	require.True(t, o.failureIsCurrent(), "new since the baseline, although this clock is an hour ahead")
	o.baselineEvents[event] = struct{}{}
	require.False(t, o.failureIsCurrent(), "already in the baseline")
}

func TestDeployEvidenceSanitizationAndBounds(t *testing.T) {
	message := "\x1b[31m" + badRunDeployMessage + "\x1b[0m\x00\r\x1b]8;;https://evil.test\a"
	// Infrastructure/URL filtering still wins over terminal sanitization: the line
	// holding the link is dropped, not the whole message.
	_, _, kept := failureEvidence(deployEventJSON(t, "buildimage.failed", "r1", message, time.Now()))
	require.NotContains(t, kept, "evil.test")
	require.Contains(t, kept, "Dockerfile:2")
	message = "\x1b[31m" + badRunDeployMessage + "\x1b[0m\x00\r\u202e"
	_, _, clean := failureEvidence(deployEventJSON(t, "buildimage.failed", "r1", message, time.Now()))
	require.Contains(t, clean, "Dockerfile:2")
	for _, control := range []string{"\x1b", "\x00", "\r", "\u202e"} {
		require.NotContains(t, clean, control)
	}
	tail := deployEvidenceTail(strings.Repeat("line\n", 100) + strings.Repeat("界", 9000))
	require.LessOrEqual(t, len(tail), 20)
	require.LessOrEqual(t, len(strings.Join(tail, "\n")), deployEvidenceBytes)
	require.NotContains(t, strings.Join(deployEvidenceTail("hello\nAWS private\nlast"), "\n"), "AWS")
	for _, private := range []string{"AWS unavailable", "https://private.test/path", "gateway 10.0.0.1 unavailable", "arn:aws:lambda:x"} {
		_, _, kept := failureEvidence(deployEventJSON(t, "buildimage.failed", "r1", private, time.Now()))
		require.Empty(t, kept)
	}
}

func TestDeployLatestAttemptSelection(t *testing.T) {
	started := time.Now()
	old := deployEventJSON(t, "buildimage.failed", "r0", "old failure", started.Add(-time.Minute))
	newest := deployEventJSON(t, "buildimage.failed", "r1", badRunDeployMessage, started.Add(time.Second))
	unsorted := append(append(append(json.RawMessage{}, newest[:len(newest)-1]...), ','), old[1:]...)
	_, _, message := failureEvidence(unsorted)
	require.Equal(t, badRunDeployMessage, message)
	o := deployObservation{started: started, baseline: rolloutBaseline{revision: "r0", deployedRevision: "deployed-old", known: true}, rollout: resourceRollout{Events: old}}
	require.False(t, o.failureIsCurrent())
	o.rollout.Events = deployEventJSON(t, "deployment.failed", "deployed-old", "in-flight old failure", started.Add(time.Second))
	require.False(t, o.failureIsCurrent(), "a failure of a pre-existing revision is not this deploy's")
	o.rollout.Events = newest
	require.True(t, o.failureIsCurrent())
	// An empty newest failure or a newer attempt never borrows older cause.
	for _, event := range []failureEvent{{Type: "buildimage.failed", Time: started.Add(2 * time.Second).Format(time.RFC3339Nano)}, {Type: "buildimage.created", Time: started.Add(2 * time.Second).Format(time.RFC3339Nano)}} {
		data, err := json.Marshal([]failureEvent{event, {Type: "buildimage.failed", Time: started.Format(time.RFC3339Nano), Message: "old"}})
		require.NoError(t, err)
		_, _, message := failureEvidence(data)
		require.Empty(t, message)
	}
}

func TestDeployNextCommandsDoNotReplaySecrets(t *testing.T) {
	d := Deployment{nextType: "agent", nextName: "explicit-name", folder: "svc"}
	require.Equal(t, []string{"bl", "deploy", "--yes", "--wait", "-w", "ws", "-t", "agent", "-n", "explicit-name", "-d", "svc"}, d.redeployCommand("ws"))
	require.Equal(t, "bl logs agent 'a; touch /tmp/nope'", deployCommandText([]string{"bl", "logs", "agent", "a; touch /tmp/nope"}))
}
