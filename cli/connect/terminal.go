package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

// TerminalMessage represents a message to/from the terminal websocket
type TerminalMessage struct {
	Type string `json:"type"` // "input", "output", "resize", "error"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// TerminalClient manages the websocket connection to a remote terminal
type TerminalClient struct {
	conn       *websocket.Conn
	mu         sync.Mutex
	done       chan struct{}
	closeOnce  sync.Once
	oldState   *term.State
	stateMu    sync.Mutex // Protects oldState access
	stdin      int
	stdout     int
	closedChan chan struct{} // Signals that Close() has completed
}

// NewTerminalClient creates a new terminal client and connects to the remote terminal
func NewTerminalClient(sandboxURL, token string) (*TerminalClient, error) {
	// Build websocket URL
	wsURL, err := buildWebSocketURL(sandboxURL, token)
	if err != nil {
		return nil, fmt.Errorf("failed to build websocket URL: %w", err)
	}

	// Get initial terminal size
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Default size if we can't get terminal size
		cols, rows = 80, 24
	}

	// Add size to URL
	wsURL = fmt.Sprintf("%s&cols=%d&rows=%d", wsURL, cols, rows)

	// Connect to websocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to terminal: %w", err)
	}

	return &TerminalClient{
		conn:       conn,
		done:       make(chan struct{}),
		stdin:      int(os.Stdin.Fd()),
		stdout:     int(os.Stdout.Fd()),
		closedChan: make(chan struct{}),
	}, nil
}

// buildWebSocketURL converts the sandbox HTTP URL to a websocket URL
func buildWebSocketURL(sandboxURL, token string) (string, error) {
	u, err := url.Parse(sandboxURL)
	if err != nil {
		return "", err
	}

	// Convert http(s) to ws(s)
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}

	// Set the path to the terminal websocket endpoint
	u.Path = strings.TrimSuffix(u.Path, "/") + "/terminal/ws"

	// Add token as query parameter
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// Run starts the terminal session
// This blocks until the session ends (user exits or connection closes)
func (t *TerminalClient) Run(ctx context.Context) error {
	// Check if stdin is a terminal
	if !term.IsTerminal(t.stdin) {
		return fmt.Errorf("stdin is not a terminal")
	}

	// Put terminal in raw mode
	oldState, err := term.MakeRaw(t.stdin)
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	t.stateMu.Lock()
	t.oldState = oldState
	t.stateMu.Unlock()

	// Ensure we restore terminal state on exit
	defer t.restoreTerminal()

	// Handle Ctrl+C and other signals to restore terminal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		select {
		case <-sigChan:
			t.Close()
		case <-t.done:
		}
		signal.Stop(sigChan)
	}()

	// Handle terminal resize (SIGWINCH)
	t.setupResizeHandler()

	// Start goroutine to read from websocket and write to stdout
	go t.readLoop()

	// Start goroutine to read from stdin and write to websocket
	go t.writeLoop(ctx)

	// Wait for done signal
	<-t.done

	return nil
}

// readLoop reads messages from the websocket and writes output to stdout
func (t *TerminalClient) readLoop() {
	defer t.Close() // Close when connection ends (e.g., remote shell exits)

	for {
		_, message, err := t.conn.ReadMessage()
		if err != nil {
			// Connection closed or error - exit
			return
		}

		var msg TerminalMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "output":
			// Write directly to stdout
			os.Stdout.WriteString(msg.Data)
		case "error":
			// Write error in red
			os.Stdout.WriteString("\r\n\x1b[31mError: " + msg.Data + "\x1b[0m\r\n")
		}
	}
}

// writeLoop reads from stdin and sends input to the websocket
func (t *TerminalClient) writeLoop(ctx context.Context) {
	defer t.Close()

	buf := make([]byte, 1024)
	for {
		select {
		case <-t.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		n, err := os.Stdin.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Read error
			}
			return
		}

		if n > 0 {
			// Check for Ctrl+D (EOT, 0x04) - local exit shortcut
			for i := 0; i < n; i++ {
				if buf[i] == 0x04 {
					return // This will trigger Close() via defer
				}
			}

			// Send input to remote
			msg := TerminalMessage{
				Type: "input",
				Data: string(buf[:n]),
			}

			t.mu.Lock()
			err := t.conn.WriteJSON(msg)
			t.mu.Unlock()

			if err != nil {
				return
			}
		}
	}
}

// setupResizeHandler sets up handling for terminal resize events
func (t *TerminalClient) setupResizeHandler() {
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)

	go func() {
		for {
			select {
			case <-t.done:
				signal.Stop(sigwinch)
				return
			case <-sigwinch:
				t.sendResize()
			}
		}
	}()
}

// sendResize sends the current terminal size to the remote terminal
func (t *TerminalClient) sendResize() {
	cols, rows, err := term.GetSize(t.stdout)
	if err != nil {
		return
	}

	msg := TerminalMessage{
		Type: "resize",
		Cols: cols,
		Rows: rows,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	_ = t.conn.WriteJSON(msg)
}

// restoreTerminal restores the terminal to its original state
func (t *TerminalClient) restoreTerminal() {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.oldState != nil {
		term.Restore(t.stdin, t.oldState)
		t.oldState = nil
	}
}

// Close closes the terminal connection and cleans up
func (t *TerminalClient) Close() {
	t.closeOnce.Do(func() {
		// Restore terminal FIRST so output is visible
		t.restoreTerminal()

		// Close the websocket
		if t.conn != nil {
			_ = t.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			t.conn.Close()
		}

		// Signal done to unblock Run()
		close(t.done)

		// Signal that Close() has completed
		close(t.closedChan)

		// Note: os.Stdin.Read() in writeLoop cannot be interrupted in Go.
		// The goroutine will remain blocked until the process exits, which is
		// acceptable since it will be cleaned up when the process terminates.
		// We do NOT call os.Exit() here to allow proper cleanup by the caller.
	})
}

// Done returns a channel that is closed when Close() has completed
func (t *TerminalClient) Done() <-chan struct{} {
	return t.closedChan
}
