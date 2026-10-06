package cli

import (
	"bytes"
	"context"
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

// Regression versions of every bridge characterization in the independent review.
func TestBridgeRedirectDoesNotLeakCredentials(t *testing.T) {
	var leaks atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaks.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		jsonResult(w, json.RawMessage("1"), `{"content":[{"type":"text","text":"redirect sink"}]}`)
	}))
	defer sink.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	b, a := testBridge(t, nil)
	a.endpoint = source.URL
	b.client = source.Client()
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	_, isError := toolText(t, answerFor(t, answers, 1))
	assert.True(t, isError)
	auth := &bridgeAuth{client: source.Client(), baseURL: source.URL}
	_, err := auth.refresh(context.Background(), blaxel.Credentials{RefreshToken: "fake-refresh-secret", DeviceCode: "fake-device-secret"})
	require.Error(t, err)
	assert.Zero(t, leaks.Load(), "neither Authorization nor POST grants may reach a redirect target")
}

func TestBridgeTransientRefreshIsNotLogout(t *testing.T) {
	for _, status := range []int{408, 429, 500, 503, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			a := &bridgeAuth{client: server.Client(), baseURL: server.URL}
			_, err := a.refresh(context.Background(), blaxel.Credentials{RefreshToken: "fake", DeviceCode: "fake"})
			require.Error(t, err)
			assert.NotErrorIs(t, err, errNotLoggedIn)
		})
	}
}

func TestBridgeRemembersSyntheticNegotiationAndInitializesUpstream(t *testing.T) {
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, method string, id json.RawMessage) {
		switch method {
		case "initialize":
			jsonResult(w, id, `{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"s","version":"1"}}`)
		case "notifications/initialized":
			w.WriteHeader(202)
		default:
			jsonResult(w, id, `{"tools":[]}`)
		}
	}}
	b, a := testBridge(t, server)
	var out bytes.Buffer
	b.out = &out
	a.setLoggedIn(false)
	msg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"roots":{"listChanged":true}},"clientInfo":{"name":"test","version":"1"}}}`
	var m rpcEnvelope
	require.NoError(t, json.Unmarshal([]byte(msg), &m))
	b.handle(context.Background(), []byte(msg), m)
	a.setLoggedIn(true)
	msg = `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	require.NoError(t, json.Unmarshal([]byte(msg), &m))
	b.handle(context.Background(), []byte(msg), m)
	assert.Equal(t, "2025-06-18", server.request(t, "tools/list").Header.Get("MCP-Protocol-Version"))
	assert.Equal(t, "2025-06-18", server.request(t, "initialize").Header.Get("MCP-Protocol-Version"))
	server.request(t, "notifications/initialized")
	server.mu.Lock()
	initialize := server.bodies[0]
	server.mu.Unlock()
	assert.Contains(t, initialize, `"roots":{"listChanged":true}`)
	assert.Contains(t, initialize, `"protocolVersion":"2025-06-18"`)
	assert.Len(t, decodeLines(t, out.String()), 2, "upstream initialize must not give the client a second result")
}

