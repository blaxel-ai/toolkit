package sandbox

import (
	"context"
	"encoding/json"
	"fmt"

	mcp_golang "github.com/agentuity/mcp-golang/v2"
	"github.com/blaxel-ai/toolkit/sdk/mcp"
)

type SandboxClient struct {
	MCPClient   *mcp.MCPClient
	Workspace   string
	SandboxName string
}

func NewSandboxClientWithURL(workspace, sandboxName, serverURL string, authHeaders map[string]string) (*SandboxClient, error) {
	// Create MCP client with WebSocket transport
	mcpClient, err := mcp.NewMCPClient(serverURL, authHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	return &SandboxClient{
		MCPClient:   mcpClient,
		Workspace:   workspace,
		SandboxName: sandboxName,
	}, nil
}

// File and Directory structures for MCP responses
type File struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Subdirectory struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type Directory struct {
	Path           string          `json:"path"`
	Name           string          `json:"name"`
	Files          []*File         `json:"files"`
	Subdirectories []*Subdirectory `json:"subdirectories"`
}

// Process structures for MCP responses
type ProcessResponse struct {
	PID string `json:"pid"`
}

type ProcessResponseWithLogs struct {
	ProcessResponse
	Logs string `json:"logs"`
}

// MCP-based API Methods

func (c *SandboxClient) ExecuteCommand(ctx context.Context, command, name, workingDir string) (*ProcessResponseWithLogs, error) {
	params := map[string]interface{}{
		"command":           command,
		"name":              name,
		"workingDir":        workingDir,
		"waitForCompletion": true,
		"timeout":           0,
		"waitForPorts":      []int{},
		"includeLogs":       true,
	}

	response, err := c.MCPClient.CallTool(ctx, "processExecute", params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	return c.parseProcessResponse(response)
}

func (c *SandboxClient) ListDirectory(ctx context.Context, path string) (*Directory, error) {
	params := map[string]interface{}{
		"path": path,
	}

	response, err := c.MCPClient.CallTool(ctx, "fsListDirectory", params)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	return c.parseDirectoryResponse(response)
}

// Helper method to parse process response with error handling
func (c *SandboxClient) parseProcessResponse(response *mcp_golang.ToolResponse) (*ProcessResponseWithLogs, error) {

	// Check for MCP errors
	if response == nil || len(response.Content) == 0 {
		return nil, fmt.Errorf("unexpected response format from processExecute")
	}

	// Extract the process information from the MCP response
	if response.Content[0].Type == mcp_golang.ContentTypeText {
		text := response.Content[0].TextContent.Text
		var processWithLogs ProcessResponseWithLogs
		if err := json.Unmarshal([]byte(text), &processWithLogs); err != nil {
			return nil, fmt.Errorf("failed to parse process response: %w", err)
		}
		return &processWithLogs, nil
	}

	return nil, fmt.Errorf("unexpected response format from processExecute")
}

// Helper method to parse directory response with error handling
func (c *SandboxClient) parseDirectoryResponse(response *mcp_golang.ToolResponse) (*Directory, error) {

	if response == nil || len(response.Content) == 0 {
		return nil, fmt.Errorf("unexpected response format from fsListDirectory")
	}

	if response.Content[0].Type == mcp_golang.ContentTypeText {
		text := response.Content[0].TextContent.Text
		var dir Directory
		if err := json.Unmarshal([]byte(text), &dir); err != nil {
			return nil, fmt.Errorf("failed to parse directory response: %w", err)
		}
		return &dir, nil
	}
	return nil, fmt.Errorf("unexpected response format from fsListDirectory")
}

func (c *SandboxClient) Close() error {
	if c.MCPClient != nil {
		return c.MCPClient.Close()
	}
	return nil
}
