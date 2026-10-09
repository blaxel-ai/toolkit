package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

var errMCPIncomplete = errors.New("blaxel MCP response ended before the request received a result; the call was not replayed")

type mcpHTTPError struct{ status int }

func (e *mcpHTTPError) Error() string {
	if e.status == http.StatusForbidden {
		return "Blaxel MCP answered HTTP 403: requested workspace is unavailable; check its name and your access"
	}
	return fmt.Sprintf("Blaxel MCP answered HTTP %d (the call was not replayed)", e.status)
}

// mcpResponseHeaderTimeout is how long the server may take to start answering.
// It is a variable so that a test can shorten it.
var mcpResponseHeaderTimeout = 30 * time.Second

func newMCPHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: mcpResponseHeaderTimeout,
		IdleConnTimeout: 90 * time.Second, MaxIdleConns: 20, MaxIdleConnsPerHost: 20,
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Refuse all redirects, including same-origin ones and token POST 307/308.
// Copy the client so test/custom transports cannot weaken credential confinement.
func doMCPRequest(client *http.Client, r *http.Request) (*http.Response, error) {
	if client == nil {
		client = newMCPHTTPClient()
	}
	hardened := *client
	hardened.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return hardened.Do(r)
}

func (b *mcpBridge) forward(ctx context.Context, message []byte, m rpcEnvelope, c mcpCredentials, relay func([]byte)) error {
	err := b.exchange(ctx, message, m, c, relay)
	var status *mcpHTTPError
	if errors.As(err, &status) && status.status == http.StatusUnauthorized {
		// A definitive HTTP 401 is pre-dispatch. Refresh and retry only this case,
		// exactly once. Network/SSE/5xx/timeout failures must never replay writes.
		b.auth.reject(c)
		fresh, authErr := b.auth.resolve(ctx)
		if authErr != nil {
			return authErr
		}
		err = b.exchange(ctx, message, m, fresh, relay)
		// A second 401 is not permanently recorded against stored credentials.
		if errors.As(err, &status) && status.status == 401 {
			err = errors.New("blaxel MCP refused the refreshed access token; retry shortly or run `bl login` and reconnect")
		}
	}
	return err
}

func (b *mcpBridge) exchange(ctx context.Context, message []byte, m rpcEnvelope, c mcpCredentials, relay func([]byte)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(message))
	if err != nil {
		return errors.New("invalid Blaxel MCP endpoint")
	}
	for name, value := range c.headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("User-Agent", b.userAgent)
	b.stateMu.Lock()
	session, version := b.session, b.protocolVersion
	b.stateMu.Unlock()
	if session != "" {
		request.Header.Set("Mcp-Session-Id", session)
	}
	if version != "" {
		request.Header.Set("MCP-Protocol-Version", version)
	}
	response, err := doMCPRequest(b.client, request)
	if err != nil {
		var netErr net.Error
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return errors.New("the Blaxel MCP call timed out (the call was not replayed)")
		case errors.As(err, &netErr) && netErr.Timeout():
			return fmt.Errorf("the Blaxel MCP server did not answer within %s (the call was not replayed)", mcpResponseHeaderTimeout)
		}
		return errors.New("cannot reach the Blaxel MCP server; retry shortly (the call was not replayed)")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 300 {
		return &mcpHTTPError{status: response.StatusCode}
	}
	if session := response.Header.Get("Mcp-Session-Id"); session != "" {
		b.stateMu.Lock()
		b.session = session
		b.stateMu.Unlock()
	}
	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNoContent {
		if m.isRequest() {
			return errMCPIncomplete
		}
		return nil
	}
	matched := !m.isRequest()
	send := func(payload []byte) {
		if ctx.Err() != nil {
			return
		}
		var answer rpcEnvelope
		if json.Unmarshal(payload, &answer) != nil || answer.JSONRPC != "2.0" {
			return
		}
		if answer.Method == "" && string(answer.ID) == string(m.ID) && (len(answer.Result) > 0 || len(answer.Error) > 0) {
			matched = true
		}
		relay(payload)
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType == "text/event-stream" {
		err = relayEvents(response.Body, send)
	} else {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, mcpBridgeMaxMessage+1))
		if readErr != nil {
			return errMCPIncomplete
		}
		if len(body) > mcpBridgeMaxMessage {
			return fmt.Errorf("blaxel MCP response exceeds %d bytes", mcpBridgeMaxMessage)
		}
		body = bytes.TrimSpace(body)
		if len(body) > 0 {
			if body[0] != '{' || !json.Valid(body) {
				return errors.New("invalid Blaxel MCP JSON-RPC response")
			}
			send(body)
		}
	}
	if err != nil && !matched {
		return fmt.Errorf("%w: %v", errMCPIncomplete, err)
	}
	if !matched {
		return errMCPIncomplete
	}
	return nil
}

// Event limits include all fields, not just each scanner line. Events dispatch
// on a blank line, and scanner/read errors and truncated events are explicit.
func relayEvents(body io.Reader, relay func([]byte)) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64<<10), mcpBridgeMaxMessage)
	var data bytes.Buffer
	eventSize := 0
	for scanner.Scan() {
		line := scanner.Text()
		eventSize += len(line) + 1
		if eventSize > mcpBridgeMaxMessage {
			return errors.New("SSE event exceeds the MCP message limit")
		}
		if line == "" {
			payload := bytes.TrimSpace(data.Bytes())
			if len(payload) > 0 {
				if payload[0] != '{' || !json.Valid(payload) {
					return errors.New("invalid SSE JSON-RPC event")
				}
				relay(append([]byte(nil), payload...))
			}
			data.Reset()
			eventSize = 0
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.New("SSE read failed or line exceeds the MCP message limit")
	}
	if eventSize > 0 {
		return errors.New("SSE ended in a partial event")
	}
	return nil
}
