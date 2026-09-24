package core

import (
	"bytes"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

func TestBuildResourceTOMLPresenceRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, raw      string
		memory, volume *int
	}{
		{name: "omitted", raw: "[build]\n"},
		{name: "memory only", raw: "[build]\nmemoryMb=16384", memory: resourceSize(16384)},
		{name: "volume only", raw: "[build]\nvolumeMb=32768", volume: resourceSize(32768)},
		{name: "zero volume", raw: "[build]\nvolumeMb=0", volume: resourceSize(0)},
		{name: "zero memory", raw: "[build]\nmemoryMb=0", memory: resourceSize(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			require.NoError(t, toml.Unmarshal([]byte(tc.raw), &cfg))
			require.NotNil(t, cfg.Build)
			require.Equal(t, tc.memory, cfg.Build.MemoryMb)
			require.Equal(t, tc.volume, cfg.Build.VolumeMb)
			var encoded bytes.Buffer
			require.NoError(t, toml.NewEncoder(&encoded).Encode(cfg))
			var decoded Config
			require.NoError(t, toml.Unmarshal(encoded.Bytes(), &decoded))
			require.Equal(t, tc.memory, decoded.Build.MemoryMb)
			require.Equal(t, tc.volume, decoded.Build.VolumeMb)
		})
	}
}

func resourceSize(value int) *int { return &value }
