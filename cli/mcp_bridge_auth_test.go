package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bridgeJWT(issued, expires time.Time) string {
	payload, _ := json.Marshal(map[string]int64{"iat": issued.Unix(), "exp": expires.Unix()})
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

type bridgeRoundTripFunc func(*http.Request) (*http.Response, error)

func (f bridgeRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func tokenResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

// testBridgeAuth reads the given workspaces and sends token requests to
// transport, with the origin pinned to a stored prod environment.
func testBridgeAuth(transport http.RoundTripper, workspaces ...blaxel.WorkspaceConfig) *bridgeAuth {
	a := newBridgeAuth("main")
	a.env = func(string) string { return "" }
	a.loadConfig = func() (blaxel.Config, error) { return blaxel.Config{Workspaces: workspaces}, nil }
	a.client = &http.Client{Transport: transport}
	return a
}

func mainWorkspace(c blaxel.Credentials) blaxel.WorkspaceConfig {
	return blaxel.WorkspaceConfig{Name: "main", Credentials: c}
}

// bridgeTestAuth is testBridgeAuth against a test server.
func bridgeTestAuth(t *testing.T, server *httptest.Server, c blaxel.Credentials) *bridgeAuth {
	t.Helper()
	a := testBridgeAuth(server.Client().Transport, mainWorkspace(c))
	a.client, a.baseURL, a.pinnedEnv = server.Client(), server.URL+"/v0", "prod"
	return a
}

func TestBridgeAuthRefreshesInMemoryAndKeepsTheWorkspace(t *testing.T) {
	now := time.Now()
	fresh := bridgeJWT(now, now.Add(2*time.Hour))
	var refreshes []map[string]string
	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		refreshes = append(refreshes, body)
		assert.Equal(t, "/v0/oauth/token", r.URL.Path)
		_, _ = fmt.Fprintf(w, `{"access_token":%q}`, fresh)
	}))
	defer tokens.Close()
	config := blaxel.Config{Context: blaxel.ContextConfig{Workspace: "main"}, Workspaces: []blaxel.WorkspaceConfig{
		mainWorkspace(blaxel.Credentials{AccessToken: bridgeJWT(now.Add(-2*time.Hour), now.Add(-time.Minute)), RefreshToken: "refresh-1", DeviceCode: "device-1"}),
		{Name: "other", Credentials: blaxel.Credentials{APIKey: "bl_other"}},
	}}
	a := bridgeTestAuth(t, tokens, blaxel.Credentials{})
	a.loadConfig = func() (blaxel.Config, error) { return config, nil }
	a.explicit, a.now = "", func() time.Time { return now }

	credentials, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", credentials.workspace)
	assert.Equal(t, tokens.URL+"/v0/mcp", credentials.endpoint)
	assert.Equal(t, "Bearer "+fresh, credentials.headers["Authorization"])
	assert.Equal(t, []map[string]string{{"grant_type": "refresh_token", "refresh_token": "refresh-1", "client_id": "blaxel", "device_code": "device-1"}}, refreshes)

	// The refreshed token is reused, and a later bl workspaces switch does not
	// move the agent's session.
	config.Context.Workspace = "other"
	credentials, err = a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", credentials.workspace)
	assert.Len(t, refreshes, 1)

	// A refused access token forces a refresh, not a logout.
	a.reject(credentials)
	_, err = a.resolve(context.Background())
	require.NoError(t, err)
	assert.Len(t, refreshes, 2)

	// Logging out removes the credentials.
	config.Workspaces = nil
	_, err = a.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
}

func TestBridgeAuthTokenEndpointAnswers(t *testing.T) {
	for status, loggedOut := range map[int]bool{400: true, 401: true, 404: false, 408: false, 429: false, 500: false, 503: false} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			a := testBridgeAuth(bridgeRoundTripFunc(func(*http.Request) (*http.Response, error) { return tokenResponse(status, ``), nil }),
				mainWorkspace(blaxel.Credentials{AccessToken: "expired", RefreshToken: "fake"}))
			_, err := a.resolve(context.Background())
			require.Error(t, err)
			assert.Equal(t, loggedOut, errors.Is(err, errNotLoggedIn), "only a refused grant is a logout")
		})
	}
}

// A failed token exchange is not repeated at once, and is retried later.
func TestBridgeAuthRetriesAFailedRefreshAfterTheCooldown(t *testing.T) {
	now := time.Now()
	calls := 0
	a := testBridgeAuth(bridgeRoundTripFunc(func(*http.Request) (*http.Response, error) {
		if calls++; calls == 1 {
			return tokenResponse(http.StatusTooManyRequests, `{}`), nil
		}
		return tokenResponse(http.StatusOK, `{"access_token":"fake-fresh"}`), nil
	}), mainWorkspace(blaxel.Credentials{AccessToken: "expired", RefreshToken: "fake"}))
	a.now = func() time.Time { return now }
	for range 2 {
		_, err := a.resolve(context.Background())
		require.Error(t, err)
	}
	assert.Equal(t, 1, calls)
	now = now.Add(mcpExchangeCooldown + time.Second)
	c, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer fake-fresh", c.headers["Authorization"])
}

