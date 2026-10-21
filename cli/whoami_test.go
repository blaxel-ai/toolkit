package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// whoamiFixture gives bl whoami an empty home and points the API at handler.
func whoamiFixture(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, name := range []string{"BL_API_KEY", "BL_CLIENT_CREDENTIALS", "BL_WORKSPACE", "BL_ENV"} {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	env, baseURL := blaxel.GetEnvironment(), blaxel.GetBaseURL()
	t.Cleanup(func() {
		blaxel.SetEnvironment(env)
		blaxel.SetBaseURL(baseURL)
	})
	t.Setenv("BL_API_URL", server.URL)
	return server.URL
}

func saveWhoamiLogin(t *testing.T, workspace string, creds blaxel.Credentials) {
	t.Helper()
	require.NoError(t, blaxel.WriteConfig(blaxel.Config{
		Context:    blaxel.ContextConfig{Workspace: workspace},
		Workspaces: []blaxel.WorkspaceConfig{{Name: workspace, Credentials: creds}},
		Tracking:   false,
	}))
}

func deviceLoginToken(t *testing.T) string {
	t.Helper()
	claims := map[string]any{
		"sub":         "user-sub",
		"sub_type":    "user",
		"email":       "jane@example.com",
		"given_name":  "Jane",
		"family_name": "Doe",
		"exp":         time.Now().Add(time.Hour).Unix(),
	}
	return fmt.Sprintf("%s.%s.", testJWTPart(t, map[string]string{"alg": "none", "typ": "JWT"}), testJWTPart(t, claims))
}

func runWhoamiJSON(t *testing.T, workspace string) (map[string]any, int, string) {
	t.Helper()
	var out bytes.Buffer
	code := runWhoami(context.Background(), &out, workspace, "json")
	var report map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &report), out.String())
	return report, code, out.String()
}