func TestBridgeIncompleteResponsesAlwaysAnswer(t *testing.T) {
	for _, kind := range []string{"sse", "empty", "202", "partial", "scanner"} {
		t.Run(kind, func(t *testing.T) {
			b, _ := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "sse":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "id: resume-me\ndata:\nretry: 10\n\n")
				case "202":
					w.WriteHeader(202)
				case "partial":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {\"jsonrpc\":\"2.0\",\"id\":8,\"result\":{}}\n")
				case "scanner":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: "+strings.Repeat("x", mcpBridgeMaxMessage+1))
				}
			}))
			answers := runBridge(t, b, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"slow"}}`)
			assert.NotNil(t, answerFor(t, answers, 8)["error"], "request gets a JSON-RPC error, never silence or replay")
		})
	}
}

func TestBridgeSSEAggregateLimit(t *testing.T) {
	var data strings.Builder
	data.WriteString("data: {\"items\":[\n")
	for i := 0; i < 33; i++ {
		data.WriteString("data: \"")
		data.WriteString(strings.Repeat("x", 1<<20))
		data.WriteString("\"")
		if i < 32 {
			data.WriteByte(',')
		}
		data.WriteByte('\n')
	}
	data.WriteString("data: ]}\n\n")
	relayed := false
	err := relayEvents(strings.NewReader(data.String()), func([]byte) { relayed = true })
	require.Error(t, err)
	assert.False(t, relayed, "the complete event exceeds the limit")
}

func TestBridgeContextCancelStopsIdleInput(t *testing.T) {
	b, a := testBridge(t, nil)
	a.setLoggedIn(false)
	ctx, cancel := context.WithCancel(context.Background())
	in, w := io.Pipe()
	defer func() { _ = w.Close() }()
	done := make(chan error, 1)
	go func() { done <- b.serve(ctx, in, io.Discard) }()
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("cancelled serve remained blocked on idle stdin")
	}
}

func TestBridgeSaturationKeepsControlTrafficFlowing(t *testing.T) {
	started := make(chan struct{}, 16)
	cancelled := make(chan struct{}, 1)
	response := make(chan struct{}, 1)
	b, _ := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m rpcEnvelope
		_ = json.NewDecoder(r.Body).Decode(&m)
		if m.Method != "tools/call" {
			if m.Method == "" {
				response <- struct{}{}
			}
			w.WriteHeader(202)
			return
		}
		started <- struct{}{}
		<-r.Context().Done()
		if string(m.ID) == "1" {
			cancelled <- struct{}{}
		}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in, w := io.Pipe()
	defer func() { _ = w.Close() }()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- b.serve(ctx, in, &out) }()
	for i := 1; i <= 16; i++ {
		_, err := fmt.Fprintf(w, "{\"jsonrpc\":\"2.0\",\"id\":%d,\"method\":\"tools/call\"}\n", i)
		require.NoError(t, err)
	}
	for i := 0; i < 16; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("calls did not start")
		}
	}
	wrote := make(chan struct{})
	go func() {
		_, _ = io.WriteString(w, "{\"jsonrpc\":\"2.0\",\"id\":17,\"method\":\"tools/call\"}\n{\"jsonrpc\":\"2.0\",\"method\":\"notifications/cancelled\",\"params\":{\"requestId\":1}}\n{\"jsonrpc\":\"2.0\",\"id\":18,\"method\":\"ping\"}\n{\"jsonrpc\":\"2.0\",\"id\":99,\"result\":{}}\n")
		close(wrote)
	}()
	select {
	case <-wrote:
	case <-time.After(time.Second):
		t.Fatal("reader blocked on excess request")
	}
	answers := out.waitFor(t, 2)
	assert.Contains(t, answerFor(t, answers, 17)["error"].(map[string]any)["message"], "busy")
	assert.NotNil(t, answerFor(t, answers, 18)["result"])
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked by saturation")
	}
	select {
	case <-response:
	case <-time.After(time.Second):
		t.Fatal("client response blocked by saturation")
	}
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}

type bridgeRoundTripFunc func(*http.Request) (*http.Response, error)

func (f bridgeRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBridgeIgnoresInheritedCredentialDestinations(t *testing.T) {
	t.Setenv("BL_API_URL", "http://attacker.invalid/v0")
	t.Setenv("BL_ENV", "local")
	for _, env := range []string{"prod", "dev"} {
		t.Run(env, func(t *testing.T) {
			a := newBridgeAuth("main")
			a.loadConfig = func() (blaxel.Config, error) {
				return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Env: env, Credentials: blaxel.Credentials{AccessToken: "fake-secret", RefreshToken: "fake-refresh", DeviceCode: "fake-device"}}}}, nil
			}
			expected := "api.blaxel.ai"
			if env == "dev" {
				expected = "api.blaxel.dev"
			}
			a.client = &http.Client{Transport: bridgeRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				assert.Equal(t, expected, r.URL.Host)
				assert.Equal(t, "https", r.URL.Scheme)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fake-access"}`)), Header: make(http.Header)}, nil
			})}
			c, err := a.resolve(context.Background())
			require.NoError(t, err)
			assert.Equal(t, "https://"+expected+"/v0/mcp", c.endpoint)
			assert.Equal(t, "Bearer fake-access", c.headers["Authorization"])
			assert.Empty(t, c.headers["X-Blaxel-Authorization"])
		})
	}
	_, err := bridgeBaseURL("local")
	require.Error(t, err)
}

