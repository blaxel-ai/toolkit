package cli

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveLoginWorkspace(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		expectedWorkspace  string
		expectedSuggestion string
		expectedErr        string
	}{
		{
			name:              "positional workspace",
			args:              []string{"login", "target-workspace"},
			expectedWorkspace: "target-workspace",
		},
		{
			name:              "inherited workspace flag after command",
			args:              []string{"login", "--workspace", "target-workspace"},
			expectedWorkspace: "target-workspace",
		},
		{
			name:              "inherited workspace shorthand after command",
			args:              []string{"login", "-w", "target-workspace"},
			expectedWorkspace: "target-workspace",
		},
		{
			name:              "inherited workspace flag before command",
			args:              []string{"--workspace", "target-workspace", "login"},
			expectedWorkspace: "target-workspace",
		},
		{
			name:              "matching positional and flag workspace",
			args:              []string{"login", "target-workspace", "--workspace", "target-workspace"},
			expectedWorkspace: "target-workspace",
		},
		{
			name:        "conflicting positional and flag workspace",
			args:        []string{"login", "target-workspace", "--workspace", "other-workspace"},
			expectedErr: `workspace specified twice: positional workspace "target-workspace" conflicts with --workspace "other-workspace"`,
		},
		{
			name: "plain login keeps workspace picker behavior",
			args: []string{"login"},
		},
		{
			name:               "multi word positional suggestion",
			args:               []string{"login", "My", "Workspace"},
			expectedSuggestion: "my-workspace",
		},
		{
			name: "unchanged flag default is ignored",
			args: []string{"login"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace, suggestion, err := executeLoginWorkspaceResolver(t, tt.args)

			assert.Equal(t, tt.expectedWorkspace, workspace)
			assert.Equal(t, tt.expectedSuggestion, suggestion)
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func executeLoginWorkspaceResolver(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	rootCmd := &cobra.Command{
		Use:           "bl",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.PersistentFlags().StringP("workspace", "w", "default-workspace", "Specify the workspace name")

	var (
		ran        bool
		workspace  string
		suggestion string
		resolveErr error
	)
	loginCmd := &cobra.Command{
		Use: "login [workspace]",
		Run: func(cmd *cobra.Command, args []string) {
			ran = true
			workspace, suggestion, resolveErr = resolveLoginWorkspace(cmd, args)
		},
	}
	rootCmd.AddCommand(loginCmd)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	require.NoError(t, rootCmd.Execute())
	require.True(t, ran, "login command did not run")

	return workspace, suggestion, resolveErr
}
