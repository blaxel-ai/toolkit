package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/require"
)

func TestPushExistingImageResourceRequest(t *testing.T) {
	for _, tc := range []struct {
		name, config, image string
		memory, volume      *int
	}{
		{name: "omitted"},
		{name: "empty build", config: "[build]\n"},
		{name: "both", config: "[build]\nmemoryMb=16384\nvolumeMb=32768", memory: buildSize(16384), volume: buildSize(32768)},
		{name: "memory only", config: "[build]\nmemoryMb=8192", memory: buildSize(8192)},
		{name: "volume only", config: "[build]\nvolumeMb=65536", volume: buildSize(65536)},
		{name: "explicit zero", config: "[build]\nvolumeMb=0", volume: buildSize(0)},
		{name: "invalid values reach validation", config: "[build]\nmemoryMb=0\nvolumeMb=-1", memory: buildSize(0), volume: buildSize(-1)},
		{name: "plain registration", image: "sandbox/existing:latest", config: "[build]\nmemoryMb=16384\nvolumeMb=32768"},
		{name: "dotted registry with port", image: "registry.example.com:5000/app:latest", config: "[build]\nmemoryMb=16384\nvolumeMb=32768", memory: buildSize(16384), volume: buildSize(32768)},
		{name: "dotless registry follows API registration", image: "localhost:5000/app:latest", config: "[build]\nmemoryMb=16384\nvolumeMb=32768"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg core.Config
			require.NoError(t, toml.Unmarshal([]byte(tc.config), &cfg))
			requests := make(chan map[string]json.RawMessage, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/images", r.URL.Path)
				require.Equal(t, "true", r.URL.Query().Get("skip-build"))
				var request map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				requests <- request
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"phone","resourceType":"sandbox","build":true}`))
			}))
			defer server.Close()
			client := blaxel.NewClient(option.WithBaseURL(server.URL+"/"), option.WithAPIKey("test"), option.WithMaxRetries(0))
			image := tc.image
			if image == "" {
				image = "registry.example.com/phone:tag"
			}
			response, err := pushExistingImage(t.Context(), &client, createImageRequest{Name: "phone", ResourceType: "sandbox", Image: image, DockerConfig: `{"auths":{}}`}, cfg.Build, option.WithQuery("skip-build", "true"))
			require.NoError(t, err)
			require.True(t, response.Build)
			body := <-requests
			for key, want := range map[string]*int{"memoryMb": tc.memory, "volumeMb": tc.volume} {
				raw, present := body[key]
				require.Equal(t, want != nil, present, key)
				if want != nil {
					var got int
					require.NoError(t, json.Unmarshal(raw, &got))
					require.Equal(t, *want, got, key)
				}
			}
			require.NotContains(t, body, "labels", "registry import must not send source-upload labels")
			require.JSONEq(t, `"{\"auths\":{}}"`, string(body["dockerConfig"]))
		})
	}
}

func TestSourceUploadKeepsExplicitZeroInSignedLabels(t *testing.T) {
	var cfg core.Config
	require.NoError(t, toml.Unmarshal([]byte("[build]\nmemoryMb=16384\nvolumeMb=0"), &cfg))
	request := createImageRequest{Name: "phone", ResourceType: "sandbox", Labels: buildLabels(cfg.Build)}
	raw, err := json.Marshal(request)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	require.NotContains(t, body, "memoryMb")
	require.NotContains(t, body, "volumeMb")
	require.Equal(t, "0", request.Labels["x-blaxel-build-volume"])
	require.Equal(t, "16384", request.Labels["x-blaxel-build-memory"])
	deployment := Deployment{}
	deployment.WithUploadMetadata(request.Labels)
	require.Equal(t, request.Labels, deployment.uploadMetadata)
}

func TestPushCommandResourceFlagsReachHTTP(t *testing.T) {
	for _, tc := range []struct {
		name           string
		flags          []string
		memory, volume int
		noConfig       bool
	}{
		{name: "config values", memory: 8192, volume: 16384},
		{name: "flags without a config file", flags: []string{"--memory", "16384", "--volume", "0"}, memory: 16384, volume: 0, noConfig: true},
		{name: "both flags", flags: []string{"--memory", "16384", "--volume", "32768"}, memory: 16384, volume: 32768},
		{name: "memory only", flags: []string{"--memory", "32768"}, memory: 32768, volume: 16384},
		{name: "volume zero", flags: []string{"--volume", "0"}, memory: 8192, volume: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			t.Chdir(directory)
			if !tc.noConfig {
				require.NoError(t, os.WriteFile(filepath.Join(directory, "blaxel.toml"), []byte("type='sandbox'\nname='phone'\nimage='registry.example.com/config:tag'\n[build]\nmemoryMb=8192\nvolumeMb=16384\n"), 0600))
			}
			core.ResetConfig()
			t.Cleanup(core.ResetConfig)
			original := core.GetClient()
			t.Cleanup(func() { core.SetClient(original) })
			requests := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				requests <- request
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"phone","resourceType":"sandbox","build":false}`))
			}))
			defer server.Close()
			client := blaxel.NewClient(option.WithBaseURL(server.URL+"/"), option.WithAPIKey("test"), option.WithMaxRetries(0))
			core.SetClient(&client)
			cmd := PushCmd()
			cmd.SetArgs(append([]string{"--yes", "--type", "sandbox", "--name", "phone", "--image", "registry.example.com/flag:tag"}, tc.flags...))
			require.NoError(t, cmd.Execute())
			request := <-requests
			require.Equal(t, float64(tc.memory), request["memoryMb"])
			require.Equal(t, float64(tc.volume), request["volumeMb"])
			require.Equal(t, "registry.example.com/flag:tag", request["image"])
			require.NotContains(t, request, "labels")
			// The override is local to this invocation, never persisted into the TOML config.
			if tc.noConfig {
				require.Nil(t, core.GetConfig().Build)
			} else {
				require.Equal(t, 8192, *core.GetConfig().Build.MemoryMb)
				require.Equal(t, 16384, *core.GetConfig().Build.VolumeMb)
			}
		})
	}
}

func TestPushResourceFlagValidationAndSourceLabels(t *testing.T) {
	for _, args := range [][]string{{"--memory", "0"}, {"--memory", "32769"}, {"--volume", "-1"}, {"--volume", "131073"}} {
		cmd := PushCmd()
		require.NoError(t, cmd.ParseFlags(args))
		_, err := pushBuildConfig(cmd, nil)
		require.Error(t, err)
	}
	cmd := PushCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--memory", "16384", "--volume", "0"}))
	build, err := pushBuildConfig(cmd, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"x-blaxel-build-memory": "16384", "x-blaxel-build-volume": "0"}, buildLabels(build))
}