func reportKeys(report map[string]any) []string {
	keys := make([]string, 0, len(report))
	for key := range report {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func failOnRequest(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("signed out whoami must not call the API, got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func TestWhoamiSignedOut(t *testing.T) {
	apiURL := whoamiFixture(t, failOnRequest(t))

	report, code, _ := runWhoamiJSON(t, "")
	assert.Equal(t, 1, code)
	assert.Equal(t, []string{"api_url", "error", "logged_in", "next_step", "verified", "workspace"}, reportKeys(report))
	assert.Equal(t, false, report["logged_in"])
	assert.Equal(t, false, report["verified"])
	assert.Equal(t, "not logged in", report["error"])
	assert.Equal(t, "bl login", report["next_step"])
	assert.Equal(t, apiURL, report["api_url"])

	var text bytes.Buffer
	assert.Equal(t, 1, runWhoami(context.Background(), &text, "", ""))
	assert.Contains(t, text.String(), "Not logged in.")
	assert.Contains(t, text.String(), "Next step:        bl login\n")
	assert.Contains(t, text.String(), "API URL:          "+apiURL)

	var yamlOut bytes.Buffer
	assert.Equal(t, 1, runWhoami(context.Background(), &yamlOut, "", "yaml"))
	assert.Contains(t, yamlOut.String(), "logged_in: false")
}

func TestWhoamiSignedOutOfRequestedWorkspace(t *testing.T) {
	whoamiFixture(t, failOnRequest(t))
	saveWhoamiLogin(t, "other", blaxel.Credentials{APIKey: "bl_other_key"})

	report, code, out := runWhoamiJSON(t, "acme")
	assert.Equal(t, 1, code)
	assert.Equal(t, false, report["logged_in"])
	assert.Equal(t, "acme", report["workspace"])
	assert.Equal(t, "bl login acme", report["next_step"])
	assert.NotContains(t, out, "bl_other_key")
}

func TestWhoamiDeviceLogin(t *testing.T) {
	token := deviceLoginToken(t)
	apiURL := whoamiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/profile" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assert.Equal(t, "Bearer "+token, r.Header.Get("X-Blaxel-Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"jane.doe@example.com","given_name":"Jane","family_name":"Doe","sub":"user-sub"}`))
	})
	saveWhoamiLogin(t, "acme", blaxel.Credentials{AccessToken: token, RefreshToken: "refresh-secret", ExpiresIn: 3600})

	report, code, out := runWhoamiJSON(t, "acme")
	assert.Equal(t, 0, code)
	assert.Equal(t, []string{"account_type", "api_url", "auth_method", "credentials_source", "email", "logged_in", "name", "verified", "workspace"}, reportKeys(report))
	assert.Equal(t, true, report["logged_in"])
	assert.Equal(t, true, report["verified"])
	assert.Equal(t, "user", report["account_type"])
	assert.Equal(t, "jane.doe@example.com", report["email"], "the API's answer wins over the token claims")
	assert.Equal(t, "Jane Doe", report["name"])
	assert.Equal(t, "acme", report["workspace"])
	assert.Equal(t, "device_login", report["auth_method"])
	assert.Equal(t, "~/.blaxel/config.yaml", report["credentials_source"])
	assert.Equal(t, apiURL, report["api_url"])
	assert.NotContains(t, out, token)
	assert.NotContains(t, out, "refresh-secret")

	var text bytes.Buffer
	assert.Equal(t, 0, runWhoami(context.Background(), &text, "acme", ""))
	assert.Equal(t, "Logged in to Blaxel.\n"+
		"User:             jane.doe@example.com (Jane Doe)\n"+
		"Workspace:        acme\n"+
		"Auth method:      device login (saved in ~/.blaxel/config.yaml)\n"+
		"API URL:          "+apiURL+"\n", text.String())
}

func TestWhoamiKeepsLocalIdentityWhenAPIFails(t *testing.T) {
	token := deviceLoginToken(t)
	whoamiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-should-retry", "false")
		w.WriteHeader(http.StatusInternalServerError)
	})
	saveWhoamiLogin(t, "acme", blaxel.Credentials{AccessToken: token})

	report, code, _ := runWhoamiJSON(t, "acme")
	assert.Equal(t, 1, code)
	assert.Equal(t, true, report["logged_in"])
	assert.Equal(t, false, report["verified"])
	assert.Equal(t, "jane@example.com", report["email"])
	assert.Equal(t, "Jane Doe", report["name"])
	assert.Equal(t, "the API returned 500 Internal Server Error", report["error"])
	assert.NotContains(t, report, "next_step")

	var text bytes.Buffer
	runWhoami(context.Background(), &text, "acme", "")
	assert.Contains(t, text.String(), "Logged in to Blaxel, but the API could not confirm it.")
	assert.Contains(t, text.String(), "User:             jane@example.com (Jane Doe), not confirmed by the API\n")
	assert.Contains(t, text.String(), "Error:            the API returned 500 Internal Server Error\n")
}

func TestWhoamiUnreachableAPI(t *testing.T) {
	whoamiFixture(t, failOnRequest(t))
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	t.Setenv("BL_API_URL", closed.URL)
	saveWhoamiLogin(t, "acme", blaxel.Credentials{AccessToken: deviceLoginToken(t)})

	report, code, _ := runWhoamiJSON(t, "acme")
	assert.Equal(t, 1, code)
	assert.Equal(t, true, report["logged_in"])
	assert.Equal(t, false, report["verified"])
	assert.Equal(t, "jane@example.com", report["email"])
	assert.Equal(t, closed.URL, report["api_url"])
	assert.Contains(t, report["error"], "the API check failed: ")
}

func TestWhoamiRejectedCredentials(t *testing.T) {
	whoamiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	saveWhoamiLogin(t, "acme", blaxel.Credentials{AccessToken: deviceLoginToken(t)})

	report, code, _ := runWhoamiJSON(t, "acme")
	assert.Equal(t, 1, code)
	assert.Equal(t, false, report["logged_in"])
	assert.Equal(t, false, report["verified"])
	assert.Equal(t, "the API rejected the credentials (401 Unauthorized)", report["error"])
	assert.Equal(t, "bl login acme", report["next_step"])
}

func TestWhoamiAPIKeyOfServiceAccount(t *testing.T) {
	var workspaceChecks int
	whoamiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer bl_service_key", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/profile":
			w.WriteHeader(http.StatusUnauthorized)
		case "/workspaces/acme":
			workspaceChecks++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"acme"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	saveWhoamiLogin(t, "acme", blaxel.Credentials{APIKey: "bl_service_key"})

	report, code, out := runWhoamiJSON(t, "acme")
	assert.Equal(t, 0, code)
	assert.Equal(t, 1, workspaceChecks)
	assert.Equal(t, true, report["verified"])
	assert.Equal(t, "service_account", report["account_type"])
	assert.Equal(t, "api_key", report["auth_method"])
	assert.NotContains(t, report, "email")
	assert.NotContains(t, out, "bl_service_key")
}

func TestWhoamiClientCredentialsFromEnvironment(t *testing.T) {
	whoamiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = w.Write([]byte(`{"access_token":"service-access","token_type":"Bearer","expires_in":3600}`))
		case "/workspaces/acme":
			assert.Equal(t, "Bearer service-access", r.Header.Get("X-Blaxel-Authorization"))
			_, _ = w.Write([]byte(`{"name":"acme"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	t.Setenv("BL_CLIENT_CREDENTIALS", base64.StdEncoding.EncodeToString([]byte("client-123:client-secret")))

	report, code, out := runWhoamiJSON(t, "acme")
	assert.Equal(t, 0, code)
	assert.Equal(t, true, report["logged_in"])
	assert.Equal(t, true, report["verified"])
	assert.Equal(t, "service_account", report["account_type"])
	assert.Equal(t, "client-123", report["client_id"])
	assert.Equal(t, "client_credentials", report["auth_method"])
	assert.Equal(t, "BL_CLIENT_CREDENTIALS", report["credentials_source"])
	assert.NotContains(t, out, "client-secret")
	assert.NotContains(t, out, "service-access")
}

func TestWhoamiCmd(t *testing.T) {
	cmd := WhoamiCmd()
	assert.Equal(t, "whoami", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.Error(t, cmd.Args(cmd, []string{"extra"}))
}
