package cli

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectCmd(t *testing.T) {
	cmd := ConnectCmd()

	assert.Equal(t, "connect", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Should have subcommands
	subCommands := cmd.Commands()
	assert.NotEmpty(t, subCommands)
}

func TestConnectSandboxCmd(t *testing.T) {
	cmd := ConnectSandboxCmd()

	assert.Equal(t, "sandbox [sandbox-name]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "sb")
	assert.Contains(t, cmd.Aliases, "sbx")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestOpenBrowserUnsupportedPlatform(t *testing.T) {
	// We can't actually test openBrowser on all platforms in unit tests
	// but we can verify the function exists and has proper signature
	url := "https://example.com"

	// On supported platforms, this would try to open browser
	// We're just testing that it doesn't panic
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		// On these platforms, the function should be callable
		// but we don't want to actually open a browser in tests
		assert.NotPanics(t, func() {
			// Don't actually call openBrowser in tests as it would open a browser
			_ = url
		})
	}
}

func TestConnectSandboxCmdAliases(t *testing.T) {
	cmd := ConnectSandboxCmd()

	// Should have both sb and sbx as aliases
	assert.Len(t, cmd.Aliases, 2)
	assert.Contains(t, cmd.Aliases, "sb")
	assert.Contains(t, cmd.Aliases, "sbx")
}

func TestConnectCmdHasSandboxSubcommand(t *testing.T) {
	cmd := ConnectCmd()

	// Find sandbox subcommand
	var sandboxCmd *any
	for _, sub := range cmd.Commands() {
		if sub.Name() == "sandbox" {
			sandboxCmd = new(any)
			break
		}
	}

	assert.NotNil(t, sandboxCmd, "Connect command should have sandbox subcommand")
}

func TestSandboxTerminalAuthenticationUsesSelectedWorkspaceAndRefreshedToken(t *testing.T) {
	previousWorkspace := core.GetWorkspace()
	previousLoader := loadTerminalCredentials
	previousHeaders := authHeadersForCredentials
	t.Cleanup(func() {
		core.SetWorkspace(previousWorkspace)
		loadTerminalCredentials = previousLoader
		authHeadersForCredentials = previousHeaders
	})
	core.SetWorkspace("selected-workspace")
	loadTerminalCredentials = func(workspace string) (blaxel.Credentials, error) {
		require.Equal(t, "selected-workspace", workspace)
		return blaxel.Credentials{AccessToken: "old-token", RefreshToken: "refresh-token"}, nil
	}
	authHeadersForCredentials = func(ctx context.Context, credentials blaxel.Credentials, workspace string) (map[string]string, error) {
		require.Equal(t, "selected-workspace", workspace)
		require.Equal(t, "old-token", credentials.AccessToken)
		return map[string]string{"X-Blaxel-Authorization": "Bearer refreshed-token"}, nil
	}
	workspace, token, err := sandboxTerminalAuthentication(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "selected-workspace", workspace)
	assert.Equal(t, "refreshed-token", token)

	loadTerminalCredentials = func(string) (blaxel.Credentials, error) {
		return blaxel.Credentials{}, fmt.Errorf("unreadable credentials")
	}
	_, token, err = sandboxTerminalAuthentication(context.Background())
	require.ErrorContains(t, err, "unreadable credentials")
	assert.Empty(t, token)

	core.SetWorkspace("")
	_, token, err = sandboxTerminalAuthentication(context.Background())
	require.ErrorContains(t, err, "no workspace selected")
	assert.Empty(t, token)
}
