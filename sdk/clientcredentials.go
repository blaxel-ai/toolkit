package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClientCredentials struct {
	credentials   Credentials
	workspaceName string
	baseUrl       string
}

func NewClientCredentialsProvider(credentials Credentials, workspaceName string, baseUrl string) *ClientCredentials {
	return &ClientCredentials{credentials: credentials, workspaceName: workspaceName, baseUrl: baseUrl}
}

func (c *ClientCredentials) GetCredentials() Credentials {
	return c.credentials
}

func (c *ClientCredentials) RefreshIfNeeded() error {
	// Handle client credentials flow if needed
	if c.credentials.ClientCredentials != "" && c.credentials.RefreshToken == "" {
		headers := map[string]string{
			"Authorization": fmt.Sprintf("Basic %s", c.credentials.ClientCredentials),
			"Content-Type":  "application/json",
		}

		body := map[string]string{
			"grant_type": "client_credentials",
		}

		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal client credentials data: %v", err)
		}

		req, err := http.NewRequest("POST", c.baseUrl+"/oauth/token", strings.NewReader(string(jsonData)))
		if err != nil {
			return fmt.Errorf("failed to create request: %v", err)
		}

		for k, v := range headers {
			req.Header.Add(k, v)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to execute request: %v", err)
		}
		defer func() { _ = res.Body.Close() }()

		if res.StatusCode == 401 || res.StatusCode == 403 {
			body, _ := io.ReadAll(res.Body)
			return fmt.Errorf("authentication failed (HTTP %d): %s\nFor more information on authentication, visit: https://docs.blaxel.ai/sdk-reference/introduction#how-authentication-works", res.StatusCode, string(body))
		}

		if res.StatusCode >= 400 {
			body, _ := io.ReadAll(res.Body)
			return fmt.Errorf("request failed (HTTP %d): %s", res.StatusCode, string(body))
		}

		var response DeviceLoginFinalizeResponse
		if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}

		c.credentials.AccessToken = response.AccessToken
		c.credentials.RefreshToken = response.RefreshToken
		c.credentials.ExpiresIn = response.ExpiresIn

		return nil
	}

	// Need to refresh the token if access token expires in less than 10 minutes
	parts := strings.Split(c.credentials.AccessToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWT token format")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("failed to decode JWT claims: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return fmt.Errorf("failed to parse JWT claims: %v", err)
	}

	expTime := time.Unix(int64(claims["exp"].(float64)), 0)
	// Refresh if token expires in less than 10 minutes
	if time.Now().Add(10 * time.Minute).After(expTime) {
		return c.DoRefresh()
	}

	return nil
}

func (c *ClientCredentials) Intercept(ctx context.Context, req *http.Request) error {
	headers, err := c.GetHeaders()
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return nil
}

func (c *ClientCredentials) GetHeaders() (map[string]string, error) {
	if err := c.RefreshIfNeeded(); err != nil {
		return nil, err
	}
	osArch := GetOsArch()
	commitHash := GetCommitHash()
		headers := map[string]string{
		"X-Blaxel-Authorization": fmt.Sprintf("Bearer %s", c.credentials.AccessToken),
		"X-Blaxel-Workspace":     c.workspaceName,
		"User-Agent":             fmt.Sprintf("blaxel/sdk/golang/%s (%s) blaxel/%s", GetVersion(), osArch, commitHash),
	}

	return headers, nil
}

func (c *ClientCredentials) DoRefresh() error {
	if c.credentials.RefreshToken == "" {
		return fmt.Errorf("no refresh token to refresh")
	}
	url := c.baseUrl + "/oauth/token"
	refreshData := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": c.credentials.RefreshToken,
		"device_code":   c.credentials.DeviceCode,
		"client_id":     "blaxel",
	}

	jsonData, err := json.Marshal(refreshData)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh data: %v", err)
	}

	payload := strings.NewReader(string(jsonData))

	req, _ := http.NewRequest("POST", url, payload)

	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)

	if res.StatusCode == 401 || res.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d): %s\nFor more information on authentication, visit: https://docs.blaxel.ai/sdk-reference/introduction#how-authentication-works", res.StatusCode, string(body))
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("failed to refresh token (HTTP %d): %s", res.StatusCode, string(body))
	}

	var finalizeResponse DeviceLoginFinalizeResponse
	if err := json.Unmarshal(body, &finalizeResponse); err != nil {
		return fmt.Errorf("failed to parse refresh response: %v", err)
	}
	if finalizeResponse.RefreshToken == "" {
		finalizeResponse.RefreshToken = c.credentials.RefreshToken
	}
	creds := Credentials{
		AccessToken:  finalizeResponse.AccessToken,
		RefreshToken: finalizeResponse.RefreshToken,
		ExpiresIn:    finalizeResponse.ExpiresIn,
		DeviceCode:   c.credentials.DeviceCode,
	}

	c.credentials = creds

	SaveCredentials(c.workspaceName, creds)

	return nil
}
