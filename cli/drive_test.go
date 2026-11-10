package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realJSONParseError returns a genuine *json.SyntaxError, the kind produced
// when the sandbox or an intervening gateway answers a `bl drive` request with
// a non-JSON body (an HTML error page, a truncated stream, etc.).
func realJSONParseError(t *testing.T) error {
	t.Helper()
	var resp driveMountResponse
	err := json.Unmarshal([]byte("<html>not json</html>"), &resp)
	require.Error(t, err)
	return err
}

// A drive response-body read or parse failure is an environmental/server
// condition, not a CLI implementation defect, so after classification it must
// be an expected error (never reported to Sentry as an internal crash).
func TestClassifyDriveResponseErrorMarksTransportFailuresExpected(t *testing.T) {
	transportErr := classifyDriveResponseError(fmt.Errorf("failed to read response: %w", io.ErrUnexpectedEOF))
	assert.True(t, core.IsExpectedCLIError(transportErr),
		"a truncated/reset response-body read must be classified as expected")

	parseErr := classifyDriveResponseError(fmt.Errorf("failed to parse response: %w", realJSONParseError(t)))
	assert.True(t, core.IsExpectedCLIError(parseErr),
		"a non-JSON response body must be classified as expected")
}

// An already-classified cause keeps its own, more specific category: the helper
// must return it untouched rather than re-wrapping it as operational (which
// would downgrade, e.g., a 404 not_found).
func TestClassifyDriveResponseErrorPreservesTypedClassification(t *testing.T) {
	wrapped := fmt.Errorf("failed to read response: %w", &blaxel.Error{StatusCode: http.StatusNotFound})
	require.True(t, core.IsExpectedCLIError(wrapped),
		"a wrapped HTTP error is already expected before classification")

	got := classifyDriveResponseError(wrapped)
	assert.Equal(t, wrapped, got,
		"an already-expected error must be returned unchanged, preserving its category")
	assert.True(t, core.IsExpectedCLIError(got))
}

// nil must flow through untouched and must never count as an expected error.
func TestClassifyDriveResponseErrorNilIsNotReported(t *testing.T) {
	assert.Nil(t, classifyDriveResponseError(nil))
	assert.False(t, core.IsExpectedCLIError(nil))
}

// Guard: the classification is load-bearing. The raw wrapped transport/parse
// errors are NOT expected on their own; only classifyDriveResponseError makes
// them so. If this flips, the fix has regressed and the noise would return.
func TestDriveResponseErrorUnclassifiedIsReported(t *testing.T) {
	rawRead := fmt.Errorf("failed to read response: %w", io.ErrUnexpectedEOF)
	assert.False(t, core.IsExpectedCLIError(rawRead),
		"an unclassified response-read error would be reported as an internal defect")

	rawParse := fmt.Errorf("failed to parse response: %w", realJSONParseError(t))
	assert.False(t, core.IsExpectedCLIError(rawParse),
		"an unclassified response-parse error would be reported as an internal defect")
}
