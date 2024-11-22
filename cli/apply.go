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

func (r *Operations) ApplyCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a configuration to a resource by file",
		Long:  "Apply a configuration to a resource by file",
		Example: `
			beamlit apply -f ./my-deployment.yaml
			# Or using stdin
			cat file.yaml | beamlit apply -f -
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
						resource.PutFn(resource.Kind, result.Metadata.Name, result.Spec)
					}
				}
			}
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file to apply")
	cmd.MarkFlagRequired("file")

	return cmd
}

// Helper function to handle common resource operations
func (resource Resource) handleResourceOperation(resourceName, name string, spec interface{}, operation string) (*http.Response, error) {
	ctx := context.Background()

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

	// Handle spec conversion
	specJson, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("error marshaling spec: %v", err)
	}

	destBody := reflect.New(resource.SpecType).Interface()
	if err := json.Unmarshal(specJson, destBody); err != nil {
		return nil, fmt.Errorf("error unmarshaling to target type: %v", err)
	}

	// Call the function
	results := fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(name),
		reflect.ValueOf(destBody).Elem(),
	})

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

func (resource Resource) PutFn(resourceName string, name string, spec interface{}) {
	response, err := resource.handleResourceOperation(resourceName, name, spec, "put")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if response == nil {
		return
	}

	defer response.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if response.StatusCode >= 404 {
		// Need to create the resource
		resource.PostFn(resourceName, name, spec)
		return
	}

	if response.StatusCode >= 400 {
		ErrorHandler(buf.String())
		os.Exit(1)
	}

	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Resource %s:%s configured\n", resourceName, name)
}

func (resource Resource) PostFn(resourceName string, name string, spec interface{}) {
	response, err := resource.handleResourceOperation(resourceName, name, spec, "post")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if response == nil {
		return
	}

	defer response.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if response.StatusCode >= 400 {
		ErrorHandler(buf.String())
		os.Exit(1)
	}

	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Resource %s:%s created\n", resourceName, name)
}
