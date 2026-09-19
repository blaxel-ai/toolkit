package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/require"
)

func TestPrepareApplyMetadata(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata interface{}
		want     string
		wantErr  string
	}{
		{"explicit name", map[string]interface{}{"name": "existing", "displayName": "Node"}, "existing", ""},
		{"display name", map[string]interface{}{"displayName": "My Node_123!"}, "my-node-123", ""},
		{"empty name", map[string]interface{}{"name": "", "displayName": "Node"}, "node", ""},
		{"missing metadata", nil, "", ""},
		{"empty metadata", map[string]interface{}{}, "", ""},
		{"blank names", map[string]interface{}{"name": " ", "displayName": "\t"}, "", ""},
		{"invalid metadata", "Node", "", "metadata must be an object"},
		{"invalid name", map[string]interface{}{"name": 42}, "", "metadata.name must be a string"},
		{"invalid display name", map[string]interface{}{"displayName": true}, "", "metadata.displayName must be a string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := core.Result{Metadata: tc.metadata}
			metadata, name, err := prepareApplyMetadata(&result)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.want != "" {
				require.Equal(t, tc.want, name)
			} else {
				require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`), name)
				other := core.Result{}
				_, otherName, err := prepareApplyMetadata(&other)
				require.NoError(t, err)
				require.NotEqual(t, name, otherName)
			}
			require.Equal(t, name, metadata["name"])
			require.Equal(t, metadata, result.Metadata)
		})
	}
}

func TestApplyGeneratedNameReachesSandboxAPI(t *testing.T) {
	var resource *core.Resource
	for _, r := range core.GetResources() {
		if r.Kind == "Sandbox" {
			resource = r
			break
		}
	}
	require.NotNil(t, resource)
	previous := *resource
	t.Cleanup(func() { *resource = previous })
	requests := 0
	conflict := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if conflict && r.Method == http.MethodPut {
			require.Equal(t, "/sandboxes/node", r.URL.Path)
		} else {
			require.Equal(t, http.MethodPost, r.Method)
		}
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		metadata := body["metadata"].(map[string]interface{})
		require.Equal(t, "node", metadata["name"])
		require.Equal(t, "Node", metadata["displayName"])
		w.Header().Set("Content-Type", "application/json")
		if conflict && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"already exists"}`))
			return
		}
		_, _ = w.Write([]byte(`{"metadata":{"name":"node"}}`))
	}))
	defer server.Close()
	client := blaxel.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))
	resource.Post = client.Sandboxes.New
	resource.Put = client.Sandboxes.Update
	path := filepath.Join(t.TempDir(), "sandbox.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`apiVersion: blaxel.ai/v1alpha1
kind: Sandbox
metadata:
  displayName: Node
spec:
  runtime:
    image: blaxel/android:latest
    ports:
      - name: sandbox-api
        target: 8080
        protocol: HTTP
    generation: mk3
    memory: 4096
    ttl: 7d
  region: us-was-1
`), 0600))
	results, err := Apply(path)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "node", results[0].Name)
	require.Equal(t, "created", results[0].Result.Status)
	require.Equal(t, 1, requests)
	conflict = true
	results, err = Apply(path)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "node", results[0].Name)
	require.Equal(t, "configured", results[0].Result.Status)
	require.Equal(t, 3, requests)
	failures, err := ApplyResources([]core.Result{{Kind: "Sandbox", Metadata: map[string]interface{}{"name": 42}}})
	require.NoError(t, err)
	require.Len(t, failures, 1)
	require.Equal(t, "metadata.name must be a string", failures[0].Result.ErrorMsg)
	require.True(t, core.IsExpectedCLIError(failures[0].Result.cause))
	require.Equal(t, 3, requests, "invalid metadata must not reach the API")
}