func bridgeTestAuth(t *testing.T, server *httptest.Server, c blaxel.Credentials) *bridgeAuth {
	t.Helper()
	a := newBridgeAuth("main")
	a.environment = func(string) string { return server.URL + "/v0" }
	a.client = server.Client()
	a.loadConfig = func() (blaxel.Config, error) {
		return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: c}}}, nil
	}
	return a
}
func TestBridge401ForcesOneRefreshAndOneRetry(t *testing.T) {
	var calls, refreshes atomic.Int32
	now := time.Now()
	old := bridgeJWT(now, now.Add(time.Hour))
	fresh := bridgeJWT(now, now.Add(2*time.Hour))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/oauth/token") {
			refreshes.Add(1)
			_, _ = fmt.Fprintf(w, `{"access_token":%q}`, fresh)
			return
		}
		calls.Add(1)
		if r.Header.Get("Authorization") == "Bearer "+old {
			w.WriteHeader(401)
			return
		}
		assert.Equal(t, "Bearer "+fresh, r.Header.Get("Authorization"))
		jsonResult(w, json.RawMessage("2"), `{"content":[]}`)
	}))
	defer server.Close()
	a := bridgeTestAuth(t, server, blaxel.Credentials{AccessToken: old, RefreshToken: "fake", DeviceCode: "fake"})
	b := newMCPBridge(a)
	b.poll = 0
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write"}}`)
	assert.NotNil(t, answerFor(t, answers, 2)["result"])
	assert.EqualValues(t, 2, calls.Load())
	assert.EqualValues(t, 1, refreshes.Load())
}
func TestBridgeNeverFailsInitializeForBadCredentials(t *testing.T) {
	for _, mode := range []string{"garbage", "expired", "invalid-grant"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "oauth/token") {
					w.WriteHeader(400)
					_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
				} else {
					w.WriteHeader(401)
				}
			}))
			defer server.Close()
			c := blaxel.Credentials{AccessToken: "fake-garbage"}
			if mode == "expired" {
				c.AccessToken = bridgeJWT(time.Now().Add(-time.Hour), time.Now().Add(-time.Minute))
			}
			if mode == "invalid-grant" {
				c.RefreshToken = "fake-rejected-grant"
			}
			b := newMCPBridge(bridgeTestAuth(t, server, c))
			b.poll = 0
			answers := runBridge(t, b, initializeMessage, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
			assert.NotNil(t, answerFor(t, answers, 1)["result"])
			assert.Nil(t, answerFor(t, answers, 1)["error"])
			assert.Empty(t, answerFor(t, answers, 2)["result"].(map[string]any)["tools"])
		})
	}
}
func TestBridgeRejectsBatches(t *testing.T) {
	b, a := testBridge(t, nil)
	a.setLoggedIn(false)
	answers := runBridge(t, b, initializeMessage, `[{"jsonrpc":"2.0","id":2,"method":"ping"}]`)
	require.Len(t, answers, 2)
	assert.EqualValues(t, -32600, answers[1]["error"].(map[string]any)["code"])
}
func TestBridgeDoesNotReplayAmbiguousFailures(t *testing.T) {
	for _, mode := range []string{"502", "network", "sse"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			b, _ := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "502":
					w.WriteHeader(502)
					_, _ = io.WriteString(w, "fake-secret must never be echoed")
				case "network":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
				case "sse":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {}\n\n")
				}
			}))
			answers := runBridge(t, b, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write"}}`)
			assert.EqualValues(t, 1, calls.Load())
			assert.NotContains(t, fmt.Sprint(answers), "fake-secret")
			require.NotEmpty(t, answerFor(t, answers, 2))
		})
	}
}

type bridgeBrokenWriter struct{}

func (bridgeBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestBridgeStdoutFailureShutsDown(t *testing.T) {
	b, a := testBridge(t, nil)
	a.setLoggedIn(false)
	in, w := io.Pipe()
	defer func() { _ = w.Close() }()
	done := make(chan error, 1)
	go func() { done <- b.serve(context.Background(), in, bridgeBrokenWriter{}) }()
	_, _ = io.WriteString(w, initializeMessage+"\n")
	select {
	case err := <-done:
		require.ErrorIs(t, err, io.ErrClosedPipe)
	case <-time.After(time.Second):
		t.Fatal("stdout failure did not shut down")
	}
}
func TestBridgeEOFDrainIsBounded(t *testing.T) {
	b, _ := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	start := time.Now()
	runBridge(t, b, `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	assert.Less(t, time.Since(start), 5*time.Second)
}
func TestBridgeRefreshSingleFlightAndContextAwareWait(t *testing.T) {
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
	a := bridgeTestAuth(t, server, blaxel.Credentials{AccessToken: "fake-expired", RefreshToken: "fake"})
	var wg sync.WaitGroup
	wg.Add(8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
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
func TestBridgePartialConfigRetriesOnce(t *testing.T) {
	a := newBridgeAuth("main")
	reads := 0
	a.loadConfig = func() (blaxel.Config, error) {
		reads++
		if reads == 1 {
			return blaxel.Config{}, errors.New("partial YAML")
		}
		return blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: blaxel.Credentials{APIKey: "fake"}}}}, nil
	}
	_, err := a.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, reads)
}
