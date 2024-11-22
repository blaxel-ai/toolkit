package cli

import (
	"regexp"
	"strings"
)

// Entries : ListOperations, ListEnvironments, GetOperation, GetEnvironment
// Results [list, operations, list environments, get operation, get environment]
func formatOperationId(operationId string) []string {
	// Regular expression to match capital letters
	re := regexp.MustCompile(`[A-Z][^A-Z]*`)

	// Find all matches and convert them to lowercase
	words := re.FindAllString(operationId, -1)
	for i, word := range words {
		words[i] = strings.ToLower(word)
	}

	return words
}
