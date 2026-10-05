package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMCPServer answers like the hosted Blaxel MCP server and records requests.
type fakeMCPServer struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	handle   func(w http.ResponseWriter, method string, id json.RawMessage)
}

func (f *fakeMCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.bodies = append(f.bodies, string(body))
	f.mu.Unlock()
	var message struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	_ = json.Unmarshal(body, &message)
	f.handle(w, message.Method, message.ID)
}

func (f *fakeMCPServer) request(t *testing.T, method string) *http.Request {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, body := range f.bodies {
		if strings.Contains(body, `"method":"`+method+`"`) {
			return f.requests[i]
		}
	}
	t.Fatalf("no %s request in %v", method, f.bodies)
	return nil
}

func jsonResult(w http.ResponseWriter, id json.RawMessage, result string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":` + result + `}`))
}

// fakeAuth signs every request in, or reports no login.
type fakeAuth struct {
	mu        sync.Mutex
	loggedIn  bool
	endpoint  string
	rejected  int
	workspace string
}

func (f *fakeAuth) resolve(context.Context) (mcpCredentials, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.loggedIn {
		return mcpCredentials{}, errNotLoggedIn
	}
	return mcpCredentials{workspace: f.workspace, endpoint: f.endpoint, fingerprint: "fp",
		headers: map[string]string{"X-Blaxel-Authorization": "Bearer token-1", "X-Blaxel-Workspace": f.workspace}}, nil
}

func (f *fakeAuth) reject(mcpCredentials) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejected++
	f.loggedIn = false
}

func (f *fakeAuth) setLoggedIn(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loggedIn = value
}

func testBridge(t *testing.T, server http.Handler) (*mcpBridge, *fakeAuth) {
	t.Helper()
	authenticator := &fakeAuth{loggedIn: true, workspace: "my-workspace"}
	bridge := newMCPBridge(authenticator)
	bridge.version, bridge.userAgent, bridge.poll = "9.9.9", "blaxel-cli/test (bl mcp)", 0
	bridge.logf = func(string, ...any) {}
	if server != nil {
		httpServer := httptest.NewServer(server)
		t.Cleanup(httpServer.Close)
		authenticator.endpoint = httpServer.URL + "/v0/mcp"
		bridge.client = httpServer.Client()
	}
	return bridge, authenticator
}

// runBridge sends the messages and returns everything the bridge wrote, one
// decoded message per line.
func runBridge(t *testing.T, bridge *mcpBridge, messages ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, bridge.serve(context.Background(), strings.NewReader(strings.Join(messages, "\n")+"\n"), &out))
	return decodeLines(t, out.String())
}

func decodeLines(t *testing.T, output string) []map[string]any {
	t.Helper()
	var answers []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var answer map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &answer), "every line on stdout is one JSON-RPC message: %q", line)
		answers = append(answers, answer)
	}
	return answers
}

func answerFor(t *testing.T, answers []map[string]any, id float64) map[string]any {
	t.Helper()
	for _, answer := range answers {
		if answer["id"] == id {
			return answer
		}
	}
	t.Fatalf("no answer for %v in %v", id, answers)
	return nil
}

