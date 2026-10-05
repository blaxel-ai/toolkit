package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/auth"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

const (
	// mcpBridgeMaxInFlight bounds concurrent requests from one agent.
	mcpBridgeMaxInFlight = 16
	// mcpBridgeMaxMessage bounds one JSON-RPC message in either direction.
	mcpBridgeMaxMessage = 32 << 20
	// mcpLoginTool is the only tool offered before the bl login exists.
	mcpLoginTool = "blaxel_login"
	// mcpLoginPoll is how often a logged-out bridge looks for a bl login made
	// elsewhere, such as `bl login` in a terminal.
	mcpLoginPoll = 2 * time.Second
	// mcpRefreshTimeout bounds one token refresh.
	mcpRefreshTimeout = 30 * time.Second
)

// MCP protocol versions the bridge can answer initialize with by itself.
var mcpProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

func init() {
	core.RegisterCommand("mcp", MCPCmd)
}

func MCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve the Blaxel MCP server to a coding agent, signed in with your bl login",
		Long: `Serve the Blaxel MCP server to a coding agent over stdio, signed in with your bl login.

Coding agents start this command themselves: bl setup adds it to their MCP
configuration. Requests go to the hosted Blaxel MCP server, so agents need no
sign-in of their own and no token is stored in their configuration.

The workspace is the current one when the agent starts (see bl workspaces),
--workspace or BL_WORKSPACE. It stays the same for the agent's session; tools
take a workspace argument to use another workspace you belong to. BL_API_KEY
and BL_CLIENT_CREDENTIALS work as for every other bl command.

Without a login, the agent gets one tool, blaxel_login, which opens the Blaxel
login page in your browser. Once you confirm, the Blaxel tools appear in the
agent; you can also run bl login in a terminal.`,
		Example: `  # Claude Code
  claude mcp add --scope user blaxel -- bl mcp

  # Any agent that starts local (stdio) MCP servers
  {"mcpServers": {"blaxel": {"command": "bl", "args": ["mcp"]}}}`,
		Args:         cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, _ := explicitWorkspaceFlag(cmd)
			if workspace == "" {
				workspace = strings.TrimSpace(os.Getenv("BL_WORKSPACE"))
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			bridge := newMCPBridge(newBridgeAuth(workspace))
			bridge.workspace = workspace
			return bridge.serve(ctx, os.Stdin, os.Stdout)
		},
	}
}

// errNotLoggedIn means there is no usable bl login for the workspace.
var errNotLoggedIn = errors.New("not logged in to Blaxel")

// mcpCredentials is where one request goes, and how it signs in.
type mcpCredentials struct {
	workspace string
	endpoint  string
	headers   map[string]string
	// fingerprint identifies the login, to recognize it after a rejection.
	fingerprint string
}

type mcpAuthenticator interface {
	// resolve returns the credentials for the next request, or an error
	// wrapping errNotLoggedIn when there is no usable login.
	resolve(ctx context.Context) (mcpCredentials, error)
	// reject records that the server refused these credentials, so they are
	// not used again until the login changes.
	reject(mcpCredentials)
}

// bridgeAuth reads the bl login for every request, so a new login or logout
// applies without restarting the agent. Refreshed tokens stay in memory: many
// agents run a bridge at once, and the bl configuration is not written.
type bridgeAuth struct {
	explicit   string
	env        func(string) string
	loadConfig func() (blaxel.Config, error)
	// environment selects the Blaxel environment of a workspace and returns
	// its API base URL.
	environment func(workspace string) string
	client      *http.Client
	now         func() time.Time

	mu        sync.Mutex
	workspace string // fixed by the first successful sign-in
	baseURL   string
	rejected  string
	// cached is the last refreshed access token for refreshToken.
	cached struct {
		refreshToken, accessToken string
		expires, issued           time.Time
	}
}

func newBridgeAuth(workspace string) *bridgeAuth {
	return &bridgeAuth{
		explicit: workspace, env: os.Getenv, loadConfig: blaxel.LoadConfig, client: &http.Client{}, now: time.Now,
		environment: func(workspace string) string {
			blaxel.InitializeEnvironment(workspace)
			return blaxel.GetBaseURL()
		},
	}
}

