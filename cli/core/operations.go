package core

import (
	"context"
)

// RegisterResourceOperations registers the SDK client methods for each resource
// This replaces the old client.RegisterCliCommands pattern
func RegisterResourceOperations(ctx context.Context) {
	c := GetClient()
	if c == nil {
		return
	}

	// Register Agent operations
	for _, resource := range resources {
		switch resource.Kind {
		case "Agent":
			resource.List = c.Agents.List
			resource.Get = c.Agents.Get
			resource.Delete = c.Agents.Delete
			resource.Put = c.Agents.Update
			resource.Post = c.Agents.New
		case "Policy":
			resource.List = c.Policies.List
			resource.Get = c.Policies.Get
			resource.Delete = c.Policies.Delete
			resource.Put = c.Policies.Update
			resource.Post = c.Policies.New
		case "Model":
			resource.List = c.Models.List
			resource.Get = c.Models.Get
			resource.Delete = c.Models.Delete
			resource.Put = c.Models.Update
			resource.Post = c.Models.New
		case "Function":
			resource.List = c.Functions.List
			resource.Get = c.Functions.Get
			resource.Delete = c.Functions.Delete
			resource.Put = c.Functions.Update
			resource.Post = c.Functions.New
		case "IntegrationConnection":
			resource.List = c.Integrations.Connections.List
			resource.Get = c.Integrations.Connections.Get
			resource.Delete = c.Integrations.Connections.Delete
			resource.Put = c.Integrations.Connections.Update
			resource.Post = c.Integrations.Connections.New
		case "Sandbox":
			resource.List = c.Sandboxes.List
			resource.Get = c.Sandboxes.Get
			resource.Delete = c.Sandboxes.Delete
			resource.Put = c.Sandboxes.Update
			resource.Post = c.Sandboxes.New
		case "Job":
			resource.List = c.Jobs.List
			resource.Get = c.Jobs.Get
			resource.Delete = c.Jobs.Delete
			resource.Put = c.Jobs.Update
			resource.Post = c.Jobs.New
		case "Volume":
			resource.List = c.Volumes.List
			resource.Get = c.Volumes.Get
			resource.Delete = c.Volumes.Delete
			resource.Put = c.Volumes.Update
			resource.Post = c.Volumes.New
		case "VolumeTemplate":
			resource.List = c.VolumeTemplates.List
			resource.Get = c.VolumeTemplates.Get
			resource.Delete = c.VolumeTemplates.Delete
			resource.Put = c.VolumeTemplates.Upsert
			resource.Post = c.VolumeTemplates.New
		case "Image":
			resource.List = c.Images.List
			resource.Get = c.Images.Get
			resource.Delete = c.Images.Delete
			// Images don't have Put/Post operations
		case "Drive":
			resource.List = c.Drives.List
			resource.Get = c.Drives.Get
			resource.Delete = c.Drives.Delete
			resource.Put = c.Drives.Update
			resource.Post = c.Drives.New
		}
	}
}

// GetResources returns the list of resources
func GetResources() []*Resource {
	return resources
}
