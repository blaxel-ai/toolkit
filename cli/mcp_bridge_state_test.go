package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeChoosesOriginWhenLoginAppears(t *testing.T) {
	config := blaxel.Config{Context: blaxel.ContextConfig{Workspace: "main"}}
	a := newBridgeAuth("")
	a.env = func(string) string { return "" }
	a.loadConfig = func() (blaxel.Config, error) { return config, nil }
	_, err := a.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
	config.Workspaces = []blaxel.WorkspaceConfig{{Name: "main", Env: "dev", Credentials: blaxel.Credentials{APIKey: "fake-secret"}}}
	c, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "https://api.blaxel.dev/v0/mcp", c.endpoint)
	// Changing the stored environment must not export the new login to the old one.
	config.Workspaces[0].Env = "prod"
	_, err = a.resolve(context.Background())
	assert.ErrorContains(t, err, "environment changed")
}

func TestBridgeRequestDeadlineReturnsAnError(t *testing.T) {
	b, _ := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	b.requestTimeout = 25 * time.Millisecond
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write"}}`)
	text, isError := toolText(t, answerFor(t, answers, 1))
	assert.True(t, isError)
	assert.Contains(t, text, "not replayed")
}

func TestBridgeRefreshCooldownExpiresAndKeepsTools(t *testing.T) {
	now := time.Now()
	a := newBridgeAuth("main")
	a.env = func(string) string { return "" }
	a.now = func() time.Time { return now }
	a.loadConfig = func() (blaxel.Config, error) {
		return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: blaxel.Credentials{AccessToken: "fake-expired", RefreshToken: "fake"}}}}, nil
	}
	calls := 0
	a.client = &http.Client{Transport: bridgeRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		status, body := http.StatusTooManyRequests, `{}`
		if calls > 1 {
			status, body = http.StatusOK, `{"access_token":"fake-fresh"}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	_, err := a.resolve(context.Background())
	require.Error(t, err)
	assert.NotErrorIs(t, err, errNotLoggedIn)
	_, err = a.resolve(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, calls, "cooldown bounds refresh attempts")
	now = now.Add(10 * time.Second)
	c, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer fake-fresh", c.headers["Authorization"])
	assert.Equal(t, 2, calls)
}

func TestBridgeUpstreamNegotiationMustMatchSavedVersion(t *testing.T) {
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, _ string, id json.RawMessage) {
		jsonResult(w, id, `{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"s","version":"1"}}`)
	}}
	b, a := testBridge(t, server)
	a.setLoggedIn(false)
	runBridge(t, b, initializeMessage)
	a.setLoggedIn(true)
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	assert.Contains(t, answerFor(t, answers, 2)["error"].(map[string]any)["message"], "negotiation changed")
}

func TestBridgePreservesStoredAuthSourcePrecedence(t *testing.T) {
	now := time.Now()
	access := bridgeJWT(now, now.Add(time.Hour))
	for _, fixture := range []struct {
		name        string
		credentials blaxel.Credentials
		want        string
	}{
		{"api-key", blaxel.Credentials{APIKey: "fake-key", AccessToken: access, RefreshToken: "fake", ClientCredentials: "fake:secret"}, "Bearer fake-key"},
		{"access-token", blaxel.Credentials{AccessToken: access, ClientCredentials: "fake:secret"}, "Bearer " + access},
		{"refreshable-access-token", blaxel.Credentials{AccessToken: access, RefreshToken: "fake", ClientCredentials: "fake:secret"}, "Bearer " + access},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			a := newBridgeAuth("main")
			a.env = func(string) string { return "" }
			a.loadConfig = func() (blaxel.Config, error) {
				return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: fixture.credentials}}}, nil
			}
			a.client = &http.Client{Transport: bridgeRoundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("unexpected client-credentials exchange")
				return nil, io.ErrUnexpectedEOF
			})}
			c, err := a.resolve(context.Background())
			require.NoError(t, err)
			assert.Equal(t, fixture.want, c.headers["Authorization"])
		})
	}
}

func TestBridgeClientCredentialsKeepTheSDKGrantFormat(t *testing.T) {
	a := newBridgeAuth("main")
	a.env = func(string) string { return "" }
	a.loadConfig = func() (blaxel.Config, error) {
		return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: blaxel.Credentials{ClientCredentials: "fake-client:fake-secret"}}}}, nil
	}
	a.client = &http.Client{Transport: bridgeRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "https://api.blaxel.ai/v0/oauth/token", r.URL.String())
		id, secret, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "fake-client", id)
		assert.Equal(t, "fake-secret", secret)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"grant_type": "client_credentials"}, body)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"fake-access"}`))}, nil
	})}
	c, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer fake-access", c.headers["Authorization"])
}

func TestBridgeConcurrentStale401sReuseTheFreshToken(t *testing.T) {
	now := time.Now()
	old := bridgeJWT(now, now.Add(time.Hour))
	fresh := bridgeJWT(now, now.Add(2*time.Hour))
	a := newBridgeAuth("main")
	a.env = func(string) string { return "" }
	a.loadConfig = func() (blaxel.Config, error) {
		return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: blaxel.Credentials{AccessToken: old, RefreshToken: "fake"}}}}, nil
	}
	exchanges := 0
	a.client = &http.Client{Transport: bridgeRoundTripFunc(func(*http.Request) (*http.Response, error) {
		exchanges++
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"` + fresh + `"}`))}, nil
	})}
	original, err := a.resolve(context.Background())
	require.NoError(t, err)
	a.reject(original)
	current, err := a.resolve(context.Background())
	require.NoError(t, err)
	// Late 401s on other requests using the old token share the first refresh.
	for i := 0; i < 16; i++ {
		a.reject(original)
		c, err := a.resolve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "Bearer "+fresh, c.headers["Authorization"])
	}
	assert.Equal(t, 1, exchanges)
	a.reject(current)
	_, err = a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, exchanges, "a genuine 401 on the new token still refreshes")
}

func TestBridgeSSEFailureAfterCompletedResponseDoesNotAnswerTwice(t *testing.T) {
	b, _ := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\ndata: partial")
	}))
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	require.Len(t, answers, 1)
	assert.NotNil(t, answers[0]["result"])
}
