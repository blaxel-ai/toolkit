package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotResource(t *testing.T) {
	var snapshot *Resource
	for _, r := range GetResources() {
		if r.Kind == "Snapshot" {
			snapshot = r
			break
		}
	}
	if !assert.NotNil(t, snapshot, "Snapshot resource should be registered") {
		return
	}

	assert.Equal(t, "snapshots", snapshot.Plural)
	assert.Equal(t, "snapshot", snapshot.Singular)
	assert.Equal(t, "snapshots", snapshot.APIPath)
	assert.True(t, snapshot.Paginated)
	assert.Empty(t, snapshot.ParentField, "snapshots are workspace-level, not nested under a sandbox")

	keys := make([]string, 0, len(snapshot.Fields))
	for _, f := range snapshot.Fields {
		keys = append(keys, f.Key)
	}
	assert.Equal(t, []string{"WORKSPACE", "ID", "NAME", "SOURCE", "IMAGE", "REGION", "STATUS", "CREATED_AT"}, keys)
}

func TestSnapshotOperationsAreRegistered(t *testing.T) {
	resource := &Resource{Kind: "Snapshot"}
	snapshotOperations(resource, &blaxel.Client{})

	assert.NotNil(t, resource.Get)
	assert.NotNil(t, resource.Delete)
	// Listing is served by the paginated APIPath, and snapshots are created
	// from their source object, not from a manifest.
	assert.Nil(t, resource.List)
	assert.Nil(t, resource.Put)
	assert.Nil(t, resource.Post)
}

func TestSnapshotOperationsHitWorkspaceEndpoints(t *testing.T) {
	type requestRecord struct {
		method string
		path   string
	}
	var records []requestRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records = append(records, requestRecord{method: r.Method, path: r.URL.EscapedPath()})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/snapshots/my snapshot"):
			_, _ = w.Write([]byte(`{"name":"my snapshot","workspace":"ws","status":"ready","createdAt":"2026-01-01T00:00:00Z","source":{"name":"my-sandbox"},"spec":{"image":"blaxel/base:latest"}}`))
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/snapshots/my snapshot"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/snapshots/missing"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"snapshot not found"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := blaxel.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test-api-key"))
	resource := &Resource{Kind: "Snapshot"}
	snapshotOperations(resource, &client)
	ctx := context.Background()
	get := resource.Get.(func(context.Context, string, ...option.RequestOption) (*blaxel.SandboxSnapshot, error))
	del := resource.Delete.(func(context.Context, string) (any, error))

	snapshot, err := get(ctx, "my snapshot")
	require.NoError(t, err)
	assert.Equal(t, "my snapshot", snapshot.Name)
	assert.Equal(t, "ws", snapshot.Workspace)
	assert.Equal(t, "my-sandbox", snapshot.Source.Name)

	deleted, err := del(ctx, "my snapshot")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"id": "my snapshot"}, deleted)

	_, err = get(ctx, "missing")
	require.Error(t, err)

	require.Len(t, records, 3)
	assert.Equal(t, http.MethodGet, records[0].method)
	assert.True(t, strings.HasSuffix(records[0].path, "/snapshots/my%20snapshot"), records[0].path)
	assert.Equal(t, http.MethodDelete, records[1].method)
	assert.True(t, strings.HasSuffix(records[1].path, "/snapshots/my%20snapshot"), records[1].path)
}

func TestFlatSnapshotKeepsItsIdentityInJsonAndYaml(t *testing.T) {
	resource := Resource{Kind: "Snapshot"}
	snapshot := map[string]interface{}{
		"name":      "my-snapshot",
		"workspace": "ws",
		"status":    "ready",
		"createdAt": "2026-01-01T00:00:00Z",
		"source":    map[string]interface{}{"name": "my-sandbox"},
		"spec":      map[string]interface{}{"image": "blaxel/base:latest"},
	}

	result := toResult(resource, snapshot)
	metadata := result.Metadata.(map[string]interface{})
	assert.Equal(t, "my-snapshot", metadata["name"])
	assert.Equal(t, "ws", metadata["workspace"])
	assert.Equal(t, "2026-01-01T00:00:00Z", metadata["createdAt"])
	assert.Equal(t, map[string]interface{}{"name": "my-sandbox"}, metadata["source"])
	assert.NotContains(t, metadata, "spec")
	assert.NotContains(t, metadata, "status")
	assert.Equal(t, snapshot["spec"], result.Spec)
	assert.Equal(t, "ready", result.Status)

	yamlOut := string(renderYaml(resource, []interface{}{snapshot}, false))
	assert.Contains(t, yamlOut, "name: my-snapshot")
	assert.Contains(t, yamlOut, "workspace: ws")
	assert.NotContains(t, yamlOut, "metadata: null")

	// Objects that already carry metadata are rendered as before.
	nested := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-sandbox"},
		"spec":     map[string]interface{}{},
		"status":   "DEPLOYED",
	}
	assert.Equal(t, nested["metadata"], toResult(resource, nested).Metadata)
}
