package core

import (
	"fmt"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
)

func TestResolveAuthSource_EnvAPIKey(t *testing.T) {
	t.Setenv("BL_API_KEY", "test-key")
	t.Setenv("BL_CLIENT_CREDENTIALS", "")
	src := ResolveAuthSource("nonexistent-workspace")
	assert.Equal(t, "API key", src.Method)
	assert.Contains(t, src.Origin, "environment variable BL_API_KEY")
}

func TestResolveAuthSource_EnvClientCredentials(t *testing.T) {
	t.Setenv("BL_API_KEY", "")
	t.Setenv("BL_CLIENT_CREDENTIALS", "test-creds")
	src := ResolveAuthSource("nonexistent-workspace")
	assert.Equal(t, "client credentials", src.Method)
	assert.Contains(t, src.Origin, "environment variable BL_CLIENT_CREDENTIALS")
}

func TestResolveAuthSource_NoAuth(t *testing.T) {
	t.Setenv("BL_API_KEY", "")
	t.Setenv("BL_CLIENT_CREDENTIALS", "")
	src := ResolveAuthSource("nonexistent-workspace")
	assert.Empty(t, src.Method)
	assert.Empty(t, src.Origin)
}

func TestSetGetAuthSource(t *testing.T) {
	original := GetAuthSource()
	defer SetAuthSource(original)

	expected := AuthSource{Method: "API key", Origin: "environment variable BL_API_KEY"}
	SetAuthSource(expected)
	assert.Equal(t, expected, GetAuthSource())
}

func TestIsAuthError_401(t *testing.T) {
	assert.True(t, IsAuthError(fmt.Errorf("401 Unauthorized")))
}

func TestIsAuthError_403(t *testing.T) {
	assert.True(t, IsAuthError(fmt.Errorf("403 Forbidden")))
}

func TestIsAuthError_UnauthorizedText(t *testing.T) {
	assert.True(t, IsAuthError(fmt.Errorf("unauthorized access to resource")))
}

func TestIsAuthError_PermissionDenied(t *testing.T) {
	assert.True(t, IsAuthError(fmt.Errorf("permission denied for workspace")))
}

func TestIsAuthError_RegularError(t *testing.T) {
	assert.False(t, IsAuthError(fmt.Errorf("connection timeout")))
}

func TestIsAuthError_Nil(t *testing.T) {
	assert.False(t, IsAuthError(nil))
}

func TestIsAuthError_BlaxelError(t *testing.T) {
	err := &blaxel.Error{StatusCode: 401}
	assert.True(t, IsAuthError(err))
}

func TestIsAuthError_BlaxelError403(t *testing.T) {
	err := &blaxel.Error{StatusCode: 403}
	assert.True(t, IsAuthError(err))
}

func TestIsAuthError_BlaxelErrorOther(t *testing.T) {
	err := &blaxel.Error{StatusCode: 500}
	assert.False(t, IsAuthError(err))
}

func TestPrintAuthSourceHint_NoSource(t *testing.T) {
	original := GetAuthSource()
	defer SetAuthSource(original)

	SetAuthSource(AuthSource{})
	// Should not panic with empty source
	PrintAuthSourceHint()
}

func TestPrintAuthSourceHint_WithEnvSource(t *testing.T) {
	original := GetAuthSource()
	defer SetAuthSource(original)

	SetAuthSource(AuthSource{Method: "API key", Origin: "environment variable BL_API_KEY"})
	// Should not panic
	PrintAuthSourceHint()
}
