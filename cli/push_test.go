package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageRefToName(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "simple image with tag",
			ref:      "nginx:latest",
			expected: "nginx",
		},
		{
			name:     "image with registry and tag",
			ref:      "docker.io/library/nginx:latest",
			expected: "nginx",
		},
		{
			name:     "image with org and tag",
			ref:      "ghcr.io/myorg/my-app:v2",
			expected: "my-app",
		},
		{
			name:     "image without tag",
			ref:      "docker.io/myorg/myimage",
			expected: "myimage",
		},
		{
			name:     "image with digest",
			ref:      "docker.io/myorg/myimage@sha256:abc123",
			expected: "myimage",
		},
		{
			name:     "simple image without tag",
			ref:      "alpine",
			expected: "alpine",
		},
		{
			name:     "localhost with port and image",
			ref:      "localhost:5000/myimage:v1",
			expected: "myimage",
		},
		{
			name:     "deep nested path",
			ref:      "registry.example.com/org/team/project:latest",
			expected: "project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imageRefToName(tt.ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPushCmd(t *testing.T) {
	cmd := PushCmd()

	assert.Equal(t, "push", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flag := cmd.Flags().Lookup("name")
	assert.NotNil(t, flag)
	assert.Equal(t, "n", flag.Shorthand)

	typeFlag := cmd.Flags().Lookup("type")
	assert.NotNil(t, typeFlag)
	assert.Equal(t, "t", typeFlag.Shorthand)

	imageFlag := cmd.Flags().Lookup("image")
	assert.NotNil(t, imageFlag)
	assert.Equal(t, "", imageFlag.DefValue)

	registryCredFlag := cmd.Flags().Lookup("registry-cred")
	assert.NotNil(t, registryCredFlag)

	dockerConfigFlag := cmd.Flags().Lookup("docker-config")
	assert.NotNil(t, dockerConfigFlag)
}
