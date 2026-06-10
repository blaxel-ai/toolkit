package core

import (
	"fmt"
	"os"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/fatih/color"
)

// AuthSource describes where the current authentication credentials came from.
type AuthSource struct {
	Method string // e.g. "API key", "access token", "client credentials"
	Origin string // e.g. "environment variable BL_API_KEY", "credentials file (~/.blaxel/config.yaml)"
}

// authSource holds the resolved authentication source for the current session.
var authSource AuthSource

// SetAuthSource stores the authentication source for later use in error messages.
func SetAuthSource(src AuthSource) {
	authSource = src
}

// GetAuthSource returns the currently stored authentication source.
func GetAuthSource() AuthSource {
	return authSource
}

// ResolveAuthSource determines which credentials the CLI is actually using.
//
// Resolution order (mirrors the SDK):
//  1. Config-file credentials for the workspace (~/.blaxel/config.yaml) override
//     environment variables because NewClientFromConfig appends them after
//     DefaultClientOptions.
//  2. If no config-file credentials exist for the workspace, environment
//     variables BL_API_KEY / BL_CLIENT_CREDENTIALS are used.
func ResolveAuthSource(workspace string) AuthSource {
	creds, _ := blaxel.LoadCredentials(workspace)

	// Config-file credentials take precedence (added last → "last wins").
	if creds.APIKey != "" {
		return AuthSource{Method: "API key", Origin: fmt.Sprintf("credentials file (~/.blaxel/config.yaml) for workspace %q", workspace)}
	}
	if creds.AccessToken != "" || creds.RefreshToken != "" {
		return AuthSource{Method: "access token", Origin: fmt.Sprintf("credentials file (~/.blaxel/config.yaml) for workspace %q", workspace)}
	}
	if creds.ClientCredentials != "" {
		return AuthSource{Method: "client credentials", Origin: fmt.Sprintf("credentials file (~/.blaxel/config.yaml) for workspace %q", workspace)}
	}

	// Fall back to environment variables.
	if os.Getenv("BL_API_KEY") != "" {
		return AuthSource{Method: "API key", Origin: "environment variable BL_API_KEY"}
	}
	if os.Getenv("BL_CLIENT_CREDENTIALS") != "" {
		return AuthSource{Method: "client credentials", Origin: "environment variable BL_CLIENT_CREDENTIALS"}
	}

	return AuthSource{}
}

// IsAuthError returns true when err looks like an authentication or
// authorisation failure (HTTP 401/403).
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	// Try the SDK concrete type first.
	if e, ok := err.(*blaxel.Error); ok {
		return e.StatusCode == 401 || e.StatusCode == 403
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "permission denied")
}

// PrintAuthSourceHint prints a coloured hint about the authentication source
// to stderr. Call this after printing an auth-related error so the user can
// immediately see where the credentials came from.
func PrintAuthSourceHint() {
	src := GetAuthSource()
	if src.Origin == "" {
		return
	}

	hint := fmt.Sprintf("Authentication is using %s from %s", src.Method, src.Origin)
	PrintDiagnostic(fmt.Sprintf("%s %s",
		color.New(color.FgYellow, color.Bold).Sprint("⚠"),
		color.New(color.FgYellow).Sprint(hint)))

	// Extra guidance when the auth comes from an env var — the most common
	// source of "stale credential" mistakes.
	if strings.Contains(src.Origin, "environment variable") {
		envVar := "BL_API_KEY"
		if strings.Contains(src.Origin, "BL_CLIENT_CREDENTIALS") {
			envVar = "BL_CLIENT_CREDENTIALS"
		}
		PrintDiagnostic(fmt.Sprintf("  %s",
			color.New(color.FgYellow).Sprintf(
				"If this is unexpected, unset the variable with 'unset %s' or update its value.", envVar)))
	}
}
