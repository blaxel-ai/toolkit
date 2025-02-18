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
	req.Header.Set("X-Blaxel-Api-Key", s.credentials.APIKey)
	req.Header.Set("X-Blaxel-Workspace", s.workspaceName)
	return nil
}
