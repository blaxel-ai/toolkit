package server

import (
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
)

func TestGetAllPackages(t *testing.T) {
	t.Run("returns empty map for empty config", func(t *testing.T) {
		config := core.Config{}
		packages := GetAllPackages(config)
		assert.Empty(t, packages)
	})

	t.Run("returns functions from config", func(t *testing.T) {
		config := core.Config{
			Function: map[string]core.Package{
				"func1": {Path: "./functions/func1", Port: 8001},
				"func2": {Path: "./functions/func2", Port: 8002},
			},
		}

		packages := GetAllPackages(config)
		assert.Len(t, packages, 2)
		assert.Equal(t, "function", packages["func1"].Type)
		assert.Equal(t, "./functions/func1", packages["func1"].Path)
		assert.Equal(t, 8001, packages["func1"].Port)
	})

	t.Run("returns agents from config", func(t *testing.T) {
		config := core.Config{
			Agent: map[string]core.Package{
				"agent1": {Path: "./agents/agent1", Port: 8001},
			},
		}

		packages := GetAllPackages(config)
		assert.Len(t, packages, 1)
		assert.Equal(t, "agent", packages["agent1"].Type)
	})

	t.Run("returns jobs from config", func(t *testing.T) {
		config := core.Config{
			Job: map[string]core.Package{
				"job1": {Path: "./jobs/job1", Port: 8001},
			},
		}

		packages := GetAllPackages(config)
		assert.Len(t, packages, 1)
		assert.Equal(t, "job", packages["job1"].Type)
	})

	t.Run("returns mixed packages from config", func(t *testing.T) {
		config := core.Config{
			Function: map[string]core.Package{
				"func1": {Path: "./functions/func1", Port: 8001},
			},
			Agent: map[string]core.Package{
				"agent1": {Path: "./agents/agent1", Port: 8002},
			},
			Job: map[string]core.Package{
				"job1": {Path: "./jobs/job1", Port: 8003},
			},
		}

		packages := GetAllPackages(config)
		assert.Len(t, packages, 3)
		assert.Equal(t, "function", packages["func1"].Type)
		assert.Equal(t, "agent", packages["agent1"].Type)
		assert.Equal(t, "job", packages["job1"].Type)
	})
}

func TestColorize(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		color    string
		notEmpty bool
	}{
		{"red", "test", "red", true},
		{"green", "test", "green", true},
		{"blue", "test", "blue", true},
		{"yellow", "test", "yellow", true},
		{"purple", "test", "purple", true},
		{"cyan", "test", "cyan", true},
		{"white", "test", "white", true},
		{"unknown color returns uncolored", "test", "unknown", true},
		{"empty color returns uncolored", "test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := colorize(tt.text, tt.color)
			if tt.notEmpty {
				assert.NotEmpty(t, result)
				// The result should contain the original text
				// (colors add ANSI codes but text is preserved)
				assert.Contains(t, result, tt.text)
			}
		})
	}
}

func TestPackageCommand(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		cmd := PackageCommand{
			Name:    "my-agent",
			Cwd:     "/path/to/agent",
			Command: "bl",
			Args:    []string{"serve", "--port", "8080"},
			Color:   "blue",
			Envs:    core.CommandEnv{"KEY": "value"},
		}

		assert.Equal(t, "my-agent", cmd.Name)
		assert.Equal(t, "/path/to/agent", cmd.Cwd)
		assert.Equal(t, "bl", cmd.Command)
		assert.Equal(t, []string{"serve", "--port", "8080"}, cmd.Args)
		assert.Equal(t, "blue", cmd.Color)
		assert.Equal(t, "value", cmd.Envs["KEY"])
	})
}
