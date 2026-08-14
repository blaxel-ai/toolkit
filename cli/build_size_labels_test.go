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
	zero := 0
	big := 40960

	for _, c := range []struct {
		name  string
		build *core.BuildConfig
		want  map[string]string
	}{
		{"unset leaves it to the platform", nil, map[string]string{}},
		{"no sizes declared", &core.BuildConfig{}, map[string]string{}},
		{
			"memory only",
			&core.BuildConfig{Memory: 16384},
			map[string]string{"x-blaxel-build-memory": "16384"},
		},
		{
			// 0 is a request, not an absence: no disk, build in memory. Treating
			// it as unset would silently give back the default scratch.
			"scratch zero asks for an in-memory build",
			&core.BuildConfig{Scratch: &zero},
			map[string]string{"x-blaxel-build-scratch": "0"},
		},
		{
			"both",
			&core.BuildConfig{Memory: 16384, Scratch: &big},
			map[string]string{
				"x-blaxel-build-memory":  "16384",
				"x-blaxel-build-scratch": "40960",
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := buildSizeLabels(c.build)
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