func (a *bridgeAuth) resolve(ctx context.Context) (mcpCredentials, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	config, _ := a.loadConfig()
	workspace := a.workspace
	if workspace == "" {
		workspace = a.explicit
	}
	if workspace == "" {
		workspace = config.Context.Workspace
	}
	if workspace == "" {
		return mcpCredentials{}, errNotLoggedIn
	}
	var credentials blaxel.Credentials
	if key := strings.TrimSpace(a.env("BL_API_KEY")); key != "" {
		credentials = blaxel.Credentials{APIKey: key}
	} else if clientCredentials := strings.TrimSpace(a.env("BL_CLIENT_CREDENTIALS")); clientCredentials != "" {
		credentials = blaxel.Credentials{ClientCredentials: clientCredentials}
	} else {
		for _, entry := range config.Workspaces {
			if entry.Name == workspace {
				credentials = entry.Credentials
			}
		}
	}
	if !credentials.IsValid() {
		return mcpCredentials{}, fmt.Errorf("%w for workspace %s", errNotLoggedIn, workspace)
	}
	fingerprint := credentialsFingerprint(credentials)
	if fingerprint == a.rejected {
		return mcpCredentials{}, fmt.Errorf("%w: the login for workspace %s was refused", errNotLoggedIn, workspace)
	}
	if a.workspace == "" {
		// The agent's session stays in this workspace, even if bl workspaces
		// switches later: tools never move to another tenant silently.
		a.workspace, a.baseURL = workspace, strings.TrimSuffix(a.environment(workspace), "/")
	}
	authorization, err := a.authorization(ctx, workspace, credentials)
	if err != nil {
		return mcpCredentials{}, err
	}
	return mcpCredentials{
		workspace: workspace, endpoint: a.baseURL + "/mcp", fingerprint: fingerprint,
		headers: map[string]string{"X-Blaxel-Authorization": authorization, "X-Blaxel-Workspace": workspace},
	}, nil
}

func (a *bridgeAuth) reject(credentials mcpCredentials) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rejected = credentials.fingerprint
	a.cached.accessToken = ""
}

// credentialsFingerprint changes whenever the login changes.
func credentialsFingerprint(c blaxel.Credentials) string {
	return strings.Join([]string{c.APIKey, c.AccessToken, c.RefreshToken, c.ClientCredentials}, "\x00")
}

// authorization returns the Authorization header value for the credentials.
func (a *bridgeAuth) authorization(ctx context.Context, workspace string, c blaxel.Credentials) (string, error) {
	switch {
	case c.APIKey != "":
		return "Bearer " + c.APIKey, nil
	case c.AccessToken != "" && c.RefreshToken != "":
		token, err := a.accessToken(ctx, c)
		if err != nil {
			return "", err
		}
		return "Bearer " + token, nil
	case c.AccessToken != "":
		return "Bearer " + c.AccessToken, nil
	default:
		// Client credentials are exchanged for a token; nothing is saved.
		headers, err := c.AuthHeaders(ctx, workspace)
		if err != nil {
			return "", err
		}
		return headers["X-Blaxel-Authorization"], nil
	}
}

// accessToken returns a token that is not about to expire, refreshing it when needed.
func (a *bridgeAuth) accessToken(ctx context.Context, c blaxel.Credentials) (string, error) {
	now := a.now()
	if a.cached.refreshToken == c.RefreshToken && a.cached.accessToken != "" && tokenFresh(a.cached.issued, a.cached.expires, now) {
		return a.cached.accessToken, nil
	}
	if issued, expires, ok := jwtLifetime(c.AccessToken); ok && tokenFresh(issued, expires, now) {
		return c.AccessToken, nil
	}
	token, err := a.refresh(ctx, c)
	if err != nil {
		return "", err
	}
	issued, expires, ok := jwtLifetime(token)
	if !ok {
		issued, expires = now, now.Add(5*time.Minute)
	}
	a.cached.refreshToken, a.cached.accessToken, a.cached.issued, a.cached.expires = c.RefreshToken, token, issued, expires
	return token, nil
}

func (a *bridgeAuth) refresh(ctx context.Context, c blaxel.Credentials) (string, error) {
	body, err := json.Marshal(map[string]string{
		"grant_type": "refresh_token", "refresh_token": c.RefreshToken, "client_id": "blaxel", "device_code": c.DeviceCode,
	})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, mcpRefreshTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("refreshing your bl login: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		return "", fmt.Errorf("%w: your bl login expired", errNotLoggedIn)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refreshing your bl login: %s", response.Status)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &token); err != nil || token.AccessToken == "" {
		return "", errors.New("refreshing your bl login: no access token in the answer")
	}
	return token.AccessToken, nil
}

