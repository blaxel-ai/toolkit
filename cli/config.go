package cli

import (
	"reflect"

	"github.com/beamlit/toolkit/sdk"
)

type Resource struct {
	Kind             string
	Short            string
	Plural           string
	Singular         string
	SpecType         reflect.Type
	ListParamsType   reflect.Type
	GetParamsType    reflect.Type
	DeleteParamsType reflect.Type
	List             interface{}
	Get              interface{}
	Delete           interface{}
	Put              interface{}
	Post             interface{}
	WithStatus       bool
}

var resources = []*Resource{
	{
		Kind:     "Environment",
		Short:    "env",
		Plural:   "environments",
		Singular: "environment",
		SpecType: reflect.TypeOf(sdk.Environment{}),
	},
	{
		Kind:     "Policy",
		Short:    "pol",
		Plural:   "policies",
		Singular: "policy",
		SpecType: reflect.TypeOf(sdk.Policy{}),
	},
	{
		Kind:             "Model",
		Short:            "ml",
		Plural:           "models",
		Singular:         "model",
		SpecType:         reflect.TypeOf(sdk.Model{}),
		ListParamsType:   reflect.TypeOf(sdk.ListModelsParams{}),
		GetParamsType:    reflect.TypeOf(sdk.GetModelParams{}),
		DeleteParamsType: reflect.TypeOf(sdk.DeleteModelParams{}),
		WithStatus:       true,
	},
	{
		Kind:     "ModelProvider",
		Short:    "mlp",
		Plural:   "modelproviders",
		Singular: "modelprovider",
		SpecType: reflect.TypeOf(sdk.ModelProvider{}),
	},
	{
		Kind:             "Function",
		Short:            "fn",
		Plural:           "functions",
		Singular:         "function",
		SpecType:         reflect.TypeOf(sdk.Function{}),
		ListParamsType:   reflect.TypeOf(sdk.ListFunctionsParams{}),
		GetParamsType:    reflect.TypeOf(sdk.GetFunctionParams{}),
		DeleteParamsType: reflect.TypeOf(sdk.DeleteFunctionParams{}),
		WithStatus:       true,
	},
	{
		Kind:             "Agent",
		Short:            "ag",
		Plural:           "agents",
		Singular:         "agent",
		SpecType:         reflect.TypeOf(sdk.Agent{}),
		ListParamsType:   reflect.TypeOf(sdk.ListAgentsParams{}),
		GetParamsType:    reflect.TypeOf(sdk.GetAgentParams{}),
		DeleteParamsType: reflect.TypeOf(sdk.DeleteAgentParams{}),
		WithStatus:       true,
	},
	{
		Kind:     "IntegrationConnection",
		Short:    "ic",
		Plural:   "integrationconnections",
		Singular: "integrationconnection",
		SpecType: reflect.TypeOf(sdk.IntegrationConnection{}),
	},
}
