package sdk

import "strings"

func BlString(s string) *string {
	return &s
}

func Pluralize(word string) string {
	lower := strings.ToLower(word)

	switch {
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh"):
		return word + "es"
	case strings.HasSuffix(lower, "y") && !strings.HasSuffix(lower, "ay") && !strings.HasSuffix(lower, "ey") && !strings.HasSuffix(lower, "iy") && !strings.HasSuffix(lower, "oy") && !strings.HasSuffix(lower, "uy"):
		return word[:len(word)-1] + "ies"
	default:
		return word + "s"
	}
}
