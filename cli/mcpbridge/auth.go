package mcpbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"gopkg.in/yaml.v3"
)

var errNotLoggedIn = errors.New("not logged in to Blaxel")

type mcpCredentials struct {
	workspace, endpoint, fingerprint string
	headers                          map[string]string
}

type mcpAuthenticator interface {
	resolve(context.Context) (mcpCredentials, error)
	// reject invalidates this access token; resolve forces a fresh exchange.
	reject(mcpCredentials)
}

// mcpExchangeCooldown is how long a failed token exchange is not repeated.
const mcpExchangeCooldown = 5 * time.Second

// Each process re-reads credentials and keeps refreshed tokens only in memory.
// The context-aware gate serializes refreshes without blocking cancelled callers.
type bridgeAuth struct {
	explicit   string // --workspace or BL_WORKSPACE
	env        func(string) string
	loadConfig func() (blaxel.Config, error)
	client     *http.Client
	now        func() time.Time
	once       sync.Once
	gate       chan struct{}
	// The default workspace and the origin of its stored environment are pinned
	// by the first request that finds a login.
	workspace, baseURL, pinnedEnv string
	// forced is the login whose access token the server refused.
	forced string
	failed struct {
		grant string
		until time.Time
		err   error
	}
	cached struct {
		refreshToken, accessToken string
		expires, issued           time.Time
	}
}

func newBridgeAuth(workspace string) *bridgeAuth {
	return &bridgeAuth{explicit: workspace, env: os.Getenv, loadConfig: loadBridgeConfig, client: newMCPHTTPClient(), now: time.Now}
}

// Unlike SDK LoadConfig, malformed/partial YAML is not silently an empty login.
func loadBridgeConfig() (blaxel.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return blaxel.Config{}, errors.New("cannot locate the bl login")
	}
	data, err := os.ReadFile(filepath.Join(home, ".blaxel", "config.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return blaxel.Config{}, nil
	}
	var config blaxel.Config
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return config, errors.New("cannot read the bl login; retry shortly")
	}
	if yaml.Unmarshal(data, &config) != nil {
		return blaxel.Config{}, errors.New("cannot parse the bl login; retry shortly")
	}
	return config, nil
}

