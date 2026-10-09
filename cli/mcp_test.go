package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// bl mcp sends credentials only to the stored login's Blaxel origin: no flag
// can point it elsewhere.
func TestMCPCommandHasNoAPIURLFlag(t *testing.T) {
	assert.Nil(t, MCPCmd().Flags().Lookup("api-url"))
}
