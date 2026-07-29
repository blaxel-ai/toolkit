package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenForCredentialsUsesRefreshedBearer(t *testing.T) {
	storedToken := testJWT(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	const refreshedToken = "fresh-access-token"

	previous := authHeadersForCredentials
	authHeadersForCredentials = func(ctx context.Context, credentials blaxel.Credentials, workspace string) (map[string]string, error) {
		assert.Equal(t, "main", workspace)
		assert.Equal(t, storedToken, credentials.AccessToken)
		assert.Equal(t, "refresh-token", credentials.RefreshToken)
		return map[string]string{
			"X-Blaxel-Authorization": "Bearer " + refreshedToken,
		}, nil
	}
	t.Cleanup(func() { authHeadersForCredentials = previous })

	token, err := tokenForCredentials(context.Background(), "main", blaxel.Credentials{
		AccessToken:  storedToken,
		RefreshToken: "refresh-token",
		ExpiresIn:    7200,
	})

	require.NoError(t, err)
	assert.Equal(t, refreshedToken, token)
}

func TestTokenForCredentialsRefreshFailureShowsLoginGuidance(t *testing.T) {
	previous := authHeadersForCredentials
	authHeadersForCredentials = func(ctx context.Context, credentials blaxel.Credentials, workspace string) (map[string]string, error) {
		return nil, fmt.Errorf("failed to refresh token: invalid refresh token")
	}
	t.Cleanup(func() { authHeadersForCredentials = previous })

	token, err := tokenForCredentials(context.Background(), "main", blaxel.Credentials{
		AccessToken:  "expired-access-token",
		RefreshToken: "invalid-refresh-token",
	})

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "invalid refresh token")
	assert.Contains(t, err.Error(), "bl login main")
}

func TestTokenForCredentialsRejectsExpiredUnrefreshedBearer(t *testing.T) {
	expiredToken := testJWT(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))

	previous := authHeadersForCredentials
	authHeadersForCredentials = func(ctx context.Context, credentials blaxel.Credentials, workspace string) (map[string]string, error) {
		return map[string]string{
			"X-Blaxel-Authorization": "Bearer " + credentials.AccessToken,
		}, nil
	}
	t.Cleanup(func() { authHeadersForCredentials = previous })

	token, err := tokenForCredentials(context.Background(), "main", blaxel.Credentials{
		AccessToken: expiredToken,
	})

	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "bl login main")
}

func TestTokenCmdExpiredAccessTokenWithoutRefreshShowsLoginGuidance(t *testing.T) {
	workspace := "eng-2815-workspace"
	expiredToken := testJWT(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))

	home := t.TempDir()
	configDir := filepath.Join(home, ".blaxel")
	require.NoError(t, os.MkdirAll(configDir, 0700))
	config := fmt.Sprintf(`context:
  workspace: %s
workspaces:
  - name: %s
    credentials:
      access_token: %s
`, workspace, workspace, expiredToken)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config), 0600))

	cmd := exec.Command(os.Args[0], "-test.run=TestTokenCmdExpiredAccessTokenWithoutRefreshShowsLoginGuidanceHelper")
	cmd.Env = append(os.Environ(),
		"BLAXEL_TEST_TOKEN_HOME="+home,
		"BLAXEL_TEST_TOKEN_WORKSPACE="+workspace,
		"HOME="+home,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	require.Error(t, err)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "bl login "+workspace)
}

func TestTokenCmdExpiredAccessTokenWithoutRefreshShowsLoginGuidanceHelper(t *testing.T) {
	home := os.Getenv("BLAXEL_TEST_TOKEN_HOME")
	workspace := os.Getenv("BLAXEL_TEST_TOKEN_WORKSPACE")
	if home == "" || workspace == "" {
		t.Skip("helper only runs in a subprocess")
	}

	require.NoError(t, os.Setenv("HOME", home))
	cmd := TokenCmd()
	cmd.SetArgs([]string{workspace})
	require.NoError(t, cmd.Execute())
}

func TestBearerTokenFromHeaders(t *testing.T) {
	assert.Equal(t, "api-key", bearerTokenFromHeaders(map[string]string{
		"X-Blaxel-Authorization": "Bearer api-key",
	}))
	assert.Equal(t, "access-token", bearerTokenFromHeaders(map[string]string{
		"Authorization": "Bearer access-token",
	}))
	assert.Empty(t, bearerTokenFromHeaders(map[string]string{
		"Authorization": "Basic nope",
	}))
}

func testJWT(t *testing.T, issuedAt time.Time, expiresAt time.Time) string {
	t.Helper()

	header := map[string]string{
		"alg": "none",
		"typ": "JWT",
	}
	claims := map[string]int64{
		"iat": issuedAt.Unix(),
		"exp": expiresAt.Unix(),
	}

	return fmt.Sprintf("%s.%s.",
		testJWTPart(t, header),
		testJWTPart(t, claims),
	)
}

func testJWTPart(t *testing.T, value interface{}) string {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString(data)
}
