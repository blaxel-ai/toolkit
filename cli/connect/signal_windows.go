//go:build windows

package connect

import (
	"os"
	"syscall"

	"golang.org/x/term"
)

// getInterruptSignals returns the signals to listen for graceful shutdown
func getInterruptSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

// setupResizeHandler sets up handling for terminal resize events
// On Windows, SIGWINCH doesn't exist, so we use a polling approach
func (t *TerminalClient) setupResizeHandler() {
	// Send initial size
	t.sendResize()

	// Windows doesn't have SIGWINCH, so we don't set up a resize handler
	// The terminal size is sent on initial connection
	// For a more robust solution, we could poll for size changes,
	// but that adds complexity for minimal benefit in most use cases
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
