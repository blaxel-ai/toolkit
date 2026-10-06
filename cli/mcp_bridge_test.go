package cli

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBridge returns a bridge signed in with an API key to a test server.
func testBridge(t *testing.T, handler http.Handler) *mcpBridge {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	b := newMCPBridge(bridgeTestAuth(t, server, blaxel.Credentials{APIKey: "token-1"}))
	b.version, b.userAgent, b.client = "9.9.9", "blaxel-cli/test (bl mcp)", server.Client()
	return b
}

// loggedOutBridge returns a bridge without a login.
func loggedOutBridge() *mcpBridge { return newMCPBridge(testBridgeAuth(nil)) }

func jsonResult(w http.ResponseWriter, id json.RawMessage, result string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(id) + `,"result":` + result + `}`))
}

// holdUntilClientLeaves answers nothing until the bridge gives up on the call.
func holdUntilClientLeaves(_ http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	<-r.Context().Done()
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

func (s *syncBuffer) waitFor(t *testing.T, count int) []map[string]any {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		s.mu.Lock()
		answers := decodeLines(t, s.buf.String())
		s.mu.Unlock()
		if len(answers) >= count {
			return answers
		}
	}
	t.Fatalf("waited for %d messages", count)
	return nil
}

const initializeMessage = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

func TestBridgeRelaysJSONAndSignsRequestsIn(t *testing.T) {
	var mu sync.Mutex
	headers := map[string]http.Header{}
	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m rpcEnvelope
		_ = json.NewDecoder(r.Body).Decode(&m)
		mu.Lock()
		headers[m.Method] = r.Header
		mu.Unlock()
		switch m.Method {
		case "initialize":
			jsonResult(w, m.ID, `{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"blaxel-mcp-server","version":"1"}}`)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		default:
			jsonResult(w, m.ID, `{"tools":[{"name":"list_sandboxes"}]}`)
		}
	}))
	answers := runBridge(t, b, initializeMessage, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	require.Len(t, answers, 2, "notifications get no answer")
	initialize := answerFor(t, answers, 1)["result"].(map[string]any)
	assert.Equal(t, "2025-03-26", initialize["protocolVersion"])
	assert.Contains(t, initialize["instructions"], "workspace main", "the agent learns which workspace the tools use")
	assert.Equal(t, "list_sandboxes", answerFor(t, answers, 2)["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)["name"])

	list := headers["tools/list"]
	assert.Equal(t, "Bearer token-1", list.Get("Authorization"))
	assert.Equal(t, "main", list.Get("X-Blaxel-Workspace"))
	assert.Equal(t, "2025-03-26", list.Get("MCP-Protocol-Version"))
	assert.Equal(t, "blaxel-cli/test (bl mcp)", list.Get("User-Agent"))
	assert.Contains(t, list.Get("Accept"), "text/event-stream")
	assert.Empty(t, headers["initialize"].Get("MCP-Protocol-Version"))
}

func TestBridgeRelaysServerSentEvents(t *testing.T) {
	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{\"progress\":1}}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"jsonrpc\":\"2.0\",\"id\":7,\n")
		_, _ = fmt.Fprint(w, "data: \"result\":{\"content\":[{\"type\":\"text\",\"text\":\"done\"}]}}\n\n")
	}))
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"run"}}`)
	require.Len(t, answers, 2)
	assert.Equal(t, "notifications/progress", answers[0]["method"])
	text, isError := toolText(t, answerFor(t, answers, 7))
	assert.Equal(t, "done", text)
	assert.False(t, isError)
}

func TestBridgeTurnsRefusedCallsIntoToolErrors(t *testing.T) {
	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	answers := runBridge(t, b,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_sandboxes","arguments":{"workspace":"other"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	text, isError := toolText(t, answerFor(t, answers, 1))
	assert.True(t, isError)
	assert.Contains(t, text, "requested workspace is unavailable")
	assert.Contains(t, answerFor(t, answers, 2)["error"].(map[string]any)["message"], "403")
}

func TestBridgeStandsInBeforeLogin(t *testing.T) {
	answers := runBridge(t, loggedOutBridge(), initializeMessage, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_sandboxes"}}`)
	initialize := answerFor(t, answers, 1)["result"].(map[string]any)
	assert.Equal(t, "2025-03-26", initialize["protocolVersion"])
	assert.Contains(t, initialize["instructions"], mcpLoginInstructions)
	assert.Empty(t, answerFor(t, answers, 2)["result"].(map[string]any)["tools"])
	text, isError := toolText(t, answerFor(t, answers, 3))
	assert.True(t, isError)
	assert.Contains(t, text, mcpLoginInstructions)
}

