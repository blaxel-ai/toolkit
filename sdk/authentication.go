package sdk

import (
	"context"
	"net/http"
)

type PublicProvider struct{}

func NewPublicProvider() *PublicProvider {
	return &PublicProvider{}
}

func (s *PublicProvider) Intercept(ctx context.Context, req *http.Request) error {
	return nil
}

func (s *PublicProvider) GetHeaders() (map[string]string, error) {
	return nil, nil
}

type AuthProvider interface {
	Intercept(ctx context.Context, req *http.Request) error
	GetHeaders() (map[string]string, error)
}

type RunClientWithCredentials struct {
	ApiURL      string
	RunURL      string
	Credentials Credentials
	Workspace   string
	Headers     map[string]string
}

func GetAuthProvider(credentials Credentials, workspace string, apiUrl string) AuthProvider {
	if credentials.APIKey != "" {
		return NewApiKeyProvider(credentials, workspace)
	} else if credentials.AccessToken != "" {
		return NewBearerTokenProvider(credentials, workspace, apiUrl)
	} else if credentials.ClientCredentials != "" {
		return NewClientCredentialsProvider(credentials, workspace, apiUrl)
	}
	return NewPublicProvider()
}

func NewClientWithCredentials(config RunClientWithCredentials) (*ClientWithResponses, error) {
	provider := GetAuthProvider(config.Credentials, config.Workspace, config.ApiURL)
	return NewClientWithResponses(config.ApiURL, config.RunURL, WithRequestEditorFn(provider.Intercept), WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		for k, v := range config.Headers {
			req.Header.Set(k, v)
		}
		return nil
	}))
}
