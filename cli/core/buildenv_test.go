package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBuildEnvBasic(t *testing.T) {
	args, err := parseBuildEnv("FOO=bar\nBAZ=qux\n")
	require.NoError(t, err)
	assert.Equal(t, "bar", args["FOO"])
	assert.Equal(t, "qux", args["BAZ"])
}

func TestParseBuildEnvEmptyValue(t *testing.T) {
	args, err := parseBuildEnv("KEY=\n")
	require.NoError(t, err)
	assert.Equal(t, "", args["KEY"])
}

func TestParseBuildEnvCommentsAndBlanks(t *testing.T) {
	args, err := parseBuildEnv("# comment\n\nFOO=bar\n  # another comment\nBAZ=qux\n\n")
	require.NoError(t, err)
	assert.Len(t, args, 2)
	assert.Equal(t, "bar", args["FOO"])
	assert.Equal(t, "qux", args["BAZ"])
}

func TestParseBuildEnvValueWithEquals(t *testing.T) {
	args, err := parseBuildEnv("TOKEN=abc=def==\n")
	require.NoError(t, err)
	assert.Equal(t, "abc=def==", args["TOKEN"])
}

func TestParseBuildEnvInvalidNoEquals(t *testing.T) {
	_, err := parseBuildEnv("INVALID_LINE\n")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestParseBuildEnvEmptyKey(t *testing.T) {
	_, err := parseBuildEnv("=value\n")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty key")
}

func TestParseBuildEnvEmpty(t *testing.T) {
	args, err := parseBuildEnv("")
	require.NoError(t, err)
	assert.Nil(t, args)
}

func TestParseBuildEnvWhitespaceTrimming(t *testing.T) {
	args, err := parseBuildEnv("  FOO = bar  \n  BAZ=  qux  \n")
	require.NoError(t, err)
	assert.Equal(t, "bar", args["FOO"])
	assert.Equal(t, "qux", args["BAZ"])
}

func TestReadBuildEnvDefaultMissing(t *testing.T) {
	dir := t.TempDir()
	args, err := ReadBuildEnv(dir, "")
	require.NoError(t, err)
	assert.Nil(t, args)
}

func TestReadBuildEnvDefaultExists(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".build-env"), []byte("FOO=bar\n"), 0644)
	require.NoError(t, err)

	args, err := ReadBuildEnv(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "bar", args["FOO"])
}

func TestReadBuildEnvCustomPathMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadBuildEnv(dir, ".build-env.production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestReadBuildEnvCustomPathExists(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".build-env.production"), []byte("NODE_ENV=production\n"), 0644)
	require.NoError(t, err)

	args, err := ReadBuildEnv(dir, ".build-env.production")
	require.NoError(t, err)
	assert.Equal(t, "production", args["NODE_ENV"])
}

func TestMergeBuildEnvContent(t *testing.T) {
	tomlArgs := map[string]string{"NODE_ENV": "production", "SHARED": "from-toml"}
	envArgs := map[string]string{"TOKEN": "secret", "SHARED": "from-env"}

	result, count := MergeBuildEnvContent(tomlArgs, envArgs)
	assert.NotNil(t, result)
	assert.Equal(t, 3, count) // NODE_ENV, TOKEN, SHARED (deduplicated)

	// Parse back to verify
	parsed, err := parseBuildEnv(string(result))
	require.NoError(t, err)
	assert.Equal(t, "production", parsed["NODE_ENV"])
	assert.Equal(t, "secret", parsed["TOKEN"])
	assert.Equal(t, "from-env", parsed["SHARED"]) // .build-env wins
}

func TestMergeBuildEnvContentBothNil(t *testing.T) {
	result, count := MergeBuildEnvContent(nil, nil)
	assert.Nil(t, result)
	assert.Equal(t, 0, count)
}

func TestMergeBuildEnvContentOnlyToml(t *testing.T) {
	tomlArgs := map[string]string{"FOO": "bar"}
	result, count := MergeBuildEnvContent(tomlArgs, nil)
	assert.NotNil(t, result)
	assert.Equal(t, 1, count)
	assert.Contains(t, string(result), "FOO=bar")
}

func TestMergeBuildEnvContentOnlyEnv(t *testing.T) {
	envArgs := map[string]string{"FOO": "bar"}
	result, count := MergeBuildEnvContent(nil, envArgs)
	assert.NotNil(t, result)
	assert.Equal(t, 1, count)
	assert.Contains(t, string(result), "FOO=bar")
}
