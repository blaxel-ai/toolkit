package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
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

// Each process re-reads credentials and keeps refreshed tokens only in memory.
// The context-aware gate serializes refreshes without blocking cancelled callers.
type bridgeAuth struct {
	explicit                   string
	env                        func(string) string
	loadConfig                 func() (blaxel.Config, error)
	environment                func(string) string // test seam; production uses stored workspace env
	client                     *http.Client
	now                        func() time.Time
	once                       sync.Once
	gate                       chan struct{}
	workspace, baseURL, forced string
	pinnedEnv                  string
	retryAt                    time.Time
	retryFingerprint           string
	retryErr                   error
	failures                   int
	refreshRetryAt             time.Time
	refreshRetryFingerprint    string
	refreshRetryErr            error
	refreshFailures            int
	cached                     struct {
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
		if err = waitMCP(ctx, 50*time.Millisecond); err != nil {
			return mcpCredentials{}, err
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
		if a.environment != nil {
			base, err = a.environment(workspace), nil
		}
		if err != nil {
			return mcpCredentials{}, err
		}
		a.baseURL, a.pinnedEnv = strings.TrimSuffix(base, "/"), storedEnv
	} else if a.environment == nil && a.pinnedEnv != storedEnv {
		return mcpCredentials{}, errors.New("the pinned workspace environment changed; restart or reconnect this agent")
	}
	fp := credentialsFingerprint(c)
	if a.retryFingerprint == fp && a.now().Before(a.retryAt) {
		return mcpCredentials{}, a.retryErr
	}
	authorization, err := a.authorization(ctx, workspace, c)
	if err != nil {
		a.retryFingerprint, a.retryErr = fp, err
		a.failures++
		delay := min(time.Second*time.Duration(1<<min(a.failures-1, 3)), 8*time.Second) + time.Duration(rand.IntN(500))*time.Millisecond
		a.retryAt = a.now().Add(delay)
		return mcpCredentials{}, err
	}
	a.retryErr = nil
	a.failures = 0
	a.retryFingerprint = ""
	return mcpCredentials{workspace: workspace, endpoint: a.baseURL + "/mcp", fingerprint: fp, headers: map[string]string{"Authorization": authorization, "X-Blaxel-Workspace": workspace}}, nil
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
	a.retryFingerprint = ""
	a.refreshRetryFingerprint = ""
}

func credentialsFingerprint(c blaxel.Credentials) string {
	return strings.Join([]string{c.APIKey, c.AccessToken, c.RefreshToken, c.DeviceCode, c.ClientCredentials}, "\x00")
}

func (a *bridgeAuth) authorization(ctx context.Context, _ string, c blaxel.Credentials) (string, error) {
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
	if a.refreshRetryFingerprint == fp && now.Before(a.refreshRetryAt) {
		if !forced && !errors.Is(a.refreshRetryErr, errNotLoggedIn) && ok && now.Before(expires) {
			return token, nil
		}
		return "", a.refreshRetryErr
	}
	refreshed, err := a.refresh(ctx, c)
	if err != nil {
		a.refreshFailures++
		a.refreshRetryFingerprint, a.refreshRetryErr = fp, err
		a.refreshRetryAt = now.Add(time.Second*time.Duration(1<<min(a.refreshFailures-1, 3)) + time.Duration(rand.IntN(500))*time.Millisecond)
		if !forced && !errors.Is(err, errNotLoggedIn) && ok && now.Before(expires) {
			return token, nil
		}
		return "", err
	}
	a.refreshRetryFingerprint = ""
	a.refreshFailures = 0
	a.cache(fp, refreshed)
	a.forced = ""
	return refreshed, nil
}

func (a *bridgeAuth) refresh(ctx context.Context, c blaxel.Credentials) (string, error) {
	return a.exchange(ctx, map[string]string{"grant_type": "refresh_token", "refresh_token": c.RefreshToken, "client_id": "blaxel", "device_code": c.DeviceCode})
}

// All grants use our hardened HTTP client, never the SDK's global OAuth cache.
func (a *bridgeAuth) exchange(ctx context.Context, grant map[string]string) (string, error) {
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

func waitMCP(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