func TestBridgeAuthClientCredentialsAndEnvironmentKeys(t *testing.T) {
	a := testBridgeAuth(bridgeRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "https://api.blaxel.ai/v0/oauth/token", r.URL.String())
		id, secret, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "fake-client", id)
		assert.Equal(t, "fake-secret", secret)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"grant_type": "client_credentials"}, body)
		return tokenResponse(http.StatusOK, `{"access_token":"fake-access"}`), nil
	}), mainWorkspace(blaxel.Credentials{ClientCredentials: "fake-client:fake-secret"}))
	c, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer fake-access", c.headers["Authorization"])

	// BL_API_KEY wins over the stored login, and needs no stored workspace.
	a = testBridgeAuth(nil)
	a.env = func(key string) string { return map[string]string{"BL_API_KEY": "bl_key"}[key] }
	c, err = a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer bl_key", c.headers["Authorization"])
	assert.Equal(t, "https://api.blaxel.ai/v0/mcp", c.endpoint)

	// No workspace at all is no login.
	a = testBridgeAuth(nil)
	a.explicit = ""
	_, err = a.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
}

// Credentials go to the origin of the stored workspace environment, whatever
// the inherited environment says, and never to a new one.
func TestBridgeAuthPinsTheStoredOrigin(t *testing.T) {
	t.Setenv("BL_API_URL", "http://attacker.invalid/v0")
	t.Setenv("BL_ENV", "local")
	for env, host := range map[string]string{"prod": "api.blaxel.ai", "dev": "api.blaxel.dev"} {
		t.Run(env, func(t *testing.T) {
			workspace := mainWorkspace(blaxel.Credentials{AccessToken: "expired", RefreshToken: "fake"})
			workspace.Env = env
			a := testBridgeAuth(bridgeRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				assert.Equal(t, "https://"+host+"/v0/oauth/token", r.URL.String())
				return tokenResponse(http.StatusOK, `{"access_token":"fake-access"}`), nil
			}), workspace)
			a.env = func(string) string { return "" }
			c, err := a.resolve(context.Background())
			require.NoError(t, err)
			assert.Equal(t, "https://"+host+"/v0/mcp", c.endpoint)

			// A changed stored environment must not export the login to the new origin.
			workspace.Env = map[string]string{"prod": "dev", "dev": "prod"}[env]
			a.loadConfig = func() (blaxel.Config, error) {
				return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{workspace}}, nil
			}
			_, err = a.resolve(context.Background())
			assert.ErrorContains(t, err, "environment changed")
		})
	}
	_, err := bridgeBaseURL("local")
	assert.Error(t, err)
}

// A 401 refreshes the token and retries the call once.
func TestBridge401ForcesOneRefreshAndOneRetry(t *testing.T) {
	var calls, refreshes atomic.Int32
	now := time.Now()
	old, fresh := bridgeJWT(now, now.Add(time.Hour)), bridgeJWT(now, now.Add(2*time.Hour))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/oauth/token") {
			refreshes.Add(1)
			_, _ = fmt.Fprintf(w, `{"access_token":%q}`, fresh)
			return
		}
		calls.Add(1)
		if r.Header.Get("Authorization") == "Bearer "+old {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.Equal(t, "Bearer "+fresh, r.Header.Get("Authorization"))
		jsonResult(w, json.RawMessage("2"), `{"content":[]}`)
	}))
	defer server.Close()
	b := newMCPBridge(bridgeTestAuth(t, server, blaxel.Credentials{AccessToken: old, RefreshToken: "fake", DeviceCode: "fake"}))
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write"}}`)
	assert.NotNil(t, answerFor(t, answers, 2)["result"])
	assert.EqualValues(t, 2, calls.Load())
	assert.EqualValues(t, 1, refreshes.Load())
}

func TestBridgeAuthRefreshesOnceAtATimeAndWaitsWithTheCallersContext(t *testing.T) {
	var refreshes atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if refreshes.Add(1) == 1 {
			close(started)
		}
		<-release
		_, _ = fmt.Fprintf(w, `{"access_token":%q}`, bridgeJWT(time.Now(), time.Now().Add(time.Hour)))
	}))
	defer server.Close()
	a := bridgeTestAuth(t, server, blaxel.Credentials{AccessToken: "expired", RefreshToken: "fake"})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := a.resolve(context.Background()); errs <- err }()
	}
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.resolve(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, refreshes.Load())
}

func TestBridgeAuthRereadsAPartialConfigOnce(t *testing.T) {
	a := newBridgeAuth("main")
	reads := 0
	a.loadConfig = func() (blaxel.Config, error) {
		if reads++; reads == 1 {
			return blaxel.Config{}, errors.New("partial YAML")
		}
		return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{mainWorkspace(blaxel.Credentials{APIKey: "fake"})}}, nil
	}
	_, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, reads)
}
