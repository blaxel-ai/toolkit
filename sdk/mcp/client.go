package mcp

import (
	"context"
	"fmt"
	"strings"

	mcp_golang "github.com/agentuity/mcp-golang/v2"
)

type MCPClient struct {
	client    *mcp_golang.Client
	transport interface{} // Store the transport for proper cleanup
}

func NewMCPClient(serverURL string, headers map[string]string) (*MCPClient, error) {
	wsServerURL := strings.Replace(serverURL, "http", "ws", 1)
	transport := NewWebSocketClientTransport(wsServerURL)
	for key, value := range headers {
		transport.WithHeader(key, value)
	}
	underlyingClient := mcp_golang.NewClient(transport)
	_, err := underlyingClient.Initialize(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}
	client := &MCPClient{client: underlyingClient, transport: transport}
	return client, nil
}

func (c *MCPClient) ListTools(ctx context.Context) (*mcp_golang.ToolsResponse, error) {
	cursor := ""
	return c.client.ListTools(ctx, &cursor)
}

func (c *MCPClient) CallTool(ctx context.Context, toolName string, params any) (*mcp_golang.ToolResponse, error) {
	return c.client.CallTool(ctx, toolName, params)
}

func (c *MCPClient) Close() error {
	// Close the WebSocket connection properly
	if wsTransport, ok := c.transport.(*WebSocketClientTransport); ok {
		return wsTransport.Close(context.Background())
	}
	// Fallback for other transport types
	return nil
}
