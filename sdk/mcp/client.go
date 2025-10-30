package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPClient wraps the official MCP SDK client
type MCPClient struct {
	client        *mcp.Client
	session       *mcp.ClientSession
	transport     mcp.Transport
	serverURL     string
	headers       map[string]string
	transportType TransportType
	mu            sync.Mutex
}

// TransportType represents the type of transport to use
type TransportType string

const (
	TransportTypeAuto       TransportType = "auto"        // Auto-detect transport type
	TransportTypeWebSocket  TransportType = "websocket"   // Use WebSocket transport
	TransportTypeHTTPStream TransportType = "http-stream" // Use HTTP streaming transport
)

// NewMCPClient creates a new MCP client with auto-detected transport
func NewMCPClient(serverURL string, headers map[string]string) (*MCPClient, error) {
	return NewMCPClientWithTransport(serverURL, headers, TransportTypeAuto)
}

// NewMCPClientWithTransport creates a new MCP client with specified transport type
func NewMCPClientWithTransport(serverURL string, headers map[string]string, transportType TransportType) (*MCPClient, error) {
	// Store the original server URL for reconnection
	originalServerURL := serverURL

	// Detect transport type if auto
	if transportType == TransportTypeAuto {
		detectedType, err := detectTransportType(serverURL, headers)
		if err != nil {
			// Default to websocket if detection fails
			transportType = TransportTypeWebSocket
		} else {
			transportType = detectedType
		}
	}

	fmt.Println("transportType", transportType)
	// Create MCP client implementation
	impl := &mcp.Implementation{
		Name:    "mcp-client",
		Version: "1.0.0",
	}

	// Create the MCP client
	client := mcp.NewClient(impl, nil)

	// Create the appropriate transport
	var transport mcp.Transport
	var actualEndpoint string

	switch transportType {
	case TransportTypeHTTPStream:
		// Use official SDK's StreamableClientTransport for HTTP
		// Ensure URL ends with /mcp for HTTP stream transport if needed
		actualEndpoint = serverURL
		if !strings.HasSuffix(actualEndpoint, "/mcp") {
			u, err := url.Parse(actualEndpoint)
			if err != nil {
				return nil, fmt.Errorf("failed to parse server URL: %w", err)
			}
			if !strings.Contains(u.Path, "/mcp") {
				if u.Path == "" || u.Path == "/" {
					u.Path = "/mcp"
				} else {
					u.Path = u.Path + "/mcp"
				}
				actualEndpoint = u.String()
			}
		}

		// Create HTTP client with headers
		httpClient := &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: headers,
			},
		}

		transport = &mcp.StreamableClientTransport{
			Endpoint:   actualEndpoint,
			HTTPClient: httpClient,
			MaxRetries: 3,
		}

	case TransportTypeWebSocket:
		fallthrough
	default:
		// Use our custom WebSocket transport
		wsTransport := NewWebSocketTransport(serverURL)
		for key, value := range headers {
			wsTransport.WithHeader(key, value)
		}
		transport = wsTransport
	}

	// Connect to the server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &MCPClient{
		client:        client,
		session:       session,
		transport:     transport,
		serverURL:     originalServerURL, // Store the original URL
		headers:       headers,
		transportType: transportType,
	}, nil
}

// detectTransportType detects the transport type by making a test request to the server
func detectTransportType(serverURL string, headers map[string]string) (TransportType, error) {
	// Create a client with a short timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &headerTransport{
			base:    http.DefaultTransport,
			headers: headers,
		},
	}

	// Make a GET request to the base URL
	testURL := serverURL
	if !strings.HasSuffix(testURL, "/") {
		testURL += "/"
	}

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return TransportTypeWebSocket, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return TransportTypeWebSocket, err
	}
	defer resp.Body.Close()

	// Read a small portion of the response
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	responseText := strings.ToLower(string(buf[:n]))

	// Check if the response mentions websocket
	if strings.Contains(responseText, "websocket") {
		return TransportTypeWebSocket, nil
	}

	// Default to HTTP stream if not websocket
	return TransportTypeHTTPStream, nil
}

