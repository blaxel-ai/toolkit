package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
)

// [build] memoryMb and volumeMb cannot reach the builder the way [build] slim
// does: slim is read inside the build environment, while these two size that
// environment and must be known before it exists. Labels are the only channel
// that runs early enough, so a value that fails to become one is a value the
// build never sees.
func TestBuildSizesBecomeLabels(t *testing.T) {
	for _, c := range []struct {
		name  string
		build *core.BuildConfig
		want  map[string]string
	}{
		{"unset leaves it to the platform", nil, map[string]string{}},
		{"no sizes declared", &core.BuildConfig{}, map[string]string{}},
		{
			"memory only",
			&core.BuildConfig{MemoryMb: 16384},
			map[string]string{"x-blaxel-build-memory": "16384"},
		},
		{
			// No volume is the default, so absence carries nothing at all.
			"no volume declared means an in-memory build",
			&core.BuildConfig{MemoryMb: 8192},
			map[string]string{"x-blaxel-build-memory": "8192"},
		},
		{
			"experimental opts into the new builder",
			&core.BuildConfig{Experimental: true},
			map[string]string{"x-blaxel-builder": "sandbox"},
		},
		{
			"both",
			&core.BuildConfig{MemoryMb: 16384, VolumeMb: 60000},
			map[string]string{
				"x-blaxel-build-memory": "16384",
				"x-blaxel-build-volume": "60000",
			},
		},
		{
			"region only",
			&core.BuildConfig{Region: "us-pdx-1"},
			map[string]string{"x-blaxel-build-region": "us-pdx-1"},
		},
		{
			"cacheDrive only",
			&core.BuildConfig{CacheDrive: "build-cache"},
			map[string]string{"x-blaxel-build-cache": "build-cache"},
		},
		{
			"everything together",
			&core.BuildConfig{
				Experimental: true,
				MemoryMb:     16384,
				VolumeMb:     60000,
				Region:       "us-pdx-1",
				CacheDrive:   "build-cache",
			},
			map[string]string{
				"x-blaxel-builder":      "sandbox",
				"x-blaxel-build-memory": "16384",
				"x-blaxel-build-volume": "60000",
				"x-blaxel-build-region": "us-pdx-1",
				"x-blaxel-build-cache":  "build-cache",
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := buildLabels(c.build)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("%s = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// `bl push` builds an image without creating a deployment, so there is no
// resource record for a label to live on. The choices travel on the uploaded
// object instead: the platform signs them into the presigned URL and the upload
// repeats them as x-amz-meta-* headers. The two must agree exactly — a header
// the signature did not cover is rejected as a mismatch, which is what makes
// this a security boundary rather than a convenience.
func TestBuildLabelsAreTheSameOnBothSidesOfTheUpload(t *testing.T) {
	build := &core.BuildConfig{Experimental: true, MemoryMb: 16384, VolumeMb: 60000}

	// What POST /images asks the platform to sign.
	signed := buildLabels(build)
	// What Upload puts on the wire, derived from the same source.
	sent := map[string]string{}
	for name, value := range buildLabels(build) {
		sent["x-amz-meta-"+name] = value
	}

	if len(signed) != len(sent) {
		t.Fatalf("signed %d labels but sent %d headers", len(signed), len(sent))
	}
	for name, value := range signed {
		if got := sent["x-amz-meta-"+name]; got != value {
			t.Errorf("header for %s = %q, signed %q", name, got, value)
		}
	}

	// A project that declares nothing must sign nothing and send nothing, so an
	// ordinary push is unchanged.
	if got := buildLabels(nil); len(got) != 0 {
		t.Errorf("a project with no [build] section produced %v", got)
	}
}

// Push and source-building deploys sign the selected labels into their upload
// URLs, then repeat them as x-amz-meta-* headers. Metadata is set explicitly
// through WithUploadMetadata so Upload never reads global config: a zero-value
// Deployment carries nothing until its caller opts in.
func TestUploadOnlySendsMetadataItWasGiven(t *testing.T) {
	bare := &Deployment{}
	if len(bare.uploadMetadata) != 0 {
		t.Errorf("a bare Deployment must carry no metadata until WithUploadMetadata is called, got %v", bare.uploadMetadata)
	}

	configured := &Deployment{}
	signed := buildLabels(&core.BuildConfig{Experimental: true, MemoryMb: 8192})
	configured.WithUploadMetadata(signed)
	if len(configured.uploadMetadata) != len(signed) {
		t.Fatalf("configured deployment carries %v, signed %v", configured.uploadMetadata, signed)
	}
	for k, v := range signed {
		if configured.uploadMetadata[k] != v {
			t.Errorf("%s = %q, signed %q", k, configured.uploadMetadata[k], v)
		}
	}
}

func TestGenerateDeploymentBuildLabelsOnlyWhenBuildRuns(t *testing.T) {
	server := mockServer(t, map[string]interface{}{
		"GET /agents/": map[string]interface{}{
			"metadata": map[string]interface{}{"name": "build-label-test"},
			"spec": map[string]interface{}{
				"runtime": map[string]interface{}{"image": "registry.blaxel.ai/test-workspace/build-label-test:existing"},
			},
		},
	})
	defer server.Close()
	setupMockClient(t, server.URL)

	for _, test := range []struct {
		name       string
		resource   string
		image      string
		skipBuild  bool
		wantLabels bool
	}{
		{name: "source build", resource: "agent", wantLabels: true},
		{name: "skip build", resource: "agent", skipBuild: true, wantLabels: false},
		{name: "prebuilt image", resource: "agent", image: "docker.io/example/prebuilt:latest", wantLabels: false},
		{name: "volume template upload", resource: "volume-template", wantLabels: false},
		{name: "volume template upload with skip build", resource: "volume-template", skipBuild: true, wantLabels: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			config := "name = \"build-label-test\"\n" +
				"type = \"" + test.resource + "\"\n"
			if test.image != "" {
				config += "image = \"" + test.image + "\"\n"
			}
			config += `
[build]
experimental = true
memoryMb = 8192
volumeMb = 30000
`
			if err := os.WriteFile(filepath.Join(dir, "blaxel.toml"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = os.Chdir(cwd)
				core.ResetConfig()
			})

			core.ResetConfig()
			core.ReadConfigToml(".", true)
			result := (&Deployment{name: "build-label-test"}).GenerateDeployment(test.skipBuild)
			metadata := result.Metadata.(map[string]interface{})
			labels := metadata["labels"].(map[string]interface{})

			want := map[string]string{
				"x-blaxel-builder":      "sandbox",
				"x-blaxel-build-memory": "8192",
				"x-blaxel-build-volume": "30000",
			}
			for name, value := range want {
				got, ok := labels[name]
				if test.wantLabels && (!ok || got != value) {
					t.Errorf("%s = %q, want %q", name, got, value)
				}
				if !test.wantLabels && ok {
					t.Errorf("%s = %q, want label omitted when no build runs", name, got)
				}
			}
		})
	}
}

func TestDeployUploadMetadataUsesFinalGeneratedLabels(t *testing.T) {
	for _, test := range []struct {
		name         string
		config       string
		experimental bool
		wantFinal    map[string]string
		wantUpload   map[string]string
	}{
		{
			name: "legacy manual builder label is retained",
			config: `
[labels]
"x-blaxel-builder" = "legacy"
`,
			wantFinal:  map[string]string{"x-blaxel-builder": "legacy"},
			wantUpload: map[string]string{"x-blaxel-builder": "legacy"},
		},
		{
			name:         "deploy experimental label is sent",
			experimental: true,
			wantFinal:    map[string]string{"x-blaxel-experimental": "true"},
			wantUpload:   map[string]string{"x-blaxel-experimental": "true"},
		},
		{
			name: "ordinary resource label is excluded",
			config: `
[labels]
team = "platform"
`,
			wantFinal:  map[string]string{"team": "platform"},
			wantUpload: nil,
		},
		{
			name: "generated build label overrides manual value",
			config: `
[labels]
"x-blaxel-build-memory" = "manual"

[build]
memoryMb = 8192
`,
			wantFinal:  map[string]string{"x-blaxel-build-memory": "8192"},
			wantUpload: map[string]string{"x-blaxel-build-memory": "8192"},
		},
		{
			name: "build region and cache drive are sent",
			config: `
[build]
region = "us-pdx-1"
cacheDrive = "build-cache"
`,
			wantFinal: map[string]string{
				"x-blaxel-build-region": "us-pdx-1",
				"x-blaxel-build-cache":  "build-cache",
			},
			wantUpload: map[string]string{
				"x-blaxel-build-region": "us-pdx-1",
				"x-blaxel-build-cache":  "build-cache",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, config := generateBuildLabelDeployment(t, test.config, test.experimental, false)
			labels := buildLabelTestResultLabels(t, result)
			for name, value := range test.wantFinal {
				if got := labels[name]; got != value {
					t.Errorf("final label %s = %q, want %q", name, got, value)
				}
			}

			got := deployUploadMetadata(result, config, false)
			if !reflect.DeepEqual(got, test.wantUpload) {
				t.Errorf("upload metadata = %v, want %v", got, test.wantUpload)
			}
		})
	}
}

func TestDeployUploadMetadataFiltersLabelValues(t *testing.T) {
	for _, test := range []struct {
		name   string
		labels map[string]interface{}
		want   map[string]string
	}{
		{
			name:   "ordinary label",
			labels: map[string]interface{}{"team": "platform"},
			want:   nil,
		},
		{
			name:   "empty allowed label",
			labels: map[string]interface{}{"x-blaxel-builder": ""},
			want:   nil,
		},
		{
			name:   "65 byte allowed label",
			labels: map[string]interface{}{"x-blaxel-builder": strings.Repeat("a", 65)},
			want:   nil,
		},
		{
			name:   "64 byte allowed label",
			labels: map[string]interface{}{"x-blaxel-builder": strings.Repeat("a", 64)},
			want:   map[string]string{"x-blaxel-builder": strings.Repeat("a", 64)},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := core.Result{Metadata: map[string]interface{}{"labels": test.labels}}
			got := deployUploadMetadata(result, core.Config{Type: "agent"}, false)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("upload metadata = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDeployUploadMetadataReturnsNilWithoutSourceBuild(t *testing.T) {
	result := core.Result{Metadata: map[string]interface{}{
		"labels": map[string]interface{}{"x-blaxel-builder": "sandbox"},
	}}
	for _, test := range []struct {
		name      string
		config    core.Config
		skipBuild bool
	}{
		{
			name:   "prebuilt image",
			config: core.Config{Type: "agent", Image: "docker.io/example/prebuilt:latest"},
		},
		{
			name:      "skip build",
			config:    core.Config{Type: "agent"},
			skipBuild: true,
		},
		{
			name:   "volume template",
			config: core.Config{Type: "volume-template"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := deployUploadMetadata(result, test.config, test.skipBuild); got != nil {
				t.Errorf("upload metadata = %v, want nil", got)
			}
		})
	}
}

func generateBuildLabelDeployment(t *testing.T, extraConfig string, experimental, skipBuild bool) (core.Result, core.Config) {
	t.Helper()
	dir := t.TempDir()
	config := `name = "build-label-test"
type = "agent"
` + extraConfig
	if err := os.WriteFile(filepath.Join(dir, "blaxel.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
		core.ResetConfig()
	})

	core.ResetConfig()
	core.ReadConfigToml(".", true)
	configValue := core.GetConfig()
	result := (&Deployment{name: "build-label-test", experimental: experimental}).GenerateDeployment(skipBuild)
	return result, configValue
}

func buildLabelTestResultLabels(t *testing.T, result core.Result) map[string]interface{} {
	t.Helper()
	metadata, ok := result.Metadata.(map[string]interface{})
	if !ok {
		t.Fatalf("metadata has type %T, want map", result.Metadata)
	}
	labels, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		t.Fatalf("metadata.labels has type %T, want map", metadata["labels"])
	}
	return labels
}
