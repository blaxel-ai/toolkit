package cli

import (
	"context"
	"fmt"
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
  bl delete agent my-agent # Delete after confirmation

  # --- Bulk deletion with jq filtering ---
  # WARNING: Bulk deletions are irreversible. Always preview first!

  # STEP 1: Preview what would be deleted (ALWAYS DO THIS FIRST)
  bl get jobs -o json | jq -r '.[] | select(.status == "DELETING") | .metadata.name'

  # STEP 2: After verifying the list, proceed with deletion
  bl delete jobs $(bl get jobs -o json | jq -r '.[] | select(.status == "DELETING") | .metadata.name')

  # More bulk deletion examples (always preview first):
  bl delete sandboxes $(bl get sandboxes -o json | jq -r '.[] | select(.status == "FAILED") | .metadata.name')
  bl delete agents $(bl get agents -o json | jq -r '.[] | select(.metadata.name | contains("test")) | .metadata.name')
  bl delete volumes $(bl get volumes -o json | jq -r '.[] | select(.metadata.labels.environment == "dev") | .metadata.name')
  bl delete sandboxes $(bl get sandboxes -o json | jq -r '.[] | select(.metadata.name | test("^temp-")) | .metadata.name')`,
		Run: func(cmd *cobra.Command, args []string) {
			results, err := core.GetResults("delete", filePath, recursive)
			if err != nil {
				err = fmt.Errorf("failed to get results: %w", err)
				core.PrintError("Delete", err)
				core.ExitWithError(err)
			}

			// At this point, results contains all your YAML documents
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
				core.ExitWithError(fmt.Errorf("one or more deletions failed"))
			}
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")
	cmd.Flags().StringVarP(&filePath, "filename", "f", "", "containing the resource to delete.")
	err := cmd.MarkFlagRequired("filename")
	if err != nil {
		fmt.Println(err)
		core.ExitWithError(err)
	}

	for _, resource := range core.GetResources() {
		aliases := []string{resource.Plural, resource.Short}
		if len(resource.Aliases) > 0 {
			aliases = append(aliases, resource.Aliases...)
		}

		// Special handling for images - use custom command
		if resource.Kind == "Image" {
			imageCmd := DeleteImagesCmd()
			cmd.AddCommand(imageCmd)
			continue
		}

		// Capture resource kind in closure for ValidArgsFunction
		resourceKind := resource.Kind

		subcmd := &cobra.Command{
			Use:               fmt.Sprintf("%s name [name...] [flags]", resource.Singular),
			Aliases:           aliases,
			Short:             fmt.Sprintf("Delete %s", resource.Singular),
			ValidArgsFunction: GetResourceValidArgsFunction(resourceKind),
			Run: func(cmd *cobra.Command, args []string) {
				if len(args) == 0 {
					err := fmt.Errorf("no resource name provided")
					fmt.Println(err)
					core.ExitWithError(err)
				}
				hasFailures := false
				for _, name := range args {
					if err := DeleteFn(resource, name); err != nil {
						hasFailures = true
					}
				}
				if hasFailures {
					core.ExitWithError(fmt.Errorf("one or more deletions failed"))
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

	// Build arguments: (ctx, name, ...opts)
	fnargs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(name)}

	// Check if the function expects more arguments (e.g., params struct)
	// Some SDK methods may require (ctx, name, params, ...opts)
	// We need to add zero values for any non-variadic params between name and the variadic opts
	funcType := funcValue.Type()
	if funcType.NumIn() > 2 {
		// For variadic functions, the last param is the variadic (e.g., ...option.RequestOption)
		// We need to add parameters between index 2 and the variadic parameter
		lastNonVariadicIdx := funcType.NumIn()
		if funcType.IsVariadic() {
			lastNonVariadicIdx = funcType.NumIn() - 1
		}
		for i := 2; i < lastNonVariadicIdx; i++ {
			paramsType := funcType.In(i)
			fnargs = append(fnargs, reflect.Zero(paramsType))
		}
	}

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

	// The new SDK returns typed responses, not *http.Response
	// Success if we get here without error
	fmt.Printf("Resource %s:%s deleted\n", resource.Kind, name)
	return nil
}
