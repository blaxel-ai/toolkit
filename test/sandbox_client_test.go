package test

import (
	"context"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/sandbox"
)

func TestSandboxClient(t *testing.T) {
	// Test URLs for different transports
	websocketURL := "https://sbx-base-ctyi35.us-pdx-1.bl.run"
	httpStreamURL := "https://sbx-base-image-ctyi35.us-pdx-1.bl.run"

	headers, _ := getAuthHeaders(t)
	if headers == nil {
		t.Skip("No authentication available")
	}

	t.Run("WebSocket Transport", func(t *testing.T) {
		testSandboxClient(t, websocketURL, headers)
	})

	t.Run("HTTP Stream Transport", func(t *testing.T) {
		testSandboxClient(t, httpStreamURL, headers)
	})
}

func testSandboxClient(t *testing.T, serverURL string, headers map[string]string) {
	t.Logf("Testing with URL: %s", serverURL)

	// Create sandbox client
	client, err := sandbox.NewSandboxClientWithURL("test-workspace", "test-sandbox", serverURL, headers)
	if err != nil {
		t.Fatalf("Failed to create sandbox client: %v", err)
	}
	defer client.Close()

	// Test executing a command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Executing echo command...")
	result, err := client.ExecuteCommand(ctx, "echo 'Hello from sandbox client'", "test-echo", "/")
	if err != nil {
		t.Errorf("Failed to execute command: %v", err)
		return
	}

	t.Logf("PID: %s", result.PID)
	t.Logf("Logs: %s", result.Logs)

	if result.Logs == "" {
		t.Error("No logs received - THIS IS THE BUG!")
	} else {
		t.Log("✓ Successfully received command output")
	}

	// Test listing directory
	t.Log("Listing root directory...")
	dir, err := client.ListDirectory(ctx, "/")
	if err != nil {
		t.Errorf("Failed to list directory: %v", err)
		return
	}

	t.Logf("Found %d files and %d subdirectories", len(dir.Files), len(dir.Subdirectories))
}
