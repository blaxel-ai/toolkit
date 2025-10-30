package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WebSocketTransport implements the official MCP SDK Transport interface for WebSocket connections
type WebSocketTransport struct {
	serverURL string
	headers   map[string]string
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(serverURL string) *WebSocketTransport {
	// Convert http/https to ws/wss
	wsURL := serverURL
	if len(wsURL) > 4 {
		if wsURL[:4] == "http" {
			wsURL = "ws" + wsURL[4:]
		}
	}

	return &WebSocketTransport{
		serverURL: wsURL,
		headers:   make(map[string]string),
	}
}

// WithHeader adds a header to the WebSocket handshake
func (t *WebSocketTransport) WithHeader(key, value string) *WebSocketTransport {
	t.headers[key] = value
	return t
}

// Connect implements the Transport interface from the official MCP SDK
func (t *WebSocketTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	u, err := url.Parse(t.serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %v", err)
	}

	// Connect to the WebSocket server
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}

	// Add headers to the handshake request
	requestHeader := http.Header{}
	for key, value := range t.headers {
		requestHeader.Add(key, value)
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), requestHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to establish WebSocket connection: %v", err)
	}

	return &webSocketConnection{
		conn:      conn,
		sessionID: uuid.New().String(),
		readCh:    make(chan jsonrpc.Message, 10),
		errCh:     make(chan error, 1),
		closeCh:   make(chan struct{}),
		closeOnce: sync.Once{},
	}, nil
}

// webSocketConnection implements the mcp.Connection interface for WebSocket
type webSocketConnection struct {
	conn      *websocket.Conn
	sessionID string
	mu        sync.RWMutex
	closed    bool
	readCh    chan jsonrpc.Message
	errCh     chan error
	closeCh   chan struct{}
	closeOnce sync.Once
	readOnce  sync.Once
}

// startReader starts the WebSocket reader goroutine
func (c *webSocketConnection) startReader() {
	go func() {
		defer close(c.readCh)
		defer close(c.errCh)

		for {
			select {
			case <-c.closeCh:
				return
			default:
				messageType, data, err := c.conn.ReadMessage()
				if err != nil {
					c.mu.RLock()
					closed := c.closed
					c.mu.RUnlock()

					if !closed {
						c.errCh <- err
					}
					return
				}

				if messageType != websocket.TextMessage {
					continue
				}

				// Parse the JSON-RPC message using the official decoder
				msg, err := jsonrpc.DecodeMessage(data)
				if err != nil {
					// Try to send error if channel has space
					select {
					case c.errCh <- fmt.Errorf("failed to decode message: %v", err):
					default:
					}
					continue
				}

				select {
				case c.readCh <- msg:
				case <-c.closeCh:
					return
				}
			}
		}
	}()
}

// Read reads the next message from the WebSocket connection
func (c *webSocketConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	// Start the reader goroutine on first Read
	c.readOnce.Do(func() {
		c.startReader()
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-c.errCh:
		return nil, err
	case msg, ok := <-c.readCh:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}
		return msg, nil
	case <-c.closeCh:
		return nil, fmt.Errorf("connection closed")
	}
}

// Write writes a message to the WebSocket connection
func (c *webSocketConnection) Write(ctx context.Context, msg jsonrpc.Message) error {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return fmt.Errorf("connection closed")
	}

	// Use the official encoder for messages
	data, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}

	// Set write deadline from context
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetWriteDeadline(deadline); err != nil {
			return err
		}
		defer c.conn.SetWriteDeadline(time.Time{})
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	return nil
}

// Close closes the WebSocket connection
func (c *webSocketConnection) Close() error {
	var err error

	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()

		close(c.closeCh)

		// Send close message
		closeErr := c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if closeErr != nil {
			err = closeErr
		}

		// Close the connection
		if closeErr := c.conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	})

	return err
}

// SessionID returns the session ID
func (c *webSocketConnection) SessionID() string {
	return c.sessionID
}
