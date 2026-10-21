package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func writeWorkspaceConfig(t *testing.T, current string, names ...string) string {
	t.Helper()
	home := t.TempDir()
	cfg := "context:\n  workspace: " + current + "\nworkspaces:\n"
	for _, name := range names {
		cfg += "  - name: " + name + "\n    credentials:\n      apiKey: test\n"
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".blaxel"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".blaxel", "config.yaml"), []byte(cfg), 0o600))
	return home
}

func TestValidateWorkspaceSwitch(t *testing.T) {
	cfg := blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "dev"}, {Name: "prod"}}}

	assert.NoError(t, validateWorkspaceSwitch("prod", nil, cfg))

	err := validateWorkspaceSwitch("staging", nil, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace 'staging' is not one you are logged in to")
	assert.Contains(t, err.Error(), "the current workspace was not changed")
	assert.Contains(t, err.Error(), "dev, prod")
	assert.Contains(t, err.Error(), "'bl workspaces'")
	assert.Contains(t, err.Error(), "'bl login staging'")

	err = validateWorkspaceSwitch("switch", []string{"prod"}, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace 'switch' is not one you are logged in to")
	assert.Contains(t, err.Error(), "pass the name directly: 'bl workspaces prod'")

	err = validateWorkspaceSwitch("dev", nil, blaxel.Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in to any workspace yet: run 'bl login dev'")

	// A workspace that happens to be named like a verb is still a workspace.
	named := blaxel.Config{Workspaces: []blaxel.WorkspaceConfig{{Name: "switch"}}}
	assert.NoError(t, validateWorkspaceSwitch("switch", []string{"prod"}, named))
}

func TestWorkspacesSwitchToKnownWorkspace(t *testing.T) {
	home := writeWorkspaceConfig(t, "dev", "dev", "prod")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cmd := ListOrSetWorkspacesCmd()
	cmd.SetArgs([]string{"prod"})
	require.NoError(t, cmd.Execute())

	ctx, err := blaxel.CurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "prod", ctx.Workspace)
}

func TestWorkspacesSwitchRejectsUnknownWorkspace(t *testing.T) {
	home := writeWorkspaceConfig(t, "dev", "dev", "prod")

	cmd := exec.Command(os.Args[0], "-test.run=TestWorkspacesSwitchRejectsUnknownWorkspaceHelper")
	cmd.Env = append(os.Environ(), "BLAXEL_TEST_WORKSPACE_SWITCH=1", "HOME="+home, "USERPROFILE="+home, "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.NotContains(t, stdout.String(), "Current workspace set to")
	assert.Contains(t, stderr.String(), "workspace 'switch' is not one you are logged in to")
	assert.Contains(t, stderr.String(), "'bl workspaces prod'")

	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ctx, err := blaxel.CurrentContext()
	require.NoError(t, err)
	assert.Equal(t, "dev", ctx.Workspace)
}

func TestWorkspacesSwitchRejectsUnknownWorkspaceHelper(t *testing.T) {
	if os.Getenv("BLAXEL_TEST_WORKSPACE_SWITCH") == "" {
		t.Skip("helper only runs in a subprocess")
	}
	cmd := ListOrSetWorkspacesCmd()
	cmd.SetArgs([]string{"switch", "prod"})
	require.NoError(t, cmd.Execute())
}