// reconnect attempts to reconnect to the server
func (c *MCPClient) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close the old session if it exists
	if c.session != nil {
		_ = c.session.Close()
		c.session = nil
	}

	// Create a new transport
	var transport mcp.Transport

	switch c.transportType {
	case TransportTypeHTTPStream:
		// Ensure URL ends with /mcp for HTTP stream transport
		serverURL := c.serverURL
		if !strings.HasSuffix(serverURL, "/mcp") {
			u, err := url.Parse(serverURL)
			if err != nil {
				return fmt.Errorf("failed to parse server URL: %w", err)
			}
			if !strings.Contains(u.Path, "/mcp") {
				if u.Path == "" || u.Path == "/" {
					u.Path = "/mcp"
				} else {
					u.Path = u.Path + "/mcp"
				}
				serverURL = u.String()
			}
		}

		// Create HTTP client with headers
		httpClient := &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: c.headers,
			},
		}

		transport = &mcp.StreamableClientTransport{
			Endpoint:   serverURL,
			HTTPClient: httpClient,
			MaxRetries: 3,
		}

	case TransportTypeWebSocket:
		fallthrough
	default:
		// Use our custom WebSocket transport
		wsTransport := NewWebSocketTransport(c.serverURL)
		for key, value := range c.headers {
			wsTransport.WithHeader(key, value)
		}
		transport = wsTransport
	}

	c.transport = transport

	// Create a new client
	impl := &mcp.Implementation{
		Name:    "mcp-client",
		Version: "1.0.0",
	}
	c.client = mcp.NewClient(impl, nil)

	// Connect to the server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to reconnect to %s: %w", c.serverURL, err)
	}
	c.session = session
	return nil
}

// isConnectionError checks if an error indicates a connection problem that should trigger reconnection
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())

	// Check for various connection error patterns
	errorPatterns := []string{
		"connection closed",
		"session not found",
		"use of closed network connection",
		"eof",
		"broken pipe",
		"hanging get",
		"failed to reconnect",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"no such host",
		"i/o timeout",
		"context deadline exceeded",
	}

	for _, pattern := range errorPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// ListTools returns the list of available tools from the server
func (c *MCPClient) ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		// Check if the error indicates a connection problem
		if isConnectionError(err) {
			// Try to reconnect once
			if reconnectErr := c.reconnect(); reconnectErr != nil {
				return nil, fmt.Errorf("failed to reconnect after connection error: %w", reconnectErr)
			}

			// Retry the operation with the new session
			c.mu.Lock()
			session = c.session
			c.mu.Unlock()
			result, err = session.ListTools(ctx, &mcp.ListToolsParams{})
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// CallTool calls a tool on the server
func (c *MCPClient) CallTool(ctx context.Context, toolName string, params any) (*mcp.CallToolResult, error) {
	// Convert params to arguments map
	var arguments map[string]any

	switch v := params.(type) {
	case map[string]interface{}:
		// map[string]interface{} and map[string]any are the same type in Go
		arguments = v
	default:
		// If params is already the correct type, use it directly
		if args, ok := params.(map[string]any); ok {
			arguments = args
		} else {
			// Fallback: wrap in a map
			arguments = map[string]any{
				"value": params,
			}
		}
	}

	c.mu.Lock()
	session := c.session
	c.mu.Unlock()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		// Check if the error indicates a connection problem
		if isConnectionError(err) {
			// Try to reconnect once
			if reconnectErr := c.reconnect(); reconnectErr != nil {
				return nil, fmt.Errorf("failed to reconnect after connection error: %w", reconnectErr)
			}

			// Retry the operation with the new session
			c.mu.Lock()
			session = c.session
			c.mu.Unlock()
			result, err = session.CallTool(ctx, &mcp.CallToolParams{
				Name:      toolName,
				Arguments: arguments,
			})
		}
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// Close closes the MCP client connection
func (c *MCPClient) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

// headerTransport is an http.RoundTripper that adds headers to requests
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
