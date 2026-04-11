package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

func init() {
	core.RegisterCommand("get", func() *cobra.Command {
		return GetCmd()
	})
}

func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "List or retrieve Blaxel resources in your workspace",
		Long: `Retrieve information about Blaxel resources in your workspace.

A "resource" in Blaxel refers to any deployable or manageable entity:
- agents: AI agent applications
- functions/mcp: Model Context Protocol servers (tool providers)
- jobs: Batch processing tasks
- sandboxes: Isolated execution environments
- models: AI model configurations
- policies: Access control policies
- volumes: Persistent storage
- integrationconnections: External service integrations

Hub Discovery (pre-built resources available in the Blaxel Hub):
- sandbox-hub: Pre-built sandbox images with pre-installed tools and runtimes
- mcp-hub: Pre-built MCP servers for tool integrations (GitHub, Slack, etc.)
- templates: Project scaffolding templates for bl new

Output Formats:
Use -o flag to control output format:
- pretty: Human-readable colored output (default)
- json: Machine-readable JSON (for scripting)
- yaml: YAML format
- table: Tabular format with columns

Watch Mode:
Use --watch to continuously monitor a resource and see updates in real-time.
Useful for tracking deployment status or watching for changes.

The command can list all resources of a type or get details for a specific one.`,
		Example: `  # List all agents
  bl get agents

  # Get specific agent details
  bl get agent my-agent

  # Get in JSON format (useful for scripting)
  bl get agent my-agent -o json

  # Watch agent status in real-time
  bl get agent my-agent --watch

  # List all resources with table output
  bl get agents -o table

  # Get MCP servers (also called functions)
  bl get functions
  bl get mcp

  # List jobs
  bl get jobs

  # Get specific job
  bl get job my-job

  # List executions for a job (nested resource)
  bl get job my-job executions

  # Get specific execution for a job
  bl get job my-job execution EXECUTION_ID

  # List pre-built sandbox images from the Hub
  bl get sandbox-hub
  bl get sandbox-hub -o json

  # List pre-built MCP servers from the Hub
  bl get mcp-hub
  bl get mcp-hub -o json

  # Monitor sandbox status
  bl get sandbox my-sandbox --watch

  # List processes in a sandbox
  bl get sandbox my-sandbox process
  bl get sbx my-sandbox ps

  # Get specific process in a sandbox
  bl get sandbox my-sandbox process my-process

  # List previews for a sandbox
  bl get sandbox my-sandbox previews

  # Get a specific preview
  bl get sandbox my-sandbox preview my-preview

  # List tokens for a sandbox preview
  bl get sandbox my-sandbox preview my-preview tokens

  # Get a specific token
  bl get sandbox my-sandbox preview my-preview token my-token

  # --- Filtering with jq ---

  # Get names of all jobs with status DELETING
  bl get jobs -o json | jq -r '.[] | select(.status == "DELETING") | .metadata.name'

  # Get names of all deployed sandboxes
  bl get sandboxes -o json | jq -r '.[] | select(.status == "DEPLOYED") | .metadata.name'

  # Get all agents with name containing "test"
  bl get agents -o json | jq -r '.[] | select(.metadata.name | contains("test")) | .metadata.name'

  # Get sandboxes with specific label (e.g., environment=dev)
  bl get sandboxes -o json | jq -r '.[] | select(.metadata.labels.environment == "dev") | .metadata.name'

  # Get all job names
  bl get jobs -o json | jq -r '.[] | .metadata.name'

  # Count resources by status
  bl get agents -o json | jq 'group_by(.status) | map({status: .[0].status, count: length})'`,
	}
	var watch bool
	resources := core.GetResources()
	for _, resource := range resources {
		aliases := []string{resource.Singular, resource.Short}
		if len(resource.Aliases) > 0 {
			aliases = append(aliases, resource.Aliases...)
		}

		// Special handling for images - use custom command
		if resource.Kind == "Image" {
			imageCmd := GetImagesCmd()
			// Add both singular and plural
			cmd.AddCommand(imageCmd)
			continue
		}

		// Capture resource in closure for ValidArgsFunction
		resourceKind := resource.Kind

		subcmd := &cobra.Command{
			Use:               resource.Plural,
			Aliases:           aliases,
			Short:             fmt.Sprintf("List all %s or get details of a specific one", resource.Plural),
			ValidArgsFunction: GetResourceValidArgsFunction(resourceKind),
			Run: func(cmd *cobra.Command, args []string) {
				// Check if this is a nested resource request
				isNestedResource := false
				var nestedResourceFn func()

				if resource.Kind == "Job" && len(args) >= 2 {
					// Check if this looks like a nested resource request
					nestedResource := args[1]
					if nestedResource == "executions" || nestedResource == "execution" {
						isNestedResource = true
						nestedResourceFn = func() {
							HandleJobNestedResource(args)
						}
					}
				}

				if resource.Kind == "Sandbox" && len(args) >= 2 {
					// Check if this looks like a nested resource request
					nestedResource := args[1]
					if nestedResource == "processes" || nestedResource == "process" || nestedResource == "proc" || nestedResource == "procs" || nestedResource == "ps" {
						isNestedResource = true
						nestedResourceFn = func() {
							HandleSandboxNestedResource(args)
						}
					}
					if nestedResource == "previews" || nestedResource == "preview" || nestedResource == "pv" {
						isNestedResource = true
						nestedResourceFn = func() {
							HandleSandboxPreviewNestedResource(args)
						}
					}
				}

				if watch {
					seconds := 2
					duration := time.Duration(seconds) * time.Second

					// Create a ticker to periodically fetch updates
					ticker := time.NewTicker(duration)
					defer ticker.Stop()

					// Handle Ctrl+C gracefully
					sigChan := make(chan os.Signal, 1)
					signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

					// Listen for 'q' key press - start BEFORE first execution
					// so terminal is in raw mode consistently for all output
					quitChan := make(chan struct{})
					go listenForQuit(quitChan)

					// Execute immediately before starting the ticker
					if isNestedResource && nestedResourceFn != nil {
						executeNestedResourceWatch(nestedResourceFn, seconds)
					} else {
						executeAndDisplayWatch(args, *resource, seconds)
					}

					for {
						select {
						case <-ticker.C:
							if isNestedResource && nestedResourceFn != nil {
								executeNestedResourceWatch(nestedResourceFn, seconds)
							} else {
								executeAndDisplayWatch(args, *resource, seconds)
							}
						case <-sigChan:
							fmt.Println("\nStopped watching.")
							return
						case <-quitChan:
							fmt.Println("\nStopped watching.")
							return
						}
					}
				} else {
					// Non-watch mode
					if isNestedResource && nestedResourceFn != nil {
						nestedResourceFn()
						return
					}

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

	// Add templates subcommand (non-CRUD, fetches from GitHub API)
	cmd.AddCommand(getTemplatesCmd())

	// Add hub subcommands (pre-built definitions from Blaxel Hub)
	cmd.AddCommand(getSandboxHubCmd())
	cmd.AddCommand(getMCPHubCmd())

	cmd.PersistentFlags().BoolVarP(&watch, "watch", "", false, "After listing/getting the requested object, watch for changes.")
	return cmd
}

func getTemplatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "templates [type]",
		Short:   "List available project templates",
		Aliases: []string{"template", "tpl"},
		Args:    cobra.MaximumNArgs(1),
		Long: `List available templates that can be used with 'bl new'.

Templates are grouped by type (agent, mcp, sandbox, job, volume-template).
Use an optional type argument to filter results.

Output formats:
  -o json   Machine-readable JSON array
  -o yaml   YAML output
  default   Table with NAME, TYPE, LANGUAGE, DESCRIPTION columns`,
		Example: `  # List all templates
  bl get templates

  # List agent templates only
  bl get templates agent

  # List templates as JSON
  bl get templates -o json

  # List MCP templates
  bl get templates mcp`,
		Run: func(cmd *cobra.Command, args []string) {
			filterType := ""
			if len(args) == 1 {
				filterType = args[0]
			}
			listAvailableTemplates(filterType, core.GetOutputFormat())
		},
	}
}

func getSandboxHubCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "sandbox-hub",
		Short:   "List pre-built sandbox images available in the Blaxel Hub",
		Aliases: []string{"sbx-hub", "sandbox-images"},
		Args:    cobra.NoArgs,
		Long: `List pre-built sandbox images from the Blaxel Hub.

Each image comes with pre-installed tools, runtimes, and configurations.
Use the 'image' field value in your sandbox YAML spec when deploying
with 'bl apply -f sandbox.yaml':

  apiVersion: blaxel/v1alpha1
  kind: Sandbox
  metadata:
    name: my-sandbox
  spec:
    image: <image-from-hub>

Output formats:
  -o json   Machine-readable JSON array
  -o yaml   YAML output
  default   Table with NAME, IMAGE, MEMORY, DESCRIPTION columns`,
		Example: `  # List all available sandbox hub images
  bl get sandbox-hub

  # List as JSON (for automation/agents)
  bl get sandbox-hub -o json

  # List as YAML
  bl get sandbox-hub -o yaml`,
		Run: func(cmd *cobra.Command, args []string) {
			client := core.GetClient()
			if client == nil {
				core.PrintError("Sandbox Hub", fmt.Errorf("client not initialized, please log in with 'bl login'"))
				core.ExitWithError(fmt.Errorf("client not initialized"))
			}
			resp, err := client.Sandboxes.GetHub(context.Background())
			if err != nil {
				core.PrintError("Sandbox Hub", err)
				core.ExitWithError(err)
			}

			// Filter out hidden and coming-soon images
			var images []blaxel.SandboxGetHubResponse
			if resp != nil {
				for _, img := range *resp {
					if !img.Hidden && !img.ComingSoon {
						images = append(images, img)
					}
				}
			}

			if len(images) == 0 {
				core.PrintInfo("No sandbox hub images found")
				return
			}

			outputFmt := core.GetOutputFormat()
			switch outputFmt {
			case "json":
				data, _ := json.MarshalIndent(images, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				yamlData, _ := yaml.Marshal(images)
				fmt.Print(string(yamlData))
			default:
				printSandboxHubTable(images)
			}
		},
	}
}