func TestBridgeAnswersInitializeFirstAndCancels(t *testing.T) {
	started := make(chan struct{})
	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m rpcEnvelope
		_ = json.NewDecoder(r.Body).Decode(&m)
		switch m.Method {
		case "initialize":
			time.Sleep(50 * time.Millisecond)
			jsonResult(w, m.ID, `{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"s","version":"1"},"instructions":"server text"}`)
		case "tools/call":
			close(started)
			<-r.Context().Done()
		default:
			jsonResult(w, m.ID, `{"tools":[]}`)
		}
	}))
	in, writer := io.Pipe()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- b.serve(context.Background(), in, &out) }()
	_, _ = writer.Write([]byte(initializeMessage + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"slow"}}` + "\n"))
	answers := out.waitFor(t, 2)
	assert.Equal(t, float64(1), answers[0]["id"], "initialize is answered before anything sent after it")
	assert.Equal(t, "server text", answers[0]["result"].(map[string]any)["instructions"], "server instructions are kept")
	<-started
	_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":3}}` + "\n"))
	_ = writer.Close()
	require.NoError(t, <-done)
	assert.Len(t, out.waitFor(t, 2), 2, "a cancelled request gets no answer")
}

// A call that fails gets an error answer, and is sent once: it is never replayed.
func TestBridgeAnswersFailedCallsOnceAndNeverReplaysThem(t *testing.T) {
	for _, kind := range []string{"502", "dropped", "hangs", "sse without a result", "sse partial", "empty", "202", "oversize line", "oversize body"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch kind {
				case "502":
					w.WriteHeader(http.StatusBadGateway)
					_, _ = io.WriteString(w, "upstream-body-must-not-be-echoed")
				case "dropped":
					conn, _, _ := w.(http.Hijacker).Hijack()
					_ = conn.Close()
				case "hangs":
					holdUntilClientLeaves(w, r)
				case "sse without a result":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "id: resume-me\ndata:\nretry: 10\n\n")
				case "sse partial":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {\"jsonrpc\":\"2.0\",\"id\":8,\"result\":{}}\n")
				case "202":
					w.WriteHeader(http.StatusAccepted)
				case "oversize line":
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: "+strings.Repeat("x", mcpBridgeMaxMessage+1))
				case "oversize body":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":8,"result":"`+strings.Repeat("x", mcpBridgeMaxMessage)+`"}`)
				}
			}))
			b.requestTimeout = 100 * time.Millisecond
			answers := runBridge(t, b, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"write"}}`)
			answer := answerFor(t, answers, 8)
			result, _ := answer["result"].(map[string]any)
			assert.True(t, answer["error"] != nil || result["isError"] == true, "the call gets an error answer: %v", answer)
			assert.EqualValues(t, 1, calls.Load())
			assert.NotContains(t, fmt.Sprint(answers), "upstream-body-must-not-be-echoed")
		})
	}
}

// An SSE event is limited as a whole, not just per line, and a failure after the
// result does not answer twice.
func TestBridgeSSEEvents(t *testing.T) {
	var event strings.Builder
	event.WriteString("data: {\"items\":[\n")
	for range 33 {
		event.WriteString("data: \"" + strings.Repeat("x", 1<<20) + "\",\n")
	}
	event.WriteString("data: 0]}\n\n")
	relayed := false
	require.Error(t, relayEvents(strings.NewReader(event.String()), func([]byte) { relayed = true }))
	assert.False(t, relayed)

	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\ndata: partial")
	}))
	answers := runBridge(t, b, `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	require.Len(t, answers, 1)
	assert.NotNil(t, answers[0]["result"])
}

