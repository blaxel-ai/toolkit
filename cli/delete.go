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
		Example: `
bl delete -f ./my-resource.yaml
# Or using stdin
cat file.yaml | blaxel delete -f -
		`,
		Run: func(cmd *cobra.Command, args []string) {
			results, err := core.GetResults("delete", filePath, recursive)
			if err != nil {
				fmt.Printf("error getting results: %v", err)
				os.Exit(1)
			}

			// À ce stade, results contient tous vos documents YAML
			for _, result := range results {
				for _, resource := range core.GetResources() {
					if resource.Kind == result.Kind {
						name := result.Metadata.(map[string]interface{})["name"].(string)
						DeleteFn(resource, name)
					}
				}
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
			Use:     fmt.Sprintf("%s name [flags]", resource.Singular),
			Aliases: []string{resource.Plural, resource.Short},
			Short:   fmt.Sprintf("Delete %s", resource.Singular),
			Run: func(cmd *cobra.Command, args []string) {
				if len(args) == 0 {
					fmt.Println("no resource name provided")
					os.Exit(1)
				}
				if len(args) == 1 {
					DeleteFn(resource, args[0])
				}
			},
		}
		cmd.AddCommand(subcmd)
	}

	return cmd
}

func DeleteFn(resource *core.Resource, name string) {
	ctx := context.Background()
	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.Delete)
	if funcValue.Kind() != reflect.Func {
		fmt.Println("fn is not a valid function")
		os.Exit(1)
	}
	// Create a slice for the arguments
	fnargs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(name)} // Add the context and the resource name

	// Call the function with the arguments
	results := funcValue.Call(fnargs)

	// Handle the results based on your needs
	if len(results) <= 1 {
		return
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		fmt.Println(err)
		return
	}

	// Check if the first result is a pointer to http.Response
	response, ok := results[0].Interface().(*http.Response)
	if !ok {
		fmt.Println("the result is not a pointer to http.Response")
		return
	}
	// Read the content of http.Response.Body
	defer func() { _ = response.Body.Close() }() // Ensure to close the ReadCloser
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Println(err)
		return
	}

	if response.StatusCode >= 400 {
		fmt.Println(core.ErrorHandler(response.Request, resource.Kind, name, buf.String()))
		os.Exit(1)
		return
	}

	// Check if the content is an array or an object
	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Resource %s:%s deleted\n", resource.Kind, name)
}
