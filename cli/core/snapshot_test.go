package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, []string{"WORKSPACE", "NAME", "SOURCE", "IMAGE", "REGION", "STATUS", "CREATED_AT"}, keys)
}

func TestSnapshotPath(t *testing.T) {
	assert.Equal(t, "snapshots/my-snapshot", snapshotPath("my-snapshot"))
	assert.Equal(t, "snapshots/my%20snapshot", snapshotPath("my snapshot"))
}

func TestSnapshotOperationsAreRegistered(t *testing.T) {
	resource := &Resource{Kind: "Snapshot"}
	snapshotOperations(resource, nil)

	assert.NotNil(t, resource.Get)
	assert.NotNil(t, resource.Delete)
	// Listing is served by the paginated APIPath, and snapshots are created
	// from their source object, not from a manifest.
	assert.Nil(t, resource.List)
	assert.Nil(t, resource.Put)
	assert.Nil(t, resource.Post)
}