func printSandboxHubTable(images []blaxel.SandboxGetHubResponse) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.AppendHeader(table.Row{"NAME", "IMAGE", "MEMORY (MB)", "DESCRIPTION"})
	for _, img := range images {
		name := img.DisplayName
		if name == "" {
			name = img.Name
		}
		desc := img.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		tw.AppendRow(table.Row{name, img.Image, img.Memory, desc})
	}
	fmt.Println(tw.Render())
}

// mcpHubDefinition represents a pre-built MCP server from the Blaxel Hub.
// The Go SDK does not yet expose GET /mcp/hub, so we call it via client.Get.
type mcpHubDefinition struct {
	Name        string `json:"name" yaml:"name"`
	DisplayName string `json:"displayName" yaml:"displayName"`
	Image       string `json:"image" yaml:"image"`
	Description string `json:"description" yaml:"description"`
	Integration string `json:"integration,omitempty" yaml:"integration,omitempty"`
	Icon        string `json:"icon,omitempty" yaml:"icon,omitempty"`
	Hidden      bool   `json:"hidden" yaml:"hidden"`
	ComingSoon  bool   `json:"coming_soon" yaml:"coming_soon"`
	Enterprise  bool   `json:"enterprise" yaml:"enterprise"`
	Transport   string `json:"transport,omitempty" yaml:"transport,omitempty"`
}

func getMCPHubCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "mcp-hub",
		Short:   "List pre-built MCP servers available in the Blaxel Hub",
		Aliases: []string{"function-hub"},
		Args:    cobra.NoArgs,
		Long: `List pre-built MCP servers from the Blaxel Hub.

These provide ready-to-use tool integrations (e.g. GitHub, Slack,
databases). Connect one to your agent by creating an integration
connection with 'bl apply -f connection.yaml':

  apiVersion: blaxel/v1alpha1
  kind: IntegrationConnection
  metadata:
    name: my-github
  spec:
    integration: <integration-from-hub>

Output formats:
  -o json   Machine-readable JSON array
  -o yaml   YAML output
  default   Table with NAME, INTEGRATION, DESCRIPTION columns`,
		Example: `  # List all available MCP hub servers
  bl get mcp-hub

  # List as JSON (for automation/agents)
  bl get mcp-hub -o json

  # List as YAML
  bl get mcp-hub -o yaml`,
		Run: func(cmd *cobra.Command, args []string) {
			client := core.GetClient()
			if client == nil {
				core.PrintError("MCP Hub", fmt.Errorf("client not initialized, please log in with 'bl login'"))
				core.ExitWithError(fmt.Errorf("client not initialized"))
			}

			var resp []mcpHubDefinition
			err := client.Get(context.Background(), "mcp/hub", nil, &resp)
			if err != nil {
				core.PrintError("MCP Hub", err)
				core.ExitWithError(err)
			}

			// Filter out hidden and coming-soon entries
			var definitions []mcpHubDefinition
			for _, d := range resp {
				if !d.Hidden && !d.ComingSoon {
					definitions = append(definitions, d)
				}
			}

			if len(definitions) == 0 {
				core.PrintInfo("No MCP hub servers found")
				return
			}

			outputFmt := core.GetOutputFormat()
			switch outputFmt {
			case "json":
				data, _ := json.MarshalIndent(definitions, "", "  ")
				fmt.Println(string(data))
			case "yaml":
				yamlData, _ := yaml.Marshal(definitions)
				fmt.Print(string(yamlData))
			default:
				printMCPHubTable(definitions)
			}
		},
	}
}

