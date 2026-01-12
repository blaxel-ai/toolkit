package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

	envFileFlag := cmd.Flags().Lookup("env-file")
	assert.NotNil(t, envFileFlag)
	assert.Equal(t, "e", envFileFlag.Shorthand)

	secretsFlag := cmd.Flags().Lookup("secrets")
	assert.NotNil(t, secretsFlag)
	assert.Equal(t, "s", secretsFlag.Shorthand)

	dirFlag := cmd.Flags().Lookup("directory")
	assert.NotNil(t, dirFlag)

	outputFlag := cmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "o", outputFlag.Shorthand)
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
	defer os.RemoveAll(tempDir)

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
