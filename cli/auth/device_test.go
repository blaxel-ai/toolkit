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
	"regexp"
	"strings"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deviceTokenServer(t *testing.T, responses ...func(http.ResponseWriter)) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request DeviceLoginFinalizeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "urn:ietf:params:oauth:grant-type:device_code", request.GrantType)
		assert.Equal(t, "device-code", request.DeviceCode)
		index := calls
		if index >= len(responses) {
			index = len(responses) - 1
		}
		calls++
		responses[index](w)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func pending(w http.ResponseWriter) { w.WriteHeader(http.StatusAccepted) }

func pendingError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
}

func TestPollDeviceTokenWaitsForConfirmation(t *testing.T) {
	server, calls := deviceTokenServer(t, pending, pendingError, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`))
	})
	token, err := pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 10)
	require.NoError(t, err)
	assert.Equal(t, DeviceLoginFinalizeResponse{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, token)
	assert.Equal(t, 3, *calls)
}

func TestPollDeviceTokenReportsFailures(t *testing.T) {
	server, calls := deviceTokenServer(t, pending)
	_, err := pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 4)
	assert.ErrorContains(t, err, "timed out waiting for confirmation")
	assert.Equal(t, 4, *calls)

	server, _ = deviceTokenServer(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	})
	_, err = pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 4)
	assert.ErrorContains(t, err, "status 400")

	server, _ = deviceTokenServer(t, func(w http.ResponseWriter) { _, _ = w.Write([]byte("not json")) })
	_, err = pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 4)
	assert.ErrorContains(t, err, "unmarshalling")
}

func TestRequestDeviceLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request DeviceLogin
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, DeviceLogin{ClientID: "blaxel", Scope: "offline_access"}, request)
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"device_code":"device-code","verification_uri_complete":"https://app.blaxel.ai/device?code=1"}`))
	}))
	defer server.Close()
	response, err := requestDeviceLogin(context.Background(), server.URL+"/login/device")
	require.NoError(t, err)
	assert.Equal(t, "device-code", response.DeviceCode)

	_, err = requestDeviceLogin(context.Background(), server.URL+"/broken")
	assert.ErrorContains(t, err, "status 503", "an unusable answer stops the login instead of polling for nothing")
}

func TestRequestDeviceLoginStopsWhenCancelled(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	started := time.Now()
	_, err := requestDeviceLogin(ctx, server.URL)
	assert.ErrorIs(t, err, context.Canceled, "skipping the login stops a request that hangs")
	assert.Less(t, time.Since(started), 2*time.Second)
}

func TestPollDeviceTokenStopsWhenCancelled(t *testing.T) {
	server, calls := deviceTokenServer(t, pending)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	started := time.Now()
	_, err := pollDeviceToken(ctx, server.URL, "device-code", 10*time.Millisecond, 1000)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(started), time.Second)
	assert.Less(t, *calls, 10)
}

// useCurrentWorkspace runs a test in an empty home whose config has the
// workspace in use set to workspace (none when empty).
func useCurrentWorkspace(t *testing.T, workspace string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if workspace != "" {
		require.NoError(t, blaxel.SetCurrentWorkspace(workspace))
	}
}

func TestCurrentWorkspaceIndex(t *testing.T) {
	names := []string{"alpha", "beta", "gamma"}
	for current, want := range map[string]int{"beta": 1, "gamma": 2, "alpha": 0, "gone": -1, "": -1} {
		useCurrentWorkspace(t, current)
		assert.Equal(t, want, currentWorkspaceIndex(names), "current workspace %q", current)
	}
}

