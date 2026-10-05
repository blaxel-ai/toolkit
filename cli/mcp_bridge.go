package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

const (
	mcpBridgeMaxInFlight = 16
	mcpBridgeMaxMessage  = 32 << 20
	mcpLoginPoll         = 2 * time.Second
	mcpRefreshTimeout    = 30 * time.Second
	mcpRequestTimeout    = 10 * time.Minute
	mcpDrainTimeout      = 3 * time.Second
	mcpLoginInstructions = "Run `bl login` in a terminal, then restart or reconnect this agent."
)

var mcpProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

func init() { core.RegisterCommand("mcp", MCPCmd) }

func MCPCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use: "mcp", Short: "Serve the Blaxel MCP server to a coding agent, signed in with your bl login",
		Long: `Serve the hosted Blaxel MCP tools over stdio using your existing bl login.

Run bl login before starting your agent. bl setup configures local MCP targets;
no token is stored in agent configurations and no separate MCP OAuth is needed.
Without a usable login, the connection still initializes, with an empty tool
list and instructions to run bl login, then restart or reconnect the agent.
Some agents discover tools live after login; reconnect is the reliable fallback.

The default workspace is pinned when the bridge starts resolving credentials:
the current bl workspace, --workspace or BL_WORKSPACE. A tool's workspace
argument can intentionally select another authorized workspace. This pin is
not a restriction on the user's authority.

Tokens refresh in memory only. bl logout removes local credentials, affecting
future requests, but does not revoke refresh grants or work already in flight.
BL_API_KEY and BL_CLIENT_CREDENTIALS override stored credentials and are not
removed by bl logout. Environment inheritance varies between agent clients.

The trusted HTTPS origin comes from the stored workspace environment (prod or
dev), not inherited BL_API_URL or BL_ENV. --api-url explicitly opts into sending
credentials to a custom HTTPS origin and prints a warning to stderr. Redirects
are refused for both MCP requests and token exchanges.`,
		Example: `  claude mcp add --scope user blaxel -- bl mcp

  {"mcpServers": {"blaxel": {"command": "/absolute/path/to/bl", "args": ["mcp"]}}}`,
		Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, _ := explicitWorkspaceFlag(cmd)
			if workspace == "" {
				workspace = strings.TrimSpace(os.Getenv("BL_WORKSPACE"))
			}
			authenticator := newBridgeAuth(workspace)
			if apiURL != "" {
				base, err := bridgeBaseURL("", apiURL)
				if err != nil {
					return err
				}
				authenticator.customURL = base
				_, _ = fmt.Fprintf(os.Stderr, "bl mcp: explicitly sending Blaxel credentials to custom origin %s\n", base)
			}
			if os.Getenv("BL_API_URL") != "" || os.Getenv("BL_ENV") != "" {
				_, _ = fmt.Fprintln(os.Stderr, "bl mcp: ignoring inherited BL_API_URL/BL_ENV; using the stored workspace environment")
			}
			if os.Getenv("BL_API_KEY") != "" {
				_, _ = fmt.Fprintln(os.Stderr, "bl mcp: using BL_API_KEY (bl logout does not remove it)")
			} else if os.Getenv("BL_CLIENT_CREDENTIALS") != "" {
				_, _ = fmt.Fprintln(os.Stderr, "bl mcp: using BL_CLIENT_CREDENTIALS (bl logout does not remove it)")
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return newMCPBridge(authenticator).serve(ctx, os.Stdin, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&apiURL, "api-url", "", "Explicit trusted HTTPS API origin for this bridge (credentials will be sent there)")
	return cmd
}

// This is a stateless POST relay, not a second hosted tool implementation.
// Stateful GET streams/resumption and newer protocol features are deliberately
// not advertised by synthetic initialization.
type mcpBridge struct {
	auth                     mcpAuthenticator
	client                   *http.Client
	userAgent, version       string
	logf                     func(string, ...any)
	poll                     time.Duration
	requestTimeout           time.Duration
	out                      io.Writer
	writeMu                  sync.Mutex
	writeErr                 error
	shutdown                 context.CancelFunc
	stateMu                  sync.Mutex
	session, protocolVersion string
	initialize               json.RawMessage
	upstreamFingerprint      string
	initGate                 chan struct{}
	cancels                  map[string]context.CancelFunc
	listed                   string // "empty" or "tools", or "" before listing
	retryAt                  time.Time
	failures                 int
}

