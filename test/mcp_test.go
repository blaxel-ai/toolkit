package test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	officialMcp "github.com/modelcontextprotocol/go-sdk/mcp"
	blaxel "github.com/stainless-sdks/blaxel-go"
	"github.com/stainless-sdks/blaxel-go/lib/mcp"
)

// getAuthHeaders retrieves authentication headers from environment or SDK credentials
func getAuthHeaders(t *testing.T) (map[string]string, string) {
	// First, check environment variables
	if token := os.Getenv("BLAXEL_AUTH_TOKEN"); token != "" {
		return map[string]string{
			"Authorization": "Bearer " + token,
		}, ""
	}

	if apiKey := os.Getenv("BL_API_KEY"); apiKey != "" {
		return map[string]string{
			"X-Blaxel-Authorization": "Bearer " + apiKey,
			"X-Blaxel-Workspace":     os.Getenv("BL_WORKSPACE"),
		}, os.Getenv("BL_WORKSPACE")
	}

	// Try to get from SDK credentials (for users who are logged in via CLI)
	ctx, _ := blaxel.CurrentContext()
	workspace := ctx.Workspace
	if workspace == "" {
		workspace = os.Getenv("BL_WORKSPACE")
	}

	if workspace != "" {
		credentials, _ := blaxel.LoadCredentials(workspace)
		if credentials.IsValid() {
			// Set authentication headers directly based on credential type
			if credentials.APIKey != "" {
				return map[string]string{
					"Authorization": "Bearer " + credentials.APIKey,
				}, workspace
			} else if credentials.AccessToken != "" {
				return map[string]string{
					"Authorization": "Bearer " + credentials.AccessToken,
				}, workspace
			}
		}
	}

	return nil, ""
}

func TestMCPTransports(t *testing.T) {
	// Get auth headers
	headers, workspace := getAuthHeaders(t)
	if headers == nil {
		t.Skip("No authentication available. Set BLAXEL_AUTH_TOKEN, BL_API_KEY, or login via 'bl login'")
	}

	// Get sandbox URL from environment or use default
	serverURL := os.Getenv("SANDBOX_URL")
	if serverURL == "" {
		// Apply any environment overrides
		blaxel.ApplyEnvironmentOverrides()

		if workspace != "" {
			serverURL = blaxel.BuildSandboxURL(workspace, "base-image")
		} else {
			serverURL = blaxel.BuildSandboxURL("blaxel", "base-image")
		}
		t.Logf("Using default sandbox URL: %s", serverURL)
	}

	t.Run("Auto-detect transport", func(t *testing.T) {
		testClient(t, serverURL, headers, mcp.TransportTypeAuto)
	})

	t.Run("HTTP Stream transport", func(t *testing.T) {
		testHTTPStreamTransport(t, serverURL, headers)
	})

	t.Run("WebSocket transport", func(t *testing.T) {
		testClient(t, serverURL, headers, mcp.TransportTypeWebSocket)
	})
}

func testClient(t *testing.T, serverURL string, headers map[string]string, transportType mcp.TransportType) {
	t.Logf("Creating client with transport type: %s", transportType)

	client, err := mcp.NewMCPClientWithTransport(serverURL, headers, transportType)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
		return
	}
	defer client.Close()

	t.Log("Client created successfully!")

	// List tools
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Errorf("Failed to list tools: %v", err)
		return
	}

	t.Logf("Found %d tools:", len(tools.Tools))
	for _, tool := range tools.Tools {
		t.Logf("  - %s: %s", tool.Name, tool.Description)
	}

	// Try to execute a simple command
	if len(tools.Tools) > 0 {
		t.Log("Testing tool execution...")

		// Try to list directory
		params := map[string]interface{}{
			"path": "/",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := client.CallTool(ctx, "fsListDirectory", params)
		if err != nil {
			t.Errorf("Failed to call tool: %v", err)
			return
		}

		if result.IsError {
			t.Log("Tool returned an error")
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(*officialMcp.TextContent); ok {
					t.Logf("Error: %s", textContent.Text)
				}
			}
		} else {
			t.Log("Tool executed successfully!")
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(*officialMcp.TextContent); ok {
					// Just show first 200 chars of response
					response := textContent.Text
					if len(response) > 200 {
						response = response[:200] + "..."
					}
					t.Logf("Response: %s", response)
				}
			}
		}

		// Test command execution with output
		testCommandExecution(t, client)
	}
}

