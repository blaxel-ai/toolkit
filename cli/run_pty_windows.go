//go:build windows

package cli

import (
	"io"
	"os"
	"os/exec"
)

// startCmdWithOutput starts the command with stdout and stderr merged into a
// single pipe. On Windows, PTY is not available so we fall back to os.Pipe.
func startCmdWithOutput(cmd *exec.Cmd) (io.ReadCloser, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, err
	}
	// Close the write end in the parent process. The child process holds its
	// own file descriptor, so writes continue until the child exits. Once the
	// child exits the reader will get EOF.
	pw.Close()
	return pr, nil
}
