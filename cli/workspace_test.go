package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListOrSetWorkspacesCmd(t *testing.T) {
	cmd := ListOrSetWorkspacesCmd()

	assert.Equal(t, "workspaces [workspace]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "ws")
	assert.Contains(t, cmd.Aliases, "workspace")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify --current flag exists
	flag := cmd.Flags().Lookup("current")
	assert.NotNil(t, flag)
}

func TestCheckWorkspaceAccessFunction(t *testing.T) {
	// Testing that the function exists and has correct signature
	assert.NotNil(t, CheckWorkspaceAccess)
}

func TestVersionCmd(t *testing.T) {
	cmd := VersionCmd()

	assert.Equal(t, "version", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestTokenCmd(t *testing.T) {
	cmd := TokenCmd()

	assert.Equal(t, "token [workspace]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestLoginCmd(t *testing.T) {
	cmd := LoginCmd()

	assert.Equal(t, "login [workspace]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestLogoutCmd(t *testing.T) {
	cmd := LogoutCmd()

	assert.Equal(t, "logout [workspace]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestListOrSetWorkspacesCmdExamples(t *testing.T) {
	cmd := ListOrSetWorkspacesCmd()

	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "bl workspaces")
}

func TestListOrSetWorkspacesCmdLong(t *testing.T) {
	cmd := ListOrSetWorkspacesCmd()

	assert.Contains(t, cmd.Long, "workspace")
	assert.Contains(t, cmd.Long, "Isolation")
}

func TestLoginCmdDescription(t *testing.T) {
	cmd := LoginCmd()

	assert.NotEmpty(t, cmd.Long)
	assert.Contains(t, cmd.Long, "Authenticate")
}

func TestLogoutCmdDescription(t *testing.T) {
	cmd := LogoutCmd()

	assert.NotEmpty(t, cmd.Long)
	assert.Contains(t, cmd.Long, "credentials")
}

func TestTokenCmdArguments(t *testing.T) {
	cmd := TokenCmd()

	// Token cmd takes optional workspace argument
	assert.NotNil(t, cmd.Args)
}
