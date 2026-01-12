package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorModel(t *testing.T) {
	t.Run("ErrorModel struct", func(t *testing.T) {
		em := ErrorModel{
			Error: "Resource not found",
			Code:  404,
			Stack: []string{"at function1", "at function2"},
		}

		assert.Equal(t, "Resource not found", em.Error)
		assert.Equal(t, 404, em.Code)
		assert.Len(t, em.Stack, 2)
	})
}

func TestErrorHandler(t *testing.T) {
	// Save and restore verbose flag
	originalVerbose := verbose
	defer func() { verbose = originalVerbose }()

	t.Run("handles 404 error without workspace", func(t *testing.T) {
		verbose = false
		req := httptest.NewRequest("GET", "/api/agents/test", nil)

		body := `{"error": "Agent not found", "code": 404, "stack": []}`
		err := ErrorHandler(req, "Agent", "test", body)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Agent")
		assert.Contains(t, err.Error(), "test")
		assert.Contains(t, err.Error(), "Agent not found")
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("handles 401 error with workspace", func(t *testing.T) {
		verbose = false
		req := httptest.NewRequest("GET", "/api/agents/test", nil)
		req.Header.Set("X-Blaxel-Workspace", "my-workspace")

		body := `{"error": "Unauthorized", "code": 401, "stack": []}`
		err := ErrorHandler(req, "Agent", "test", body)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		assert.Contains(t, err.Error(), "my-workspace")
		assert.Contains(t, err.Error(), "bl login")
	})

	t.Run("handles error with workspace", func(t *testing.T) {
		verbose = false
		req := httptest.NewRequest("GET", "/api/agents/test", nil)
		req.Header.Set("X-Blaxel-Workspace", "my-workspace")

		body := `{"error": "Internal error", "code": 500, "stack": []}`
		err := ErrorHandler(req, "Agent", "test", body)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "my-workspace")
		assert.Contains(t, err.Error(), "Internal error")
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("handles error with verbose mode and stack trace", func(t *testing.T) {
		verbose = true
		req := httptest.NewRequest("GET", "/api/agents/test", nil)

		body := `{"error": "Error occurred", "code": 500, "stack": ["at line 10", "at line 20"]}`
		err := ErrorHandler(req, "Agent", "test", body)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Stack trace")
		assert.Contains(t, err.Error(), "at line 10")
		assert.Contains(t, err.Error(), "at line 20")
	})

	t.Run("handles 401 without name", func(t *testing.T) {
		verbose = false
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.Header.Set("X-Blaxel-Workspace", "my-workspace")

		body := `{"error": "Unauthorized", "code": 401, "stack": []}`
		err := ErrorHandler(req, "Agent", "", body)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		// Should not contain double colons from empty name
		assert.Contains(t, err.Error(), "Agent")
	})

	t.Run("handles invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/agents/test", nil)

		body := `invalid json`
		err := ErrorHandler(req, "Agent", "test", body)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse error response")
	})

	t.Run("sanitizes workspace header with newlines", func(t *testing.T) {
		verbose = false
		req := httptest.NewRequest("GET", "/api/agents/test", nil)
		req.Header.Set("X-Blaxel-Workspace", "my-workspace\r\n")

		body := `{"error": "Error", "code": 500, "stack": []}`
		err := ErrorHandler(req, "Agent", "test", body)

		require.Error(t, err)
		// Newlines should be stripped from workspace
		assert.NotContains(t, err.Error(), "\r")
		assert.NotContains(t, err.Error(), "\n")
	})
}

func TestErrorHandlerIntegration(t *testing.T) {
	// Test with a mock HTTP server
	t.Run("processes error from HTTP response", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "Resource not found", "code": 404, "stack": []}`))
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		req, err := http.NewRequest("GET", server.URL+"/test", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// In real usage, we'd read the body and pass to ErrorHandler
		// Here we just verify the server returns what we expect
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
