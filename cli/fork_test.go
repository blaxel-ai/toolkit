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
	memory := 2048
	request := buildForkRequest("target", "application", &traffic, &port, &memory)

	payload, err := json.Marshal(request)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "target", body["target_name"])
	assert.Equal(t, "application", body["type"])
	assert.Equal(t, float64(20), body["traffic"])
	assert.Equal(t, float64(8080), body["port"])
	assert.Equal(t, float64(2048), body["memory"])
	assert.NotContains(t, body, "spec")
}
