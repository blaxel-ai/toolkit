package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("apply", func() *cobra.Command {
		return ApplyCmd()
	})
}

type ResourceOperationResult struct {
	Status         string
	UploadURL      string
	ErrorMsg       string
	CallbackSecret string
}

type ApplyResult struct {
	Kind   string
	Name   string
	Result ResourceOperationResult
}

// ApplyOption defines a function type for apply options
type ApplyOption func(*applyOptions)

// applyOptions holds all possible options for Apply
type applyOptions struct {
	recursive bool
}

// WithRecursive sets the recursive option
func WithRecursive(recursive bool) ApplyOption {
	return func(o *applyOptions) {
		o.recursive = recursive
	}
}

func ApplyCmd() *cobra.Command {
	var filePath string
	var recursive bool
	var envFiles []string
	var commandSecrets []string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a configuration to a resource by file",
		Long: `Apply configuration changes to resources declaratively using YAML files.

This command is similar to Kubernetes 'kubectl apply' - it creates resources
if they don't exist, or updates them if they do (idempotent operation).

Use 'apply' for Infrastructure as Code workflows where you:
- Manage resources via configuration files
- Version control your infrastructure
- Deploy multiple related resources together
- Implement GitOps practices

Difference from 'deploy':
- 'apply' manages resource configuration (metadata, settings, specs)
- 'deploy' builds and uploads code as container images

For deploying code changes to agents/jobs, use 'bl deploy'.
For managing resource configuration, use 'bl apply'.

The command respects environment variables and secrets, which can be injected
via -e flag for .env files or -s flag for command-line secrets.`,
		Example: `  # Apply a single resource
  bl apply -f agent.yaml

  # Apply all resources in directory
  bl apply -f ./resources/ -R

  # Apply with environment variable substitution
  bl apply -f deployment.yaml -e .env.production

  # Apply from stdin (useful for CI/CD)
  cat config.yaml | bl apply -f -

  # Apply with secrets
  bl apply -f config.yaml -s API_KEY=xxx -s DB_PASSWORD=yyy

  # Example YAML structure:
  # apiVersion: blaxel.ai/v1alpha1
  # kind: Agent
  # metadata:
  #   name: my-agent
  # spec:
  #   runtime:
	#     generation: mk3
	#     image: agent/my-template-agent:latest
  #     memory: 4096`,
		Run: func(cmd *cobra.Command, args []string) {
			core.LoadCommandSecrets(commandSecrets)
			core.ReadSecrets("", envFiles)
			applyResults, err := Apply(filePath, WithRecursive(recursive))
			if err != nil {
				core.PrintError("Apply", err)
				core.ExitWithError(err)
			}

			// Check if any resources failed
			hasFailures := false
			for _, result := range applyResults {
				if result.Result.Status == "failed" {
					hasFailures = true
					break
				}
			}

			if hasFailures {
				core.ExitWithError(fmt.Errorf("one or more resources failed to apply"))
			}
		},
	}

	cmd.Flags().StringVarP(&filePath, "filename", "f", "", "Path to YAML file to apply")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")
	cmd.Flags().StringSliceVarP(&envFiles, "env-file", "e", []string{".env"}, "Environment file to load")
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to deploy")
	err := cmd.MarkFlagRequired("filename")
	if err != nil {
		core.PrintError("Apply", err)
		core.ExitWithError(err)
	}

	return cmd
}

func ApplyResources(results []core.Result) ([]ApplyResult, error) {
	applyResults := []ApplyResult{}
	resources := core.GetResources()

	// À ce stade, results contient tous vos documents YAML
	for _, result := range results {
		for _, resource := range resources {
			if resource.Kind == result.Kind {
				name := result.Metadata.(map[string]interface{})["name"].(string)
				resultOp := PutFn(resource, result.Kind, name, result)
				if resultOp != nil {
					applyResults = append(applyResults, ApplyResult{
						Kind:   resource.Kind,
						Name:   name,
						Result: *resultOp,
					})
				}
			}
		}
	}
	return applyResults, nil
}

func Apply(filePath string, opts ...ApplyOption) ([]ApplyResult, error) {
	// Default options
	options := &applyOptions{
		recursive: false,
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(options)
	}

	results, err := core.GetResults("apply", filePath, options.recursive)
	if err != nil {
		return nil, fmt.Errorf("error getting results: %w", err)
	}

	applyResults, err := ApplyResources(results)
	if err != nil {
		return nil, fmt.Errorf("error applying resources: %w", err)
	}

	return applyResults, nil
}

