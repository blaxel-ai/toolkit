package core

import (
	"context"
	"fmt"

	"github.com/blaxel-ai/sdk-go/option"
)

// ApplicationParam mirrors the JSON body for Application resources.
// The SDK does not yet have typed Application methods, so we use the
// raw client underneath while exposing the same function signatures
// that handleResourceOperation expects via reflection.
type ApplicationParam struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Spec     map[string]interface{} `json:"spec,omitempty"`
}

// ApplicationNewParams wraps ApplicationParam for the New (POST) operation.
type ApplicationNewParams struct {
	Application ApplicationParam
}

// ApplicationUpdateParams wraps ApplicationParam for the Update (PUT) operation.
type ApplicationUpdateParams struct {
	Application ApplicationParam
}

// Application is the response type returned by the raw client.
type Application map[string]interface{}

// applicationService provides typed method wrappers over the raw client
// so that the generic resource operation machinery can call them via
// reflection just like every other SDK service.
type applicationService struct{}

func (s *applicationService) New(ctx context.Context, body ApplicationNewParams, opts ...option.RequestOption) (*Application, error) {
	c := GetClient()
	if c == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	var result Application
	err := c.Post(ctx, "applications", body.Application, &result, opts...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *applicationService) Update(ctx context.Context, name string, body ApplicationUpdateParams, opts ...option.RequestOption) (*Application, error) {
	c := GetClient()
	if c == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	var result Application
	err := c.Put(ctx, fmt.Sprintf("applications/%s", name), body.Application, &result, opts...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *applicationService) Get(ctx context.Context, name string, opts ...option.RequestOption) (*Application, error) {
	c := GetClient()
	if c == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	var result Application
	err := c.Get(ctx, fmt.Sprintf("applications/%s", name), nil, &result, opts...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *applicationService) Delete(ctx context.Context, name string, opts ...option.RequestOption) (*Application, error) {
	c := GetClient()
	if c == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	var result Application
	err := c.Delete(ctx, fmt.Sprintf("applications/%s", name), nil, &result, opts...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
