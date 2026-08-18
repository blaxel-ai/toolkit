package cli

import (
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
)

// [build] memory and scratch cannot reach the builder the way [build] slim
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

// The two upload paths are signed differently, and sending a header the URL was
// not signed for fails the upload outright. push gets a URL signed with the
// [build] choices; deploy gets its URL from the resource endpoint, which signs
// none — so reading the metadata from global config, as a first version did,
// broke every deploy of a project that declared a [build] section.
func TestUploadOnlySendsMetadataItWasGiven(t *testing.T) {
	deploy := &Deployment{}
	if len(deploy.uploadMetadata) != 0 {
		t.Errorf("a deploy must send no metadata, got %v", deploy.uploadMetadata)
	}

	push := &Deployment{}
	signed := buildLabels(&core.BuildConfig{Experimental: true, MemoryMb: 8192})
	push.WithUploadMetadata(signed)
	if len(push.uploadMetadata) != len(signed) {
		t.Fatalf("push carries %v, signed %v", push.uploadMetadata, signed)
	}
	for k, v := range signed {
		if push.uploadMetadata[k] != v {
			t.Errorf("%s = %q, signed %q", k, push.uploadMetadata[k], v)
		}
	}
}
