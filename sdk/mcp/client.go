package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	mcp_golang "github.com/agentuity/mcp-golang/v2"
	"github.com/agentuity/mcp-golang/v2/transport/stdio"
)

type MCPClient struct {
	client *mcp_golang.Client
	cmd    *exec.Cmd
}

type MCPClientTransport string

const (
	TransportWebSocket MCPClientTransport = "websocket"
	TransportStdio     MCPClientTransport = "stdio"
)

func NewMCPClient(mode MCPClientTransport, command string, serverURL string, headers map[string]string) *MCPClient {
	switch mode {
	case TransportWebSocket:
		transport := NewWebSocketClientTransport(serverURL)
		for key, value := range headers {
			transport.WithHeader(key, value)
		}
		underlyingClient := mcp_golang.NewClient(transport)
		_, err := underlyingClient.Initialize(context.Background())
		if err != nil {
			log.Fatalf("Failed to initialize client: %v", err)
		}
		client := &MCPClient{client: underlyingClient}
		return client
	case TransportStdio:
		if command == "" {
			log.Fatalf("Command is required for stdio transport")
		}

		commandParts := strings.Split(command, " ")
		fmt.Printf("Command parts: %v\n", commandParts)
		cmd := exec.Command(commandParts[0], commandParts[1:]...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatalf("Failed to get stdin pipe: %v", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatalf("Failed to get stdout pipe: %v", err)
		}

		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}

		// Create and initialize client
		underlyingClient := mcp_golang.NewClient(stdio.NewStdioServerTransportWithIO(stdout, stdin))
		_, err = underlyingClient.Initialize(context.Background())
		if err != nil {
			log.Fatalf("Failed to initialize client: %v", err)
		}
		client := &MCPClient{client: underlyingClient, cmd: cmd}
		return client
	}
	return nil
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
	return c.cmd.Process.Kill()
}