func newMCPBridge(a mcpAuthenticator) *mcpBridge {
	return &mcpBridge{auth: a, client: newMCPHTTPClient(), version: core.GetVersion(), userAgent: "blaxel-cli/" + core.GetVersion() + " (bl mcp)", poll: mcpLoginPoll, initGate: make(chan struct{}, 1), logf: func(format string, args ...any) { _, _ = fmt.Fprintf(os.Stderr, "bl mcp: "+format+"\n", args...) }}
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
	Params  struct {
		RequestID       json.RawMessage `json:"requestId"`
		ProtocolVersion string          `json:"protocolVersion"`
		Capabilities    json.RawMessage `json:"capabilities"`
		Name            string          `json:"name"`
	} `json:"params"`
}

func (m rpcEnvelope) isRequest() bool {
	return m.Method != "" && len(m.ID) > 0 && string(m.ID) != "null"
}

func (b *mcpBridge) serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	b.out, b.shutdown = out, cancel
	// Closing real stdin unblocks Scan on SIGTERM. The select also returns
	// promptly for readers that are not closable (their owner must unblock Read).
	defer func() {
		if closer, ok := in.(io.Closer); ok {
			_ = closer.Close()
		}
	}()
	go b.watchLogin(ctx)
	type input struct {
		message []byte
		err     error
	}
	lines := make(chan input, 1)
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 64<<10), mcpBridgeMaxMessage)
		defer close(lines)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- input{message: line}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- input{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	slots := make(chan struct{}, mcpBridgeMaxInFlight)
	controls := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var readErr error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case line, ok := <-lines:
			if !ok {
				break loop
			}
			if line.err != nil {
				readErr = line.err
				break loop
			}
			message := bytes.TrimSpace(line.message)
			if len(message) == 0 {
				continue
			}
			var envelope rpcEnvelope
			if !json.Valid(message) {
				b.writeError(json.RawMessage("null"), -32700, "parse error")
				continue
			}
			if message[0] != '{' || json.Unmarshal(message, &envelope) != nil || envelope.JSONRPC != "2.0" || (envelope.Method == "" && len(envelope.Result) == 0 && len(envelope.Error) == 0) {
				b.writeError(json.RawMessage("null"), -32600, "invalid request: JSON-RPC batches are not supported")
				continue
			}
			if envelope.Method == "notifications/cancelled" {
				b.cancel(string(envelope.Params.RequestID))
			}
			if envelope.Method == "ping" {
				if envelope.isRequest() {
					b.writeResult(envelope.ID, map[string]any{})
				}
				continue
			}
			// Complete negotiation before accepting subsequent requests.
			if envelope.Method == "initialize" {
				b.handle(ctx, message, envelope)
				continue
			}
			pool := slots
			if !envelope.isRequest() {
				pool = controls
			}
			select {
			case pool <- struct{}{}:
				requestCtx, requestCancel := context.WithCancel(ctx)
				if envelope.isRequest() {
					b.track(string(envelope.ID), requestCancel)
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						<-pool
						requestCancel()
						if envelope.isRequest() {
							b.untrack(string(envelope.ID))
						}
					}()
					b.handle(requestCtx, message, envelope)
				}()
			default:
				b.fail(ctx, envelope, -32000, "Blaxel MCP bridge is busy; retry after an in-flight request finishes")
			}
		}
	}
	drained := make(chan struct{})
	go func() { wg.Wait(); close(drained) }()
	if ctx.Err() == nil {
		timer := time.NewTimer(mcpDrainTimeout)
		select {
		case <-drained:
		case <-timer.C:
		case <-ctx.Done():
		}
		timer.Stop()
	}
	cancel()
	// Network contexts are cancelled; do not wait indefinitely for a broken peer.
	b.writeMu.Lock()
	err := b.writeErr
	b.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("writing MCP stdout: %w", err)
	}
	return readErr
}

