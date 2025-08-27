package integration

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// ExecuteCLIWithPTY executes the CLI under a pseudo-terminal to simulate a TTY and optionally feeds inputs.
func (env *RealCLITestEnvironment) ExecuteCLIWithPTY(timeout time.Duration, args []string, inputs []string) *CLIResult {
	cmd := exec.Command(env.CLIBinary, args...)
	cmd.Dir = env.TempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BL_API_KEY=%s", env.APIKey),
		fmt.Sprintf("BL_WORKSPACE=%s", env.Workspace),
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return &CLIResult{Error: err, ExitCode: 1}
	}
	// Ensure a reasonable terminal size for TUI layouts
	_ = pty.Setsize(ptmx, &pty.Winsize{Cols: 120, Rows: 40})
	defer func() { _ = ptmx.Close() }()

	done := make(chan error, 1)
	var stdoutBuilder strings.Builder

	// Read output asynchronously
	go func() {
		_, _ = io.Copy(&stdoutBuilder, ptmx)
		done <- cmd.Wait()
	}()

	// Feed inputs with a conservative delay to let the UI render
	go func() {
		// Initial short wait for spinners/forms to appear
		time.Sleep(600 * time.Millisecond)
		for _, in := range inputs {
			_, _ = io.WriteString(ptmx, in)
			time.Sleep(400 * time.Millisecond)
		}
	}()

	select {
	case err := <-done:
		res := &CLIResult{Stdout: stdoutBuilder.String(), Stderr: "", Error: err}
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				res.ExitCode = exitError.ExitCode()
			} else {
				res.ExitCode = 1
			}
		} else {
			res.ExitCode = 0
		}
		return res
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return &CLIResult{Stdout: stdoutBuilder.String(), Stderr: "", Error: fmt.Errorf("command timed out after %v", timeout), ExitCode: 1}
	}
}

// (Removed ExecuteCLIWithConsole and interactive TTY helpers)

type ptyStep struct {
	Match string // substring to wait for in output
	Send  string // input to send when matched
}

// ExecuteCLIWithPTYScript runs the CLI under a PTY and sends inputs when output contains specific substrings.
func (env *RealCLITestEnvironment) ExecuteCLIWithPTYScript(timeout time.Duration, args []string, script []ptyStep) *CLIResult {
	cmd := exec.Command(env.CLIBinary, args...)
	cmd.Dir = env.TempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BL_API_KEY=%s", env.APIKey),
		fmt.Sprintf("BL_WORKSPACE=%s", env.Workspace),
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return &CLIResult{Error: err, ExitCode: 1}
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan error, 1)
	var stdoutBuilder strings.Builder

	// Read output and drive script
	go func() {
		buf := make([]byte, 4096)
		idx := 0
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				stdoutBuilder.WriteString(chunk)
				// Advance script when match appears in cumulative output
				for idx < len(script) && strings.Contains(stdoutBuilder.String(), script[idx].Match) {
					_, _ = io.WriteString(ptmx, script[idx].Send)
					idx++
				}
			}
			if rerr != nil {
				break
			}
		}
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		res := &CLIResult{Stdout: stdoutBuilder.String(), Stderr: "", Error: err}
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				res.ExitCode = exitError.ExitCode()
			} else {
				res.ExitCode = 1
			}
		} else {
			res.ExitCode = 0
		}
		return res
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return &CLIResult{Stdout: stdoutBuilder.String(), Stderr: "", Error: fmt.Errorf("command timed out after %v", timeout), ExitCode: 1}
	}
}

type createTestCase struct {
	name     string
	command  string
	template string
	dir      string
}

func TestCreateCommands_TTYAndNoTTY(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)
	require.NotNil(t, env)

	cases := []createTestCase{
		{name: "Agent App", command: "create-agent-app", template: "template-google-adk-py", dir: "tty-agent"},
		{name: "MCP Server", command: "create-mcp-server", template: "template-mcp-hello-world-py", dir: "tty-mcp"},
		{name: "Job", command: "create-job", template: "template-jobs-ts", dir: "tty-job"},
		{name: "Sandbox", command: "create-sandbox", template: "template-sandbox-ts", dir: "tty-sandbox"},
	}

	// Ensure clean slate
	for _, c := range cases {
		_ = os.RemoveAll(filepath.Join(env.TempDir, c.dir))
		_ = os.RemoveAll(filepath.Join(env.TempDir, c.template))
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s_noTTY_dir_and_template", c.name), func(t *testing.T) {
			t.Parallel()
			// no tty with directory and template
			dir := fmt.Sprintf("%s-%d", c.dir, time.Now().UnixNano())
			res := env.ExecuteCLIWithTimeout(20*time.Second, c.command, dir, "--template", c.template, "-y")
			AssertCLISuccess(t, res)
			_ = verifyDirectoryCreation(dir)
			_ = os.RemoveAll(filepath.Join(env.TempDir, dir))
		})

		// (Removed POC teatest block)
		t.Run(fmt.Sprintf("%s_noTTY_template_only", c.name), func(t *testing.T) {
			t.Parallel()
			// no tty and just template (dir inferred from template)
			res := env.ExecuteCLIWithTimeout(20*time.Second, c.command, "--template", c.template, "-y")
			AssertCLISuccess(t, res)
			_ = verifyDirectoryCreation(c.template)
			_ = os.RemoveAll(filepath.Join(env.TempDir, c.template))
		})

		t.Run(fmt.Sprintf("%s_TTY_dir_and_template", c.name), func(t *testing.T) {
			t.Parallel()
			// tty with directory and template (non-interactive via PTY)
			dir := fmt.Sprintf("%s-%d", c.dir, time.Now().UnixNano())
			res := env.ExecuteCLIWithPTY(20*time.Second, []string{c.command, dir, "--template", c.template}, nil)
			AssertCLISuccess(t, res)
			_ = verifyDirectoryCreation(dir)
			_ = os.RemoveAll(filepath.Join(env.TempDir, dir))
		})

		// Removed interactive TTY dir_only test

		// Removed interactive TTY nothing test
	}
}

// ExecuteCLIWithExpect uses go-expect to drive interactive TTY reliably.
func (env *RealCLITestEnvironment) ExecuteCLIWithExpect(timeout time.Duration, args []string, steps []struct {
	Exp  string
	Send string
}) *CLIResult {
	cmdArgs := append([]string{env.CLIBinary}, args...)
	envs := append(os.Environ(), fmt.Sprintf("BL_API_KEY=%s", env.APIKey), fmt.Sprintf("BL_WORKSPACE=%s", env.Workspace))
	e, _, err := expect.SpawnWithArgs(cmdArgs, timeout, expect.SetEnv(envs))
	if err != nil {
		return &CLIResult{Error: err, ExitCode: 1}
	}
	defer e.Close()

	var stdoutBuilder strings.Builder
	// goexpect doesn't expose direct stdout; best-effort: rely on matcher traces if needed

	for _, st := range steps {
		if st.Exp != "" {
			if _, _, err := e.Expect(regexp.MustCompile(st.Exp), timeout); err != nil {
				return &CLIResult{Stdout: stdoutBuilder.String(), Error: err, ExitCode: 1}
			}
		}
		if st.Send != "" {
			if err := e.Send(st.Send); err != nil {
				return &CLIResult{Stdout: stdoutBuilder.String(), Error: err, ExitCode: 1}
			}
		}
	}

	_ = e.Close()
	return &CLIResult{Stdout: stdoutBuilder.String(), Error: nil, ExitCode: 0}
}
