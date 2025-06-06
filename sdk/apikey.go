package sdk

import (
	"context"
	"net/http"
)

type ApiKeyAuth struct {
	credentials   Credentials
	workspaceName string
}

func NewApiKeyProvider(credentials Credentials, workspaceName string) *ApiKeyAuth {
	return &ApiKeyAuth{credentials: credentials, workspaceName: workspaceName}
}

func (s *ApiKeyAuth) Intercept(ctx context.Context, req *http.Request) error {
	headers, err := s.GetHeaders()
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return nil
}

func (s *ApiKeyAuth) GetHeaders() (map[string]string, error) {
	return map[string]string{
		"X-Blaxel-Authorization": "Bearer " + s.credentials.APIKey,
		"X-Blaxel-Workspace":     s.workspaceName,
	}, nil
}
