package sdk

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AuthErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

type DeviceLogin struct {
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

type DeviceLoginResponse struct {
	ClientID                string `json:"client_id"`
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
}
type DeviceLoginFinalizeRequest struct {
	GrantType  string `json:"grant_type"`
	ClientID   string `json:"client_id"`
	DeviceCode string `json:"device_code"`
}

type DeviceLoginFinalizeResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type BearerToken struct {
	credentials   Credentials
	workspaceName string
	baseUrl       string
}

func NewBearerTokenProvider(credentials Credentials, workspaceName string, baseUrl string) *BearerToken {
	return &BearerToken{credentials: credentials, workspaceName: workspaceName, baseUrl: baseUrl}
}

func (s *BearerToken) GetCredentials() Credentials {
	return s.credentials
}

func (s *BearerToken) RefreshIfNeeded() error {
	// Need to refresh the token if access token expires in less than 10 minutes
	parts := strings.Split(s.credentials.AccessToken, ".")
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
		return s.DoRefresh()
	}

	return nil
}

func (s *BearerToken) Intercept(ctx context.Context, req *http.Request) error {
	headers, err := s.GetHeaders()
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return nil
}

func (s *BearerToken) GetHeaders() (map[string]string, error) {
	if err := s.RefreshIfNeeded(); err != nil {
		return nil, err
	}
	return map[string]string{
		"X-Blaxel-Authorization": fmt.Sprintf("Bearer %s", s.credentials.AccessToken),
		"X-Blaxel-Workspace":     s.workspaceName,
	}, nil
}

func (s *BearerToken) DoRefresh() error {
	if s.credentials.RefreshToken == "" {
		return fmt.Errorf("no refresh token to refresh")
	}
	url := s.baseUrl + "/oauth/token"
	refreshData := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": s.credentials.RefreshToken,
		"device_code":   s.credentials.DeviceCode,
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
	defer res.Body.Close()

	// Read the response body first
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if res.StatusCode >= 400 {
		var errorResponse AuthErrorResponse
		if err := json.Unmarshal(buf.Bytes(), &errorResponse); err != nil {
			// If JSON parsing fails, return the raw response
			return fmt.Errorf("failed to refresh token: %s", buf.String())
		}
		return fmt.Errorf("failed to refresh token: %v", errorResponse.Error)
	}

	// Continue with successful response handling
	var finalizeResponse DeviceLoginFinalizeResponse
	if err := json.Unmarshal(buf.Bytes(), &finalizeResponse); err != nil {
		return fmt.Errorf("failed to parse refresh response: %v", err)
	}
	if finalizeResponse.RefreshToken == "" {
		finalizeResponse.RefreshToken = s.credentials.RefreshToken
	}
	creds := Credentials{
		AccessToken:  finalizeResponse.AccessToken,
		RefreshToken: finalizeResponse.RefreshToken,
		ExpiresIn:    finalizeResponse.ExpiresIn,
		DeviceCode:   s.credentials.DeviceCode,
	}

	s.credentials = creds

	SaveCredentials(s.workspaceName, creds)

	return nil
}
