package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("get", func() *cobra.Command {
		return GetCmd()
	})
}

func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource",
	}
	var watch bool
	resources := core.GetResources()
	for _, resource := range resources {
		subcmd := &cobra.Command{
			Use:     resource.Plural,
			Aliases: []string{resource.Singular, resource.Short},
			Short:   fmt.Sprintf("Get a %s", resource.Kind),
			Run: func(cmd *cobra.Command, args []string) {
				if watch {
					seconds := 2
					duration := time.Duration(seconds) * time.Second

					// Execute immediately before starting the ticker
					executeAndDisplayWatch(args, *resource, seconds)

					// Create a ticker to periodically fetch updates
					ticker := time.NewTicker(duration)
					defer ticker.Stop()

					// Handle Ctrl+C gracefully
					sigChan := make(chan os.Signal, 1)
					signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

					for {
						select {
						case <-ticker.C:
							executeAndDisplayWatch(args, *resource, seconds)
						case <-sigChan:
							fmt.Println("\nStopped watching.")
							return
						}
					}
				} else {
					if len(args) == 0 {
						ListFn(resource)
						return
					}
					if len(args) == 1 {
						GetFn(resource, args[0])
					}
				}
			},
		}
		cmd.AddCommand(subcmd)
	}
	cmd.PersistentFlags().BoolVarP(&watch, "watch", "", false, "After listing/getting the requested object, watch for changes.")
	return cmd
}

func GetFn(resource *core.Resource, name string) {
	ctx := context.Background()
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resource.Kind, name)
	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.Get)
	if funcValue.Kind() != reflect.Func {
		fmt.Printf("%s%s", formattedError, "fn is not a valid function")
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
		fmt.Printf("%s%v", formattedError, err)
		os.Exit(1)
	}

	// Check if the first result is a pointer to http.Response
	response, ok := results[0].Interface().(*http.Response)
	if !ok {
		fmt.Printf("%s%s", formattedError, "the result is not a pointer to http.Response")
		os.Exit(1)
	}
	// Read the content of http.Response.Body
	defer func() { _ = response.Body.Close() }() // Ensure to close the ReadCloser
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Printf("%s%v", formattedError, err)
		os.Exit(1)
	}

	if response.StatusCode >= 400 {
		fmt.Printf("Resource %s:%s error: %s\n", resource.Kind, name, buf.String())
		os.Exit(1)
	}

	// Check if the content is an array or an object
	var res interface{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Printf("%s%v", formattedError, err)
		os.Exit(1)
	}
	core.Output(*resource, []interface{}{res}, core.GetOutputFormat())
}

func ListFn(resource *core.Resource) {
	slices, err := ListExec(resource)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Check the output format
	core.Output(*resource, slices, core.GetOutputFormat())
}

func ListExec(resource *core.Resource) ([]interface{}, error) {
	formattedError := fmt.Sprintf("Resource %s error: ", resource.Kind)

	ctx := context.Background()
	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.List)
	if funcValue.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn is not a valid function")
	}
	// Create a slice for the arguments
	fnargs := []reflect.Value{reflect.ValueOf(ctx)} // Add the context

	// Call the function with the arguments
	results := funcValue.Call(fnargs)
	// Handle the results based on your needs
	if len(results) <= 1 {
		return nil, nil
	}
	if err, ok := results[1].Interface().(error); ok && err != nil {
		return nil, fmt.Errorf("%s%v", formattedError, err)
	}
	// Check if the first result is a pointer to http.Response
	response, ok := results[0].Interface().(*http.Response)
	if !ok {
		return nil, fmt.Errorf("the result is not a pointer to http.Response")
	}
	// Read the content of http.Response.Body
	defer func() { _ = response.Body.Close() }() // Ensure to close the ReadCloser
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("resource %s error: %s", resource.Kind, buf.String())
	}

	// Check if the content is an array or an object
	var slices []interface{}
	if err := json.Unmarshal(buf.Bytes(), &slices); err != nil {
		return nil, err
	}
	return slices, nil
}

// Helper function to execute and display results
func executeAndDisplayWatch(args []string, resource core.Resource, seconds int) {
	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	// Save the original stdout
	stdout := os.Stdout
	// Set stdout to our pipe
	os.Stdout = w

	// Execute the resource function
	if len(args) == 0 {
		ListFn(&resource)
	} else if len(args) == 1 {
		GetFn(&resource, args[0])
	}

	// Close the write end of the pipe
	_ = w.Close()

	// Read the output from the pipe
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// Restore stdout
	os.Stdout = stdout

	// Clear the screen
	fmt.Print("\033[2J\033[H")

	// Print the timestamp and output
	fmt.Printf("Every %ds: %s\n", seconds, time.Now().Format("Mon Jan 2 15:04:05 2006"))
	fmt.Print(buf.String())
}