func toolText(t *testing.T, answer map[string]any) (string, bool) {
	t.Helper()
	result := answer["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	isError, _ := result["isError"].(bool)
	return content["text"].(string), isError
}

const initializeMessage = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

func TestBridgeRelaysJSONAndSignsRequestsIn(t *testing.T) {
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, method string, id json.RawMessage) {
		switch method {
		case "initialize":
			jsonResult(w, id, `{"protocolVersion":"2025-03-26","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"blaxel-mcp-server","version":"1"}}`)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		default:
			jsonResult(w, id, `{"tools":[{"name":"list_sandboxes"}]}`)
		}
	}}
	bridge, _ := testBridge(t, server)
	answers := runBridge(t, bridge, initializeMessage, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	require.Len(t, answers, 2, "notifications get no answer")
	initialize := answerFor(t, answers, 1)["result"].(map[string]any)
	assert.Equal(t, "2025-03-26", initialize["protocolVersion"])
	assert.Contains(t, initialize["instructions"], "workspace my-workspace", "the agent learns which workspace the tools use")
	assert.Equal(t, "list_sandboxes", answerFor(t, answers, 2)["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)["name"])

	list := server.request(t, "tools/list")
	assert.Equal(t, "/v0/mcp", list.URL.Path)
	assert.Equal(t, "Bearer token-1", list.Header.Get("X-Blaxel-Authorization"))
	assert.Equal(t, "my-workspace", list.Header.Get("X-Blaxel-Workspace"))
	assert.Equal(t, "2025-03-26", list.Header.Get("MCP-Protocol-Version"))
	assert.Equal(t, "blaxel-cli/test (bl mcp)", list.Header.Get("User-Agent"))
	assert.Contains(t, list.Header.Get("Accept"), "text/event-stream")
	assert.Empty(t, server.request(t, "initialize").Header.Get("MCP-Protocol-Version"))
}

func TestBridgeRelaysServerSentEvents(t *testing.T) {
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, _ string, id json.RawMessage) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{\"progress\":1}}\n\n")
		_, _ = fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%s,\n", id)
		_, _ = fmt.Fprintf(w, "data: \"result\":{\"content\":[{\"type\":\"text\",\"text\":\"done\"}]}}\n\n")
	}}
	bridge, _ := testBridge(t, server)
	answers := runBridge(t, bridge, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"run"}}`)
	require.Len(t, answers, 2)
	assert.Equal(t, "notifications/progress", answers[0]["method"])
	text, isError := toolText(t, answerFor(t, answers, 7))
	assert.Equal(t, "done", text)
	assert.False(t, isError)
}

func TestBridgeTurnsRefusedCallsIntoToolErrors(t *testing.T) {
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, method string, id json.RawMessage) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":403,"error":"requested workspace is unavailable; check its name and your access"}`))
	}}
	bridge, _ := testBridge(t, server)
	answers := runBridge(t, bridge,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_sandboxes","arguments":{"workspace":"other"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	text, isError := toolText(t, answerFor(t, answers, 1))
	assert.True(t, isError)
	assert.Contains(t, text, "requested workspace is unavailable")
	assert.Contains(t, answerFor(t, answers, 2)["error"].(map[string]any)["message"], "403")
}

func TestBridgeStandsInBeforeLogin(t *testing.T) {
	bridge, authenticator := testBridge(t, nil)
	authenticator.setLoggedIn(false)
	confirm := make(chan struct{})
	var asked []string
	bridge.login = func(_ context.Context, workspace string) (string, func(context.Context) (string, error), error) {
		asked = append(asked, workspace)
		return "https://app.blaxel.ai/device?code=ABC", func(context.Context) (string, error) {
			<-confirm
			authenticator.setLoggedIn(true)
			return "main", nil
		}, nil
	}
	in, writer := io.Pipe()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- bridge.serve(context.Background(), in, &out) }()
	send := func(message string) { _, _ = writer.Write([]byte(message + "\n")) }

	send(initializeMessage)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_sandboxes","arguments":{}}}`)
	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"blaxel_login","arguments":{"workspace":"main"}}}`)
	answers := out.waitFor(t, 4)

	initialize := answerFor(t, answers, 1)["result"].(map[string]any)
	assert.Equal(t, "2025-03-26", initialize["protocolVersion"], "the agent's protocol version is kept")
	assert.Equal(t, map[string]any{"tools": map[string]any{"listChanged": true}}, initialize["capabilities"])
	tools := answerFor(t, answers, 2)["result"].(map[string]any)["tools"].([]any)
	require.Len(t, tools, 1)
	assert.Equal(t, mcpLoginTool, tools[0].(map[string]any)["name"])
	text, isError := toolText(t, answerFor(t, answers, 3))
	assert.True(t, isError)
	assert.Contains(t, text, "blaxel_login")
	text, isError = toolText(t, answerFor(t, answers, 4))
	assert.False(t, isError)
	assert.Contains(t, text, "https://app.blaxel.ai/device?code=ABC")
	assert.Equal(t, []string{"main"}, asked)

	// Once the user confirms, the agent is told to list the tools again.
	close(confirm)
	answers = out.waitFor(t, 5)
	assert.Equal(t, "notifications/tools/list_changed", answers[4]["method"])
	_ = writer.Close()
	require.NoError(t, <-done)
}

func TestBridgeOffersTheLoginAgainWhenTheServerRefusesIt(t *testing.T) {
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, method string, id json.RawMessage) {
		if method == "tools/list" {
			jsonResult(w, id, `{"tools":[{"name":"list_sandboxes"}]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token","error_description":"Unauthorized"}`))
	}}
	bridge, authenticator := testBridge(t, server)
	in, writer := io.Pipe()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- bridge.serve(context.Background(), in, &out) }()
	_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"))
	out.waitFor(t, 1)
	_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_sandboxes"}}` + "\n"))
	answers := out.waitFor(t, 3)
	_ = writer.Close()
	require.NoError(t, <-done)

	text, isError := toolText(t, answerFor(t, answers, 2))
	assert.True(t, isError)
	assert.Contains(t, text, "login expired")
	assert.Equal(t, "notifications/tools/list_changed", answers[2]["method"])
	assert.Equal(t, 1, authenticator.rejected)
}

