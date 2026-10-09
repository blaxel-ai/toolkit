package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workspaceLoginFixture struct {
	tokens, lists, gets atomic.Int32
	opened              []string
}

func workspaceLoginServer(t *testing.T, list func(http.ResponseWriter, *http.Request, int32)) *workspaceLoginFixture {
	t.Helper()
	fixture := &workspaceLoginFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/login/device":
			_, _ = fmt.Fprintf(w, `{"device_code":"device-code","verification_uri_complete":%q}`, deviceLoginURL)
		case r.URL.Path == "/oauth/token":
			fixture.tokens.Add(1)
			_, _ = w.Write([]byte(`{"access_token":"issued-access","refresh_token":"issued-refresh","expires_in":3600}`))
		case r.URL.Path == "/workspaces":
			assert.Equal(t, "Bearer issued-access", r.Header.Get("X-Blaxel-Authorization"))
			list(w, r, fixture.lists.Add(1))
		case strings.HasPrefix(r.URL.Path, "/workspaces/"):
			fixture.gets.Add(1)
			_, _ = fmt.Fprintf(w, `{"name":%q}`, strings.TrimPrefix(r.URL.Path, "/workspaces/"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	baseURL, appURL := blaxel.GetBaseURL(), blaxel.GetAppURL()
	interval, waitInterval, timeout, open, noColor := devicePollInterval, workspacePollInterval, workspaceWaitTimeout, openBrowser, color.NoColor
	t.Cleanup(func() {
		blaxel.SetBaseURL(baseURL)
		blaxel.SetAppURL(appURL)
		devicePollInterval, workspacePollInterval, workspaceWaitTimeout, openBrowser, color.NoColor = interval, waitInterval, timeout, open, noColor
	})
	t.Setenv("BL_API_URL", server.URL)
	t.Setenv("BL_APP_URL", "https://console.example.com")
	t.Setenv("BL_API_KEY", "")
	t.Setenv("BL_CLIENT_CREDENTIALS", "")
	require.NoError(t, os.Unsetenv("BL_API_KEY"))
	require.NoError(t, os.Unsetenv("BL_CLIENT_CREDENTIALS"))
	blaxel.ApplyEnvironmentOverrides()
	devicePollInterval, workspacePollInterval = time.Millisecond, time.Millisecond
	openBrowser = func(url string) error { fixture.opened = append(fixture.opened, url); return nil }
	color.NoColor = true
	return fixture
}

func TestLoginWaitsForFirstWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name, current, named, want string
		interactive, browserFails  bool
		names                      []string
	}{
		{name: "terminal single workspace", interactive: true, names: []string{"solo"}, want: "solo"},
		{name: "no terminal multiple workspaces", names: []string{"beta", "alpha"}, want: "alpha"},
		{name: "no terminal keeps current workspace", current: "beta", names: []string{"alpha", "beta"}, want: "beta"},
		{name: "browser failure still waits", browserFails: true, names: []string{"solo"}, want: "solo"},
		{name: "explicit workspace skips listing", named: "named", want: "named"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCurrentWorkspace(t, tc.current)
			fixture := workspaceLoginServer(t, func(w http.ResponseWriter, _ *http.Request, call int32) {
				names := []map[string]string{}
				if call > 2 {
					for _, name := range tc.names {
						names = append(names, map[string]string{"name": name})
					}
				}
				require.NoError(t, json.NewEncoder(w).Encode(names))
			})
			if tc.browserFails {
				openBrowser = func(url string) error {
					fixture.opened = append(fixture.opened, url)
					return errors.New("no browser")
				}
			}
			var err error
			out := captureStdout(t, func() { err = loginWithDevice(context.Background(), tc.named, tc.interactive) })
			require.NoError(t, err)
			assert.Equal(t, int32(1), fixture.tokens.Load())
			assert.Equal(t, int32(2), fixture.gets.Load())
			creds, err := blaxel.LoadCredentials(tc.want)
			require.NoError(t, err)
			assert.Equal(t, "issued-access", creds.AccessToken)
			assert.Equal(t, "issued-refresh", creds.RefreshToken)
			cfg, err := blaxel.LoadConfig()
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Context.Workspace)
			if tc.named != "" {
				assert.Zero(t, fixture.lists.Load())
				assert.Equal(t, []string{deviceLoginURL}, fixture.opened)
				assert.NotContains(t, out, "Create or join")
				return
			}
			assert.Equal(t, int32(3), fixture.lists.Load())
			assert.Equal(t, []string{deviceLoginURL, blaxel.GetAppURL()}, fixture.opened)
			assert.Contains(t, out, "Create or join a workspace")
			assert.Contains(t, out, "\n"+blaxel.GetAppURL()+"\n")
			assert.Contains(t, out, "Waiting up to 5 minutes")
		})
	}
}

