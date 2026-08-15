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
