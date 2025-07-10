package test

import (
	"fmt"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
)

func TestConfig(t *testing.T) {
	core.ReadConfigToml(".")
	config := core.GetConfig()
	fmt.Println(config.Runtime)
}
