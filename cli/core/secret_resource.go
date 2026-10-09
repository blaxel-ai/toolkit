package core

import (
	"context"
	"fmt"
	"net/url"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
)

// WorkspaceSecret is the metadata of a workspace secret. Values are write-only:
// the API never returns them.
type WorkspaceSecret struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ListWorkspaceSecrets returns the metadata of every secret of the workspace.
func ListWorkspaceSecrets(ctx context.Context, c *blaxel.Client) (*[]WorkspaceSecret, error) {
	out := []WorkspaceSecret{}
	if err := c.Get(ctx, "secrets", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetWorkspaceSecret creates or replaces the secret; every call stores a new version.
func SetWorkspaceSecret(ctx context.Context, c *blaxel.Client, name, value string) (*WorkspaceSecret, error) {
	var out WorkspaceSecret
	body := map[string]string{"name": name, "value": value}
	if err := c.Post(ctx, "secrets", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWorkspaceSecret removes the secret and all its versions.
func DeleteWorkspaceSecret(ctx context.Context, c *blaxel.Client, name string) (*WorkspaceSecret, error) {
	var out WorkspaceSecret
	if err := c.Delete(ctx, fmt.Sprintf("secrets/%s", url.PathEscape(name)), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// secretOperations wires `bl get secrets` / `bl delete secret <name>`. Secrets
// are written with `bl secret set`, never through apply.
func secretOperations(resource *Resource, c *blaxel.Client) {
	resource.List = func(ctx context.Context) (*[]WorkspaceSecret, error) {
		return ListWorkspaceSecrets(ctx, c)
	}
	resource.Delete = func(ctx context.Context, name string) (any, error) {
		return DeleteWorkspaceSecret(ctx, c, name)
	}
}
