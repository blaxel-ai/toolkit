package cli

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeprecatedCreateCommandsExitNonZero(t *testing.T) {
	commands := []string{
		"create-agent-app",
		"create-sandbox",
		"create-job",
		"create-mcp",
	}

	for _, commandName := range commands {
		t.Run(commandName, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestDeprecatedCreateCommandsExitNonZeroHelper")
			cmd.Env = append(os.Environ(), "BLAXEL_TEST_DEPRECATED_COMMAND="+commandName)

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
			assert.Contains(t, stderr.String(), "deprecated")
		})
	}
}

func TestDeprecatedCreateCommandsExitNonZeroHelper(t *testing.T) {
	commandName := os.Getenv("BLAXEL_TEST_DEPRECATED_COMMAND")
	if commandName == "" {
		t.Skip("helper only runs in a subprocess")
	}

	var cmd *cobra.Command
	switch commandName {
	case "create-agent-app":
		cmd = CreateAgentAppCmd()
	case "create-sandbox":
		cmd = CreateSandboxCmd()
	case "create-job":
		cmd = CreateJobCmd()
	case "create-mcp":
		cmd = CreateMCPCmd()
	default:
		t.Fatalf("unknown deprecated command %q", commandName)
	}

	cmd.SetArgs(nil)
	require.NoError(t, cmd.Execute())
}
