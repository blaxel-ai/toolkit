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

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("delete", func() *cobra.Command {
		return DeleteCmd()
	})
}

func DeleteCmd() *cobra.Command {
	var filePath string
	var recursive bool
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource",
		Long: `Delete Blaxel resources from your workspace.

WARNING: Deletion is permanent and cannot be undone. Resources are immediately
deactivated and removed along with their configurations.

Two deletion modes:
1. By name: Use subcommands like 'bl delete agent my-agent'
2. By file: Use 'bl delete -f resource.yaml' for declarative management

What Happens:
- Resource is immediately stopped and deactivated
- Configuration and metadata are removed
- Associated logs and metrics may be retained (check workspace policy)
- Data volumes are NOT automatically deleted (use 'bl delete volume')

Before Deleting:
- Backup any important configuration or data
- Check dependencies (other resources using this one)
- Consider stopping instead of deleting for temporary disablement

Note: Deleting an agent/job stops it immediately but may not delete associated
storage volumes. Use 'bl get volumes' to see persistent storage and delete
separately if needed.`,
		Example: `  # Delete by name (using subcommands)
  bl delete agent my-agent
  bl delete job my-job
  bl delete sandbox my-sandbox

  # Delete multiple resources by name
  bl delete volume vol1 vol2 vol3
  bl delete agent agent1 agent2

  # Delete from YAML file
  bl delete -f my-resource.yaml

  # Delete multiple resources from directory
  bl delete -f ./resources/ -R

  # Delete from stdin (useful in pipelines)
  cat resource.yaml | bl delete -f -

  # Safe deletion workflow
  bl get agent my-agent    # Review resource first
  bl delete agent my-agent # Delete after confirmation`,
		Run: func(cmd *cobra.Command, args []string) {
			results, err := core.GetResults("delete", filePath, recursive)
			if err != nil {
				core.PrintError("Delete", fmt.Errorf("failed to get results: %w", err))
				os.Exit(1)
			}

			// À ce stade, results contient tous vos documents YAML
			hasFailures := false
			for _, result := range results {
				for _, resource := range core.GetResources() {
					if resource.Kind == result.Kind {
						name := result.Metadata.(map[string]interface{})["name"].(string)
						if err := DeleteFn(resource, name); err != nil {
							hasFailures = true
						}
					}
				}
			}

			if hasFailures {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")
	cmd.Flags().StringVarP(&filePath, "filename", "f", "", "containing the resource to delete.")
	err := cmd.MarkFlagRequired("filename")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, resource := range core.GetResources() {
		subcmd := &cobra.Command{
			Use:     fmt.Sprintf("%s name [name...] [flags]", resource.Singular),
			Aliases: []string{resource.Plural, resource.Short},
			Short:   fmt.Sprintf("Delete %s", resource.Singular),
			Run: func(cmd *cobra.Command, args []string) {
				if len(args) == 0 {
					fmt.Println("no resource name provided")
					os.Exit(1)
				}
				hasFailures := false
				for _, name := range args {
					if err := DeleteFn(resource, name); err != nil {
						hasFailures = true
					}
				}
				if hasFailures {
					os.Exit(1)
				}
			},
		}
		cmd.AddCommand(subcmd)
	}

	return cmd
}

func DeleteFn(resource *core.Resource, name string) error {
	ctx := context.Background()
	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.Delete)
	if funcValue.Kind() != reflect.Func {
		err := fmt.Errorf("fn is not a valid function")
		fmt.Println(err)
		return err
	}
	// Create a slice for the arguments
	fnargs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(name)} // Add the context and the resource name

	// Call the function with the arguments
	results := funcValue.Call(fnargs)

	// Handle the results based on your needs
	if len(results) <= 1 {
		return nil
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		fmt.Println(err)
		return err
	}

	// Check if the first result is a pointer to http.Response
	response, ok := results[0].Interface().(*http.Response)
	if !ok {
		err := fmt.Errorf("the result is not a pointer to http.Response")
		fmt.Println(err)
		return err
	}
	// Read the content of http.Response.Body
	defer func() { _ = response.Body.Close() }() // Ensure to close the ReadCloser
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Println(err)
		return err
	}

	if response.StatusCode >= 400 {
		err := core.ErrorHandler(response.Request, resource.Kind, name, buf.String())
		fmt.Println(err)
		return err
	}

	// Check if the content is an array or an object
	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Printf("Resource %s:%s deleted\n", resource.Kind, name)
	return nil
}
