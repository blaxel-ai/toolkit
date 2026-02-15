//go:build !windows

package cli

import (
	"io"
	"os/exec"

	"github.com/creack/pty"
)

// startCmdWithOutput starts the command under a pseudo-terminal (PTY) so that
// child processes think they are writing to a TTY and use line-buffered output.
// Returns an io.ReadCloser that streams combined stdout+stderr.
func startCmdWithOutput(cmd *exec.Cmd) (io.ReadCloser, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return ptmx, nil
}
