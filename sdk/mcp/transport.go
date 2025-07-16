package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/agentuity/mcp-golang/v2/transport"
	"github.com/gorilla/websocket"
)

// WebSocketClientTransport provides a client transport implementation for MCP over WebSockets
type WebSocketClientTransport struct {
	serverURL      string
	conn           *websocket.Conn
	messageHandler func(ctx context.Context, message *transport.BaseJsonRpcMessage)
	closeHandler   func(ctx context.Context)
	errorHandler   func(error)
	mu             sync.RWMutex
	connected      bool
	ctx            context.Context
	cancel         context.CancelFunc
	headers        map[string]string
}

// NewWebSocketClientTransport creates a new WebSocket client transport
func NewWebSocketClientTransport(serverURL string) *WebSocketClientTransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketClientTransport{
		serverURL: serverURL,
		ctx:       ctx,
		cancel:    cancel,
		headers:   make(map[string]string),
	}
}

// WithHeader adds a header to the WebSocket handshake request
func (t *WebSocketClientTransport) WithHeader(key, value string) *WebSocketClientTransport {
	t.headers[key] = value
	return t
}

// Start connects to the WebSocket server and starts handling messages
func (t *WebSocketClientTransport) Start(ctx context.Context) error {
	u, err := url.Parse(t.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %v", err)
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
		return fmt.Errorf("failed to establish WebSocket connection: %v", err)
	}

	t.mu.Lock()
	t.conn = conn
	t.connected = true
	t.mu.Unlock()

	return nil
}

// Send sends a message to the WebSocket server
func (t *WebSocketClientTransport) Send(ctx context.Context, message *transport.BaseJsonRpcMessage) error {
	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	t.mu.RLock()
	conn := t.conn
	connected := t.connected
	t.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("not connected to server. Please establish a connection first")
	}

	// Marshal and send the message
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	var body []byte
	messageType, body, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read message: %v", err)
	}
	if messageType != websocket.TextMessage {
		return fmt.Errorf("unexpected message type: %v", messageType)
	}

	if len(body) > 0 {
		// Try to unmarshal as a response first
		var response transport.BaseJSONRPCResponse
		if err := json.Unmarshal(body, &response); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageResponse(&response))
			}
			return nil
		}

		// Try as an error
		var errorResponse transport.BaseJSONRPCError
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageError(&errorResponse))
			}
			return nil
		}

		// Try as a notification
		var notification transport.BaseJSONRPCNotification
		if err := json.Unmarshal(body, &notification); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageNotification(&notification))
			}
			return nil
		}

		// Try as a request
		var request transport.BaseJSONRPCRequest
		if err := json.Unmarshal(body, &request); err == nil {
			t.mu.RLock()
			handler := t.messageHandler
			t.mu.RUnlock()

			if handler != nil {
				handler(ctx, transport.NewBaseMessageRequest(&request))
			}
			return nil
		}

		return fmt.Errorf("received invalid response: %s", string(body))
	}

	return nil
}

// Close closes the WebSocket connection
func (t *WebSocketClientTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		// Send close message
		err := t.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			// Just log, we're closing anyway
			if t.errorHandler != nil {
				t.errorHandler(fmt.Errorf("error sending close message: %v", err))
			}
		}

		// Close the connection
		if err := t.conn.Close(); err != nil {
			return fmt.Errorf("error closing connection: %v", err)
		}

		t.conn = nil
		t.connected = false
	}

	// Cancel the context to stop any goroutines
	t.cancel()

	// Call close handler if set
	if t.closeHandler != nil {
		t.closeHandler(ctx)
	}

	return nil
}

// SetMessageHandler sets the callback for when a message is received
func (t *WebSocketClientTransport) SetMessageHandler(handler func(ctx context.Context, message *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.messageHandler = handler
}

// SetCloseHandler sets the callback for when the connection is closed
func (t *WebSocketClientTransport) SetCloseHandler(handler func(ctx context.Context)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closeHandler = handler
}

// SetErrorHandler sets the callback for when an error occurs
func (t *WebSocketClientTransport) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.errorHandler = handler
}