func TestBridgeRedirectsDoNotLeakCredentials(t *testing.T) {
	var leaks atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaks.Add(1) }))
	defer sink.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	a := bridgeTestAuth(t, source, blaxel.Credentials{APIKey: "token-1"})
	answers := runBridge(t, newMCPBridge(a), `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	_, isError := toolText(t, answerFor(t, answers, 1))
	assert.True(t, isError)
	_, err := a.refresh(context.Background(), blaxel.Credentials{RefreshToken: "fake-refresh"})
	require.Error(t, err)
	assert.Zero(t, leaks.Load(), "neither the bearer token nor a token grant reaches a redirect target")
}

func TestBridgeRejectsBatches(t *testing.T) {
	answers := runBridge(t, loggedOutBridge(), initializeMessage, `[{"jsonrpc":"2.0","id":2,"method":"ping"}]`)
	require.Len(t, answers, 2)
	assert.EqualValues(t, -32600, answers[1]["error"].(map[string]any)["code"])
}

// Sixteen calls run at once; the next is refused, and cancellations and pings
// still get through.
func TestBridgeBoundsInFlightCalls(t *testing.T) {
	started := make(chan struct{}, mcpBridgeMaxInFlight)
	cancelled := make(chan struct{}, 1)
	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m rpcEnvelope
		_ = json.NewDecoder(r.Body).Decode(&m)
		if m.Method != "tools/call" {
			w.WriteHeader(http.StatusAccepted)
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
	for i := 1; i <= mcpBridgeMaxInFlight; i++ {
		_, _ = fmt.Fprintf(w, "{\"jsonrpc\":\"2.0\",\"id\":%d,\"method\":\"tools/call\"}\n", i)
		<-started
	}
	_, _ = io.WriteString(w, "{\"jsonrpc\":\"2.0\",\"id\":17,\"method\":\"tools/call\"}\n"+
		"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/cancelled\",\"params\":{\"requestId\":1}}\n"+
		"{\"jsonrpc\":\"2.0\",\"id\":18,\"method\":\"ping\"}\n")
	answers := out.waitFor(t, 2)
	assert.Contains(t, answerFor(t, answers, 17)["error"].(map[string]any)["message"], "busy")
	assert.NotNil(t, answerFor(t, answers, 18)["result"])
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked by saturation")
	}
	cancel()
	require.NoError(t, <-done)
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// The bridge stops when stdout breaks, and when stdin closes with a call stuck.
func TestBridgeStopsPromptly(t *testing.T) {
	in, w := io.Pipe()
	defer func() { _ = w.Close() }()
	done := make(chan error, 1)
	go func() { done <- loggedOutBridge().serve(context.Background(), in, brokenWriter{}) }()
	_, _ = io.WriteString(w, initializeMessage+"\n")
	select {
	case err := <-done:
		require.ErrorIs(t, err, io.ErrClosedPipe)
	case <-time.After(time.Second):
		t.Fatal("a stdout failure did not shut the bridge down")
	}

	start := time.Now()
	runBridge(t, testBridge(t, http.HandlerFunc(holdUntilClientLeaves)), `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// bl mcp must print nothing but JSON-RPC, even on a machine without a login.
func TestMCPCommandWritesOnlyJSONRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the CLI")
	}
	binary := filepath.Join(t.TempDir(), "bl"+exeSuffix())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	require.NoError(t, exec.CommandContext(ctx, "go", "build", "-o", binary, "..").Run())
	home := t.TempDir()
	cmd := exec.CommandContext(ctx, binary, "--workspace", "main", "--skip-version-warning", "mcp")
	cmd.Dir = home
	cmd.Env = []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + os.Getenv("PATH"), "BL_INSTALL_SKILLS=false"}
	cmd.Stdin = strings.NewReader(initializeMessage + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), stderr.String())
	answers := decodeLines(t, stdout.String())
	require.Len(t, answers, 2)
	assert.Empty(t, answerFor(t, answers, 2)["result"].(map[string]any)["tools"])
	assert.NoDirExists(t, filepath.Join(home, ".blaxel"), "bl mcp writes no configuration")
}

// bl mcp sends credentials only to the stored login's Blaxel origin: no flag
// can point it elsewhere.
func TestMCPCommandHasNoAPIURLFlag(t *testing.T) {
	assert.Nil(t, MCPCmd().Flags().Lookup("api-url"))
}

