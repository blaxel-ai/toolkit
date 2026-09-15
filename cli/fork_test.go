package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseForkArg(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		resourceType string
		resourceName string
		wantErr      string
	}{
		{name: "untyped", arg: "source", resourceName: "source"},
		{name: "sandbox", arg: "sbx/source", resourceType: "sandbox", resourceName: "source"},
		{name: "application", arg: "application/target", resourceType: "application", resourceName: "target"},
		{name: "missing name", arg: "sbx/", wantErr: "missing name"},
		{name: "unknown type", arg: "agent/source", wantErr: "unknown resource type"},
		{name: "nested path", arg: "sbx/foo/bar", wantErr: "must not contain '/'"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resourceType, resourceName, err := parseForkArg(test.arg)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.resourceType, resourceType)
			assert.Equal(t, test.resourceName, resourceName)
		})
	}
}

func TestBuildForkRequestUsesFlatApplicationFields(t *testing.T) {
	traffic := 20
	port := 8080
	request := buildForkRequest("target", "application", &traffic, &port, "")

	payload, err := json.Marshal(request)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(payload, &body))
	// The API's SandboxForkRequest is camelCase; snake_case keys never reach
	// its fields, so the server would read an empty target and reject the call.
	assert.Equal(t, "target", body["targetName"])
	assert.Equal(t, "application", body["targetType"])
	assert.Equal(t, float64(20), body["traffic"])
	assert.Equal(t, float64(8080), body["port"])
	assert.NotContains(t, body, "target_name")
	assert.NotContains(t, body, "type")
	assert.NotContains(t, body, "memory")
	assert.NotContains(t, body, "spec")
}

func TestBuildForkRequestOmitsUnsetOptionalFields(t *testing.T) {
	request := buildForkRequest("target", "sandbox", nil, nil, "")

	payload, err := json.Marshal(request)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "target", body["targetName"])
	assert.Equal(t, "sandbox", body["targetType"])
	// A direct fork carries no snapshot: the source's live state is copied.
	assert.NotContains(t, body, "snapshotId")
	assert.NotContains(t, body, "traffic")
	assert.NotContains(t, body, "port")
}

func TestBuildForkRequestCarriesSnapshotID(t *testing.T) {
	request := buildForkRequest("target", "sandbox", nil, nil, "snap_abc123")

	payload, err := json.Marshal(request)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "snap_abc123", body["snapshotId"])
}
