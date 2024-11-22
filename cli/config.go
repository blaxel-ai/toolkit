package cli

import (
	"reflect"

	"github.com/tmp-moon/toolkit/sdk"
)

type Resource struct {
	Kind     string
	Short    string
	Plural   string
	Singular string
	SpecType reflect.Type
	List     interface{}
	Get      interface{}
	Delete   interface{}
	Put      interface{}
	Post     interface{}
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
		Kind:     "Model",
		Short:    "ml",
		Plural:   "models",
		Singular: "model",
		SpecType: reflect.TypeOf(sdk.ModelWithDeployments{}),
	},
	{
		Kind:     "ModelProvider",
		Short:    "mlp",
		Plural:   "modelproviders",
		Singular: "modelprovider",
		SpecType: reflect.TypeOf(sdk.ModelProvider{}),
	},
	{
		Kind:     "Location",
		Short:    "loc",
		Plural:   "locations",
		Singular: "location",
		SpecType: reflect.TypeOf(sdk.Location{}),
	},
	{
		Kind:     "Function",
		Short:    "tl",
		Plural:   "functions",
		Singular: "function",
		SpecType: reflect.TypeOf(sdk.Function{}),
	},
}