// A failed start says what failed, not that the agent is logged out.
func TestBridgeNamesTheCauseOfAStartupFailure(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	tokenDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer tokenDown.Close()
	unreachable := testBridge(t, http.NotFoundHandler())
	unreachable.auth.(*bridgeAuth).baseURL = closed.URL
	for name, test := range map[string]struct {
		bridge *mcpBridge
		want   string
	}{
		"server error": {testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })), "HTTP 500"},
		"unreachable":  {unreachable, "cannot reach"},
		"token endpoint down": {newMCPBridge(bridgeTestAuth(t, tokenDown, blaxel.Credentials{AccessToken: "expired", RefreshToken: "fake"})),
			"HTTP 503"},
	} {
		t.Run(name, func(t *testing.T) {
			answers := runBridge(t, test.bridge, initializeMessage)
			failure, _ := answerFor(t, answers, 1)["error"].(map[string]any)
			require.NotNil(t, failure, "initialize reports the failure: %v", answers)
			assert.Contains(t, failure["message"], test.want)
			assert.NotContains(t, failure["message"], "bl login")
		})
	}
}

// One failed call does not make unrelated calls fail.
func TestBridgeOneFailureDoesNotFailOtherCalls(t *testing.T) {
	var calls atomic.Int32
	b := testBridge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m rpcEnvelope
		_ = json.NewDecoder(r.Body).Decode(&m)
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		jsonResult(w, m.ID, `{"content":[{"type":"text","text":"deleted"}]}`)
	}))
	first := runBridge(t, b, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_sandbox_command"}}`)
	_, failed := toolText(t, answerFor(t, first, 1))
	assert.True(t, failed)
	second := runBridge(t, b, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"delete_sandbox"}}`)
	text, isError := toolText(t, answerFor(t, second, 2))
	assert.Equal(t, "deleted", text)
	assert.False(t, isError)
}

// bl logout stops a running bridge: the next call is refused without reaching
// the server, and tells the agent to log in again.
func TestBridgeStopsAfterLogout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BL_API_KEY", "")
	t.Setenv("BL_CLIENT_CREDENTIALS", "")
	now := time.Now()
	require.NoError(t, blaxel.WriteConfig(blaxel.Config{
		Context: blaxel.ContextConfig{Workspace: "main"},
		Workspaces: []blaxel.WorkspaceConfig{
			{Name: "main", Credentials: blaxel.Credentials{AccessToken: bridgeJWT(now, now.Add(2*time.Hour)), RefreshToken: "fake"}},
			{Name: "other", Credentials: blaxel.Credentials{APIKey: "bl_other"}},
		},
	}))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m rpcEnvelope
		_ = json.NewDecoder(r.Body).Decode(&m)
		calls.Add(1)
		jsonResult(w, m.ID, `{"content":[{"type":"text","text":"sandboxes"}]}`)
	}))
	defer server.Close()
	a := newBridgeAuth("")
	a.client, a.baseURL, a.pinnedEnv = server.Client(), server.URL+"/v0", "prod"
	b := newMCPBridge(a)
	b.client = server.Client()
	call := func(id int) map[string]any {
		answers := runBridge(t, b, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"list_sandboxes"}}`, id))
		return answerFor(t, answers, float64(id))
	}

	text, isError := toolText(t, call(1))
	assert.Equal(t, "sandboxes", text)
	assert.False(t, isError)
	require.EqualValues(t, 1, calls.Load())

	require.NoError(t, clearCredentials("main"), "what bl logout main does")
	text, isError = toolText(t, call(2))
	assert.True(t, isError)
	assert.Contains(t, text, mcpLoginInstructions)
	assert.EqualValues(t, 1, calls.Load(), "the refused call never reaches the server")
}

// A timeout is reported as a timeout, not as an unreachable server.
func TestBridgeReportsTimeoutsAsTimeouts(t *testing.T) {
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_sandbox_command"}}`

	b := testBridge(t, http.HandlerFunc(holdUntilClientLeaves))
	mcpResponseHeaderTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mcpResponseHeaderTimeout = 30 * time.Second })
	b.client = newMCPHTTPClient()
	text, isError := toolText(t, answerFor(t, runBridge(t, b, call), 1))
	assert.True(t, isError)
	assert.Contains(t, text, "did not answer within 50ms")
	assert.NotContains(t, text, "cannot reach")

	b = testBridge(t, http.HandlerFunc(holdUntilClientLeaves))
	b.requestTimeout = 50 * time.Millisecond
	text, isError = toolText(t, answerFor(t, runBridge(t, b, call), 1))
	assert.True(t, isError)
	assert.Contains(t, text, "timed out")
}
