package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
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
				os.Exit(1)
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
				os.Exit(1)
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
		os.Exit(1)
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

// Helper function to handle common resource operations
func handleResourceOperation(resource *core.Resource, name string, resourceObject interface{}, operation string) (*http.Response, error) {
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

	resourceJson, _ := json.Marshal(resourceObject)
	labelsRetriever := LabelsRetriever{}
	err := json.Unmarshal(resourceJson, &labelsRetriever)
	if err == nil {
		autogeneratedInLabels = labelsRetriever.Metadata.Labels["x-blaxel-auto-generated"] == "true"
	}

	// Handle spec conversion
	specJson, err := json.Marshal(resourceObject)
	if err != nil {
		return nil, fmt.Errorf("error marshaling spec: %v", err)
	}

	destBody := reflect.New(resource.SpecType).Interface()
	if err := json.Unmarshal(specJson, destBody); err != nil {
		return nil, fmt.Errorf("error unmarshaling to target type: %v", err)
	}

	// Call the function
	var results []reflect.Value
	var opts sdk.RequestEditorFn

	if autogeneratedInLabels {
		opts = sdk.RequestEditorFn(func(ctx context.Context, req *http.Request) error {
			q := req.URL.Query()
			q.Add("upload", "true")
			req.URL.RawQuery = q.Encode()
			return nil
		})
	}

	// Get function signature information
	funcType := fn.Type()
	numIn := funcType.NumIn()
	isVariadic := funcType.IsVariadic()

	// Calculate the number of required (non-variadic) parameters
	requiredParams := numIn
	if isVariadic {
		requiredParams = numIn - 1 // Exclude the variadic parameter
	}

	switch operation {
	case "put":
		// Build arguments: ctx, name, [params if needed], body
		values := []reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(name),
		}

		// Fill in any additional required parameters with zero values before the body
		// Body should be the last required parameter before variadic
		for i := len(values); i < requiredParams-1; i++ {
			values = append(values, reflect.Zero(funcType.In(i)))
		}

		// Add the body parameter
		values = append(values, reflect.ValueOf(destBody).Elem())

		// Add variadic opts if present
		if opts != nil {
			values = append(values, reflect.ValueOf(opts))
		}
		results = fn.Call(values)
	case "post":
		// Build arguments: ctx, [params if needed], body
		values := []reflect.Value{
			reflect.ValueOf(ctx),
		}

		// Fill in any additional required parameters with zero values before the body
		// Body should be the last required parameter before variadic
		for i := len(values); i < requiredParams-1; i++ {
			values = append(values, reflect.Zero(funcType.In(i)))
		}

		// Add the body parameter
		values = append(values, reflect.ValueOf(destBody).Elem())

		// Add variadic opts if present
		if opts != nil {
			values = append(values, reflect.ValueOf(opts))
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

	response, ok := results[0].Interface().(*http.Response)
	if !ok {
		return nil, fmt.Errorf("the result is not a pointer to http.Response")
	}

	return response, nil
}

// extractCallbackSecret extracts the callback secret from an agent API response
func extractCallbackSecret(responseBody []byte) string {
	var resource map[string]interface{}
	if err := json.Unmarshal(responseBody, &resource); err != nil {
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
		response, err := client.GetIntegrationConnectionWithResponse(context.Background(), name)
		if err == nil && response.StatusCode() == 200 {
			editUrl := fmt.Sprintf("%s/%s/workspace/settings/integrations/%s", core.GetAppURL(), core.GetWorkspace(), *response.JSON200.Spec.Integration)
			core.Print(fmt.Sprintf("Resource %s:%s already exists, skipping update\nTo edit, go to %s\n", resourceName, name, editUrl))
			return &ResourceOperationResult{
				Status: "skipped",
			}
		}
	}
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resourceName, name)
	response, err := handleResourceOperation(resource, name, resourceObject, "put")
	if err != nil {
		core.Print(fmt.Sprintf("%s%v", formattedError, err))
		return &failedResponse
	}
	if response == nil {
		return nil
	}

	defer func() { _ = response.Body.Close() }()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		core.Print(fmt.Sprintf("%s%v", formattedError, err))
		return &failedResponse
	}

	// We handle not found, and also not implemented to know we need to create
	if response.StatusCode == 404 || response.StatusCode == 405 {
		// Need to create the resource
		return PostFn(resource, resourceName, name, resourceObject)
	}

	if response.StatusCode >= 400 {
		errorMsg := buf.String()
		core.Print(fmt.Sprintf("Resource %s:%s error: %s\n", resourceName, name, errorMsg))
		// Don't exit - let the caller handle the failure and continue processing
		failedResponse.ErrorMsg = errorMsg
		return &failedResponse
	}

	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		core.Print(fmt.Sprintf("%s%v", formattedError, err))
		return &failedResponse
	}
	result := ResourceOperationResult{
		Status: "configured",
	}
	if uploadUrl := response.Header.Get("X-Blaxel-Upload-Url"); uploadUrl != "" {
		result.UploadURL = uploadUrl
	}

	// Extract callback secret from response for agents
	if resourceName == "Agent" {
		result.CallbackSecret = extractCallbackSecret(buf.Bytes())
	}

	core.Print(fmt.Sprintf("Resource %s:%s configured\n", resourceName, name))
	return &result
}

func PostFn(resource *core.Resource, resourceName string, name string, resourceObject interface{}) *ResourceOperationResult {
	failedResponse := ResourceOperationResult{
		Status: "failed",
	}
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resourceName, name)
	response, err := handleResourceOperation(resource, name, resourceObject, "post")
	if err != nil {
		core.Print(fmt.Sprintf("%s%v\n", formattedError, err))
		return &failedResponse
	}
	if response == nil {
		return &failedResponse
	}

	defer func() { _ = response.Body.Close() }()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		core.Print(fmt.Sprintf("%s%v\n", formattedError, err))
		return &failedResponse
	}

	if response.StatusCode >= 400 {
		errorMsg := buf.String()
		core.Print(fmt.Sprintf("Resource %s:%s error: %s\n", resourceName, name, errorMsg))
		// Don't exit - let the caller handle the failure and continue processing
		failedResponse.ErrorMsg = errorMsg
		return &failedResponse
	}

	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		core.Print(fmt.Sprintf("%s%v\n", formattedError, err))
		return &failedResponse
	}
	result := ResourceOperationResult{
		Status: "created",
	}
	if uploadUrl := response.Header.Get("X-Blaxel-Upload-Url"); uploadUrl != "" {
		result.UploadURL = uploadUrl
	}

	// Extract callback secret from response for agents
	if resourceName == "Agent" {
		result.CallbackSecret = extractCallbackSecret(buf.Bytes())
	}

	core.Print(fmt.Sprintf("Resource %s:%s created\n", resourceName, name))
	return &result
}
