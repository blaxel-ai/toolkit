package core

import (
	"context"
)

type Operations struct {
	BaseURL     string
	RunURL      string
	AppURL      string
	RegistryURL string
}

func (o *Operations) CliCommand(ctx context.Context, operationId string, fn interface{}) {
	operation := formatOperationId(operationId)
	if operation[0] == "list" {
		for _, resource := range resources {
			if resource.Plural == operation[1] {
				resource.List = fn
				break
			}
		}
		return
	}
	if operation[0] == "get" {
		for _, resource := range resources {
			if resource.Singular == operation[1] {
				resource.Get = fn
				break
			}
		}
		return
	}
	if operation[0] == "delete" {
		for _, resource := range resources {
			if resource.Singular == operation[1] {
				resource.Delete = fn
				break
			}
		}
		return
	}
	if operation[0] == "put" || operation[0] == "update" {
		for _, resource := range resources {
			if resource.Singular == operation[1] {
				resource.Put = fn
				break
			}
		}
		return
	}
	if operation[0] == "create" {
		for _, resource := range resources {
			if resource.Singular == operation[1] {
				resource.Post = fn
				break
			}
		}
		return
	}
}
