package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
