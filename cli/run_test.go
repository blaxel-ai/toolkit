package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCmd(t *testing.T) {
	cmd := RunCmd()

	assert.Equal(t, "run resource-type resource-name", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags
	flag := cmd.Flags().Lookup("file")
	assert.NotNil(t, flag)
	assert.Equal(t, "f", flag.Shorthand)

	dataFlag := cmd.Flags().Lookup("data")
	assert.NotNil(t, dataFlag)
	assert.Equal(t, "d", dataFlag.Shorthand)

	pathFlag := cmd.Flags().Lookup("path")
	assert.NotNil(t, pathFlag)

	methodFlag := cmd.Flags().Lookup("method")
	assert.NotNil(t, methodFlag)

	paramsFlag := cmd.Flags().Lookup("params")
	assert.NotNil(t, paramsFlag)

	uploadFileFlag := cmd.Flags().Lookup("upload-file")
	assert.NotNil(t, uploadFileFlag)

	headerFlag := cmd.Flags().Lookup("header")
	assert.NotNil(t, headerFlag)

	debugFlag := cmd.Flags().Lookup("debug")
	assert.NotNil(t, debugFlag)

	localFlag := cmd.Flags().Lookup("local")
	assert.NotNil(t, localFlag)

	portFlag := cmd.Flags().Lookup("port")
	assert.NotNil(t, portFlag)
	assert.Equal(t, "p", portFlag.Shorthand)
	assert.Equal(t, "1338", portFlag.DefValue)

	envFileFlag := cmd.Flags().Lookup("env-file")
	assert.NotNil(t, envFileFlag)
	assert.Equal(t, "e", envFileFlag.Shorthand)

	secretsFlag := cmd.Flags().Lookup("secrets")
	assert.NotNil(t, secretsFlag)
	assert.Equal(t, "s", secretsFlag.Shorthand)

	dirFlag := cmd.Flags().Lookup("directory")
	assert.NotNil(t, dirFlag)

	// --output is now the global persistent flag from root (not local to run)
	// Verify the new --stream and --timeout flags
	streamFlag := cmd.Flags().Lookup("stream")
	assert.NotNil(t, streamFlag)
	assert.Equal(t, "false", streamFlag.DefValue)

	timeoutFlag := cmd.Flags().Lookup("timeout")
	assert.NotNil(t, timeoutFlag)
	assert.Equal(t, "0", timeoutFlag.DefValue)
}

func TestRunCmdParsesGeneratedJobCommand(t *testing.T) {
	cmd := RunCmd()
	require.NoError(t, cmd.ParseFlags([]string{
		"job",
		"my-job",
		"--local",
		"--file",
		"batches/sample-batch.json",
	}))

	assert.Equal(t, []string{"job", "my-job"}, cmd.Flags().Args())
	local, err := cmd.Flags().GetBool("local")
	require.NoError(t, err)
	assert.True(t, local)
	file, err := cmd.Flags().GetString("file")
	require.NoError(t, err)
	assert.Equal(t, "batches/sample-batch.json", file)
}

func TestBatchStruct(t *testing.T) {
	batch := Batch{
		Tasks: []map[string]interface{}{
			{"task": "one", "data": "value1"},
			{"task": "two", "data": "value2"},
		},
	}

	assert.Len(t, batch.Tasks, 2)
	assert.Equal(t, "one", batch.Tasks[0]["task"])
	assert.Equal(t, "two", batch.Tasks[1]["task"])
}

func TestBatchJSONSerialization(t *testing.T) {
	batch := Batch{
		Tasks: []map[string]interface{}{
			{"id": 1, "name": "task1"},
			{"id": 2, "name": "task2"},
		},
	}

	jsonData, err := json.Marshal(batch)
	require.NoError(t, err)

	var parsed Batch
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Len(t, parsed.Tasks, 2)
}

func TestReadBatchFromFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "run_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a JSON batch file
	batchContent := `{
		"tasks": [
			{"id": 1, "action": "process"},
			{"id": 2, "action": "analyze"}
		]
	}`

	jsonPath := filepath.Join(tempDir, "batch.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte(batchContent), 0644))

	// Read and parse
	content, err := os.ReadFile(jsonPath)
	require.NoError(t, err)

	var batch Batch
	err = json.Unmarshal(content, &batch)
	require.NoError(t, err)

	assert.Len(t, batch.Tasks, 2)
	assert.Equal(t, float64(1), batch.Tasks[0]["id"])
	assert.Equal(t, "process", batch.Tasks[0]["action"])
}

func TestRunCmdLongDescription(t *testing.T) {
	cmd := RunCmd()

	// Verify long description contains key information
	assert.Contains(t, cmd.Long, "agent")
	assert.Contains(t, cmd.Long, "model")
	assert.Contains(t, cmd.Long, "job")
	assert.Contains(t, cmd.Long, "function")
	assert.Contains(t, cmd.Long, "Local vs Remote")
}

func TestRunCmdExamples(t *testing.T) {
	cmd := RunCmd()

	// Verify examples exist
	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "bl run agent")
	assert.Contains(t, cmd.Example, "bl run job")
	assert.Contains(t, cmd.Example, "--path /process --file")
}

