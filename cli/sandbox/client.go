package sandbox

import (
	"context"
	"encoding/json"
	"fmt"

	officialMcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stainless-sdks/blaxel-go/lib/mcp"
)

type SandboxClient struct {
	MCPClient   *mcp.MCPClient
	Workspace   string
	SandboxName string
}

func NewSandboxClientWithURL(workspace, sandboxName, serverURL string, authHeaders map[string]string) (*SandboxClient, error) {
	// Create MCP client with auto-detected transport (WebSocket or HTTP Stream)
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
func (c *SandboxClient) parseProcessResponse(response *officialMcp.CallToolResult) (*ProcessResponseWithLogs, error) {

	// Check for MCP errors
	if response == nil || response.IsError || len(response.Content) == 0 {
		if response != nil && response.IsError && len(response.Content) > 0 {
			// Extract error message
			if textContent, ok := response.Content[0].(*officialMcp.TextContent); ok {
				return nil, fmt.Errorf("tool error: %s", textContent.Text)
			}
		}
		return nil, fmt.Errorf("unexpected response format from processExecute")
	}

	// Extract the process information from the MCP response
	if textContent, ok := response.Content[0].(*officialMcp.TextContent); ok {
		// Try to parse as a map first to check structure
		var rawResponse map[string]interface{}
		if err := json.Unmarshal([]byte(textContent.Text), &rawResponse); err == nil {
			// Check if this is an HTTP Stream response with "withLogs" field
			if withLogs, hasWithLogs := rawResponse["withLogs"]; hasWithLogs {
				// HTTP Stream format: data is nested under "withLogs"
				withLogsBytes, _ := json.Marshal(withLogs)
				var processWithLogs ProcessResponseWithLogs
				if err := json.Unmarshal(withLogsBytes, &processWithLogs); err != nil {
					return nil, fmt.Errorf("failed to parse withLogs response: %w", err)
				}
				return &processWithLogs, nil
			}
		}

		// Try direct parsing for WebSocket format
		var processWithLogs ProcessResponseWithLogs
		if err := json.Unmarshal([]byte(textContent.Text), &processWithLogs); err != nil {
			return nil, fmt.Errorf("failed to parse process response: %w", err)
		}
		return &processWithLogs, nil
	}

	return nil, fmt.Errorf("unexpected response format from processExecute")
}

// Helper method to parse directory response with error handling
func (c *SandboxClient) parseDirectoryResponse(response *officialMcp.CallToolResult) (*Directory, error) {

	if response == nil || response.IsError || len(response.Content) == 0 {
		if response != nil && response.IsError && len(response.Content) > 0 {
			// Extract error message
			if textContent, ok := response.Content[0].(*officialMcp.TextContent); ok {
				return nil, fmt.Errorf("tool error: %s", textContent.Text)
			}
		}
		return nil, fmt.Errorf("unexpected response format from fsListDirectory")
	}

	if textContent, ok := response.Content[0].(*officialMcp.TextContent); ok {
		var dir Directory
		if err := json.Unmarshal([]byte(textContent.Text), &dir); err != nil {
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