// handleResourceOperationResult contains both the response and upload URL
type handleResourceOperationResult struct {
	Response  interface{}
	UploadURL string
}

// handleResourceOperation handles put or post operations for a resource
func handleResourceOperation(resource *core.Resource, name string, resourceObject interface{}, operation string) (*handleResourceOperationResult, error) {
	ctx := context.Background()

	if resource.Put == nil && operation == "put" {
		operation = "post"
	}

	// Get the appropriate function based on operation
	var fn reflect.Value
	if operation == "put" {
		fn = reflect.ValueOf(resource.Put)
	} else {
		fn = reflect.ValueOf(resource.Post)
	}

	if fn.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn is not a valid function")
	}

	type LabelsRetriever struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}

	autogeneratedInLabels := false

	// Marshal the original YAML object - this only contains fields that were in the YAML
	resourceJson, _ := json.Marshal(resourceObject)
	labelsRetriever := LabelsRetriever{}
	err := json.Unmarshal(resourceJson, &labelsRetriever)
	if err == nil {
		autogeneratedInLabels = labelsRetriever.Metadata.Labels["x-blaxel-auto-generated"] == "true"
	}

	// Build request options - capture HTTP response to get headers
	var httpResponse *http.Response
	var opts []option.RequestOption
	opts = append(opts, option.WithResponseInto(&httpResponse))
	if autogeneratedInLabels {
		opts = append(opts, option.WithQuery("upload", "true"))
	}

	// Get function signature information
	funcType := fn.Type()

	// Call the function with appropriate arguments based on operation
	var results []reflect.Value

	switch operation {
	case "put":
		// Update methods have signature: (ctx, name, body, ...opts)
		// Build the params type for Update operations
		values := []reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(name),
		}

		// Add the body parameter - need to wrap in the Params type
		bodyParamType := funcType.In(2)
		bodyParam := reflect.New(bodyParamType).Elem()

		// Set the body fields directly from the original YAML JSON
		setBodyFieldsFromJSON(bodyParam, resourceJson)

		values = append(values, bodyParam)

		// Add variadic opts
		for _, opt := range opts {
			values = append(values, reflect.ValueOf(opt))
		}
		results = fn.Call(values)

	case "post":
		// New methods have signature: (ctx, body, ...opts)
		values := []reflect.Value{
			reflect.ValueOf(ctx),
		}

		// Add the body parameter - need to wrap in the Params type
		bodyParamType := funcType.In(1)
		bodyParam := reflect.New(bodyParamType).Elem()

		// Set the body fields directly from the original YAML JSON
		setBodyFieldsFromJSON(bodyParam, resourceJson)

		values = append(values, bodyParam)

		// Add variadic opts
		for _, opt := range opts {
			values = append(values, reflect.ValueOf(opt))
		}
		results = fn.Call(values)

	default:
		return nil, fmt.Errorf("invalid operation: %s", operation)
	}

	if len(results) <= 1 {
		return nil, nil
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		return nil, err
	}

	result := &handleResourceOperationResult{
		Response: results[0].Interface(),
	}

	// Extract upload URL from response header
	if httpResponse != nil {
		if uploadURL := httpResponse.Header.Get("X-Blaxel-Upload-Url"); uploadURL != "" {
			result.UploadURL = uploadURL
		}
	}

	return result, nil
}

// setBodyFieldsFromJSON sets fields in dst directly from the original YAML JSON
// This ensures we only send fields that were actually present in the YAML,
// not Go's default values for missing fields
func setBodyFieldsFromJSON(dst reflect.Value, srcJSON []byte) {
	// For Param types, we need to set the inner type field
	// e.g., AgentNewParams has an Agent field of type AgentParam
	for i := 0; i < dst.NumField(); i++ {
		field := dst.Type().Field(i)
		if field.Type.Kind() == reflect.Struct {
			// Try to set nested struct fields
			dstField := dst.Field(i)
			if dstField.CanSet() {
				// Unmarshal directly from the original YAML JSON into the Param type
				// This preserves only the fields that were in the YAML
				newVal := reflect.New(field.Type).Interface()
				if err := json.Unmarshal(srcJSON, newVal); err != nil {
					// Log unmarshal errors in verbose mode to help debug YAML field issues
					if core.GetVerbose() {
						core.PrintWarning(fmt.Sprintf("Failed to unmarshal field %s: %v", field.Name, err))
					}
					continue
				}
				dstField.Set(reflect.ValueOf(newVal).Elem())
			}
		}
	}
}