func TestBridgeAnswersInitializeFirstAndCancels(t *testing.T) {
	started := make(chan struct{})
	server := &fakeMCPServer{handle: func(w http.ResponseWriter, method string, id json.RawMessage) {
		switch method {
		case "initialize":
			time.Sleep(50 * time.Millisecond)
			jsonResult(w, id, `{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"s","version":"1"},"instructions":"server text"}`)
		case "tools/call":
			close(started)
			time.Sleep(2 * time.Second)
			jsonResult(w, id, `{"content":[]}`)
		default:
			jsonResult(w, id, `{"tools":[]}`)
		}
	}}
	bridge, _ := testBridge(t, server)
	in, writer := io.Pipe()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- bridge.serve(context.Background(), in, &out) }()
	_, _ = writer.Write([]byte(initializeMessage + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"slow"}}` + "\n"))
	answers := out.waitFor(t, 2)
	assert.Equal(t, float64(1), answers[0]["id"], "initialize is answered before anything sent after it")
	assert.Equal(t, "server text", answers[0]["result"].(map[string]any)["instructions"], "server instructions are kept")
	<-started
	start := time.Now()
	_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":3}}` + "\n"))
	_ = writer.Close()
	require.NoError(t, <-done)
	assert.Less(t, time.Since(start), time.Second, "a cancelled request stops at once")
	assert.Len(t, decodeLines(t, out.String()), 2, "a cancelled request gets no answer")
}

