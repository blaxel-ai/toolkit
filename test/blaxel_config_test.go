package test

import (
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	core.ReadConfigToml(".", true)
	config := core.GetConfig()
	envs := (*config.Runtime)["envs"].([]map[string]interface{})
	ports := (*config.Runtime)["ports"].([]map[string]interface{})
	memory := (*config.Runtime)["memory"].(int64)
	assert.Equal(t, envs[0]["name"], "PLAYWRIGHT_BROWSERS_PATH")
	assert.Equal(t, envs[0]["value"], "/root/.cache/ms-playwright")
	assert.Equal(t, ports[0]["name"], "playwright")
	assert.Equal(t, ports[0]["target"], int64(3000))
	assert.Equal(t, ports[0]["protocol"], "tcp")
	assert.Equal(t, config.Name, "playwright-firefox")
	assert.Equal(t, config.Type, "sandbox")
	assert.Equal(t, memory, int64(8192))
}