func (a *bridgeAuth) lock(ctx context.Context) error {
	a.once.Do(func() { a.gate = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case a.gate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (a *bridgeAuth) unlock() { <-a.gate }

func (a *bridgeAuth) resolve(ctx context.Context) (mcpCredentials, error) {
	if err := a.lock(ctx); err != nil {
		return mcpCredentials{}, err
	}
	defer a.unlock()
	config, err := a.loadConfig()
	if err != nil || (a.workspace != "" && len(config.Workspaces) == 0) {
		// CLI writes are not atomic today. Re-read once, including apparent logout,
		// but never retain credentials indefinitely after a real logout.
		select {
		case <-ctx.Done():
			return mcpCredentials{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		config, err = a.loadConfig()
	}
	if err != nil {
		return mcpCredentials{}, errors.New("cannot read the bl login; retry shortly")
	}
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
	var c blaxel.Credentials
	storedEnv := "prod"
	for _, entry := range config.Workspaces {
		if entry.Name == workspace {
			c = entry.Credentials
			if entry.Env != "" {
				storedEnv = entry.Env
			}
		}
	}
	if key := strings.TrimSpace(a.env("BL_API_KEY")); key != "" {
		c = blaxel.Credentials{APIKey: key}
	} else if cc := strings.TrimSpace(a.env("BL_CLIENT_CREDENTIALS")); cc != "" {
		c = blaxel.Credentials{ClientCredentials: cc}
	}
	// Pin the default and origin even when refresh fails. Inherited BL_ENV and
	// BL_API_URL are deliberately not credential-destination selectors.
	if a.workspace == "" {
		a.workspace = workspace
	}
	if !c.IsValid() {
		return mcpCredentials{}, errNotLoggedIn
	}
	if a.baseURL == "" {
		base, err := bridgeBaseURL(storedEnv)
		if err != nil {
			return mcpCredentials{}, err
		}
		a.baseURL, a.pinnedEnv = base, storedEnv
	} else if a.pinnedEnv != storedEnv {
		return mcpCredentials{}, errors.New("the pinned workspace environment changed; restart or reconnect this agent")
	}
	authorization, err := a.authorization(ctx, c)
	if err != nil {
		return mcpCredentials{}, err
	}
	return mcpCredentials{
		workspace:   workspace,
		endpoint:    a.baseURL + "/mcp",
		fingerprint: credentialsFingerprint(c),
		headers:     map[string]string{"Authorization": authorization, "X-Blaxel-Workspace": workspace},
	}, nil
}

func bridgeBaseURL(env string) (string, error) {
	switch env {
	case "", "prod":
		return "https://api.blaxel.ai/v0", nil
	case "dev":
		return "https://api.blaxel.dev/v0", nil
	default:
		return "", errors.New("bl mcp supports only workspaces in the prod and dev Blaxel environments")
	}
}

func (a *bridgeAuth) reject(c mcpCredentials) {
	if a.lock(context.Background()) != nil {
		return
	}
	defer a.unlock()
	// Concurrent 401s for the old token must not invalidate a token another
	// request has already refreshed for this same login.
	if a.cached.refreshToken == c.fingerprint && a.cached.accessToken != "" && c.headers["Authorization"] != "Bearer "+a.cached.accessToken {
		return
	}
	a.forced = c.fingerprint
	a.cached.accessToken = ""
	a.failed.grant = ""
}

func credentialsFingerprint(c blaxel.Credentials) string {
	return strings.Join([]string{c.APIKey, c.AccessToken, c.RefreshToken, c.DeviceCode, c.ClientCredentials}, "\x00")
}

func (a *bridgeAuth) authorization(ctx context.Context, c blaxel.Credentials) (string, error) {
	fp := credentialsFingerprint(c)
	forced := a.forced == fp
	if forced && (c.APIKey != "" || (c.RefreshToken == "" && (c.ClientCredentials == "" || c.AccessToken != ""))) {
		// A static rejected token cannot be refreshed, but rejection is not sticky
		// for this process: retry after a short cooldown or as soon as login changes.
		a.forced = ""
		return "", errNotLoggedIn
	}
	if c.APIKey != "" {
		return "Bearer " + c.APIKey, nil
	}
	if c.ClientCredentials != "" && c.AccessToken == "" {
		now := a.now()
		if !forced && a.cached.refreshToken == fp && now.Add(time.Minute).Before(a.cached.expires) {
			return "Bearer " + a.cached.accessToken, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(c.ClientCredentials)
		if err != nil {
			decoded = []byte(c.ClientCredentials)
		}
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			return "", errors.New("invalid BL_CLIENT_CREDENTIALS format")
		}
		token, err := a.exchange(ctx, map[string]string{"grant_type": "client_credentials", "client_id": parts[0], "client_secret": parts[1]})
		if err != nil {
			return "", err
		}
		a.cache(fp, token)
		a.forced = ""
		return "Bearer " + token, nil
	}
	if c.RefreshToken != "" {
		token, err := a.accessToken(ctx, c)
		if err != nil {
			return "", err
		}
		return "Bearer " + token, nil
	}
	if _, exp, ok := jwtLifetime(c.AccessToken); ok && !a.now().Before(exp) {
		return "", errNotLoggedIn
	}
	return "Bearer " + c.AccessToken, nil
}

func (a *bridgeAuth) cache(fp, token string) {
	now := a.now()
	issued, expires, ok := jwtLifetime(token)
	if !ok {
		issued, expires = now, now.Add(5*time.Minute)
	}
	a.cached.refreshToken, a.cached.accessToken, a.cached.issued, a.cached.expires = fp, token, issued, expires
}

func (a *bridgeAuth) accessToken(ctx context.Context, c blaxel.Credentials) (string, error) {
	now := a.now()
	fp := credentialsFingerprint(c)
	forced := a.forced == fp
	token := c.AccessToken
	issued, expires, ok := jwtLifetime(token)
	if a.cached.refreshToken == fp && a.cached.accessToken != "" {
		token, issued, expires, ok = a.cached.accessToken, a.cached.issued, a.cached.expires, true
	}
	if !forced && ok && tokenFresh(issued, expires, now) {
		return token, nil
	}
	refreshed, err := a.refresh(ctx, c)
	if err != nil {
		// A transient failure is not a logout: keep using a token that is still valid.
		if !forced && !errors.Is(err, errNotLoggedIn) && ok && now.Before(expires) {
			return token, nil
		}
		return "", err
	}
	a.cache(fp, refreshed)
	a.forced = ""
	return refreshed, nil
}

func (a *bridgeAuth) refresh(ctx context.Context, c blaxel.Credentials) (string, error) {
	return a.exchange(ctx, map[string]string{"grant_type": "refresh_token", "refresh_token": c.RefreshToken, "client_id": "blaxel", "device_code": c.DeviceCode})
}

// All grants use our hardened HTTP client, never the SDK's global OAuth cache.
func (a *bridgeAuth) exchange(ctx context.Context, grant map[string]string) (token string, err error) {
	// One failure is remembered briefly, so every call does not retry it.
	key := grant["refresh_token"] + "\x00" + grant["client_id"] + "\x00" + grant["client_secret"]
	if a.failed.grant == key && a.now().Before(a.failed.until) {
		return "", a.failed.err
	}
	caller := ctx
	defer func() {
		if err != nil && caller.Err() == nil {
			a.failed.grant, a.failed.until, a.failed.err = key, a.now().Add(mcpExchangeCooldown), err
		}
	}()
	bodyGrant := grant
	if grant["grant_type"] == "client_credentials" {
		bodyGrant = map[string]string{"grant_type": "client_credentials"}
	}
	body, err := json.Marshal(bodyGrant)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, mcpRefreshTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("invalid token endpoint")
	}
	request.Header.Set("Content-Type", "application/json")
	if grant["grant_type"] == "client_credentials" {
		request.SetBasicAuth(grant["client_id"], grant["client_secret"])
	}
	response, err := doMCPRequest(a.client, request)
	if err != nil {
		return "", errors.New("cannot refresh the bl login right now; retry shortly")
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return "", errors.New("invalid token endpoint response; retry shortly")
	}
	var answer struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	_ = json.Unmarshal(data, &answer)
	if response.StatusCode == 400 || response.StatusCode == 401 || answer.Error == "invalid_grant" {
		return "", errNotLoggedIn
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint answered HTTP %d; retry shortly", response.StatusCode)
	}
	if answer.AccessToken == "" {
		return "", errors.New("token endpoint returned no access token; retry shortly")
	}
	return answer.AccessToken, nil
}

func tokenFresh(issued, expires, now time.Time) bool {
	return now.Add(max(expires.Sub(issued)/5, time.Minute)).Before(expires)
}
func jwtLifetime(token string) (issued, expires time.Time, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	var claims struct {
		IssuedAt  float64 `json:"iat"`
		ExpiresAt float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.ExpiresAt == 0 {
		return
	}
	expires = time.Unix(int64(claims.ExpiresAt), 0)
	issued = expires.Add(-time.Hour)
	if claims.IssuedAt > 0 {
		issued = time.Unix(int64(claims.IssuedAt), 0)
	}
	return issued, expires, true
}
