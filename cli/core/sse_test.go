package core

import (
	"strings"
	"testing"
)

func TestReadSSEStream_OpenAIFormat(t *testing.T) {
	input := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello" {
		t.Errorf("expected 'Hello', got %q", chunks[0])
	}
	if chunks[1] != " world" {
		t.Errorf("expected ' world', got %q", chunks[1])
	}
}

func TestReadSSEStream_ContentField(t *testing.T) {
	input := `data: {"content":"chunk1"}
data: {"content":"chunk2"}
data: [DONE]
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "chunk1" || chunks[1] != "chunk2" {
		t.Errorf("unexpected chunks: %v", chunks)
	}
}

func TestReadSSEStream_TextField(t *testing.T) {
	input := `data: {"text":"hello"}
data: [DONE]
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("expected ['hello'], got %v", chunks)
	}
}

func TestReadSSEStream_RawTextData(t *testing.T) {
	input := `data: just plain text
data: more text
data: [DONE]
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "just plain text" {
		t.Errorf("expected 'just plain text', got %q", chunks[0])
	}
}

func TestReadSSEStream_NDJSON(t *testing.T) {
	input := `{"content":"line1"}
{"content":"line2"}
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "line1" || chunks[1] != "line2" {
		t.Errorf("unexpected chunks: %v", chunks)
	}
}

func TestReadSSEStream_PlainText(t *testing.T) {
	input := "Hello world\nSecond line\n"
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello world\n" {
		t.Errorf("expected 'Hello world\\n', got %q", chunks[0])
	}
}

func TestReadSSEStream_EmptyLines(t *testing.T) {
	input := `data: {"content":"a"}

data: {"content":"b"}

data: [DONE]
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestReadSSEStream_DoneStops(t *testing.T) {
	input := `data: {"content":"before"}
data: [DONE]
data: {"content":"after"}
`
	var chunks []string
	err := ReadSSEStream(strings.NewReader(input), func(text string) {
		chunks = append(chunks, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (should stop at DONE), got %d", len(chunks))
	}
	if chunks[0] != "before" {
		t.Errorf("expected 'before', got %q", chunks[0])
	}
}

func TestIsStreamingResponse(t *testing.T) {
	tests := []struct {
		contentType string
		connection  string
		expected    bool
	}{
		{"text/event-stream", "", true},
		{"text/event-stream; charset=utf-8", "", true},
		{"text/plain", "", true},
		{"application/x-ndjson", "", true},
		{"application/json", "keep-alive", false},
		{"application/json", "", false},
	}

	for _, tt := range tests {
		got := IsStreamingResponse(tt.contentType, tt.connection)
		if got != tt.expected {
			t.Errorf("IsStreamingResponse(%q, %q) = %v, want %v", tt.contentType, tt.connection, got, tt.expected)
		}
	}
}

func TestExtractTextFromJSON_OpenAI(t *testing.T) {
	input := `{"choices":[{"delta":{"content":"hello"}}]}`
	result := extractTextFromJSON(input)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestExtractTextFromJSON_ContentField(t *testing.T) {
	input := `{"content":"test"}`
	result := extractTextFromJSON(input)
	if result != "test" {
		t.Errorf("expected 'test', got %q", result)
	}
}

func TestExtractTextFromJSON_NotJSON(t *testing.T) {
	result := extractTextFromJSON("not json")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractTextFromJSON_NoTextField(t *testing.T) {
	input := `{"id":"123","status":"ok"}`
	result := extractTextFromJSON(input)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
