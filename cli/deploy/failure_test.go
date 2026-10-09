package deploy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func deployEventJSON(t *testing.T, eventType, revision, message string, at time.Time) json.RawMessage {
	t.Helper()
	data, err := json.Marshal([]Event{{Type: "ai.blaxel.controlplane." + eventType, Revision: revision, Status: "FAILED", Message: message, Time: at.UTC().Format(time.RFC3339Nano)}})
	require.NoError(t, err)
	return data
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
	// An empty newest failure or a newer attempt never borrows older cause.
	for _, event := range []Event{{Type: "buildimage.failed", Time: started.Add(2 * time.Second).Format(time.RFC3339Nano)}, {Type: "buildimage.created", Time: started.Add(2 * time.Second).Format(time.RFC3339Nano)}} {
		data, err := json.Marshal([]Event{event, {Type: "buildimage.failed", Time: started.Format(time.RFC3339Nano), Message: "old"}})
		require.NoError(t, err)
		_, _, message := failureEvidence(data)
		require.Empty(t, message)
	}
}

func TestDeployCommandTextQuotesArguments(t *testing.T) {
	require.Equal(t, "bl logs agent 'a; touch /tmp/nope'", deployCommandText([]string{"bl", "logs", "agent", "a; touch /tmp/nope"}))
}