func TestLoginWorkspaceListFailureDoesNotWaitOrSave(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		emptyFirst bool
	}{
		{"authentication failure", http.StatusUnauthorized, false},
		{"API failure after empty list", http.StatusBadRequest, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCurrentWorkspace(t, "")
			fixture := workspaceLoginServer(t, func(w http.ResponseWriter, _ *http.Request, call int32) {
				if tc.emptyFirst && call == 1 {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":"fixture failure"}`))
			})
			err := loginWithDevice(context.Background(), "", false)
			assert.ErrorContains(t, err, "failed to list workspaces")
			assert.NotContains(t, err.Error(), "timed out")
			want := int32(1)
			if tc.emptyFirst {
				want++
			}
			assert.Equal(t, want, fixture.lists.Load())
			assert.Len(t, fixture.opened, int(want))
			assertNoDeviceLoginSaved(t)
		})
	}
}

func assertNoDeviceLoginSaved(t *testing.T) {
	t.Helper()
	_, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".blaxel", "config.yaml"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoginWorkspaceWaitStopsWithoutSaving(t *testing.T) {
	for _, tc := range []struct {
		name                string
		cancel, hangRequest bool
	}{
		{"cancel in flight", true, true},
		{"deadline in flight", false, true},
		{"deadline between requests", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCurrentWorkspace(t, "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			requestCancelled := make(chan struct{})
			fixture := workspaceLoginServer(t, func(w http.ResponseWriter, r *http.Request, call int32) {
				if tc.hangRequest && call == 2 {
					if tc.cancel {
						cancel()
					}
					<-r.Context().Done()
					close(requestCancelled)
					return
				}
				_, _ = w.Write([]byte(`[]`))
			})
			workspaceWaitTimeout = 100 * time.Millisecond
			if !tc.hangRequest {
				workspacePollInterval = time.Second
			}
			err := loginWithDevice(ctx, "", false)
			want := context.DeadlineExceeded
			if tc.cancel {
				want = context.Canceled
			}
			assert.ErrorIs(t, err, want)
			assert.ErrorContains(t, err, "then run bl login again")
			if tc.hangRequest {
				select {
				case <-requestCancelled:
				case <-time.After(time.Second):
					t.Fatal("workspace HTTP request was not cancelled")
				}
			}
			assert.Equal(t, int32(1), fixture.tokens.Load())
			assert.Equal(t, []string{deviceLoginURL, blaxel.GetAppURL()}, fixture.opened)
			assert.Zero(t, fixture.gets.Load())
			assertNoDeviceLoginSaved(t)
		})
	}
}

func TestLoginWorkspacesStillRejectsEmptyListImmediately(t *testing.T) {
	fixture := workspaceLoginServer(t, func(w http.ResponseWriter, _ *http.Request, _ int32) { _, _ = w.Write([]byte(`[]`)) })
	_, err := LoginWorkspaces(blaxel.Credentials{AccessToken: "issued-access"})
	assert.ErrorContains(t, err, "no workspaces are available")
	assert.Equal(t, int32(1), fixture.lists.Load())
	assert.Empty(t, fixture.opened)
}

func TestWorkspacePickerAfterWaiting(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	useCurrentWorkspace(t, "beta")
	workspaceLoginServer(t, func(w http.ResponseWriter, _ *http.Request, call int32) {
		if call == 1 {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"name":"alpha"},{"name":"beta"},{"name":"gamma"}]`))
	})
	names, err := WaitForLoginWorkspaces(context.Background(), blaxel.Credentials{AccessToken: "issued-access"}, func(string) {})
	require.NoError(t, err)
	workspace, err := askWorkspace(names, strings.NewReader("\x1b[B\r"), io.Discard)
	require.NoError(t, err)
	assert.Equal(t, "gamma", workspace)
}