func (b *mcpBridge) handle(ctx context.Context, message []byte, envelope rpcEnvelope) {
	parentCtx := ctx
	timeout := b.requestTimeout
	if timeout <= 0 {
		timeout = mcpRequestTimeout
	}
	if envelope.Method == "initialize" {
		timeout = mcpRefreshTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if envelope.Method == "initialize" {
		b.stateMu.Lock()
		b.initialize = append([]byte(nil), message...)
		b.stateMu.Unlock()
	}
	c, err := b.auth.resolve(ctx)
	if err != nil {
		if envelope.Method == "initialize" || errors.Is(err, errNotLoggedIn) {
			b.answerLoggedOut(parentCtx, envelope)
		} else {
			b.requestError(parentCtx, envelope, err.Error())
		}
		return
	}
	if envelope.isRequest() && envelope.Method != "initialize" {
		if err = b.ensureUpstream(ctx, c); err != nil {
			if errors.Is(err, errNotLoggedIn) {
				b.answerLoggedOut(ctx, envelope)
			} else {
				b.requestError(parentCtx, envelope, err.Error())
			}
			return
		}
	}
	err = b.forward(ctx, message, envelope, c, func(p []byte) { b.relay(p, envelope, c) })
	if err != nil {
		if envelope.Method == "initialize" {
			b.answerLoggedOut(parentCtx, envelope)
		} else if errors.Is(err, errMCPIncomplete) {
			b.fail(parentCtx, envelope, -32603, err.Error())
		} else if errors.Is(err, errNotLoggedIn) {
			b.answerLoggedOut(ctx, envelope)
			b.notifyToolsChanged(ctx)
		} else {
			b.requestError(parentCtx, envelope, err.Error())
		}
	}
}

func (b *mcpBridge) answerLoggedOut(ctx context.Context, m rpcEnvelope) {
	b.stateMu.Lock()
	b.session, b.upstreamFingerprint = "", ""
	b.stateMu.Unlock()
	if !m.isRequest() || ctx.Err() != nil {
		return
	}
	switch m.Method {
	case "initialize":
		version := mcpProtocolVersions[0]
		if slices.Contains(mcpProtocolVersions, m.Params.ProtocolVersion) {
			version = m.Params.ProtocolVersion
		}
		b.stateMu.Lock()
		b.protocolVersion = version
		b.session = ""
		b.upstreamFingerprint = ""
		b.stateMu.Unlock()
		b.writeResult(m.ID, map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{"listChanged": true}}, "serverInfo": map[string]string{"name": "blaxel", "version": b.version}, "instructions": "Not connected to Blaxel. " + mcpLoginInstructions})
	case "ping":
		b.writeResult(m.ID, map[string]any{})
	case "tools/list":
		b.setListed("empty")
		b.writeResult(m.ID, map[string]any{"tools": []any{}})
	case "tools/call":
		b.writeToolText(m.ID, "Not logged in to Blaxel. "+mcpLoginInstructions, true)
	default:
		b.writeError(m.ID, -32601, mcpLoginInstructions)
	}
}

// After synthetic initialize, establish the upstream exchange with the client's
// actual capabilities. Its response is consumed locally, never a second result.
func (b *mcpBridge) ensureUpstream(ctx context.Context, c mcpCredentials) error {
	select {
	case b.initGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-b.initGate }()
	b.stateMu.Lock()
	message := append([]byte(nil), b.initialize...)
	version := b.protocolVersion
	ready := b.upstreamFingerprint == c.fingerprint
	b.stateMu.Unlock()
	if ready || len(message) == 0 {
		return nil
	}
	var request map[string]any
	_ = json.Unmarshal(message, &request)
	params, _ := request["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
		request["params"] = params
	}
	params["protocolVersion"] = version
	message, _ = json.Marshal(request)
	var envelope rpcEnvelope
	_ = json.Unmarshal(message, &envelope)
	b.stateMu.Lock()
	b.session = ""
	b.stateMu.Unlock()
	var negotiationErr error
	err := b.forward(ctx, message, envelope, c, func(p []byte) {
		var answer struct {
			ID     json.RawMessage `json:"id"`
			Result struct {
				ProtocolVersion string `json:"protocolVersion"`
			} `json:"result"`
		}
		if json.Unmarshal(p, &answer) != nil {
			return
		}
		if string(answer.ID) != string(envelope.ID) {
			b.write(p)
			return
		}
		if answer.Result.ProtocolVersion != version {
			negotiationErr = errors.New("upstream MCP negotiation changed; restart or reconnect this agent")
		}
	})
	if err != nil {
		return err
	}
	if negotiationErr != nil {
		return negotiationErr
	}
	initialized := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if err = b.forward(ctx, initialized, rpcEnvelope{Method: "notifications/initialized"}, c, func([]byte) {}); err != nil {
		return err
	}
	b.stateMu.Lock()
	b.upstreamFingerprint = c.fingerprint
	b.stateMu.Unlock()
	return nil
}