// tokenFresh reports whether a token has more than a fifth of its lifetime,
// and at least a minute, left.
func tokenFresh(issued, expires, now time.Time) bool {
	margin := max(expires.Sub(issued)/5, time.Minute)
	return now.Add(margin).Before(expires)
}

// jwtLifetime reads iat and exp from a JWT without verifying it.
func jwtLifetime(token string) (issued, expires time.Time, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return issued, expires, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return issued, expires, false
	}
	var claims struct {
		IssuedAt  float64 `json:"iat"`
		ExpiresAt float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.ExpiresAt == 0 {
		return issued, expires, false
	}
	expires = time.Unix(int64(claims.ExpiresAt), 0)
	issued = expires.Add(-time.Hour)
	if claims.IssuedAt > 0 {
		issued = time.Unix(int64(claims.IssuedAt), 0)
	}
	return issued, expires, true
}

// mcpLoginStarter opens the browser login. wait blocks until it is
// confirmed, saves it like bl login, and returns the workspace.
type mcpLoginStarter func(ctx context.Context, workspace string) (url string, wait func(context.Context) (string, error), err error)

// mcpBridge relays newline-delimited JSON-RPC messages between an agent on
// stdio and the hosted Blaxel MCP server over Streamable HTTP. The hosted
// server is stateless, so the bridge can answer initialize itself before
// there is a login, and forward everything once there is one.
type mcpBridge struct {
	auth      mcpAuthenticator
	client    *http.Client
	userAgent string
	version   string
	login     mcpLoginStarter
	logf      func(format string, args ...any)
	poll      time.Duration
	// workspace is the one asked for with --workspace or BL_WORKSPACE.
	workspace string

	out io.Writer
	// background outlives single requests, for the login and its notification.
	background context.Context
	writeMu    sync.Mutex

	stateMu sync.Mutex
	// session and protocolVersion come from the hosted initialize exchange.
	session, protocolVersion string
	// cancels stops in-flight requests the agent cancels, keyed by request ID.
	cancels map[string]context.CancelFunc
	// listed is what the agent's last tools/list showed: "" before any,
	// "login" for the login tool alone, "tools" for the Blaxel tools.
	listed string
	// loginURL is the page of a browser login in progress.
	loginURL string
}

func newMCPBridge(authenticator mcpAuthenticator) *mcpBridge {
	return &mcpBridge{
		auth: authenticator, client: &http.Client{}, version: core.GetVersion(),
		userAgent: "blaxel-cli/" + core.GetVersion() + " (bl mcp)",
		login:     startBridgeLogin, poll: mcpLoginPoll,
		logf: func(format string, args ...any) {
			_, _ = fmt.Fprintf(os.Stderr, "bl mcp: "+format+"\n", args...)
		},
	}
}

// rpcEnvelope holds the fields the bridge needs from a JSON-RPC message.
type rpcEnvelope struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		RequestID       json.RawMessage `json:"requestId"`
		ProtocolVersion string          `json:"protocolVersion"`
		Name            string          `json:"name"`
		Arguments       struct {
			Workspace string `json:"workspace"`
		} `json:"arguments"`
	} `json:"params"`
}

func (m rpcEnvelope) isRequest() bool {
	return m.Method != "" && len(m.ID) > 0 && string(m.ID) != "null"
}

func (b *mcpBridge) serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	b.out = out
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	b.background = ctx
	go b.watchLogin(ctx)

	reader := bufio.NewScanner(in)
	reader.Buffer(make([]byte, 64<<10), mcpBridgeMaxMessage)
	slots := make(chan struct{}, mcpBridgeMaxInFlight)
	var wg sync.WaitGroup
	for reader.Scan() {
		line := bytes.TrimSpace(reader.Bytes())
		if len(line) == 0 {
			continue
		}
		message := append([]byte(nil), line...)
		var envelope rpcEnvelope
		if bytes.HasPrefix(message, []byte("{")) {
			if err := json.Unmarshal(message, &envelope); err != nil {
				b.writeError(json.RawMessage("null"), -32700, "parse error: "+err.Error())
				continue
			}
		}
		if envelope.Method == "notifications/cancelled" {
			b.cancel(string(envelope.Params.RequestID))
		}
		// initialize finishes before anything else is sent, as the protocol requires.
		if envelope.Method == "initialize" {
			b.handle(ctx, message, envelope)
			continue
		}
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			b.handle(ctx, message, envelope)
		}()
	}
	// Let in-flight requests answer before the agent's pipe closes.
	wg.Wait()
	return reader.Err()
}

