package test

import (
	"context"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/sandbox"
	blaxel "github.com/stainless-sdks/blaxel-go"
)

func TestSandboxAPIURL(t *testing.T) {
	// Get current workspace
	blaxelCtx, _ := blaxel.CurrentContext()
	workspace := blaxelCtx.Workspace
	if workspace == "" {
		t.Skip("No workspace configured")
	}

	// Test fetching sandbox URL from API
	ctx := context.Background()
	sandboxName := "base-image"

	t.Logf("Fetching URL for sandbox '%s' in workspace '%s'", sandboxName, workspace)

	url, err := sandbox.GetSandboxURL(ctx, workspace, sandboxName)
	if err != nil {
		t.Logf("Failed to get sandbox URL: %v", err)
		t.Skip("Sandbox might not exist")
	}

	t.Logf("Got sandbox URL from API: %s", url)

	// Check if it's a direct URL (not the run.blaxel.ai format)
	if url != "" {
		// Direct URLs should contain the sandbox name in the subdomain
		if contains(url, ".bl.run") || contains(url, ".blaxel.run") {
			t.Log("✓ Got direct sandbox URL (faster connection)")
		} else if contains(url, "run.blaxel.ai") {
			t.Log("Got fallback URL (run.blaxel.ai)")
		}
	}
}

func TestSandboxClientWithAPI(t *testing.T) {
	// Get current workspace
	blaxelCtx, _ := blaxel.CurrentContext()
	workspace := blaxelCtx.Workspace
	if workspace == "" {
		t.Skip("No workspace configured")
	}

	sandboxName := "base-image"

	t.Logf("Creating sandbox client for '%s' using API-fetched URL", sandboxName)

	// Create client using the new API-based method
	client, err := sandbox.NewSandboxClient(workspace, sandboxName)
	if err != nil {
		t.Logf("Failed to create sandbox client: %v", err)
		t.Skip("Sandbox might not exist or credentials not configured")
	}
	defer client.Close()

	// Test executing a simple command
	ctx := context.Background()
	result, err := client.ExecuteCommand(ctx, "echo 'Hello from API-fetched URL'", "test-api", "/")
	if err != nil {
		t.Errorf("Failed to execute command: %v", err)
		return
	}

	t.Logf("Command output: %s", result.Logs)
	t.Log("✓ Successfully connected using API-fetched URL")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
