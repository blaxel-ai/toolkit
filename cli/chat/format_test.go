package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string)
	}{
		{
			name:  "plain text",
			input: "Hello, world!",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Hello")
				assert.Contains(t, result, "world")
			},
		},
		{
			name:  "markdown bold",
			input: "**bold text**",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "bold text")
			},
		},
		{
			name:  "markdown code",
			input: "`inline code`",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "inline code")
			},
		},
		{
			name:  "markdown list",
			input: "- item 1\n- item 2",
			validate: func(t *testing.T, result string) {
				// glamour adds ANSI codes and transforms the list
				// Just check it doesn't panic and returns non-empty
				assert.NotEmpty(t, result)
			},
		},
		{
			name:  "empty string",
			input: "",
			validate: func(t *testing.T, result string) {
				// Should not panic on empty string
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatText(tt.input)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, result string)
	}{
		{
			name:  "plain text",
			input: "Hello, world!",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Hello")
			},
		},
		{
			name:  "markdown heading",
			input: "# Heading",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Heading")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdown(tt.input)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFormatMarkdownImageRegex(t *testing.T) {
	// Test that the regex correctly identifies markdown images
	// We don't actually test image conversion here (requires network)
	tests := []struct {
		name        string
		input       string
		hasImage    bool
		expectedAlt string
	}{
		{
			name:     "no image",
			input:    "Just plain text",
			hasImage: false,
		},
		{
			name:        "with image syntax",
			input:       "![alt text](https://example.com/image.png)",
			hasImage:    true,
			expectedAlt: "alt text",
		},
		{
			name:        "multiple images",
			input:       "![img1](url1) and ![img2](url2)",
			hasImage:    true,
			expectedAlt: "img1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We just verify the function doesn't panic
			// Actual image conversion would need network mocking
			result := FormatMarkdownImage(tt.input)
			if !tt.hasImage {
				assert.Equal(t, tt.input, result)
			}
		})
	}
}

func TestFormatTextWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unicode characters", "Hello 🌍 World 日本語"},
		{"html entities", "&lt;script&gt;alert('xss')&lt;/script&gt;"},
		{"newlines", "line1\nline2\nline3"},
		{"tabs", "col1\tcol2\tcol3"},
		{"backslash escapes", "path\\to\\file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := FormatText(tt.input)
			assert.NotEmpty(t, result)
		})
	}
}

func TestFormatMarkdownWithCodeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "fenced code block",
			input: `Here's some code:
` + "```" + `go
func main() {
    fmt.Println("Hello")
}
` + "```",
		},
		{
			name: "multiple code blocks",
			input: "Start\n```\ncode1\n```\nMiddle\n```\ncode2\n```\nEnd",
		},
		{
			name:  "inline code mixed with text",
			input: "Run `go test` and then `go build` to compile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdown(tt.input)
			assert.NotEmpty(t, result)
		})
	}
}

func TestFormatMarkdownWithLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "inline link",
			input: "[Click here](https://example.com)",
		},
		{
			name:  "multiple links",
			input: "Visit [Google](https://google.com) or [GitHub](https://github.com)",
		},
		{
			name:  "link with title",
			input: "[Example](https://example.com \"Example Site\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdown(tt.input)
			assert.NotEmpty(t, result)
		})
	}
}
