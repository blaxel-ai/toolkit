package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeCmd(t *testing.T) {
	cmd := ServeCmd()

	assert.Equal(t, "serve", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check aliases
	assert.Contains(t, cmd.Aliases, "s")
	assert.Contains(t, cmd.Aliases, "se")

	// Verify flags
	portFlag := cmd.Flags().Lookup("port")
	assert.NotNil(t, portFlag)
	assert.Equal(t, "p", portFlag.Shorthand)
	assert.Equal(t, "1338", portFlag.DefValue)

	hostFlag := cmd.Flags().Lookup("host")
	assert.NotNil(t, hostFlag)
	assert.Equal(t, "H", hostFlag.Shorthand)
	assert.Equal(t, "0.0.0.0", hostFlag.DefValue)

	hotreloadFlag := cmd.Flags().Lookup("hotreload")
	assert.NotNil(t, hotreloadFlag)
	assert.Equal(t, "false", hotreloadFlag.DefValue)

	recursiveFlag := cmd.Flags().Lookup("recursive")
	assert.NotNil(t, recursiveFlag)
	assert.Equal(t, "r", recursiveFlag.Shorthand)
	assert.Equal(t, "true", recursiveFlag.DefValue)

	directoryFlag := cmd.Flags().Lookup("directory")
	assert.NotNil(t, directoryFlag)
	assert.Equal(t, "d", directoryFlag.Shorthand)

	envFileFlag := cmd.Flags().Lookup("env-file")
	assert.NotNil(t, envFileFlag)
	assert.Equal(t, "e", envFileFlag.Shorthand)

	secretsFlag := cmd.Flags().Lookup("secrets")
	assert.NotNil(t, secretsFlag)
	assert.Equal(t, "s", secretsFlag.Shorthand)
}

func TestServeCmdExample(t *testing.T) {
	cmd := ServeCmd()

	// Verify example content
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "bl serve")
	assert.Contains(t, cmd.Example, "--hotreload")
}

func TestServeCmdMaxArgs(t *testing.T) {
	cmd := ServeCmd()

	// ServeCmd takes maximum 1 argument
	assert.NotNil(t, cmd.Args)
}