func (b *mcpBridge) handle(ctx context.Context, message []byte, envelope rpcEnvelope) {
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if envelope.isRequest() {
		b.track(string(envelope.ID), cancel)
		defer b.untrack(string(envelope.ID))
	}
	credentials, err := b.auth.resolve(requestCtx)
	switch {
	case errors.Is(err, errNotLoggedIn):
		b.answerLoggedOut(requestCtx, envelope, err)
	case err != nil:
		b.logf("%v", err)
		b.fail(requestCtx, envelope, -32603, err.Error())
	default:
		b.forward(requestCtx, message, envelope, credentials)
	}
}

// answerLoggedOut stands in for the hosted server until there is a login:
// the agent connects, and its only tool opens the browser login.
func (b *mcpBridge) answerLoggedOut(ctx context.Context, envelope rpcEnvelope, cause error) {
	if !envelope.isRequest() {
		return
	}
	switch envelope.Method {
	case "initialize":
		version := mcpProtocolVersions[0]
		if slices.Contains(mcpProtocolVersions, envelope.Params.ProtocolVersion) {
			version = envelope.Params.ProtocolVersion
		}
		b.writeResult(envelope.ID, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
			"serverInfo":      map[string]string{"name": "blaxel", "version": b.version},
			"instructions":    "Not logged in to Blaxel yet. Call the blaxel_login tool to log in with the browser; the Blaxel tools appear once the login is confirmed.",
		})
	case "ping":
		b.writeResult(envelope.ID, map[string]any{})
	case "tools/list":
		b.setListed("login")
		b.writeResult(envelope.ID, map[string]any{"tools": []any{map[string]any{
			"name":        mcpLoginTool,
			"title":       "Log in to Blaxel",
			"description": "Log in to Blaxel to use its tools (sandboxes, agents, jobs, MCP servers). Opens the Blaxel login page in the user's browser and returns its link; ask the user to confirm the login there. The Blaxel tools appear once they do.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"workspace": map[string]string{"type": "string", "description": "Workspace to use; by default the current one, else the first"},
			}},
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "openWorldHint": true},
		}}})
	case "tools/call":
		if envelope.Params.Name == mcpLoginTool {
			b.writeToolText(envelope.ID, b.startLogin(ctx, envelope.Params.Arguments.Workspace), false)
			return
		}
		reason := "Not logged in to Blaxel"
		if detail := strings.TrimPrefix(cause.Error(), errNotLoggedIn.Error()); detail != "" {
			reason += strings.TrimPrefix(detail, ":")
		}
		b.writeToolText(envelope.ID, reason+". Call the blaxel_login tool first, or ask the user to run `bl login`.", true)
		// The agent may still list the Blaxel tools from before a logout.
		b.notifyToolsChanged(ctx)
	default:
		b.writeError(envelope.ID, -32601, "not available before logging in to Blaxel: call the blaxel_login tool")
	}
}

// startLogin starts one browser login at a time and describes it for the agent.
func (b *mcpBridge) startLogin(ctx context.Context, workspace string) string {
	b.stateMu.Lock()
	inProgress := b.loginURL
	b.stateMu.Unlock()
	if inProgress != "" {
		return "A Blaxel login is already waiting for the user's confirmation: " + inProgress
	}
	if workspace == "" {
		workspace = b.workspace
	}
	url, wait, err := b.login(ctx, workspace)
	if err != nil {
		return "Could not start the Blaxel login: " + err.Error() + ". Ask the user to run `bl login` in a terminal."
	}
	b.stateMu.Lock()
	b.loginURL = url
	b.stateMu.Unlock()
	go func() {
		defer func() {
			b.stateMu.Lock()
			b.loginURL = ""
			b.stateMu.Unlock()
		}()
		// The login outlives the tool call that started it, not the bridge.
		loggedIn, err := wait(b.background)
		if err != nil {
			b.logf("login: %v", err)
			return
		}
		b.logf("logged in to workspace %s", loggedIn)
		b.notifyToolsChanged(b.background)
	}()
	return "Opened the Blaxel login page in the user's browser. Ask them to confirm the login there (or open " + url +
		" if no browser opened). The Blaxel tools appear once they confirm; call tools/list again if they don't."
}

// watchLogin notices a login made outside the agent, such as bl login in a
// terminal, while the agent is showing the login tool.
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
			waiting := b.listed == "login"
			b.stateMu.Unlock()
			if waiting {
				b.notifyToolsChanged(ctx)
			}
		}
	}
}

