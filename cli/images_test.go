package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name              string
		ref               string
		expectedType      string
		expectedImageName string
		expectedTag       string
		expectError       bool
	}{
		{
			name:              "full reference with tag",
			ref:               "agent/my-image:v1.0",
			expectedType:      "agent",
			expectedImageName: "my-image",
			expectedTag:       "v1.0",
			expectError:       false,
		},
		{
			name:              "reference without tag",
			ref:               "agent/my-image",
			expectedType:      "agent",
			expectedImageName: "my-image",
			expectedTag:       "",
			expectError:       false,
		},
		{
			name:              "function type",
			ref:               "function/my-func:latest",
			expectedType:      "function",
			expectedImageName: "my-func",
			expectedTag:       "latest",
			expectError:       false,
		},
		{
			name:              "job type",
			ref:               "job/data-processor:2.0",
			expectedType:      "job",
			expectedImageName: "data-processor",
			expectedTag:       "2.0",
			expectError:       false,
		},
		{
			name:              "image name with hyphen",
			ref:               "agent/my-cool-image:v1",
			expectedType:      "agent",
			expectedImageName: "my-cool-image",
			expectedTag:       "v1",
			expectError:       false,
		},
		{
			name:        "missing resource type",
			ref:         "my-image:v1.0",
			expectError: true,
		},
		{
			name:        "empty string",
			ref:         "",
			expectError: true,
		},
		{
			name:        "only resource type",
			ref:         "agent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceType, imageName, tag, err := parseImageRef(tt.ref)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, resourceType)
				assert.Equal(t, tt.expectedImageName, imageName)
				assert.Equal(t, tt.expectedTag, tag)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"bytes", 100, "100 B"},
		{"kilobytes", 1024, "1.00 KB"},
		{"megabytes", 1024 * 1024, "1.00 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.00 GB"},
		{"terabytes", 1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{"partial KB", 1536, "1.50 KB"},
		{"partial MB", 1536 * 1024, "1.50 MB"},
		{"zero", 0, "0 B"},
		{"large bytes", 500, "500 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetImagesCmd(t *testing.T) {
	cmd := GetImagesCmd()

	assert.Equal(t, "image [resourceType/imageName[:tag]]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "images")
	assert.Contains(t, cmd.Aliases, "img")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestDeleteImagesCmd(t *testing.T) {
	cmd := DeleteImagesCmd()

	assert.Equal(t, "image resourceType/imageName[:tag] [resourceType/imageName[:tag]...]", cmd.Use)
	assert.Contains(t, cmd.Aliases, "images")
	assert.Contains(t, cmd.Aliases, "img")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestGetImagesCmdExamples(t *testing.T) {
	cmd := GetImagesCmd()

	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "bl get images")
}

func TestDeleteImagesCmdExamples(t *testing.T) {
	cmd := DeleteImagesCmd()

	assert.NotEmpty(t, cmd.Example)
	assert.Contains(t, cmd.Example, "bl delete image")
}

func TestParseImageRefEdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		ref               string
		expectedType      string
		expectedImageName string
		expectedTag       string
		expectError       bool
	}{
		{
			name:              "sandbox type",
			ref:               "sandbox/my-env:dev",
			expectedType:      "sandbox",
			expectedImageName: "my-env",
			expectedTag:       "dev",
			expectError:       false,
		},
		{
			name:              "tag with numbers",
			ref:               "agent/my-agent:v1.2.3",
			expectedType:      "agent",
			expectedImageName: "my-agent",
			expectedTag:       "v1.2.3",
			expectError:       false,
		},
		{
			name:              "tag as latest",
			ref:               "function/api-server:latest",
			expectedType:      "function",
			expectedImageName: "api-server",
			expectedTag:       "latest",
			expectError:       false,
		},
		{
			name:              "image with underscores",
			ref:               "agent/my_agent_name:tag",
			expectedType:      "agent",
			expectedImageName: "my_agent_name",
			expectedTag:       "tag",
			expectError:       false,
		},
		{
			name:              "slash only (empty type and name)",
			ref:               "/",
			expectedType:      "",
			expectedImageName: "",
			expectedTag:       "",
			expectError:       false,
		},
		{
			name:              "empty image name (allowed by parser)",
			ref:               "agent/",
			expectedType:      "agent",
			expectedImageName: "",
			expectedTag:       "",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceType, imageName, tag, err := parseImageRef(tt.ref)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, resourceType)
				assert.Equal(t, tt.expectedImageName, imageName)
				assert.Equal(t, tt.expectedTag, tag)
			}
		})
	}
}

func TestFormatBytesLargeValues(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"2 TB", 2 * 1024 * 1024 * 1024 * 1024, "2.00 TB"},
		{"500 GB", 500 * 1024 * 1024 * 1024, "500.00 GB"},
		{"1.5 TB", int64(1.5 * 1024 * 1024 * 1024 * 1024), "1.50 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