func TestAskWorkspaceStartsOnCurrentWorkspace(t *testing.T) {
	names := []string{"alpha", "beta", "gamma"}
	const enter, down = "\r", "\x1b[B"
	for _, tc := range []struct{ name, current, keys, want string }{
		{"Enter keeps the current workspace", "beta", enter, "beta"},
		{"arrows move from the current workspace", "beta", down + enter, "gamma"},
		{"a workspace the login cannot use keeps the list order", "gone", enter, "alpha"},
		{"no current workspace keeps the list order", "", enter, "alpha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCurrentWorkspace(t, tc.current)
			got, err := askWorkspace(names, strings.NewReader(tc.keys), io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

const deviceLoginURL = "https://app.example.com/device?code=1"

// loginServer answers a device login, the token request and the workspace
// calls of a login for an account with workspaces, without a browser.
func loginServer(t *testing.T, workspaces ...string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"device_code":"device-code","verification_uri_complete":%q}`, deviceLoginURL)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`))
	})
	mux.HandleFunc("/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		list := make([]map[string]string, 0, len(workspaces))
		for _, name := range workspaces {
			list = append(list, map[string]string{"name": name})
		}
		require.NoError(t, json.NewEncoder(w).Encode(list))
	})
	mux.HandleFunc("/workspaces/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/workspaces/")
		if !assert.Contains(t, workspaces, name) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprintf(w, `{"name":%q,"id":"id-%s"}`, name, name)
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	baseURL, interval, open, noColor := blaxel.GetBaseURL(), devicePollInterval, openBrowser, color.NoColor
	t.Cleanup(func() {
		blaxel.SetBaseURL(baseURL)
		devicePollInterval, openBrowser, color.NoColor = interval, open, noColor
	})
	// The SDK re-reads BL_API_URL whenever it builds a client.
	t.Setenv("BL_API_URL", server.URL)
	blaxel.ApplyEnvironmentOverrides()
	devicePollInterval = time.Millisecond
	openBrowser = func(string) error { return nil }
	color.NoColor = true
}

// captureStdout returns what fn prints.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	defer func() { os.Stdout = original }()
	fn()
	require.NoError(t, writer.Close())
	out, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(out)
}

func TestDescribeWait(t *testing.T) {
	assert.Equal(t, "3 minutes", describeWait(devicePollInterval*61))
	assert.Equal(t, "2 minutes", describeWait(2*time.Minute))
	assert.Equal(t, "30 seconds", describeWait(30*time.Second))
}

func TestLoginWithoutTerminalChoosesTheWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name       string
		workspaces []string
		current    string
		named      string
		want, note string
	}{
		{"the named workspace", []string{"a", "b", "c"}, "b", "c", "c", ""},
		{"the current workspace", []string{"c", "a", "b"}, "b", "", "b", "using workspace b (your current workspace)."},
		{"the first workspace by name", []string{"c", "a", "b"}, "gone", "", "a", "using workspace a (the first of your 3 workspaces by name)."},
		{"the only workspace", []string{"solo"}, "gone", "", "solo", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCurrentWorkspace(t, tc.current)
			loginServer(t, tc.workspaces...)
			var err error
			out := captureStdout(t, func() { err = loginWithDevice(tc.named, false) })
			require.NoError(t, err)

			current, err := blaxel.CurrentContext()
			require.NoError(t, err)
			assert.Equal(t, tc.want, current.Workspace)
			credentials, err := blaxel.LoadCredentials(tc.want)
			require.NoError(t, err)
			assert.Equal(t, "access", credentials.AccessToken)
			assert.Contains(t, out, "Successfully logged in to workspace "+tc.want)
			if tc.note == "" {
				assert.NotContains(t, out, "No terminal to ask in")
				return
			}
			assert.Contains(t, out, "No terminal to ask in, so "+tc.note)
			assert.Contains(t, out, "bl login <workspace>")
			assert.Contains(t, out, "bl workspaces <workspace>")
		})
	}
}

func TestDeviceLoginMessages(t *testing.T) {
	waiting := regexp.MustCompile(`(?m)^ℹ Waiting up to \d+ seconds for you to confirm the login in your browser\.\.\.$`)
	for _, tc := range []struct {
		name                string
		interactive, opened bool
		want                string
	}{
		{"no terminal, browser opened", false, true, "ℹ Opened the login page in your browser. If it did not open, visit this URL:\n" + deviceLoginURL + "\n"},
		{"no terminal, no browser", false, false, "ℹ Visit this URL to finish logging in:\n" + deviceLoginURL + "\n"},
		{"terminal, browser opened", true, true, "ℹ Opened URL in browser. If it's not working, please open it manually: " + deviceLoginURL + "\nℹ Waiting for you to confirm the login in your browser...\n"},
		{"terminal, no browser", true, false, "ℹ Please visit the following URL to finish logging in: " + deviceLoginURL + "\nℹ Waiting for you to confirm the login in your browser...\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useCurrentWorkspace(t, "")
			loginServer(t, "solo")
			openBrowser = func(string) error {
				if tc.opened {
					return nil
				}
				return errors.New("no browser")
			}
			var err error
			out := captureStdout(t, func() { err = loginWithDevice("", tc.interactive) })
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(out, tc.want), "output starts with %q, got %q", tc.want, out)
			assert.Equal(t, !tc.interactive, waiting.MatchString(out), "only without a terminal does it say how long it waits")
		})
	}
}