func printMCPHubTable(definitions []mcpHubDefinition) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.AppendHeader(table.Row{"NAME", "INTEGRATION", "DESCRIPTION"})
	for _, d := range definitions {
		name := d.DisplayName
		if name == "" {
			name = d.Name
		}
		desc := d.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		tw.AppendRow(table.Row{name, d.Integration, desc})
	}
	fmt.Println(tw.Render())
}

func GetFn(resource *core.Resource, name string) {
	ctx := context.Background()
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resource.Kind, name)

	if resource.Get == nil {
		hint := nestedResourceHint(resource, "get")
		err := fmt.Errorf("%s'bl get %s <name>' is not supported directly.%s", formattedError, resource.Singular, hint)
		core.PrintError("Get", err)
		core.ExitWithError(err)
	}

	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.Get)
	if funcValue.Kind() != reflect.Func {
		err := fmt.Errorf("%s%s", formattedError, "fn is not a valid function")
		core.PrintError("Get", err)
		core.ExitWithError(err)
	}

	// Build arguments: (ctx, name, ...opts)
	fnargs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(name)}

	// Check if the function expects more arguments (e.g., params struct)
	// Some SDK methods like Jobs.Get require (ctx, name, params, ...opts)
	// Others like Policies.Get only have (ctx, name, ...opts)
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
		return
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		fmt.Printf("%s%v\n", formattedError, err)
		core.ExitWithError(err)
	}

	// The new SDK returns typed responses, not *http.Response
	// Convert result to interface{} for output
	result := results[0].Interface()
	if result == nil {
		err := fmt.Errorf("%s%s", formattedError, "no result returned")
		fmt.Println(err)
		core.ExitWithError(err)
	}

	// Convert to JSON and back to interface{} for consistent handling
	jsonData, err := json.Marshal(result)
	if err != nil {
		fmt.Printf("%s%v\n", formattedError, err)
		core.ExitWithError(err)
	}

	var res interface{}
	if err := json.Unmarshal(jsonData, &res); err != nil {
		fmt.Printf("%s%v\n", formattedError, err)
		core.ExitWithError(err)
	}

	core.Output(*resource, []interface{}{res}, core.GetOutputFormat())
}