func testHTTPStreamTransport(t *testing.T, serverURL string, headers map[string]string) {
	t.Log("Testing HTTP Stream transport directly with official SDK...")

	// Create HTTP client with headers
	httpClient := &http.Client{
		Transport: &headerTransport{
			base:    http.DefaultTransport,
			headers: headers,
		},
	}

	// Ensure URL ends with /mcp for HTTP stream
	if !strings.HasSuffix(serverURL, "/mcp") {
		serverURL = serverURL + "/mcp"
	}

	// Create transport
	transport := &officialMcp.StreamableClientTransport{
		Endpoint:   serverURL,
		HTTPClient: httpClient,
		MaxRetries: 3,
	}

	// Create client
	impl := &officialMcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	client := officialMcp.NewClient(impl, nil)

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Connecting to server...")
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer session.Close()

	t.Log("Connected successfully!")

	// List tools
	t.Log("Listing tools...")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	tools, err := session.ListTools(ctx2, &officialMcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	t.Logf("Found %d tools", len(tools.Tools))
	for _, tool := range tools.Tools {
		t.Logf("  - %s: %s", tool.Name, tool.Description)
	}

	// Execute a command that should return output
	if len(tools.Tools) > 0 {
		t.Log("Executing command via processExecute...")

		params := map[string]any{
			"command":           "echo 'Hello from SSE transport!'",
			"name":              "test-echo-sse",
			"workingDir":        "/",
			"waitForCompletion": true,
			"timeout":           5000,
			"waitForPorts":      []int{},
			"includeLogs":       true,
		}

		ctx3, cancel3 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel3()

		result, err := session.CallTool(ctx3, &officialMcp.CallToolParams{
			Name:      "processExecute",
			Arguments: params,
		})

		if err != nil {
			t.Fatalf("Failed to call tool: %v", err)
		}

		if result.IsError {
			t.Log("Tool returned an error")
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(*officialMcp.TextContent); ok {
					t.Logf("Error: %s", textContent.Text)
				}
			}
		} else {
			t.Log("Tool executed successfully!")
			if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(*officialMcp.TextContent); ok {
					t.Logf("Response: %s", textContent.Text)

					// Try to parse the response to extract logs
					var parsed map[string]interface{}
					if err := json.Unmarshal([]byte(textContent.Text), &parsed); err == nil {
						if logs, ok := parsed["logs"].(string); ok {
							t.Logf("Command output: %s", logs)
						} else {
							t.Log("No 'logs' field found in response")
						}
					}
				}
			} else {
				t.Log("No content in response")
			}
		}
	}
}

func testCommandExecution(t *testing.T, client *mcp.MCPClient) {
	t.Log("\nTesting command execution with output...")

	params := map[string]interface{}{
		"command":           "echo 'Hello from MCP client!'",
		"name":              "test-echo",
		"workingDir":        "/",
		"waitForCompletion": true,
		"timeout":           5000,
		"waitForPorts":      []int{},
		"includeLogs":       true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.CallTool(ctx, "processExecute", params)
	if err != nil {
		t.Errorf("Failed to execute command: %v", err)
		return
	}

	if result.IsError {
		t.Log("Command returned an error")
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*officialMcp.TextContent); ok {
				t.Logf("Error: %s", textContent.Text)
			}
		}
	} else {
		t.Log("Command executed successfully!")
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*officialMcp.TextContent); ok {
				// Parse the JSON response to extract logs
				var response map[string]interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &response); err == nil {
					if logs, ok := response["logs"].(string); ok && logs != "" {
						t.Logf("Command output: %s", logs)
					} else {
						t.Log("No output captured (logs field empty or missing)")
						t.Logf("Full response: %s", textContent.Text)
					}
				} else {
					t.Logf("Failed to parse response as JSON: %v", err)
					t.Logf("Raw response: %s", textContent.Text)
				}
			}
		} else {
			t.Log("No content in response")
		}
	}
}

// headerTransport adds headers to HTTP requests
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())

	// Add headers
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// Always include Accept header for SSE/streaming
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/event-stream")
	}

	return t.base.RoundTrip(req)
}