// syncBuffer is an output buffer safe to read while the bridge writes.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *syncBuffer) waitFor(t *testing.T, count int) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		answers := decodeLines(t, s.String())
		if len(answers) >= count {
			return answers
		}
		if time.Now().After(deadline) {
			t.Fatalf("waited for %d messages, got %v", count, answers)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func bridgeJWT(issued, expires time.Time) string {
	payload, _ := json.Marshal(map[string]int64{"iat": issued.Unix(), "exp": expires.Unix()})
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestBridgeAuthRefreshesInMemoryAndKeepsTheWorkspace(t *testing.T) {
	now := time.Now()
	var refreshes []map[string]string
	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		refreshes = append(refreshes, body)
		assert.Equal(t, "/v0/oauth/token", r.URL.Path)
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"expires_in":7200}`, bridgeJWT(now, now.Add(2*time.Hour)))
	}))
	defer tokens.Close()
	config := blaxel.Config{Context: blaxel.ContextConfig{Workspace: "main"}, Workspaces: []blaxel.WorkspaceConfig{
		{Name: "main", Credentials: blaxel.Credentials{AccessToken: bridgeJWT(now.Add(-2*time.Hour), now.Add(-time.Minute)), RefreshToken: "refresh-1", DeviceCode: "device-1"}},
		{Name: "other", Credentials: blaxel.Credentials{APIKey: "bl_other"}},
	}}
	var environments []string
	authenticator := &bridgeAuth{
		env: func(string) string { return "" }, loadConfig: func() (blaxel.Config, error) { return config, nil },
		environment: func(workspace string) string { environments = append(environments, workspace); return tokens.URL + "/v0/" },
		client:      tokens.Client(), now: func() time.Time { return now },
	}
	credentials, err := authenticator.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", credentials.workspace)
	assert.Equal(t, tokens.URL+"/v0/mcp", credentials.endpoint)
	assert.Equal(t, "Bearer "+bridgeJWT(now, now.Add(2*time.Hour)), credentials.headers["X-Blaxel-Authorization"])
	require.Len(t, refreshes, 1)
	assert.Equal(t, map[string]string{"grant_type": "refresh_token", "refresh_token": "refresh-1", "client_id": "blaxel", "device_code": "device-1"}, refreshes[0])

	// The refreshed token is reused, and a later bl workspaces switch does not
	// move the agent's session.
	config.Context.Workspace = "other"
	credentials, err = authenticator.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "main", credentials.workspace)
	assert.Len(t, refreshes, 1)
	assert.Equal(t, []string{"main"}, environments)

	// A refused login is not used again until it changes.
	authenticator.reject(credentials)
	_, err = authenticator.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
	config.Workspaces[0].Credentials = blaxel.Credentials{AccessToken: bridgeJWT(now, now.Add(2*time.Hour)), RefreshToken: "refresh-2"}
	credentials, err = authenticator.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer "+bridgeJWT(now, now.Add(2*time.Hour)), credentials.headers["X-Blaxel-Authorization"])
	assert.Len(t, refreshes, 1, "a fresh token needs no refresh")

	// Logging out removes the credentials.
	config.Workspaces = nil
	_, err = authenticator.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
}

func TestBridgeAuthTreatsARefusedRefreshAsLoggedOut(t *testing.T) {
	now := time.Now()
	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tokens.Close()
	config := blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "main", Credentials: blaxel.Credentials{
		AccessToken: bridgeJWT(now.Add(-2*time.Hour), now.Add(-time.Minute)), RefreshToken: "revoked"}}}}
	authenticator := &bridgeAuth{
		explicit: "main", env: func(string) string { return "" }, loadConfig: func() (blaxel.Config, error) { return config, nil },
		environment: func(string) string { return tokens.URL + "/v0" }, client: tokens.Client(), now: func() time.Time { return now },
	}
	_, err := authenticator.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
}

func TestBridgeAuthUsesEnvironmentKeys(t *testing.T) {
	authenticator := &bridgeAuth{
		env: func(key string) string {
			return map[string]string{"BL_API_KEY": "bl_key"}[key]
		},
		loadConfig:  func() (blaxel.Config, error) { return blaxel.Config{Context: blaxel.ContextConfig{Workspace: "ws"}}, nil },
		environment: func(string) string { return "https://api.blaxel.ai/v0" }, now: time.Now,
	}
	credentials, err := authenticator.resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer bl_key", credentials.headers["X-Blaxel-Authorization"])
	assert.Equal(t, "ws", credentials.headers["X-Blaxel-Workspace"])
	assert.Equal(t, "https://api.blaxel.ai/v0/mcp", credentials.endpoint)

	// No workspace at all is no login.
	authenticator = &bridgeAuth{env: func(string) string { return "" }, loadConfig: func() (blaxel.Config, error) { return blaxel.Config{}, nil }}
	_, err = authenticator.resolve(context.Background())
	assert.ErrorIs(t, err, errNotLoggedIn)
}

func TestTokenLifetime(t *testing.T) {
	now := time.Now()
	issued, expires, ok := jwtLifetime(bridgeJWT(now.Add(-time.Hour), now.Add(time.Hour)))
	require.True(t, ok)
	assert.Equal(t, now.Add(time.Hour).Unix(), expires.Unix())
	assert.True(t, tokenFresh(issued, expires, now))
	assert.False(t, tokenFresh(issued, expires, now.Add(40*time.Minute)), "less than a fifth of the lifetime is left")
	_, _, ok = jwtLifetime("not-a-jwt")
	assert.False(t, ok)
}

func TestDefaultLoginWorkspace(t *testing.T) {
	names := []string{"calibrator", "main"}
	chosen, err := defaultLoginWorkspace(names, "", "main")
	require.NoError(t, err)
	assert.Equal(t, "main", chosen, "the current workspace is kept")
	chosen, err = defaultLoginWorkspace(names, "", "gone")
	require.NoError(t, err)
	assert.Equal(t, "calibrator", chosen)
	chosen, err = defaultLoginWorkspace(names, "calibrator", "main")
	require.NoError(t, err)
	assert.Equal(t, "calibrator", chosen)
	_, err = defaultLoginWorkspace(names, "nope", "main")
	assert.ErrorContains(t, err, "calibrator, main")
	_, err = defaultLoginWorkspace(nil, "", "")
	assert.Error(t, err)
}

// bl mcp must print nothing but JSON-RPC, even on a machine without a login.
func TestMCPCommandWritesOnlyJSONRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the CLI")
	}
	binary := filepath.Join(t.TempDir(), "bl")
	build := execCommand(t, "go", "build", "-o", binary, "..")
	require.NoError(t, build.Run())
	home := t.TempDir()
	cmd := execCommand(t, binary, "mcp")
	cmd.Dir = home
	cmd.Env = []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + os.Getenv("PATH"), "BL_INSTALL_SKILLS=false"}
	cmd.Stdin = strings.NewReader(initializeMessage + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), stderr.String())
	answers := decodeLines(t, stdout.String())
	require.Len(t, answers, 2)
	assert.Equal(t, mcpLoginTool, answerFor(t, answers, 2)["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)["name"])
	_, err := os.Stat(filepath.Join(home, ".blaxel"))
	assert.True(t, errors.Is(err, os.ErrNotExist), "bl mcp writes no configuration")
}

func execCommand(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return exec.CommandContext(ctx, name, args...)
}
