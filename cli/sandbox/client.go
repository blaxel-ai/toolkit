package sandbox

import (
	"context"
	"encoding/json"
	"fmt"

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
func (c *SandboxClient) parseProcessResponse(response []byte) (*ProcessResponseWithLogs, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for MCP errors
	if isError, ok := result["isError"].(bool); ok && isError {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			if errorContent, ok := content[0].(map[string]interface{}); ok {
				if text, ok := errorContent["text"].(string); ok {
					return nil, fmt.Errorf("MCP error: %s", text)
				}
			}
		}
		return nil, fmt.Errorf("MCP error occurred")
	}

	// Extract the process information from the MCP response
	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if textContent, ok := content[0].(map[string]interface{}); ok {
			if text, ok := textContent["text"].(string); ok {
				var processWithLogs ProcessResponseWithLogs
				if err := json.Unmarshal([]byte(text), &processWithLogs); err != nil {
					return nil, fmt.Errorf("failed to parse process response: %w", err)
				}
				return &processWithLogs, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response format from processExecute")
}

// Helper method to parse directory response with error handling
func (c *SandboxClient) parseDirectoryResponse(response []byte) (*Directory, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for MCP errors
	if isError, ok := result["isError"].(bool); ok && isError {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			if errorContent, ok := content[0].(map[string]interface{}); ok {
				if text, ok := errorContent["text"].(string); ok {
					return nil, fmt.Errorf("MCP error: %s", text)
				}
			}
		}
		return nil, fmt.Errorf("MCP error occurred")
	}

	// Extract the directory information from the MCP response
	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if textContent, ok := content[0].(map[string]interface{}); ok {
			if text, ok := textContent["text"].(string); ok {
				var dir Directory
				if err := json.Unmarshal([]byte(text), &dir); err != nil {
					return nil, fmt.Errorf("failed to parse directory response: %w", err)
				}
				return &dir, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response format from fsListDirectory")
}

func (c *SandboxClient) Close() error {
	if c.MCPClient != nil {
		return c.MCPClient.Close()
	}
	return nil
}
