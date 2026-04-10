package core

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// ReadSSEStream reads Server-Sent Events from an io.Reader, extracting text
// content from each event and calling onChunk for each piece of text.
// It handles SSE data: lines, NDJSON, and plain text formats.
// Returns any scanner error encountered during reading.
func ReadSSEStream(reader io.Reader, onChunk func(text string)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // support up to 10 MB per line
	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			text := extractTextFromJSON(data)
			if text != "" {
				onChunk(text)
				continue
			}

			onChunk(data)
		} else if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			// NDJSON format
			text := extractTextFromJSON(line)
			if text != "" {
				onChunk(text)
				continue
			}
			onChunk(line)
		} else {
			// Plain text
			onChunk(line + "\n")
		}
	}
	return scanner.Err()
}

// extractTextFromJSON tries to extract text content from a JSON object.
// Supports OpenAI streaming format (choices[0].delta.content) and common
// content/text/message fields.
func extractTextFromJSON(data string) string {
	if !strings.HasPrefix(data, "{") || !strings.HasSuffix(data, "}") {
		return ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return ""
	}

	// OpenAI-style: choices[0].delta.content
	if choices, ok := obj["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok && content != "" {
					return content
				}
			}
		}
	}

	// Common content fields
	for _, key := range []string{"content", "text", "message"} {
		if val, ok := obj[key].(string); ok && val != "" {
			return val
		}
	}

	return ""
}

// IsStreamingResponse checks if the HTTP response indicates a streaming response
// based on Content-Type and Connection headers.
func IsStreamingResponse(contentType string, connection string) bool {
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "application/x-ndjson")
}
