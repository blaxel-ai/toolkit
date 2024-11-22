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

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func (r *Operations) DeleteCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a resource",
		Example: `
			beamlit delete -f ./my-resource.yaml
			# Or using stdin
			cat file.yaml | beamlit delete -f -
		`,
		Run: func(cmd *cobra.Command, args []string) {
			var reader io.Reader

			// Choisir la source (stdin ou fichier)
			if filePath == "-" {
				reader = os.Stdin
			} else {
				file, err := os.Open(filePath)
				if err != nil {
					fmt.Printf("Error opening file: %v\n", err)
					return
				}
				defer file.Close()
				reader = file
			}

			// Lire et parser les documents YAML
			decoder := yaml.NewDecoder(reader)
			var results []Result

			for {
				var result Result
				err := decoder.Decode(&result)
				if err == io.EOF {
					break
				}
				if err != nil {
					fmt.Printf("Error decoding YAML: %v\n", err)
					return
				}
				results = append(results, result)
			}

			// À ce stade, results contient tous vos documents YAML
			for _, result := range results {
				for _, resource := range resources {
					if resource.Kind == result.Kind {
						resource.DeleteFn(result.Metadata.Name)
					}
				}
			}
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file to apply")
	err := cmd.MarkFlagRequired("file")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, resource := range resources {
		subcmd := &cobra.Command{
			Use:     resource.Singular,
			Aliases: []string{resource.Plural, resource.Short},
			Short:   fmt.Sprintf("Delete a %s", resource.Kind),
			Run: func(cmd *cobra.Command, args []string) {
				if len(args) == 0 {
					fmt.Println("no resource name provided")
					os.Exit(1)
				}
				if len(args) == 1 {
					resource.DeleteFn(args[0])
				}
			},
		}
		cmd.AddCommand(subcmd)
	}

	return cmd
}

func (resource Resource) DeleteFn(name string) {
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
		os.Exit(1)
	}

	// Check if the first result is a pointer to http.Response
	response, ok := results[0].Interface().(*http.Response)
	if !ok {
		fmt.Println("the result is not a pointer to http.Response")
		os.Exit(1)
	}
	// Read the content of http.Response.Body
	defer response.Body.Close() // Ensure to close the ReadCloser
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if response.StatusCode >= 400 {
		ErrorHandler(buf.String())
		os.Exit(1)
	}

	// Check if the content is an array or an object
	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Resource %s:%s deleted\n", resource.Kind, name)
}