// extractCallbackSecret extracts the callback secret from an agent API response
func extractCallbackSecret(response interface{}) string {
	// Marshal to JSON and back to get a map
	jsonData, err := json.Marshal(response)
	if err != nil {
		return ""
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(jsonData, &resource); err != nil {
		return ""
	}

	// Navigate through the JSON structure to find callback secret
	// spec.triggers[].configuration.callbackSecret
	if spec, ok := resource["spec"].(map[string]interface{}); ok {
		if triggers, ok := spec["triggers"].([]interface{}); ok {
			for _, trigger := range triggers {
				if triggerMap, ok := trigger.(map[string]interface{}); ok {
					if config, ok := triggerMap["configuration"].(map[string]interface{}); ok {
						if callbackSecret, ok := config["callbackSecret"].(string); ok && callbackSecret != "" {
							// Only return if it's not masked (doesn't contain asterisks)
							if !strings.Contains(callbackSecret, "*") {
								return callbackSecret
							}
						}
					}
				}
			}
		}
	}

	return ""
}

func PutFn(resource *core.Resource, resourceName string, name string, resourceObject interface{}) *ResourceOperationResult {
	failedResponse := ResourceOperationResult{
		Status: "failed",
	}
	if resource.Kind == "IntegrationConnection" {
		client := core.GetClient()
		_, err := client.Integrations.Connections.Get(context.Background(), name)
		if err == nil {
			// Get the integration name from the resource object for the edit URL
			var resourceMap map[string]interface{}
			integrationName := ""
			resourceJson, jsonErr := json.Marshal(resourceObject)
			if jsonErr == nil {
				if unmarshalErr := json.Unmarshal(resourceJson, &resourceMap); unmarshalErr == nil {
					if spec, ok := resourceMap["spec"].(map[string]interface{}); ok {
						if integration, ok := spec["integration"].(string); ok {
							integrationName = integration
						}
					}
				}
			}
			editUrl := fmt.Sprintf("%s/%s/workspace/settings/integrations/%s", blaxel.GetAppURL(), core.GetWorkspace(), integrationName)
			core.Print(fmt.Sprintf("Resource %s:%s already exists, skipping update\nTo edit, go to %s\n", resourceName, name, editUrl))
			return &ResourceOperationResult{
				Status: "skipped",
			}
		}
	}
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resourceName, name)
	opResult, err := handleResourceOperation(resource, name, resourceObject, "put")
	if err != nil {
		// Check if it's a 404 or 405 error - need to create
		var apiErr *blaxel.Error
		if ok := isBlaxelError(err, &apiErr); ok {
			if apiErr.StatusCode == 404 || apiErr.StatusCode == 405 {
				return PostFn(resource, resourceName, name, resourceObject)
			}
		}
		core.Print(fmt.Sprintf("%s%v", formattedError, err))
		return &failedResponse
	}
	if opResult == nil {
		return nil
	}

	result := ResourceOperationResult{
		Status:    "configured",
		UploadURL: opResult.UploadURL,
	}

	// Extract callback secret from response for agents
	if resourceName == "Agent" {
		result.CallbackSecret = extractCallbackSecret(opResult.Response)
	}

	core.Print(fmt.Sprintf("Resource %s:%s configured\n", resourceName, name))
	return &result
}

func PostFn(resource *core.Resource, resourceName string, name string, resourceObject interface{}) *ResourceOperationResult {
	failedResponse := ResourceOperationResult{
		Status: "failed",
	}
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resourceName, name)
	opResult, err := handleResourceOperation(resource, name, resourceObject, "post")
	if err != nil {
		core.Print(fmt.Sprintf("%s%v\n", formattedError, err))
		return &failedResponse
	}
	if opResult == nil {
		return &failedResponse
	}

	result := ResourceOperationResult{
		Status:    "created",
		UploadURL: opResult.UploadURL,
	}

	// Extract callback secret from response for agents
	if resourceName == "Agent" {
		result.CallbackSecret = extractCallbackSecret(opResult.Response)
	}

	core.Print(fmt.Sprintf("Resource %s:%s created\n", resourceName, name))
	return &result
}

// isBlaxelError checks if an error is a blaxel API error and sets the apiErr pointer
func isBlaxelError(err error, apiErr **blaxel.Error) bool {
	if e, ok := err.(*blaxel.Error); ok {
		*apiErr = e
		return true
	}
	return false
}
