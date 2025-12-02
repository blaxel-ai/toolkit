package sdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// AuthAwareHTTPClient wraps an HTTP client and intercepts 401/403 responses
// to modify the response body with authentication documentation
type AuthAwareHTTPClient struct {
	client http.Client
}

// NewAuthAwareHTTPClient creates a new HTTP client that handles authentication errors
func NewAuthAwareHTTPClient() *AuthAwareHTTPClient {
	return &AuthAwareHTTPClient{
		client: http.Client{},
	}
}

// AuthError represents the enhanced error response with authentication documentation
type AuthError struct {
	Message       string `json:"message"`
	Error         string `json:"error,omitempty"`
	Code          int    `json:"code,omitempty"`
	Documentation string `json:"documentation"`
}

// Do executes the HTTP request and wraps authentication errors with documentation links
func (c *AuthAwareHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return resp, err
	}

	// Check for authentication errors (401/403)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Read the original response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp, err
		}
		_ = resp.Body.Close()

		// Try to parse as JSON
		var originalError map[string]interface{}
		_ = json.Unmarshal(bodyBytes, &originalError)

		// Create enhanced error response
		authError := AuthError{
			Documentation: "For more information on authentication, visit: https://docs.blaxel.ai/sdk-reference/introduction#how-authentication-works",
		}

		// Preserve original error message
		if msg, ok := originalError["message"].(string); ok {
			authError.Message = msg
		} else if errMsg, ok := originalError["error"].(string); ok {
			authError.Error = errMsg
		} else {
			authError.Message = string(bodyBytes)
		}

		if code, ok := originalError["code"].(float64); ok {
			authError.Code = int(code)
		} else {
			authError.Code = resp.StatusCode
		}

		// Marshal the enhanced error
		enhancedBody, err := json.Marshal(authError)
		if err != nil {
			// If marshaling fails, append documentation to original body
			enhancedBody = append(bodyBytes, []byte("\nFor more information on authentication, visit: https://docs.blaxel.ai/sdk-reference/introduction#how-authentication-works")...)
		}

		// Replace response body with enhanced version
		resp.Body = io.NopCloser(bytes.NewBuffer(enhancedBody))
		resp.ContentLength = int64(len(enhancedBody))
	}

	return resp, err
}
