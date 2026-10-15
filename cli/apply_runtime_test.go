package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/require"
)

func TestApplySandboxSendsLifecycleGracePeriods(t *testing.T) {
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
	var runtime map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Spec struct {
				Runtime map[string]interface{} `json:"runtime"`
			} `json:"spec"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		runtime = body.Spec.Runtime
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"metadata":{"name":"lifecycle"}}`))
	}))
	defer server.Close()
	client := blaxel.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))
	resource.Post = client.Sandboxes.New
	resource.Put = client.Sandboxes.Update
	path := filepath.Join(t.TempDir(), "sandbox.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`apiVersion: blaxel.ai/v1alpha1
kind: Sandbox
metadata:
  name: lifecycle
spec:
  runtime:
    image: blaxel/base-image:latest
    memory: 2048
    terminationGracePeriodSeconds: 30
    standbyGracePeriodSeconds: 15
  region: us-was-1
`), 0600))
	_, err := Apply(path)
	require.NoError(t, err)
	require.EqualValues(t, 30, runtime["terminationGracePeriodSeconds"])
	require.EqualValues(t, 15, runtime["standbyGracePeriodSeconds"])
}
