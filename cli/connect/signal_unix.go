//go:build !windows

package connect

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// getInterruptSignals returns the signals to listen for graceful shutdown
func getInterruptSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}
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
