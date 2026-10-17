package cli

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	mon "github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAPIStatusUsesTypedErrorIdentity(t *testing.T) {
	assert.True(t, isAPIStatus(fmt.Errorf("image lookup: %w", &blaxel.Error{StatusCode: http.StatusNotFound}), http.StatusNotFound))
	assert.False(t, isAPIStatus(errors.New("internal cache entry not found (404)"), http.StatusNotFound))
	assert.False(t, isAPIStatus(&blaxel.Error{StatusCode: http.StatusInternalServerError}, http.StatusNotFound))
}

func TestImageRefToName(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "simple image with tag",
			ref:      "nginx:latest",
			expected: "nginx",
		},
		{
			name:     "image with registry and tag",
			ref:      "docker.io/library/nginx:latest",
			expected: "nginx",
		},
		{
			name:     "image with org and tag",
			ref:      "ghcr.io/myorg/my-app:v2",
			expected: "my-app",
		},
		{
			name:     "image without tag",
			ref:      "docker.io/myorg/myimage",
			expected: "myimage",
		},
		{
			name:     "image with digest",
			ref:      "docker.io/myorg/myimage@sha256:abc123",
			expected: "myimage",
		},
		{
			name:     "simple image without tag",
			ref:      "alpine",
			expected: "alpine",
		},
		{
			name:     "localhost with port and image",
			ref:      "localhost:5000/myimage:v1",
			expected: "myimage",
		},
		{
			name:     "deep nested path",
			ref:      "registry.example.com/org/team/project:latest",
			expected: "project",
		},
		{
			name:     "image with tag and digest",
			ref:      "docker.io/myorg/myimage:v1@sha256:abc123",
			expected: "myimage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imageRefToName(tt.ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePushBuildTimeout(t *testing.T) {
	t.Run("empty uses default", func(t *testing.T) {
		d, err := parsePushBuildTimeout("")
		assert.NoError(t, err)
		assert.Equal(t, mon.DefaultBuildTimeout, d)
	})

	t.Run("valid duration", func(t *testing.T) {
		d, err := parsePushBuildTimeout("30m")
		assert.NoError(t, err)
		assert.Equal(t, 30*time.Minute, d)
	})

	// Every malformed or non-positive value is user input, not a CLI defect, so
	// it must be classified as expected and never reported to Sentry. "30" (no
	// unit) and "not-a-duration" specifically exercise the time.ParseDuration
	// failure branch that b4f101c left unwrapped.
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"not a duration", "not-a-duration"},
		{"missing unit", "30"},
		{"zero", "0"},
		{"negative", "-5m"},
	} {
		t.Run("expected validation error: "+tc.name, func(t *testing.T) {
			d, err := parsePushBuildTimeout(tc.input)
			require.Error(t, err)
			assert.Equal(t, time.Duration(0), d)
			assert.True(t, core.IsExpectedCLIError(err),
				"a bad --timeout value is user input, not a CLI defect")
		})
	}
}

// TestParsePushBuildTimeoutGuardsRawParseError documents why
// parsePushBuildTimeout must wrap the time.ParseDuration failure: an unwrapped
// parse error is unclassified and would be reported to Sentry as an internal
// "Unexpected CLI failure". If a future change reverts the wrapper, this test
// still passes but TestParsePushBuildTimeout fails — proving the wrap is
// load-bearing.
func TestParsePushBuildTimeoutGuardsRawParseError(t *testing.T) {
	_, rawErr := time.ParseDuration("not-a-duration")
	require.Error(t, rawErr)
	assert.False(t, core.IsExpectedCLIError(rawErr),
		"an unwrapped ParseDuration error is reported as an internal defect")
}

func TestPushCmd(t *testing.T) {
	cmd := PushCmd()

	assert.Equal(t, "push", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flag := cmd.Flags().Lookup("name")
	assert.NotNil(t, flag)
	assert.Equal(t, "n", flag.Shorthand)

	typeFlag := cmd.Flags().Lookup("type")
	assert.NotNil(t, typeFlag)
	assert.Equal(t, "t", typeFlag.Shorthand)

	skipBuildFlag := cmd.Flags().Lookup("skip-build")
	assert.NotNil(t, skipBuildFlag)
	assert.Equal(t, "false", skipBuildFlag.DefValue)

	registryCredFlag := cmd.Flags().Lookup("registry-cred")
	assert.NotNil(t, registryCredFlag)

	dockerConfigFlag := cmd.Flags().Lookup("docker-config")
	assert.NotNil(t, dockerConfigFlag)
}
