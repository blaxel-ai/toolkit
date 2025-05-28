package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	mcp_golang "github.com/agentuity/mcp-golang/v2"
)

type MCPClient struct {
	client    *mcp_golang.Client
	transport interface{} // Store the transport for proper cleanup
}

// checkServiceHealth converts WebSocket URL to HTTP and checks /health endpoint
func checkServiceHealth(wsURL string, headers map[string]string) error {
	// Convert WebSocket URL to HTTP URL
	healthURL := strings.Replace(wsURL, "wss://", "https://", 1)
	healthURL = strings.Replace(healthURL, "ws://", "http://", 1)
	healthURL = healthURL + "/health"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create the request
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Add authentication headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sandbox service is not reachable at %s\nPlease check:\n  - The sandbox name is correct\n  - The workspace name is correct\n  - Your network connection\n  - The sandbox service is running\n\nError: %v", healthURL, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed for sandbox service (status: %d)\nPlease check your credentials with 'bl auth login'", resp.StatusCode)
	} else if resp.StatusCode == 404 {
		return fmt.Errorf("sandbox not found (status: 404)\nPlease check:\n  - The sandbox name '%s' exists\n  - You have access to this sandbox\n  - The sandbox is running", strings.Split(strings.Split(wsURL, "/sandboxes/")[1], "?")[0])
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sandbox service health check failed (status: %d)\nThe service may be temporarily unavailable", resp.StatusCode)
	}

	return nil
}

func NewMCPClient(serverURL string, headers map[string]string) *MCPClient {
	// Check service health before attempting WebSocket connection
	if err := checkServiceHealth(serverURL, headers); err != nil {
		log.Fatalf("Service health check failed: %v", err)
	}

	transport := NewWebSocketClientTransport(serverURL)
	for key, value := range headers {
		transport.WithHeader(key, value)
	}
	underlyingClient := mcp_golang.NewClient(transport)
	_, err := underlyingClient.Initialize(context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	client := &MCPClient{client: underlyingClient, transport: transport}
	return client
}

func (c *MCPClient) ListTools(ctx context.Context) ([]byte, error) {
	cursor := ""
	response, err := c.client.ListTools(ctx, &cursor)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response.Tools)
}

func (c *MCPClient) CallTool(ctx context.Context, toolName string, params any) ([]byte, error) {
	response, err := c.client.CallTool(ctx, toolName, params)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

func (c *MCPClient) Close() error {
	// Close the WebSocket connection properly
	if wsTransport, ok := c.transport.(*WebSocketClientTransport); ok {
		return wsTransport.Close(context.Background())
	}
	// Fallback for other transport types
	return nil
}