func (b *mcpBridge) watchLogin(ctx context.Context) {
	if b.poll <= 0 {
		return
	}
	ticker := time.NewTicker(b.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.stateMu.Lock()
			waiting := b.listed == "empty"
			b.stateMu.Unlock()
			if waiting {
				b.notifyToolsChanged(ctx)
			}
		}
	}
}
func (b *mcpBridge) notifyToolsChanged(ctx context.Context) {
	b.stateMu.Lock()
	listed := b.listed
	b.stateMu.Unlock()
	if listed == "" {
		return
	}
	_, err := b.auth.resolve(ctx)
	if ctx.Err() != nil {
		return
	}
	if err != nil && !errors.Is(err, errNotLoggedIn) {
		return
	}
	if (listed == "empty") == (err == nil) {
		b.setListed("")
		b.write([]byte(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`))
	}
}
func (b *mcpBridge) setListed(s string) { b.stateMu.Lock(); b.listed = s; b.stateMu.Unlock() }

func (b *mcpBridge) relay(message []byte, m rpcEnvelope, c mcpCredentials) {
	var answer struct {
		ID     json.RawMessage `json:"id"`
		Result map[string]any  `json:"result"`
	}
	if json.Unmarshal(message, &answer) == nil && answer.Result != nil && string(answer.ID) == string(m.ID) {
		switch m.Method {
		case "initialize":
			version, _ := answer.Result["protocolVersion"].(string)
			b.stateMu.Lock()
			b.protocolVersion = version
			b.upstreamFingerprint = c.fingerprint
			b.stateMu.Unlock()
			if _, has := answer.Result["instructions"]; !has {
				answer.Result["instructions"] = "Blaxel tools default to workspace " + c.workspace + ". Pass a workspace argument to intentionally select another authorized workspace. " + mcpLoginInstructions
				b.writeResult(answer.ID, answer.Result)
				return
			}
		case "tools/list":
			b.setListed("tools")
		}
	}
	b.write(message)
}
func (b *mcpBridge) requestError(ctx context.Context, m rpcEnvelope, message string) {
	if ctx.Err() != nil || !m.isRequest() {
		return
	}
	if m.Method == "tools/call" {
		b.writeToolText(m.ID, message, true)
	} else {
		b.writeError(m.ID, -32603, message)
	}
}
func (b *mcpBridge) fail(ctx context.Context, m rpcEnvelope, code int, message string) {
	if m.isRequest() && ctx.Err() == nil {
		b.writeError(m.ID, code, message)
	}
}
func (b *mcpBridge) writeToolText(id json.RawMessage, text string, isError bool) {
	b.writeResult(id, map[string]any{"content": []any{map[string]string{"type": "text", "text": text}}, "isError": isError})
}
func (b *mcpBridge) writeResult(id json.RawMessage, result any) {
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	if err == nil {
		b.write(data)
	}
}
func (b *mcpBridge) writeError(id json.RawMessage, code int, message string) {
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
	if err == nil {
		b.write(data)
	}
}
func (b *mcpBridge) write(message []byte) {
	var line bytes.Buffer
	if json.Compact(&line, message) != nil {
		return
	}
	line.WriteByte('\n')
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if b.writeErr != nil {
		return
	}
	n, err := b.out.Write(line.Bytes())
	if err == nil && n != line.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		b.writeErr = err
		if b.shutdown != nil {
			b.shutdown()
		}
	}
}
func (b *mcpBridge) track(id string, cancel context.CancelFunc) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	if b.cancels == nil {
		b.cancels = map[string]context.CancelFunc{}
	}
	b.cancels[id] = cancel
}
func (b *mcpBridge) untrack(id string) { b.stateMu.Lock(); delete(b.cancels, id); b.stateMu.Unlock() }
func (b *mcpBridge) cancel(id string) {
	b.stateMu.Lock()
	cancel := b.cancels[id]
	b.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
