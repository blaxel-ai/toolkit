package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameRetrieverStruct(t *testing.T) {
	nr := NameRetriever{}
	nr.Metadata.Name = "test-name"

	assert.Equal(t, "test-name", nr.Metadata.Name)
}

func TestFilterCacheWithAll(t *testing.T) {
	resource := Resource{
		Kind:     "Agent",
		Plural:   "agents",
		Singular: "agent",
	}

	// Test with "all" in names
	names := []string{"all"}
	res := []interface{}{}

	result := filterCache(resource, res, names)
	// Empty result for empty input
	assert.Empty(t, result)
}

func TestFilterCacheWithSpecificNames(t *testing.T) {
	resource := Resource{
		Kind:     "Agent",
		Plural:   "agents",
		Singular: "agent",
	}

	// Test with specific names
	names := []string{"agent-1"}
	res := []interface{}{
		map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "agent-1",
			},
			"spec": map[string]interface{}{},
		},
		map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "agent-2",
			},
			"spec": map[string]interface{}{},
		},
	}

	result := filterCache(resource, res, names)
	// Should only include agent-1
	assert.Contains(t, result, "agent-1")
}

func TestFilterCacheEmptyResults(t *testing.T) {
	resource := Resource{
		Kind:     "Function",
		Plural:   "functions",
		Singular: "function",
	}

	names := []string{"my-function"}
	res := []interface{}{}

	result := filterCache(resource, res, names)
	assert.Empty(t, result)
}