func ListFn(resource *core.Resource) {
	slices, err := ListExec(resource)
	if err != nil {
		fmt.Println(err)
		core.ExitWithError(err)
	}
	// Check the output format
	core.Output(*resource, slices, core.GetOutputFormat())
}

func ListExec(resource *core.Resource) ([]interface{}, error) {
	formattedError := fmt.Sprintf("Resource %s error: ", resource.Kind)

	if resource.List == nil {
		hint := nestedResourceHint(resource, "get")
		return nil, fmt.Errorf("%s'bl get %s' is not supported directly.%s", formattedError, resource.Plural, hint)
	}

	ctx := context.Background()
	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.List)
	if funcValue.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn is not a valid function")
	}

	// Build arguments: (ctx, ...opts)
	fnargs := []reflect.Value{reflect.ValueOf(ctx)}

	// Call the function with the arguments
	results := funcValue.Call(fnargs)

	// Handle the results based on your needs
	if len(results) <= 1 {
		return nil, nil
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		return nil, fmt.Errorf("%s%v", formattedError, err)
	}

	// The new SDK returns typed responses (e.g., *[]Agent), not *http.Response
	result := results[0].Interface()
	if result == nil {
		return nil, errors.New("no result returned")
	}

	// Convert to JSON and back to []interface{} for consistent handling
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	var slices []interface{}
	if err := json.Unmarshal(jsonData, &slices); err != nil {
		return nil, err
	}

	return slices, nil
}

// executeNestedResourceWatch executes a nested resource function and displays results with watch formatting
func executeNestedResourceWatch(fn func(), seconds int) {
	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	// Save the original stdout
	stdout := os.Stdout
	// Set stdout to our pipe
	os.Stdout = w

	// Execute the nested resource function
	fn()

	// Close the write end of the pipe
	_ = w.Close()

	// Read the output from the pipe
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// Close the read end of the pipe to avoid file descriptor leak
	_ = r.Close()

	// Restore stdout
	os.Stdout = stdout

	// Clear the screen
	fmt.Print("\033[2J\033[H")

	// Print the timestamp and output
	// Use \r\n for raw mode compatibility (listenForQuit puts terminal in raw mode)
	fmt.Printf("Every %ds: %s\r\n", seconds, time.Now().Format("Mon Jan 2 15:04:05 2006"))
	// Convert \n to \r\n for raw mode compatibility
	output := strings.ReplaceAll(buf.String(), "\r\n", "\n") // Normalize first
	output = strings.ReplaceAll(output, "\n", "\r\n")        // Then convert
	fmt.Print(output)
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

	// Close the read end of the pipe to avoid file descriptor leak
	_ = r.Close()

	// Restore stdout
	os.Stdout = stdout

	// Clear the screen
	fmt.Print("\033[2J\033[H")

	// Print the timestamp and output
	// Use \r\n for raw mode compatibility (listenForQuit puts terminal in raw mode)
	fmt.Printf("Every %ds: %s\r\n", seconds, time.Now().Format("Mon Jan 2 15:04:05 2006"))
	// Convert \n to \r\n for raw mode compatibility
	output := strings.ReplaceAll(buf.String(), "\r\n", "\n") // Normalize first
	output = strings.ReplaceAll(output, "\n", "\r\n")        // Then convert
	fmt.Print(output)
}

// listenForQuit listens for 'q' key press and signals to quit
func listenForQuit(quitChan chan struct{}) {
	// Check if stdin is a terminal
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return
	}

	// Set terminal to raw mode to capture single key presses
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		// Check for 'q' or 'Q'
		if buf[0] == 'q' || buf[0] == 'Q' {
			close(quitChan)
			return
		}
		// Also handle Ctrl+C (ASCII 3)
		if buf[0] == 3 {
			close(quitChan)
			return
		}
	}
}