// notifyToolsChanged tells the agent to list the tools again when the login
// state no longer matches what it last listed.
func (b *mcpBridge) notifyToolsChanged(ctx context.Context) {
	b.stateMu.Lock()
	listed := b.listed
	b.stateMu.Unlock()
	if listed == "" {
		return
	}
	_, err := b.auth.resolve(ctx)
	loggedIn := err == nil
	if err != nil && !errors.Is(err, errNotLoggedIn) {
		return // a network problem, not a change of login
	}
	if (listed == "login") == loggedIn {
		b.setListed("")
		b.write([]byte(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`))
	}
}

func (b *mcpBridge) setListed(state string) {
	b.stateMu.Lock()
	b.listed = state
	b.stateMu.Unlock()
}

func (b *mcpBridge) forward(ctx context.Context, message []byte, envelope rpcEnvelope, credentials mcpCredentials) {
	if envelope.Method == "tools/call" && envelope.Params.Name == mcpLoginTool {
		// The agent still has the tool list from before the login.
		b.writeToolText(envelope.ID, "Already logged in to Blaxel, workspace "+credentials.workspace+". List the tools again to see the Blaxel tools.", false)
		b.notifyToolsChanged(ctx)
		return
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, credentials.endpoint, bytes.NewReader(message))
	if err != nil {
		b.fail(ctx, envelope, -32603, err.Error())
		return
	}
	for name, value := range credentials.headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("User-Agent", b.userAgent)
	b.stateMu.Lock()
	if b.session != "" {
		request.Header.Set("Mcp-Session-Id", b.session)
	}
	if b.protocolVersion != "" && envelope.Method != "initialize" {
		request.Header.Set("MCP-Protocol-Version", b.protocolVersion)
	}
	b.stateMu.Unlock()

	response, err := b.client.Do(request)
	if err != nil {
		if ctx.Err() == nil {
			b.logf("%s: %v", envelope.Method, err)
		}
		b.fail(ctx, envelope, -32603, "cannot reach the Blaxel MCP server: "+err.Error())
		return
	}
	defer func() { _ = response.Body.Close() }()
	if session := response.Header.Get("Mcp-Session-Id"); session != "" {
		b.stateMu.Lock()
		b.session = session
		b.stateMu.Unlock()
	}
	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNoContent {
		return
	}
	if response.StatusCode == http.StatusUnauthorized {
		// The login was revoked or expired: offer the login tool again.
		b.auth.reject(credentials)
		b.logf("the Blaxel MCP server refused the login for workspace %s", credentials.workspace)
		if envelope.Method == "tools/call" {
			b.writeToolText(envelope.ID, "Your Blaxel login expired. Call the blaxel_login tool, or ask the user to run `bl login`.", true)
		} else {
			b.fail(ctx, envelope, -32001, "your Blaxel login expired: call the blaxel_login tool, or run `bl login`")
		}
		b.notifyToolsChanged(ctx)
		return
	}
	relay := func(payload []byte) { b.relay(payload, envelope, credentials) }
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType == "text/event-stream" && response.StatusCode < 300 {
		relayEvents(response.Body, relay)
		return
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, mcpBridgeMaxMessage))
	if err != nil {
		b.fail(ctx, envelope, -32603, "reading the Blaxel MCP server answer: "+err.Error())
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) > 0 && json.Valid(body) && bytes.Contains(body, []byte(`"jsonrpc"`)) {
		relay(body)
		return
	}
	if response.StatusCode >= 300 {
		detail := strings.TrimSpace("the Blaxel MCP server answered " + response.Status + ": " + serverErrorText(body))
		b.logf("%s: %s", envelope.Method, detail)
		if envelope.Method == "tools/call" && ctx.Err() == nil {
			// A refused call (such as a workspace the user cannot use) is a
			// tool error the model can read and recover from.
			b.writeToolText(envelope.ID, detail, true)
			return
		}
		b.fail(ctx, envelope, -32603, detail)
		return
	}
	if len(body) > 0 {
		b.fail(ctx, envelope, -32603, "the Blaxel MCP server answered with something other than JSON-RPC")
	}
}

// serverErrorText extracts a short reason from a non-JSON-RPC error answer.
func serverErrorText(body []byte) string {
	var answer struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
		Message     string `json:"message"`
	}
	if json.Unmarshal(body, &answer) == nil {
		for _, text := range []string{answer.Description, answer.Message, answer.Error} {
			if text != "" {
				return truncate(text, 300)
			}
		}
	}
	return truncate(string(body), 300)
}

// relayEvents passes each server-sent event's data to relay as one message.
func relayEvents(body io.Reader, relay func([]byte)) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64<<10), mcpBridgeMaxMessage)
	var data bytes.Buffer
	flush := func() {
		if payload := bytes.TrimSpace(data.Bytes()); len(payload) > 0 && json.Valid(payload) {
			relay(append([]byte(nil), payload...))
		}
		data.Reset()
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	flush()
}

// relay writes a server message to the agent. The answers to initialize and
// tools/list also update what the bridge knows about the session.
func (b *mcpBridge) relay(message []byte, envelope rpcEnvelope, credentials mcpCredentials) {
	switch envelope.Method {
	case "initialize":
		var answer struct {
			ID     json.RawMessage `json:"id"`
			Result map[string]any  `json:"result"`
		}
		if json.Unmarshal(message, &answer) == nil && answer.Result != nil && string(answer.ID) == string(envelope.ID) {
			if version, _ := answer.Result["protocolVersion"].(string); version != "" {
				b.stateMu.Lock()
				b.protocolVersion = version
				b.stateMu.Unlock()
			}
			if _, has := answer.Result["instructions"]; !has {
				answer.Result["instructions"] = "Blaxel tools act on workspace " + credentials.workspace +
					" (from bl login). Pass the workspace argument to use another workspace the user belongs to."
				b.writeResult(answer.ID, answer.Result)
				return
			}
		}
	case "tools/list":
		if bytes.Contains(message, []byte(`"result"`)) {
			b.setListed("tools")
		}
	}
	b.write(message)
}

// fail answers a request with an error, unless the agent cancelled it or it
// was a notification.
func (b *mcpBridge) fail(ctx context.Context, envelope rpcEnvelope, code int, message string) {
	if envelope.isRequest() && ctx.Err() == nil {
		b.writeError(envelope.ID, code, message)
	}
}

func (b *mcpBridge) writeToolText(id json.RawMessage, text string, isError bool) {
	b.writeResult(id, map[string]any{"content": []any{map[string]string{"type": "text", "text": text}}, "isError": isError})
}

func (b *mcpBridge) writeResult(id json.RawMessage, result any) {
	data, err := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{"2.0", id, result})
	if err == nil {
		b.write(data)
	}
}

func (b *mcpBridge) writeError(id json.RawMessage, code int, message string) {
	type rpcError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	data, err := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   rpcError        `json:"error"`
	}{"2.0", id, rpcError{code, message}})
	if err == nil {
		b.write(data)
	}
}

// write sends one message per line, as the stdio transport requires.
func (b *mcpBridge) write(message []byte) {
	var line bytes.Buffer
	if json.Compact(&line, message) != nil {
		return
	}
	line.WriteByte('\n')
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	_, _ = b.out.Write(line.Bytes())
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
	defer b.stateMu.Unlock()
	delete(b.cancels, id)
}

func (b *mcpBridge) cancel(id string) {
	b.stateMu.Lock()
	cancel := b.cancels[id]
	b.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// startBridgeLogin is the browser login of bl login, without a terminal: the
// workspace is the one asked for, else the current one, else the first.
func startBridgeLogin(ctx context.Context, workspace string) (string, func(context.Context) (string, error), error) {
	login, _, err := auth.StartDeviceLogin(ctx)
	if err != nil {
		return "", nil, err
	}
	wait := func(ctx context.Context) (string, error) {
		credentials, err := auth.WaitForDeviceLogin(ctx, login.DeviceCode)
		if err != nil {
			return "", err
		}
		names, err := auth.LoginWorkspaces(credentials)
		if err != nil {
			return "", err
		}
		config, _ := blaxel.LoadConfig()
		chosen, err := defaultLoginWorkspace(names, workspace, config.Context.Workspace)
		if err != nil {
			return "", err
		}
		return chosen, auth.SaveDeviceLogin(chosen, credentials)
	}
	return login.VerificationURIComplete, wait, nil
}

// defaultLoginWorkspace picks a login's workspace without asking: the one
// requested, else the current one if the login can use it, else the first.
func defaultLoginWorkspace(names []string, requested, current string) (string, error) {
	if len(names) == 0 {
		return "", errors.New("no workspaces are available for your account")
	}
	if requested != "" {
		if slices.Contains(names, requested) {
			return requested, nil
		}
		return "", fmt.Errorf("workspace %s is not available to this login; choose one of: %s", requested, strings.Join(names, ", "))
	}
	if slices.Contains(names, current) {
		return current, nil
	}
	return names[0], nil
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}