func TestValidateInlineRunDataJSONReturnsSandboxProcessHint(t *testing.T) {
	err := validateInlineRunDataJSON(`{"command":"bad \' escape"}`, "sandbox", "process")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON passed to --data")
	assert.Contains(t, err.Error(), "sandbox /process")
	assert.Contains(t, err.Error(), "--file")
}

func TestValidateInlineRunDataJSONIgnoresOtherRunData(t *testing.T) {
	require.NoError(t, validateInlineRunDataJSON(`not json`, "agent", "/invoke"))
	require.NoError(t, validateInlineRunDataJSON(`not json`, "sandbox", "/filesystem//tmp"))
}

func TestValidateInlineRunDataJSONAcceptsValidProcessPayload(t *testing.T) {
	payload := `{"command":"sh -lc 'python3 -c \"print(\\\"hello\\\")\"'","waitForCompletion":true}`

	require.NoError(t, validateInlineRunDataJSON(payload, "sandbox", "/process"))
}

func TestBatchStructNestedTasks(t *testing.T) {
	batch := Batch{
		Tasks: []map[string]interface{}{
			{
				"id":   1,
				"data": map[string]interface{}{"nested": "value"},
			},
		},
	}

	assert.Len(t, batch.Tasks, 1)
	assert.Equal(t, 1, batch.Tasks[0]["id"])

	data := batch.Tasks[0]["data"].(map[string]interface{})
	assert.Equal(t, "value", data["nested"])
}

func TestBatchEmptyTasks(t *testing.T) {
	batch := Batch{
		Tasks: []map[string]interface{}{},
	}

	assert.Len(t, batch.Tasks, 0)

	// Marshal/unmarshal
	jsonData, err := json.Marshal(batch)
	require.NoError(t, err)

	var parsed Batch
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Len(t, parsed.Tasks, 0)
}

func TestClassifyRunRequestErrorMarksTransportFailuresExpected(t *testing.T) {
	// A failed read of the run response body or SSE stream can only be an
	// environment/server condition (connection reset, short read, oversized
	// stream line, token-refresh failure) and must not be reported as an
	// unexpected CLI defect.
	cases := []struct {
		name string
		err  error
	}{
		{"unexpected eof", io.ErrUnexpectedEOF},
		{"scanner token too long", bufio.ErrTooLong},
		{"wrapped stream read", fmt.Errorf("error reading stream: %w", io.ErrUnexpectedEOF)},
		{"wrapped body read", fmt.Errorf("error reading response: %w", io.ErrUnexpectedEOF)},
		{"wrapped request", fmt.Errorf("error making request: %w", errors.New("oauth2: cannot fetch token"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classified := classifyRunRequestError(tc.err)
			require.True(t, core.IsExpectedCLIError(classified))
			// The user-facing message is preserved; only the Sentry
			// classification changes.
			require.Equal(t, tc.err.Error(), classified.Error())
		})
	}
}

func TestClassifyRunRequestErrorNilIsNotReported(t *testing.T) {
	require.Nil(t, classifyRunRequestError(nil))
	require.False(t, core.IsExpectedCLIError(classifyRunRequestError(nil)))
}

func TestClassifyRunRequestErrorPreservesTypedClassification(t *testing.T) {
	// An HTTP error carried through %w is already expected and must be returned
	// unchanged so its more specific status category is not downgraded.
	apiErr := &blaxel.Error{StatusCode: 401}
	wrapped := fmt.Errorf("error making request: %w", apiErr)

	classified := classifyRunRequestError(wrapped)

	require.True(t, core.IsExpectedCLIError(classified))
	require.Equal(t, wrapped, classified)
	var gotAPIErr *blaxel.Error
	require.True(t, errors.As(classified, &gotAPIErr))
	require.Equal(t, 401, gotAPIErr.StatusCode)
}

func TestRunRequestTransportErrorUnclassifiedIsReported(t *testing.T) {
	// Guard: without classifyRunRequestError the raw transport failures remain
	// reportable, so the classification above is load-bearing.
	require.False(t, core.IsExpectedCLIError(io.ErrUnexpectedEOF))
	require.False(t, core.IsExpectedCLIError(bufio.ErrTooLong))
	require.False(t, core.IsExpectedCLIError(fmt.Errorf("error reading stream: %w", io.ErrUnexpectedEOF)))
	require.False(t, core.IsExpectedCLIError(fmt.Errorf("error reading response: %w", io.ErrUnexpectedEOF)))
}

func TestBatchTaskWithMultipleFields(t *testing.T) {
	batch := Batch{
		Tasks: []map[string]interface{}{
			{
				"id":     1,
				"name":   "Task One",
				"action": "process",
				"config": map[string]interface{}{
					"timeout": 30,
					"retry":   3,
				},
				"tags": []string{"important", "urgent"},
			},
		},
	}

	task := batch.Tasks[0]
	assert.Equal(t, 1, task["id"])
	assert.Equal(t, "Task One", task["name"])
	assert.Equal(t, "process", task["action"])

	config := task["config"].(map[string]interface{})
	assert.Equal(t, 30, config["timeout"])

	tags := task["tags"].([]string)
	assert.Contains(t, tags, "important")
}
