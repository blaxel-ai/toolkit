package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
)

const (
	mcpBridgeMaxInFlight = 16
	mcpBridgeMaxMessage  = 32 << 20
	mcpRefreshTimeout    = 30 * time.Second
	mcpRequestTimeout    = 10 * time.Minute
	mcpDrainTimeout      = 3 * time.Second
	mcpLoginInstructions = "Run `bl login` in a terminal, then restart or reconnect this agent."
)

var mcpProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// Serve relays MCP messages between in and out and the hosted Blaxel MCP
// server, signed in with the bl login. workspace pins the default workspace;
// empty uses the current one.
func Serve(ctx context.Context, workspace string, in io.Reader, out io.Writer) error {
	return newMCPBridge(newBridgeAuth(workspace)).serve(ctx, in, out)
}

// This is a stateless POST relay, not a second hosted tool implementation.
// Stateful GET streams/resumption and newer protocol features are deliberately
// not advertised by synthetic initialization.
type mcpBridge struct {
	auth               mcpAuthenticator
	client             *http.Client
	userAgent, version string
	requestTimeout     time.Duration
	out                io.Writer
	writeMu            sync.Mutex
	writeErr           error
	shutdown           context.CancelFunc
	stateMu            sync.Mutex
	session            string
	protocolVersion    string
	cancels            map[string]context.CancelFunc
}

func newMCPBridge(a mcpAuthenticator) *mcpBridge {
	return &mcpBridge{
		auth:      a,
		client:    newMCPHTTPClient(),
		version:   core.GetVersion(),
		userAgent: "blaxel-cli/" + core.GetVersion() + " (bl mcp)",
	}
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
	} `json:"params"`
}

func (m rpcEnvelope) valid() bool {
	return m.JSONRPC == "2.0" && (m.Method != "" || len(m.Result) > 0 || len(m.Error) > 0)
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
			if message[0] != '{' || json.Unmarshal(message, &envelope) != nil || !envelope.valid() {
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

func (b *mcpBridge) handle(ctx context.Context, message []byte, m rpcEnvelope) {
	timeout := b.requestTimeout
	if timeout <= 0 {
		timeout = mcpRequestTimeout
	}
	if m.Method == "initialize" {
		timeout = mcpRefreshTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c, err := b.auth.resolve(requestCtx)
	if err == nil {
		err = b.forward(requestCtx, message, m, c, func(p []byte) { b.relay(p, m, c.workspace) })
	}
	switch {
	case err == nil:
	case errors.Is(err, errNotLoggedIn):
		b.answerLoggedOut(ctx, m)
	case errors.Is(err, errMCPIncomplete):
		b.fail(ctx, m, -32603, err.Error())
	default:
		b.requestError(ctx, m, err.Error())
	}
}

// answerLoggedOut stands in for the hosted server: the agent still connects,
// sees no tools, and is told to log in.
func (b *mcpBridge) answerLoggedOut(ctx context.Context, m rpcEnvelope) {
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
		b.stateMu.Unlock()
		b.writeResult(m.ID, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "blaxel", "version": b.version},
			"instructions":    "Not connected to Blaxel. " + mcpLoginInstructions,
		})
	case "tools/list":
		b.writeResult(m.ID, map[string]any{"tools": []any{}})
	case "tools/call":
		b.writeToolText(m.ID, "Not logged in to Blaxel. "+mcpLoginInstructions, true)
	default:
		b.writeError(m.ID, -32601, mcpLoginInstructions)
	}
}

// relay writes what the hosted server sent. Its initialize answer is kept,
// plus a note on the default workspace when the server gave no instructions.
func (b *mcpBridge) relay(message []byte, m rpcEnvelope, workspace string) {
	var answer struct {
		ID     json.RawMessage `json:"id"`
		Result map[string]any  `json:"result"`
	}
	if m.Method == "initialize" && json.Unmarshal(message, &answer) == nil && answer.Result != nil && string(answer.ID) == string(m.ID) {
		version, _ := answer.Result["protocolVersion"].(string)
		b.stateMu.Lock()
		b.protocolVersion = version
		b.stateMu.Unlock()
		if _, has := answer.Result["instructions"]; !has {
			answer.Result["instructions"] = "Blaxel tools default to workspace " + workspace + ". Pass a workspace argument to intentionally select another authorized workspace."
			b.writeResult(answer.ID, answer.Result)
			return
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
	content := []any{map[string]string{"type": "text", "text": text}}
	b.writeResult(id, map[string]any{"content": content, "isError": isError})
}

func (b *mcpBridge) writeResult(id json.RawMessage, result any) {
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	if err == nil {
		b.write(data)
	}
}

func (b *mcpBridge) writeError(id json.RawMessage, code int, message string) {
	failure := map[string]any{"code": code, "message": message}
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": failure})
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

func (b *mcpBridge) untrack(id string) {
	b.stateMu.Lock()
	delete(b.cancels, id)
	b.stateMu.Unlock()
}

func (b *mcpBridge) cancel(id string) {
	b.stateMu.Lock()
	cancel := b.cancels[id]
	b.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
