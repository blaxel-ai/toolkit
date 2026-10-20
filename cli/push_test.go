package cli

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAPIStatusUsesTypedErrorIdentity(t *testing.T) {
	assert.True(t, isAPIStatus(fmt.Errorf("image lookup: %w", &blaxel.Error{StatusCode: http.StatusNotFound}), http.StatusNotFound))
	assert.False(t, isAPIStatus(errors.New("internal cache entry not found (404)"), http.StatusNotFound))
	assert.False(t, isAPIStatus(&blaxel.Error{StatusCode: http.StatusInternalServerError}, http.StatusNotFound))
}

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
		{
			name:     "image with tag and digest",
			ref:      "docker.io/myorg/myimage:v1@sha256:abc123",
			expected: "myimage",
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

	skipBuildFlag := cmd.Flags().Lookup("skip-build")
	assert.NotNil(t, skipBuildFlag)
	assert.Equal(t, "false", skipBuildFlag.DefValue)

	registryCredFlag := cmd.Flags().Lookup("registry-cred")
	assert.NotNil(t, registryCredFlag)

	dockerConfigFlag := cmd.Flags().Lookup("docker-config")
	assert.NotNil(t, dockerConfigFlag)
}

func TestPushDockerfileFlag(t *testing.T) {
	flag := PushCmd().Flags().Lookup("dockerfile")
	require.NotNil(t, flag)
	assert.Empty(t, flag.DefValue)
}

// Invalid selections fail before any API call or archive, as in bl deploy.
func TestPushDockerfileRejectedBeforeUpload(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	root := t.TempDir()
	writeDockerfileFixture(t, root, "blaxel.toml", "name = \"push-test\"\ntype = \"sandbox\"\n")
	writeDockerfileFixture(t, root, "custom", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"--dockerfile", ""}, "--dockerfile must not be empty"},
		{[]string{"--dockerfile=missing"}, `Dockerfile "missing" not found`},
		{[]string{"--dockerfile", "../custom"}, "must be a relative path inside the project directory"},
		{[]string{"--image", "docker.io/library/alpine:3.22", "--dockerfile", "custom"}, "requires a source build"},
	} {
		stdout, stderr, code, tmp := runCLIProcess(t, root, server.URL, "push", tt.args...)
		assert.Equal(t, 1, code, tt.args)
		assert.Empty(t, stdout, tt.args)
		assert.Contains(t, stderr, tt.want, tt.args)
		assert.Zero(t, requests.Load(), "validation must precede API calls: %v", tt.args)
		assert.Empty(t, tmp, "validation must precede archive generation: %v", tt.args)
	}
}

// The flag, or build.dockerfile in blaxel.toml, replaces the uploaded Dockerfile.
func TestPushDockerfileUpload(t *testing.T) {
	for _, tt := range []struct {
		name, toml string
		args       []string
		want       string
	}{
		{"flag", "", []string{"--dockerfile", "custom"}, "custom"},
		{"toml", "[build]\ndockerfile = \"nested/from-toml\"\n", nil, "nested/from-toml"},
		{"flag wins over toml", "[build]\ndockerfile = \"nested/from-toml\"\n", []string{"--dockerfile", "custom"}, "custom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // each run waits one build-status poll
			var mu sync.Mutex
			var uploaded []byte
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/images":
					w.Header().Set("X-Blaxel-Upload-Url", server.URL+"/upload")
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"name":"push-test","resourceType":"sandbox"}`))
				case r.Method == http.MethodPut && r.URL.Path == "/upload":
					body, _ := io.ReadAll(r.Body)
					mu.Lock()
					uploaded = body
					mu.Unlock()
				case r.Method == http.MethodGet && r.URL.Path == "/images/sandbox/push-test":
					// End the run quickly: the test only checks what was uploaded.
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"metadata":{"name":"push-test","status":"FAILED"}}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			root := t.TempDir()
			fixture := map[string]string{
				"blaxel.toml":      "name = \"push-test\"\ntype = \"sandbox\"\n" + tt.toml,
				"Dockerfile":       "FROM wrong-default\n",
				"custom":           "FROM ghcr.io/blaxel-ai/sandbox:latest\n# custom\n",
				"nested/from-toml": "FROM ghcr.io/blaxel-ai/sandbox:latest\n# from toml\n",
			}
			for path, content := range fixture {
				writeDockerfileFixture(t, root, path, content)
			}
			_, stderr, code, _ := runCLIProcess(t, root, server.URL, "push", tt.args...)
			assert.Equal(t, 1, code)
			assert.Contains(t, stderr, "image build failed")

			mu.Lock()
			defer mu.Unlock()
			require.NotEmpty(t, uploaded, "the archive was uploaded")
			reader, err := zip.NewReader(bytes.NewReader(uploaded), int64(len(uploaded)))
			require.NoError(t, err)
			var dockerfiles []string
			for _, entry := range reader.File {
				if entry.Name != "Dockerfile" {
					continue
				}
				r, err := entry.Open()
				require.NoError(t, err)
				content, err := io.ReadAll(r)
				require.NoError(t, err)
				require.NoError(t, r.Close())
				dockerfiles = append(dockerfiles, string(content))
			}
			assert.Equal(t, []string{fixture[tt.want]}, dockerfiles, "exactly one Dockerfile, with the selected bytes")
		})
	}
}
