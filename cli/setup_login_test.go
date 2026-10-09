package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/ui"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A child test process gives the real setup screens a controlling terminal
// without changing the test runner's input or signal handlers.
func TestSetupDeviceLoginTerminal(t *testing.T) {
	if mode := os.Getenv("TEST_SETUP_DEVICE_LOGIN"); mode != "" {
		blaxel.ApplyEnvironmentOverrides()
		ctx := context.Background()
		if mode == "deadline" {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 7*time.Second)
			defer cancel()
		}
		setup := &ui.Setup{
			Tasks: func(map[string]bool) []ui.Task {
				return []ui.Task{{ID: "login", Label: "Log in", Skippable: true, Run: func(ctx context.Context, c *ui.Control) (string, error) {
					return setupDeviceLogin(ctx, c, os.Getenv("TEST_SETUP_WORKSPACE"))
				}}}
			},
			Summary: func(results map[string]ui.Result) ui.Summary {
				result := results["login"]
				return ui.Summary{Title: fmt.Sprintf("LOGIN_RESULT detail=%s error=%v", result.Detail, result.Err)}
			},
		}
		_, err := setup.Run(ctx, ui.Options{Out: os.Stdout, Yes: true})
		require.NoError(t, err)
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("PTY fixture requires a Unix terminal")
	}
	for _, tc := range []struct {
		name, mode, keys, want, named string
		browserFails                  bool
	}{
		{name: "single workspace", mode: "single", want: "solo"},
		{name: "multiple workspace picker", mode: "multiple", keys: "\x1b[B\r", want: "beta"},
		{name: "browser failure", mode: "single", browserFails: true, want: "solo"},
		{name: "list failure", mode: "failure"},
		{name: "skip cancels request", mode: "hang", keys: "\x1b"},
		{name: "deadline cancels request", mode: "deadline"},
		{name: "explicit workspace skips wait", mode: "single", named: "named", want: "named"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("BL_API_KEY", "")
			t.Setenv("BL_CLIENT_CREDENTIALS", "")
			require.NoError(t, os.Unsetenv("BL_API_KEY"))
			require.NoError(t, os.Unsetenv("BL_CLIENT_CREDENTIALS"))
			t.Setenv("BL_WORKSPACE", "")
			t.Setenv("BL_APP_URL", "https://console.example.com")
			t.Setenv("TEST_SETUP_DEVICE_LOGIN", tc.mode)
			t.Setenv("TEST_SETUP_WORKSPACE", tc.named)
			t.Setenv("TERM", "xterm-256color")
			t.Setenv("NO_COLOR", "")
			openedFile := filepath.Join(home, "opened")
			bin := filepath.Join(home, "bin")
			require.NoError(t, os.Mkdir(bin, 0755))
			script := "#!/bin/sh\nprintf '%s\\n' \"$1\" >> \"$HOME/opened\"\n"
			if tc.browserFails {
				script = "#!/missing-fixture-shell\n"
			}
			for _, name := range []string{"open", "wslview"} {
				require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(script), 0755))
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			var tokens, lists atomic.Int32
			inflight, cancelled := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/login/device":
					_, _ = io.WriteString(w, `{"device_code":"fixture-code","verification_uri_complete":"https://console.example.com/device"}`)
				case "/oauth/token":
					tokens.Add(1)
					_, _ = io.WriteString(w, `{"access_token":"fixture-access","refresh_token":"fixture-refresh","expires_in":3600}`)
				case "/workspaces":
					assert.Equal(t, "Bearer fixture-access", r.Header.Get("X-Blaxel-Authorization"))
					if lists.Add(1) == 1 {
						_, _ = io.WriteString(w, `[]`)
					} else if tc.mode == "hang" || tc.mode == "deadline" {
						close(inflight)
						<-r.Context().Done()
						close(cancelled)
					} else if tc.mode == "failure" {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = io.WriteString(w, `{"error":"fixture failure"}`)
					} else if tc.mode == "multiple" {
						_, _ = io.WriteString(w, `[{"name":"alpha"},{"name":"beta"}]`)
					} else {
						_, _ = io.WriteString(w, `[{"name":"solo"}]`)
					}
				default:
					_, _ = fmt.Fprintf(w, `{"name":%q}`, tc.want)
				}
			}))
			t.Cleanup(server.Close)
			t.Setenv("BL_API_URL", server.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSetupDeviceLoginTerminal$", "-test.count=1")
			terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 110, Rows: 42})
			require.NoError(t, err)
			defer func() { _ = terminal.Close() }()
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			var output strings.Builder
			var mu sync.Mutex
			readDone := make(chan struct{})
			go func() {
				defer close(readDone)
				buffer := make([]byte, 4096)
				for {
					n, err := terminal.Read(buffer)
					mu.Lock()
					output.Write(buffer[:n])
					mu.Unlock()
					if err != nil {
						return
					}
					if strings.Contains(string(buffer[:n]), "\x1b]11;?") {
						_, _ = io.WriteString(terminal, "\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R")
					}
				}
			}()
			if tc.keys != "" {
				require.Eventually(t, func() bool {
					if tc.mode == "hang" {
						select {
						case <-inflight:
							return true
						default:
							return false
						}
					}
					mu.Lock()
					defer mu.Unlock()
					return strings.Contains(output.String(), "Choose a workspace")
				}, 10*time.Second, 10*time.Millisecond)
				_, err = io.WriteString(terminal, tc.keys)
				require.NoError(t, err)
			}
			require.NoError(t, cmd.Wait())
			<-readDone
			text := strings.Join(strings.Fields(ansi.Strip(output.String())), " ")
			assert.Equal(t, int32(1), tokens.Load())
			opened, err := os.ReadFile(openedFile)
			if tc.browserFails {
				assert.ErrorIs(t, err, os.ErrNotExist)
			} else {
				require.NoError(t, err)
			}
			cfg, err := blaxel.LoadConfig()
			require.NoError(t, err)
			if tc.named != "" {
				assert.Zero(t, lists.Load())
				assert.Equal(t, "https://console.example.com/device\n", string(opened))
			} else {
				assert.Equal(t, int32(2), lists.Load())
				if !tc.browserFails {
					assert.Equal(t, "https://console.example.com/device\nhttps://console.example.com\n", string(opened))
				}
				assert.True(t, strings.Contains(text, "Create or join a workspace"), "setup did not show the create/join guidance")
				assert.Contains(t, text, "https://console.example.com")
			}
			if tc.want != "" {
				assert.Equal(t, tc.want, cfg.Context.Workspace)
				creds, err := blaxel.LoadCredentials(tc.want)
				require.NoError(t, err)
				assert.Equal(t, "fixture-access", creds.AccessToken)
			} else {
				assert.Empty(t, cfg.Workspaces)
				if tc.mode == "failure" {
					assert.Contains(t, text, "failed to list workspaces")
					return
				}
				select {
				case <-cancelled:
				case <-time.After(time.Second):
					t.Fatal("setup left its workspace request running")
				}
				if tc.mode == "hang" {
					assert.Contains(t, text, "error=skipped")
				}
			}
		})
	}
}
