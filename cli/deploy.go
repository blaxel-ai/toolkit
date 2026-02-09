package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"archive/tar"
	"archive/zip"
	"net/http"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/deploy"
	mon "github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/blaxel-ai/toolkit/cli/server"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("deploy", func() *cobra.Command {
		return DeployCmd()
	})
}

func DeployCmd() *cobra.Command {
	var name string
	var dryRun bool
	var recursive bool
	var folder string
	var envFiles []string
	var commandSecrets []string
	var skipBuild bool
	var noTTY bool

	cmd := &cobra.Command{
		Use:     "deploy",
		Args:    cobra.ExactArgs(0),
		Aliases: []string{"d", "dp"},
		Short:   "Deploy on blaxel",
		Long: `Deploy your Blaxel project to the cloud.

This command packages your code, builds a container image, and deploys it
to your workspace. The deployment process includes:
1. Reading configuration from blaxel.toml
2. Packaging source code (respects .blaxelignore)
3. Building container image with your runtime and dependencies
4. Uploading to Blaxel's container registry
5. Creating or updating the resource in your workspace
6. Streaming build and deployment logs (interactive mode)

You must run this command from a directory containing a blaxel.toml file.

Interactive vs Non-Interactive:
- Interactive (default): Shows live logs and deployment progress with TUI
- Non-interactive (--yes or CI): Runs without interactive UI, suitable for automation

Environment Variables and Secrets:
Use -e to load .env files or -s to pass secrets directly via command line.
Secrets are injected into your container at runtime and never stored in images.

Monorepo Support:
Use -d to deploy a specific subdirectory, or -R to recursively deploy
all projects in a monorepo (looks for blaxel.toml in subdirectories).`,
		Example: `  # Basic deployment (interactive mode with live logs)
  bl deploy

  # Non-interactive deployment (for CI/CD)
  bl deploy --yes

  # Deploy with environment variables
  bl deploy -e .env.production

  # Deploy with command-line secrets
  bl deploy -s API_KEY=xxx -s DB_PASSWORD=yyy

  # Deploy without rebuilding (reuse existing image)
  bl deploy --skip-build

  # Dry run to validate configuration
  bl deploy --dryrun

  # Deploy specific subdirectory in monorepo
  bl deploy -d ./packages/my-agent

  # Recursively deploy all projects in monorepo
  bl deploy -R`,
		Run: func(cmd *cobra.Command, args []string) {
			core.LoadCommandSecrets(commandSecrets)
			core.ReadSecrets(folder, envFiles)
			// If the user did not explicitly set --yes, decide default based on TTY and CI
			if !cmd.Flags().Changed("yes") {
				// By default use TTY mode (noTTY=false) if terminal is interactive and not in CI
				if core.IsTerminalInteractive() && !core.IsCIEnvironment() {
					noTTY = false
				} else {
					noTTY = true
				}
			}

			core.SetInteractiveMode(!noTTY)
			if folder != "" {
				recursive = false
				core.ReadSecrets("", envFiles)
				core.ReadConfigToml(folder, false)
			} else {
				// Read config without setting default type, we'll handle that below
				core.ReadConfigToml("", false)
			}

			cwd, err := os.Getwd()
			if err != nil {
				err = fmt.Errorf("failed to get current working directory: %w", err)
				core.PrintError("Deploy", err)
				core.ExitWithError(err)
			}

			// Additional deployment directory, for blaxel yaml files
			deployDir := ".blaxel"
			config := core.GetConfig()
			if config.Name != "" {
				name = config.Name
			}

			// Slugify the name to ensure it's URL-safe
			if name != "" {
				name = core.Slugify(name)
			}

			deployment := Deployment{
				dir:    deployDir,
				folder: folder,
				name:   name,
				cwd:    cwd,
			}

			// Check for blaxel.toml validation warnings first
			blaxelTomlWarning := core.GetBlaxelTomlWarning()
			if blaxelTomlWarning != "" {
				handleConfigWarning(blaxelTomlWarning, noTTY)
				core.ClearBlaxelTomlWarning()
			}

			if !skipBuild {
				validationWarning := deployment.validateDeploymentConfig(config)
				if validationWarning != "" {
					handleConfigWarning(validationWarning, noTTY)
				}
			}

			// Check if type is empty and prompt user if in interactive mode
			if config.Type == "" {
				if core.IsInteractiveMode() {
					selectedType := core.PromptForDeploymentType()
					if selectedType != "" {
						core.SetConfigType(selectedType)
					} else {
						// User cancelled (Ctrl+C or ESC) - exit instead of defaulting
						fmt.Println("Deployment cancelled.")
						os.Exit(0)
					}
				} else {
					core.SetConfigType("agent")
				}
			}

			// Refresh config after potential type change
			config = core.GetConfig()

			// Check if agent/function code uses HOST/PORT environment variables
			if (config.Type == "agent" || config.Type == "function") && !skipBuild {
				projectDir := filepath.Join(cwd, folder)
				language := core.ModuleLanguage(projectDir)
				if !core.CheckServerEnvUsage(folder, language) {
					serverEnvWarning := core.BuildServerEnvWarning(language, config.Type)
					handleConfigWarning(serverEnvWarning, noTTY)
				}
			}

			if recursive {
				if deployPackage(dryRun, name) {
					return
				}
			}

			err = deployment.Generate(skipBuild)
			if err != nil {
				err = fmt.Errorf("error generating blaxel deployment: %w", err)
				core.PrintError("Deploy", err)
				core.ExitWithError(err)
			}

			if dryRun {
				err := deployment.Print(skipBuild)
				if err != nil {
					err = fmt.Errorf("error printing blaxel deployment: %w", err)
					core.PrintError("Deploy", err)
					core.ExitWithError(err)
				}
				return
			}

			if !noTTY {
				err = deployment.ApplyInteractive()
			} else {
				err = deployment.Apply()
			}
			if err != nil {
				err = fmt.Errorf("error applying blaxel deployment: %w", err)
				core.PrintError("Deploy", err)
				core.ExitWithError(err)
			}

			// Only show success message for non-interactive deployments
			if noTTY {
				deployment.Ready()
			}
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Optional name for the deployment")
	cmd.Flags().BoolVarP(&dryRun, "dryrun", "", false, "Dry run the deployment")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Deploy recursively")
	cmd.Flags().StringVarP(&folder, "directory", "d", "", "Deployment app path, can be a sub directory")
	cmd.Flags().StringSliceVarP(&envFiles, "env-file", "e", []string{".env"}, "Environment file to load")
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to deploy")
	cmd.Flags().BoolVarP(&skipBuild, "skip-build", "", false, "Skip the build step")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive mode")
	return cmd
}

type Deployment struct {
	dir                    string
	name                   string
	folder                 string
	blaxelDeployments      []core.Result
	archive                *os.File
	cwd                    string
	progressCallback       func(status string, progress int)
	uploadProgressCallback func(bytesUploaded, totalBytes int64)
	callbackSecret         string
}

func (d *Deployment) Generate(skipBuild bool) error {
	if d.name == "" {
		split := strings.Split(filepath.Join(d.cwd, d.folder), "/")
		d.name = split[len(split)-1]
	}

	// Slugify the name to ensure it's URL-safe
	d.name = core.Slugify(d.name)

	err := core.SeedCache(d.cwd)
	if err != nil {
		return fmt.Errorf("failed to seed cache: %w", err)
	}

	// Generate the blaxel deployment yaml
	d.blaxelDeployments = []core.Result{d.GenerateDeployment(skipBuild)}

	// Volume-template needs archive even without build (for file upload)
	config := core.GetConfig()
	if !skipBuild || core.IsVolumeTemplate(config.Type) {
		// Create archive (tar for volume-template, zip for others)
		if core.IsVolumeTemplate(config.Type) {
			// For interactive mode, skip compression here - it will be done during deployment
			// to show progress to the user
			if !core.IsInteractiveMode() {
				fmt.Println("Compressing volume template files...")
				err = d.Tar()
				if err != nil {
					return fmt.Errorf("failed to tar file: %w", err)
				}
				fmt.Println("Compression completed")
			}
		} else {
			err = d.Zip()
			if err != nil {
				return fmt.Errorf("failed to zip file: %w", err)
			}
		}
	}

	return nil
}

// handleConfigWarning displays a warning and asks for confirmation in interactive mode
func handleConfigWarning(warning string, noTTY bool) {
	// Print the warning
	fmt.Println(warning)

	// In non-interactive mode, just show warning and continue
	if noTTY {
		core.PrintWarning("Continuing with deployment despite configuration warning...")
	} else {
		// In interactive mode, ask for confirmation with Ctrl+C support
		// Set up signal handler for Ctrl+C
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)

		// Create channel for response
		responseChan := make(chan string, 1)

		go func() {
			fmt.Print("\nDo you want to proceed anyway? (y/N, or press Ctrl+C or 'q' to quit): ")
			var response string
			fmt.Scanln(&response)
			responseChan <- response
		}()

		// Wait for either response or interrupt
		select {
		case <-sigChan:
			fmt.Println("\nDeployment cancelled.")
			os.Exit(0)
		case response := <-responseChan:
			response = strings.ToLower(strings.TrimSpace(response))

			if response == "q" || response == "quit" {
				fmt.Println("Deployment cancelled.")
				os.Exit(0)
			}

			if response != "y" && response != "yes" {
				fmt.Println("Deployment cancelled.")
				os.Exit(0)
			}
		}

		// Clean up signal handler
		signal.Stop(sigChan)
		fmt.Println()
	}
}

// validateDeploymentConfig checks if the project has proper configuration for deployment
// Returns a warning message if configuration is missing, empty string if all is good
func (d *Deployment) validateDeploymentConfig(config core.Config) string {
	// Skip validation for volume templates - they don't need language detection, Dockerfile, or entrypoint
	if core.IsVolumeTemplate(config.Type) {
		return ""
	}

	projectDir := filepath.Join(d.cwd, d.folder)

	// Check for Dockerfile
	dockerfilePath := filepath.Join(projectDir, "Dockerfile")
	hasDockerfile := false
	if _, err := os.Stat(dockerfilePath); err == nil {
		hasDockerfile = true
	}

	// Special validation for sandbox type
	if config.Type == "sandbox" {
		if !hasDockerfile {
			// No Dockerfile for sandbox - show warning with sample
			var warningMsg strings.Builder
			warningMsg.WriteString("⚠️  Sandbox Configuration Warning\n")
			warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

			codeColor := color.New(color.FgCyan)
			warningMsg.WriteString(fmt.Sprintf("Sandbox deployments require a %s.\n\n", codeColor.Sprint("Dockerfile")))
			warningMsg.WriteString("Quick sample Dockerfile:\n\n")
			warningMsg.WriteString(codeColor.Sprint("FROM debian:bookworm-slim\n\n"))
			warningMsg.WriteString(codeColor.Sprint("WORKDIR /app\n\n"))
			warningMsg.WriteString(codeColor.Sprint("COPY --from=ghcr.io/blaxel-ai/sandbox:latest /sandbox-api /usr/local/bin/sandbox-api\n\n"))
			warningMsg.WriteString(codeColor.Sprint("ENTRYPOINT [\"/usr/local/bin/sandbox-api\"]\n\n"))
			warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			return warningMsg.String()
		}

		// Dockerfile exists, check if it contains sandbox-api
		dockerfileContent, err := os.ReadFile(dockerfilePath)
		if err == nil {
			content := string(dockerfileContent)
			hasSandboxAPI := strings.Contains(content, "sandbox-api")
			hasBlaxelSandboxImage := strings.Contains(content, "ghcr.io/blaxel-ai/sandbox-")

			if !hasSandboxAPI && !hasBlaxelSandboxImage {
				// Dockerfile exists but doesn't have sandbox-api
				var warningMsg strings.Builder
				warningMsg.WriteString("⚠️  Sandbox Configuration Warning\n")
				warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

				codeColor := color.New(color.FgCyan)
				warningMsg.WriteString(fmt.Sprintf("Dockerfile found, but it doesn't contain %s or reference %s.\n\n",
					codeColor.Sprint("sandbox-api"), codeColor.Sprint("any sandbox image from blaxel")))
				warningMsg.WriteString("Your Dockerfile should come from a sandbox image or at least include sandbox-api:\n\n")
				warningMsg.WriteString(codeColor.Sprint("COPY --from=ghcr.io/blaxel-ai/sandbox:latest /sandbox-api /usr/local/bin/sandbox-api\n\n"))
				warningMsg.WriteString(codeColor.Sprint("ENTRYPOINT [\"/usr/local/bin/sandbox-api\"]\n\n"))
				warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

				return warningMsg.String()
			}
		}
		return ""
	}

	// Check for language-specific files
	language := core.ModuleLanguage(d.folder)
	hasLanguage := language != ""
	hasEntrypoint := true

	if hasLanguage && config.Entrypoint.Production == "" {
		switch language {
		case "python":
			hasEntrypoint = core.HasPythonEntryFile(projectDir)
		case "go":
			hasEntrypoint = core.HasGoEntryFile(projectDir)
		case "typescript":
			hasEntrypoint = core.HasTypeScriptEntryFile(projectDir)
		}
	}

	// If everything is fine, return early
	if hasDockerfile || (hasLanguage && hasEntrypoint) {
		return ""
	}

	// Build concise warning message
	var warningMsg strings.Builder
	warningMsg.WriteString("⚠️  Configuration Warning\n")
	warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	codeColor := color.New(color.FgCyan)
	languageColor := color.New(color.FgGreen)

	if !hasLanguage {
		warningMsg.WriteString(fmt.Sprintf("No language detected. Missing %s or language files.\n\n", codeColor.Sprint("Dockerfile")))
		warningMsg.WriteString(fmt.Sprintf("To fix: Add a %s OR auto-detect language files:\n", codeColor.Sprint("Dockerfile")))
		warningMsg.WriteString(fmt.Sprintf("  • %s or %s (Python)\n", codeColor.Sprint("pyproject.toml"), codeColor.Sprint("requirements.txt")))
		warningMsg.WriteString(fmt.Sprintf("  • %s (TypeScript/Node)\n", codeColor.Sprint("package.json")))
		warningMsg.WriteString(fmt.Sprintf("  • %s (Go)\n\n", codeColor.Sprint("go.mod")))
	} else {
		warningMsg.WriteString(fmt.Sprintf("Detected %s project, but missing entrypoint.\n\n", languageColor.Sprint(language)))
		warningMsg.WriteString("To fix:\n")
		entrypointSection := fmt.Sprintf("  • Set entrypoint in %s by adding the following section:\n\n%s\n\n", codeColor.Sprint("blaxel.toml"), codeColor.Sprint("[entrypoint]\nprod = \"your-command\""))
		switch language {
		case "python":
			pythonFiles := codeColor.Sprint("main.py, app.py, api.py, src/main.py, src/app.py, src/api.py, app/main.py, app/app.py, app/api.py")
			warningMsg.WriteString(fmt.Sprintf("  • Add automatic entrypoint %s OR\n", pythonFiles))
			warningMsg.WriteString(entrypointSection)
		case "go":
			goFiles := codeColor.Sprint("main.go, src/main.go, cmd/main.go")
			warningMsg.WriteString(fmt.Sprintf("  • Add automatic entrypoint %s OR\n", goFiles))
			warningMsg.WriteString(entrypointSection)
		case "typescript":
			warningMsg.WriteString(fmt.Sprintf("  • Add start script in %s OR\n", codeColor.Sprint("package.json")))
			warningMsg.WriteString(entrypointSection)
		}
	}

	warningMsg.WriteString("Learn more: https://docs.blaxel.ai/Agents/Deploy-an-agent\n\n")
	warningMsg.WriteString("⚠️  Blaxel will attempt to build with default settings, but this may fail.\n")
	warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return warningMsg.String()
}

func (d *Deployment) GenerateDeployment(skipBuild bool) core.Result {
	var Spec map[string]interface{}
	var Kind string

	runtime := make(map[string]interface{})
	config := core.GetConfig()
	if config.Runtime != nil {
		runtime = *config.Runtime
	}

	// Convert human-readable timeout values (e.g., "1h", "30m") to seconds
	if err := core.ConvertRuntimeTimeouts(runtime); err != nil {
		core.PrintError("Deployment", err)
		core.ExitWithError(err)
	}

	// Convert human-readable timeout values in triggers
	if err := core.ConvertTriggersTimeouts(config.Triggers); err != nil {
		core.PrintError("Deployment", err)
		core.ExitWithError(err)
	}

	runtime["envs"] = core.GetUniqueEnvs()
	if config.Type == "function" {
		runtime["type"] = "mcp"
	}

	// Skip image resolution for volume-template as it doesn't use runtime/image
	if skipBuild && !core.IsVolumeTemplate(config.Type) {
		resource, err := getResource(config.Type, d.name)
		if err != nil {
			core.PrintError("Deployment", err)
			core.ExitWithError(err)
		}

		if spec, ok := resource["spec"].(map[string]interface{}); ok {
			if rt, ok := spec["runtime"].(map[string]interface{}); ok {
				if image, ok := rt["image"].(string); ok && image != "" {
					runtime["image"] = image
				} else {
					err := fmt.Errorf("no image found for %s. please deploy with a build first", d.name)
					core.PrintError("Deployment", err)
					core.ExitWithError(err)
				}
			}
		}
	}

	switch config.Type {
	case "function":
		Kind = "Function"
		Spec = map[string]interface{}{
			"runtime":  runtime,
			"triggers": config.Triggers,
		}
	case "agent":
		Kind = "Agent"
		Spec = map[string]interface{}{
			"runtime":  runtime,
			"triggers": config.Triggers,
		}
	case "job":
		Kind = "Job"
		Spec = map[string]interface{}{
			"runtime":  runtime,
			"triggers": config.Triggers,
		}
	case "sandbox":
		Kind = "Sandbox"
		Spec = map[string]interface{}{
			"runtime":  runtime,
			"triggers": config.Triggers,
		}
		if config.Region != "" {
			Spec["region"] = config.Region
		}
		if config.Volumes != nil {
			Spec["volumes"] = *config.Volumes
		}
	case "volume-template", "volumetemplate", "vt":
		Kind = "VolumeTemplate"
		Spec = map[string]interface{}{}
		if config.DefaultSize != nil {
			Spec["defaultSize"] = *config.DefaultSize
		}
	}
	if len(config.Policies) > 0 {
		Spec["policies"] = config.Policies
	}
	if config.Public != nil {
		Spec["public"] = *config.Public
	}
	labels := map[string]interface{}{}
	// Volume-template needs upload even without build
	if !skipBuild || core.IsVolumeTemplate(config.Type) {
		labels["x-blaxel-auto-generated"] = "true"
	}
	return core.Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       Kind,
		Metadata: map[string]interface{}{
			"name":   d.name,
			"labels": labels,
		},
		Spec: Spec,
	}
}

func getResource(resourceType, name string) (map[string]interface{}, error) {
	ctx := context.Background()
	client := core.GetClient()

	var result interface{}
	var err error

	switch resourceType {
	case "agent":
		result, err = client.Agents.Get(ctx, name, blaxel.AgentGetParams{})
	case "function":
		result, err = client.Functions.Get(ctx, name, blaxel.FunctionGetParams{})
	case "job":
		result, err = client.Jobs.Get(ctx, name, blaxel.JobGetParams{})
	case "sandbox":
		result, err = client.Sandboxes.Get(ctx, name, blaxel.SandboxGetParams{})
	case "volume-template", "volumetemplate", "vt":
		result, err = client.VolumeTemplates.Get(ctx, name)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		// Check if it's a not found error
		var apiErr *blaxel.Error
		if isBlaxelErrorDeploy(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, fmt.Errorf("%s %s not found. please deploy with a build first", resourceType, name)
		}
		return nil, err
	}

	// Convert result to map
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(jsonData, &resource); err != nil {
		return nil, err
	}

	return resource, nil
}

func getResourceStatus(resourceType, name string) (string, error) {
	ctx := context.Background()
	client := core.GetClient()

	var result interface{}
	var err error

	switch resourceType {
	case "agent":
		result, err = client.Agents.Get(ctx, name, blaxel.AgentGetParams{})
	case "function":
		result, err = client.Functions.Get(ctx, name, blaxel.FunctionGetParams{})
	case "job":
		result, err = client.Jobs.Get(ctx, name, blaxel.JobGetParams{})
	case "sandbox":
		result, err = client.Sandboxes.Get(ctx, name, blaxel.SandboxGetParams{})
	case "volume-template", "volumetemplate", "vt":
		result, err = client.VolumeTemplates.Get(ctx, name)
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		return "", err
	}

	// Convert result to map
	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(jsonData, &resource); err != nil {
		return "", err
	}

	// Extract status from the resource
	if status, ok := resource["status"].(string); ok {
		return status, nil
	}

	return "UNKNOWN", nil
}

func (d *Deployment) Apply() error {
	blaxelDir := filepath.Join(d.cwd, ".blaxel")
	if _, err := os.Stat(blaxelDir); err == nil {
		fmt.Println("Applying additional resources from .blaxel directory...")
		_, err = Apply(blaxelDir, WithRecursive(true))
		if err != nil {
			return fmt.Errorf("failed to apply .blaxel directory: %w", err)
		}
	}
	applyResults, err := ApplyResources(d.blaxelDeployments)
	if err != nil {
		return fmt.Errorf("failed to apply deployment: %w", err)
	}

	// Store callback secret from first result if present
	var callbackSecret string
	if len(applyResults) > 0 && applyResults[0].Result.CallbackSecret != "" {
		callbackSecret = applyResults[0].Result.CallbackSecret
		d.callbackSecret = callbackSecret
	}

	for _, result := range applyResults {
		if result.Result.UploadURL != "" {
			config := core.GetConfig()
			// Print upload message for all resource types
			resourceLabel := "code"
			switch strings.ToLower(config.Type) {
			case "volumetemplate":
				resourceLabel = "volume template"
			case "agent":
				resourceLabel = "agent code"
			case "function":
				resourceLabel = "function code"
			case "job":
				resourceLabel = "job code"
			case "sandbox":
				resourceLabel = "sandbox code"
			}
			fmt.Printf("Uploading %s...\n", resourceLabel)

			err := d.Upload(result.Result.UploadURL)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}
			fmt.Println("Upload completed")
		}
	}

	return nil
}

func (d *Deployment) ApplyInteractive() error {
	// Create resources for interactive UI
	resources := make([]*deploy.Resource, 0)

	// Add main deployment resources
	for _, deployment := range d.blaxelDeployments {
		metadata := deployment.Metadata.(map[string]interface{})
		name := metadata["name"].(string)

		// Safely extract labels
		autoGenerated := false
		if labels, ok := metadata["labels"].(map[string]interface{}); ok {
			if val, exists := labels["x-blaxel-auto-generated"]; exists {
				autoGenerated = val == "true" || val == true
			}
		}

		resources = append(resources, &deploy.Resource{
			Kind:          deployment.Kind,
			Name:          name,
			Status:        deploy.StatusPending,
			AutoGenerated: autoGenerated,
		})
	}

	// Check for additional resources in .blaxel directory
	blaxelDir := filepath.Join(d.cwd, ".blaxel")
	additionalResources := make([]*deploy.Resource, 0)

	if _, err := os.Stat(blaxelDir); err == nil {
		// Real mode: read .blaxel directory to get resource count
		results, err := core.GetResults("apply", blaxelDir, true)
		if err == nil && len(results) > 0 {
			for _, result := range results {
				if metadata, ok := result.Metadata.(map[string]interface{}); ok {
					name := "unknown"
					if n, exists := metadata["name"]; exists {
						name = fmt.Sprintf("%v", n)
					}

					// Check for auto-generated label
					autoGenerated := false
					if labels, ok := metadata["labels"].(map[string]interface{}); ok {
						if val, exists := labels["x-blaxel-auto-generated"]; exists {
							autoGenerated = val == "true" || val == true
						}
					}

					additionalResources = append(additionalResources, &deploy.Resource{
						Kind:          result.Kind,
						Name:          name,
						Status:        deploy.StatusPending,
						AutoGenerated: autoGenerated,
					})
				}
			}
		}

		// Add all additional resources
		resources = append(resources, additionalResources...)
	}

	// Create interactive model
	model := deploy.NewInteractiveModel(resources)

	// Start the interactive UI
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Set program reference so model can send messages
	model.SetProgram(p)

	go d.runInteractiveDeployment(resources, additionalResources, model)

	// Run the UI
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running interactive UI: %w", err)
	}

	// Check if any resources failed
	for _, r := range resources {
		if r.Status == deploy.StatusFailed {
			return fmt.Errorf("deployment failed for %s/%s: %v", r.Kind, r.Name, r.Error)
		}
	}

	return nil
}

func (d *Deployment) runInteractiveDeployment(resources []*deploy.Resource, additionalResources []*deploy.Resource, model *deploy.InteractiveModel) {
	// Add recovery to catch panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in deployment: %v\n", r)
			// Try to mark all resources as failed
			for i := range resources {
				model.UpdateResource(i, deploy.StatusFailed, fmt.Sprintf("Panic: %v", r), fmt.Errorf("%v", r))
			}
			model.Complete()
		}
	}()

	// Determine where main resources end and additional resources begin
	mainResourceCount := len(resources) - len(additionalResources)

	// Start all deployments in parallel
	var wg sync.WaitGroup

	// Deploy additional resources
	for i := mainResourceCount; i < len(resources); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC in additional resource deployment: %v\n", r)
					model.UpdateResource(idx, deploy.StatusFailed, fmt.Sprintf("Panic: %v", r), fmt.Errorf("%v", r))
				}
			}()

			resource := resources[idx]
			model.UpdateResource(idx, deploy.StatusDeploying, "Applying resource", nil)

			// Real deployment
			d.deployAdditionalResource(resource, model, idx)
		}(i)
	}

	// Deploy main resources in parallel
	for i := 0; i < mainResourceCount; i++ {
		wg.Add(1)
		go func(idx int, depl core.Result) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC in main resource deployment: %v\n", r)
					model.UpdateResource(idx, deploy.StatusFailed, fmt.Sprintf("Panic: %v", r), fmt.Errorf("%v", r))
				}
			}()
			d.deployResourceInteractive(resources[idx], model, idx, depl)
		}(i, d.blaxelDeployments[i])
	}

	wg.Wait()
	model.Complete()
}

func (d *Deployment) deployResourceInteractive(resource *deploy.Resource, model *deploy.InteractiveModel, idx int, deployment core.Result) {
	config := core.GetConfig()

	// For volume templates, handle compression first
	if core.IsVolumeTemplate(config.Type) {
		model.UpdateResource(idx, deploy.StatusCompressing, "Compressing files", nil)
		model.AddBuildLog(idx, "Starting compression of volume template files...")

		// Set up progress callback for compression
		var lastLoggedProgress int
		d.progressCallback = func(status string, progress int) {
			model.UpdateResource(idx, deploy.StatusCompressing, status, nil)
			// Log every 10% to avoid log spam
			if progress > 0 && progress%10 == 0 && progress != lastLoggedProgress {
				model.AddBuildLog(idx, fmt.Sprintf("Compression progress: %d%%", progress))
				lastLoggedProgress = progress
			}
		}

		// Create the tar archive
		err := d.Tar()
		if err != nil {
			model.UpdateResource(idx, deploy.StatusFailed, "Compression failed", err)
			model.AddBuildLog(idx, fmt.Sprintf("Failed to compress files: %v", err))
			return
		}
		model.AddBuildLog(idx, "Compression completed (100%)")
	}

	// Start deployment
	model.UpdateResource(idx, deploy.StatusDeploying, "Applying resource", nil)
	model.AddBuildLog(idx, fmt.Sprintf("Starting deployment of %s/%s", resource.Kind, resource.Name))

	// Real deployment
	model.AddBuildLog(idx, "Applying resource to platform...")
	applyResults, err := ApplyResources([]core.Result{deployment})
	if err != nil {
		model.UpdateResource(idx, deploy.StatusFailed, "Failed to apply", err)
		model.AddBuildLog(idx, fmt.Sprintf("Failed to apply resource: %v", err))
		return
	}

	// Check if apply failed (ApplyResources doesn't return errors, but the result might indicate failure)
	if len(applyResults) == 0 {
		model.UpdateResource(idx, deploy.StatusFailed, "No results from apply", fmt.Errorf("apply returned no results"))
		model.AddBuildLog(idx, "Apply operation returned no results - check if the API call succeeded")
		return
	}

	if applyResults[0].Result.Status == "failed" {
		errorDetails := "apply operation failed"
		if applyResults[0].Result.ErrorMsg != "" {
			errorDetails = applyResults[0].Result.ErrorMsg
			model.AddBuildLog(idx, fmt.Sprintf("API Error: %s", errorDetails))
		}
		model.UpdateResource(idx, deploy.StatusFailed, "Apply failed", fmt.Errorf("%s", errorDetails))
		return
	}

	// Store callback secret from apply result if present (only available on first deployment)
	if applyResults[0].Result.CallbackSecret != "" {
		resource.SetCallbackSecret(applyResults[0].Result.CallbackSecret)
		model.AddBuildLog(idx, fmt.Sprintf("Callback secret configured: %s", applyResults[0].Result.CallbackSecret))
	}

	// Handle upload if there's an upload URL
	if len(applyResults) > 0 && applyResults[0].Result.UploadURL != "" {
		model.UpdateResource(idx, deploy.StatusUploading, "Uploading code", nil)

		// Check if resource type supports detailed upload progress
		needsUploadProgress := false
		var uploadLabel string
		switch strings.ToLower(resource.Kind) {
		case "volumetemplate":
			needsUploadProgress = true
			uploadLabel = "volume template"
		case "agent":
			needsUploadProgress = true
			uploadLabel = "agent code"
		case "function":
			needsUploadProgress = true
			uploadLabel = "function code"
		case "job":
			needsUploadProgress = true
			uploadLabel = "job code"
		case "sandbox":
			needsUploadProgress = true
			uploadLabel = "sandbox code"
		}

		// Set up upload progress callback for supported resources
		if needsUploadProgress {
			model.AddBuildLog(idx, fmt.Sprintf("Starting upload of %s...", uploadLabel))

			var lastLoggedPercentage int
			var lastUpdatePercentage int
			startTime := time.Now()

			d.uploadProgressCallback = func(bytesUploaded, totalBytes int64) {
				percentage := int((bytesUploaded * 100) / totalBytes)
				now := time.Now()

				// Update status every 1% to keep UI responsive
				if percentage != lastUpdatePercentage {
					sizeMB := float64(totalBytes) / (1024 * 1024)
					uploadedMB := float64(bytesUploaded) / (1024 * 1024)

					// Calculate speed and ETA
					elapsed := now.Sub(startTime).Seconds()
					var speedStr string
					var etaStr string
					if bytesUploaded > 0 && elapsed > 0 {
						bytesPerSecond := float64(bytesUploaded) / elapsed
						mbPerSecond := bytesPerSecond / (1024 * 1024)

						// Format speed
						if mbPerSecond >= 1.0 {
							speedStr = fmt.Sprintf("%.2f MB/s", mbPerSecond)
						} else {
							kbPerSecond := bytesPerSecond / 1024
							speedStr = fmt.Sprintf("%.2f KB/s", kbPerSecond)
						}

						// Calculate ETA
						remainingBytes := totalBytes - bytesUploaded
						etaSeconds := int(float64(remainingBytes) / bytesPerSecond)

						if etaSeconds < 60 {
							etaStr = fmt.Sprintf(" - ETA %ds", etaSeconds)
						} else if etaSeconds < 3600 {
							etaStr = fmt.Sprintf(" - ETA %dm %ds", etaSeconds/60, etaSeconds%60)
						} else {
							etaStr = fmt.Sprintf(" - ETA %dh %dm", etaSeconds/3600, (etaSeconds%3600)/60)
						}
					}

					status := fmt.Sprintf("Uploading (%.2f/%.2f MB - %d%% - %s%s)", uploadedMB, sizeMB, percentage, speedStr, etaStr)
					model.UpdateResource(idx, deploy.StatusUploading, status, nil)
					lastUpdatePercentage = percentage
				}

				// Log progress every 10% to avoid log spam
				if percentage > 0 && percentage%10 == 0 && percentage != lastLoggedPercentage {
					sizeMB := float64(totalBytes) / (1024 * 1024)
					uploadedMB := float64(bytesUploaded) / (1024 * 1024)

					// Calculate speed and ETA for logs
					elapsed := now.Sub(startTime).Seconds()
					speedStr := ""
					etaStr := ""
					if bytesUploaded > 0 && elapsed > 0 {
						bytesPerSecond := float64(bytesUploaded) / elapsed
						mbPerSecond := bytesPerSecond / (1024 * 1024)

						// Format speed
						if mbPerSecond >= 1.0 {
							speedStr = fmt.Sprintf(" @ %.2f MB/s", mbPerSecond)
						} else {
							kbPerSecond := bytesPerSecond / 1024
							speedStr = fmt.Sprintf(" @ %.2f KB/s", kbPerSecond)
						}

						remainingBytes := totalBytes - bytesUploaded
						etaSeconds := int(float64(remainingBytes) / bytesPerSecond)

						if etaSeconds < 60 {
							etaStr = fmt.Sprintf(", ETA %ds", etaSeconds)
						} else if etaSeconds < 3600 {
							etaStr = fmt.Sprintf(", ETA %dm %ds", etaSeconds/60, etaSeconds%60)
						} else {
							etaStr = fmt.Sprintf(", ETA %dh %dm", etaSeconds/3600, (etaSeconds%3600)/60)
						}
					}

					model.AddBuildLog(idx, fmt.Sprintf("Upload progress: %d%% (%.2f/%.2f MB%s%s)", percentage, uploadedMB, sizeMB, speedStr, etaStr))
					lastLoggedPercentage = percentage
				}
			}
		} else {
			model.AddBuildLog(idx, "Uploading code to registry...")
		}

		err := d.Upload(applyResults[0].Result.UploadURL)
		if err != nil {
			model.UpdateResource(idx, deploy.StatusFailed, "Upload failed", err)
			model.AddBuildLog(idx, fmt.Sprintf("Upload failed: %v", err))
			return
		}
		model.AddBuildLog(idx, "Upload completed successfully")
	}

	// For resources that need status monitoring (agent, function, job, sandbox)
	needsStatusMonitoring := false
	switch strings.ToLower(resource.Kind) {
	case "agent", "function", "job", "sandbox":
		needsStatusMonitoring = true
	case "volumetemplate":
		needsStatusMonitoring = false
	}

	if needsStatusMonitoring {
		// Wait for backend to update status after apply/upload
		time.Sleep(1000 * time.Millisecond)
		model.AddBuildLog(idx, "Verifying deployment status...")

		// Get initial status before monitoring - this helps detect stale FAILED status from previous builds
		initialStatus, err := getResourceStatus(strings.ToLower(resource.Kind), resource.Name)
		if err != nil {
			// If we can't get initial status, assume it's not FAILED to avoid false positives
			model.AddBuildLog(idx, fmt.Sprintf("Warning: Could not get initial status: %v", err))
			initialStatus = "UNKNOWN"
		}

		// Start monitoring the resource status
		statusTicker := time.NewTicker(3 * time.Second)
		defer statusTicker.Stop()
		statusTimeout := time.After(15 * time.Minute) // 15 minute timeout for deployment

		// Grace period for stale FAILED status - if we don't see any status change within this time,
		// accept that the FAILED status is real (handles case where new deployment fails immediately)
		var staleFailedGracePeriod <-chan time.Time
		if initialStatus == "FAILED" {
			staleFailedGracePeriod = time.After(15 * time.Second)
		}
		staleGracePeriodExpired := false

		var logWatcher interface{ Stop() }
		buildLogStarted := false
		lastStatus := ""           // Track last status to avoid duplicate logs
		sawBuildingStatus := false // Track if we've seen BUILDING status
		sawStatusChange := false   // Track if status has changed from initial (new build started)

		for {
			select {
			case <-statusTimeout:
				if logWatcher != nil {
					logWatcher.Stop()
				}
				model.UpdateResource(idx, deploy.StatusFailed, "Deployment timeout", fmt.Errorf("deployment timed out after 15 minutes"))
				return
			case <-staleFailedGracePeriod:
				// Grace period expired - if status is still FAILED, accept it as real
				staleGracePeriodExpired = true
			case <-statusTicker.C:
				status, err := getResourceStatus(strings.ToLower(resource.Kind), resource.Name)
				if err != nil {
					// Continue polling on temporary errors
					continue
				}

				// Track if we've seen the status change from initial (indicates new build has started)
				if status != initialStatus {
					sawStatusChange = true
				}

				// Only log status changes
				if status != lastStatus {
					lastStatus = status

					// Map API status to our UI status and update
					switch status {
					case "UPLOADING":
						model.UpdateResource(idx, deploy.StatusUploading, "Uploading code", nil)
						model.AddBuildLog(idx, "Status changed to: UPLOADING")
					case "BUILDING":
						sawBuildingStatus = true
						model.UpdateResource(idx, deploy.StatusBuilding, "Building image", nil)
						model.AddBuildLog(idx, "Status changed to: BUILDING")

						// Start build log watcher if not already started
						if !buildLogStarted {
							buildLogStarted = true
							client := core.GetClient()
							workspace := core.GetWorkspace()

							// Start build log watcher in background
							lw := mon.NewBuildLogWatcher(
								client,
								workspace,
								strings.ToLower(resource.Kind),
								resource.Name,
								func(log string) {
									model.AddBuildLog(idx, log)
								},
							)
							lw.Start()
							logWatcher = lw
						}
					case "DEPLOYING":
						if logWatcher != nil {
							logWatcher.Stop()
							logWatcher = nil
						}
						model.UpdateResource(idx, deploy.StatusDeploying, "Deploying to cluster", nil)
						model.AddBuildLog(idx, "Status changed to: DEPLOYING")
					case "DEPLOYED":
						// If skipBuild is false (AutoGenerated=true), we MUST have seen BUILDING status
						if resource.AutoGenerated && !sawBuildingStatus {
							// This is a mistake - continue monitoring
							continue
						}
						if logWatcher != nil {
							logWatcher.Stop()
						}

						model.UpdateResource(idx, deploy.StatusComplete, "Deployed successfully", nil)
						model.AddBuildLog(idx, fmt.Sprintf("Deployment completed with status: %s", status))
						return
					case "FAILED":
						// Ignore stale FAILED status from previous builds, unless:
						// 1. We've seen the status change (new build started and then failed)
						// 2. The grace period has expired (no status change = new build failed immediately)
						// 3. Initial status wasn't FAILED (no stale status to worry about)
						if initialStatus == "FAILED" && !sawStatusChange && !staleGracePeriodExpired {
							continue
						}
						if logWatcher != nil {
							logWatcher.Stop()
						}
						model.UpdateResource(idx, deploy.StatusFailed, "Deployment failed", fmt.Errorf("resource deployment failed"))
						model.AddBuildLog(idx, "Status changed to: FAILED - Deployment failed")
						return
					case "DEACTIVATED", "DEACTIVATING", "DELETING":
						if logWatcher != nil {
							logWatcher.Stop()
						}
						model.UpdateResource(idx, deploy.StatusFailed, fmt.Sprintf("Unexpected status: %s", status), fmt.Errorf("resource is being deactivated or deleted"))
						model.AddBuildLog(idx, fmt.Sprintf("Unexpected status: %s", status))
						return
					default:
						// Continue monitoring for unknown statuses
						model.UpdateResource(idx, deploy.StatusDeploying, fmt.Sprintf("Status: %s", status), nil)
						model.AddBuildLog(idx, fmt.Sprintf("Status: %s", status))
					}
				}
			}
		}
	} else {
		// For resources that don't need monitoring (VolumeTemplate, Model, Policy, etc.), just mark as complete
		model.AddBuildLog(idx, fmt.Sprintf("Resource type %s does not require status monitoring", resource.Kind))
		model.UpdateResource(idx, deploy.StatusComplete, "Deployed successfully", nil)
		model.AddBuildLog(idx, "✓ Volume template deployed successfully!")
	}
}

func (d *Deployment) deployAdditionalResource(resource *deploy.Resource, model *deploy.InteractiveModel, idx int) {
	model.AddBuildLog(idx, fmt.Sprintf("Starting deployment of %s/%s", resource.Kind, resource.Name))

	// Apply the resource
	blaxelDir := filepath.Join(".", ".blaxel")
	results, err := core.GetResults("apply", blaxelDir, false)
	if err == nil && len(results) > 0 {
		// Find the matching resource
		for _, result := range results {
			if metadata, ok := result.Metadata.(map[string]interface{}); ok {
				if name, exists := metadata["name"]; exists && fmt.Sprintf("%v", name) == resource.Name {
					// Apply this specific resource
					results, err := ApplyResources([]core.Result{result})
					if err != nil {
						model.UpdateResource(idx, deploy.StatusFailed, "Failed to apply", err)
						model.AddBuildLog(idx, fmt.Sprintf("Failed to apply resource: %v", err))
						return
					}
					for _, result := range results {
						if result.Result.Status == "failed" {
							model.UpdateResource(idx, deploy.StatusFailed, "Failed to apply", errors.New(result.Result.ErrorMsg))
							model.AddBuildLog(idx, fmt.Sprintf("Resource %s failed to apply: %v", result.Name, result.Result.ErrorMsg))
							return
						}
						// Store callback secret from apply result if present (only available on first deployment)
						if result.Result.CallbackSecret != "" {
							resource.SetCallbackSecret(result.Result.CallbackSecret)
							model.AddBuildLog(idx, fmt.Sprintf("Callback secret configured: %s", result.Result.CallbackSecret))
						}
					}
					model.AddBuildLog(idx, "Resource applied, monitoring status...")

					// For resources that need monitoring, start status polling
					needsMonitoring := false
					switch strings.ToLower(resource.Kind) {
					case "agent", "function", "job", "sandbox":
						needsMonitoring = true
					case "volumetemplate":
						needsMonitoring = false
					}

					if needsMonitoring {
						// Wait for backend to update status after apply
						time.Sleep(1000 * time.Millisecond)
						model.AddBuildLog(idx, "Verifying deployment status...")

						// Simple status monitoring for additional resources
						ticker := time.NewTicker(3 * time.Second)
						timeout := time.After(10 * time.Minute)
						lastStatus := "" // Track last status to avoid duplicate logs
						var logWatcher interface{ Stop() }
						buildLogStarted := false
						sawBuildingStatus := false // Track if we've seen BUILDING status

						for {
							select {
							case <-timeout:
								model.UpdateResource(idx, deploy.StatusFailed, "Timeout", fmt.Errorf("deployment timed out"))
								ticker.Stop()
								return
							case <-ticker.C:
								status, err := getResourceStatus(strings.ToLower(resource.Kind), resource.Name)
								if err != nil {
									continue
								}

								// Logs handling
								if status != lastStatus {
									lastStatus = status
									model.AddBuildLog(idx, fmt.Sprintf("Status: %s", status))

									switch status {
									case "UPLOADING":
										model.UpdateResource(idx, deploy.StatusUploading, "Uploading code", nil)
									case "BUILDING":
										sawBuildingStatus = true
										model.UpdateResource(idx, deploy.StatusBuilding, "Building image", nil)

										// Start build log watcher if not already started
										if !buildLogStarted {
											buildLogStarted = true
											client := core.GetClient()
											workspace := core.GetWorkspace()

											lw := mon.NewBuildLogWatcher(
												client,
												workspace,
												strings.ToLower(resource.Kind),
												resource.Name,
												func(log string) {
													model.AddBuildLog(idx, log)
												},
											)
											lw.Start()
											logWatcher = lw
										}
									case "DEPLOYING":
										if logWatcher != nil {
											logWatcher.Stop()
											logWatcher = nil
										}
										model.UpdateResource(idx, deploy.StatusDeploying, "Deploying to cluster", nil)
									case "DEPLOYED":
										// If skipBuild is false (AutoGenerated=true), we MUST have seen BUILDING status
										if resource.AutoGenerated && !sawBuildingStatus {
											// This is a mistake - continue monitoring
											continue
										}
										if logWatcher != nil {
											logWatcher.Stop()
										}

										model.UpdateResource(idx, deploy.StatusComplete, "Applied successfully", nil)
										ticker.Stop()
										return
									case "FAILED":
										if logWatcher != nil {
											logWatcher.Stop()
										}
										model.UpdateResource(idx, deploy.StatusFailed, "Failed", fmt.Errorf("deployment failed"))
										ticker.Stop()
										return
									case "DEACTIVATED", "DEACTIVATING", "DELETING":
										if logWatcher != nil {
											logWatcher.Stop()
										}
										model.UpdateResource(idx, deploy.StatusFailed, fmt.Sprintf("Unexpected status: %s", status), fmt.Errorf("resource is being deactivated or deleted"))
										ticker.Stop()
										return
									default:
										// Continue monitoring for unknown statuses
										model.UpdateResource(idx, deploy.StatusDeploying, fmt.Sprintf("Status: %s", status), nil)
									}
								}
							}
						}
					} else {
						// Non-monitored resources complete immediately
						model.UpdateResource(idx, deploy.StatusComplete, "Applied successfully", nil)
						model.AddBuildLog(idx, "Resource applied successfully")
					}
					break
				}
			}
		}
	} else {
		// If we can't find/apply the resource, just mark as complete
		model.UpdateResource(idx, deploy.StatusComplete, "Applied successfully", nil)
		model.AddBuildLog(idx, "Resource marked as complete")
	}
}

func (d *Deployment) Ready() {
	config := core.GetConfig()

	// Don't show URL for volume-template deployments
	if core.IsVolumeTemplate(config.Type) {
		core.PrintSuccess("Deployment applied successfully")
		return
	}

	currentWorkspace := core.GetWorkspace()
	appUrl := blaxel.GetAppURL()
	runUrl := blaxel.GetRunURL()
	consoleUrl := fmt.Sprintf("%s/%s/global-agentic-network/%s/%s", appUrl, currentWorkspace, config.Type, d.name)

	core.PrintSuccess("Deployment applied successfully")
	fmt.Println()
	core.PrintInfoWithCommand("Console:", consoleUrl)
	core.PrintInfoWithCommand("Status: ", fmt.Sprintf("bl get %s %s --watch", config.Type, d.name))

	// Show logs hint for resource types that support it
	switch config.Type {
	case "agent", "function", "sandbox", "job":
		core.PrintInfoWithCommand("Logs:   ", fmt.Sprintf("bl logs %s %s", config.Type, d.name))
	}

	// Show run/curl hints for resource types that support it
	switch config.Type {
	case "agent":
		core.PrintInfoWithCommand("Run:    ", fmt.Sprintf("bl run %s %s -d '{\"inputs\": \"Hello\"}'", config.Type, d.name))
		core.PrintInfoWithCommand("Curl:   ", fmt.Sprintf("curl -H \"X-Blaxel-Workspace: %s\" -H \"X-Blaxel-Authorization: $(bl token)\" %s/%s/%s", currentWorkspace, runUrl, config.Type, d.name))
	case "function", "sandbox", "model":
		core.PrintInfoWithCommand("Run:    ", fmt.Sprintf("bl run %s %s", config.Type, d.name))
		core.PrintInfoWithCommand("Curl:   ", fmt.Sprintf("curl -H \"X-Blaxel-Workspace: %s\" -H \"X-Blaxel-Authorization: $(bl token)\" %s/%s/%s", currentWorkspace, runUrl, config.Type, d.name))
	case "job":
		core.PrintInfoWithCommand("Run:    ", fmt.Sprintf("bl run %s %s -f batch.json", config.Type, d.name))
	}

	// Check for callback secret (only for agents, only shown on first deployment)
	if config.Type == "agent" && d.callbackSecret != "" {
		fmt.Println()
		fmt.Printf("  Async Callback Configuration:\n")
		fmt.Printf("  Callback Secret: %s\n", color.New(color.FgGreen).Sprint(d.callbackSecret))
		fmt.Printf("  Use this secret to verify webhook callbacks from Blaxel\n\n")
		core.PrintInfoWithCommand("  Run async:", fmt.Sprintf("bl run agent %s --params async=true -d '{\"inputs\": \"Hello world\"}'", d.name))
	}
}

// progressReader wraps an io.Reader and reports progress
type progressReader struct {
	reader   io.Reader
	total    int64
	read     int64
	callback func(bytesUploaded, totalBytes int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)
	if pr.callback != nil {
		pr.callback(pr.read, pr.total)
	}
	return n, err
}

func (d *Deployment) Upload(url string) error {
	// Open the archive file
	archiveFile, err := os.Open(d.archive.Name())
	if err != nil {
		return fmt.Errorf("failed to open archive file: %w", err)
	}
	defer func() { _ = archiveFile.Close() }()

	// Get the file size
	fileInfo, err := archiveFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Wrap the file reader with progress tracking
	var reader io.Reader = archiveFile
	if d.uploadProgressCallback != nil {
		reader = &progressReader{
			reader:   archiveFile,
			total:    fileInfo.Size(),
			callback: d.uploadProgressCallback,
		}
	}

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", url, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set the content length
	req.ContentLength = fileInfo.Size()

	// Set the content type based on file extension
	config := core.GetConfig()
	if core.IsVolumeTemplate(config.Type) {
		req.Header.Set("Content-Type", "application/x-tar")
	} else {
		req.Header.Set("Content-Type", "application/zip")
	}

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}

func (d *Deployment) IgnoredPaths() []string {
	content, err := os.ReadFile(filepath.Join(d.cwd, ".blaxelignore"))
	if err != nil {
		return []string{
			".blaxel",
			".git",
			"dist",
			".venv",
			"venv",
			"node_modules",
			".env",
			".next",
			"__pycache__",
		}
	}

	// Parse the .blaxelignore file, filtering out comments and empty lines
	lines := strings.Split(string(content), "\n")
	var ignoredPaths []string
	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)
		// Skip empty lines and comments (lines starting with #)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Handle inline comments (e.g., "path #comment")
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
			// Skip if nothing remains after removing inline comment
			if line == "" {
				continue
			}
		}
		ignoredPaths = append(ignoredPaths, line)
	}
	return ignoredPaths
}

func (d *Deployment) shouldIgnorePath(path string, ignoredPaths []string) bool {
	for _, ignoredPath := range ignoredPaths {
		if strings.HasPrefix(path, filepath.Join(d.cwd, ignoredPath)) {
			return true
		}
		if strings.Contains(path, "/"+ignoredPath+"/") {
			return true
		}
		if strings.HasSuffix(path, "/"+ignoredPath) {
			return true
		}
	}
	return false
}

type archiveWriter interface {
	addFile(filePath string, headerName string) error
	close() error
}

type zipArchiveWriter struct {
	writer     *zip.Writer
	deployment *Deployment
}

func (z *zipArchiveWriter) addFile(filePath string, headerName string) error {
	return z.deployment.addFileToZip(z.writer, filePath, headerName)
}

func (z *zipArchiveWriter) close() error {
	return z.writer.Close()
}

type tarArchiveWriter struct {
	writer     *tar.Writer
	deployment *Deployment
}

func (t *tarArchiveWriter) addFile(filePath string, headerName string) error {
	return t.deployment.addFileToTar(t.writer, filePath, headerName)
}

func (t *tarArchiveWriter) close() error {
	return t.writer.Close()
}

func (d *Deployment) createArchive(fileExt string, writer archiveWriter) error {
	config := core.GetConfig()

	// For volume-template, don't apply ignore logic
	var ignoredPaths []string
	if !core.IsVolumeTemplate(config.Type) {
		ignoredPaths = d.IgnoredPaths()
	}

	// Determine the root directory to archive
	archiveRoot := d.cwd
	if core.IsVolumeTemplate(config.Type) {
		// Use the directory from config, default to "." if not specified
		volumeDir := config.Directory
		if volumeDir == "" {
			volumeDir = "."
		}
		archiveRoot = filepath.Join(d.cwd, volumeDir)

		// Validate that the directory exists
		if _, err := os.Stat(archiveRoot); err != nil {
			return fmt.Errorf("volume template directory does not exist: %s", volumeDir)
		}
	}

	// Count total files for progress tracking (only for volume-template)
	var totalFiles int
	var processedFiles int
	if core.IsVolumeTemplate(config.Type) && d.progressCallback != nil {
		_ = filepath.WalkDir(archiveRoot, func(path string, info os.DirEntry, err error) error {
			if err != nil || path == archiveRoot {
				return nil
			}
			totalFiles++
			return nil
		})
	}

	err := filepath.WalkDir(archiveRoot, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only apply ignore logic for non-volume-template types
		if !core.IsVolumeTemplate(config.Type) && d.shouldIgnorePath(path, ignoredPaths) {
			return nil
		}

		if path == archiveRoot {
			return nil
		}

		relPath, err := filepath.Rel(archiveRoot, path)
		if err != nil {
			return err
		}

		err = writer.addFile(path, relPath)
		if err != nil {
			return err
		}

		// Report progress for volume-template
		if core.IsVolumeTemplate(config.Type) && d.progressCallback != nil {
			processedFiles++
			progress := 0
			if totalFiles > 0 {
				progress = (processedFiles * 100) / totalFiles
			}
			d.progressCallback(fmt.Sprintf("Compressing files (%d/%d)", processedFiles, totalFiles), progress)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	if d.folder != "" {
		blaxelTomlPath := filepath.Join(d.cwd, d.folder, "blaxel.toml")
		if err := writer.addFile(blaxelTomlPath, "blaxel.toml"); err != nil {
			return err
		}
		dockerfilePath := filepath.Join(d.cwd, d.folder, "Dockerfile")
		if err := writer.addFile(dockerfilePath, "Dockerfile"); err != nil {
			return err
		}
	}

	return nil
}

func (d *Deployment) Zip() error {
	zipFile, err := os.CreateTemp("", ".blaxel.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	writer := &zipArchiveWriter{writer: zipWriter, deployment: d}
	if err := d.createArchive(".zip", writer); err != nil {
		return err
	}

	d.archive = zipFile
	return nil
}

func (d *Deployment) Tar() error {
	tarFile, err := os.CreateTemp("", ".blaxel.tar")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tarWriter := tar.NewWriter(tarFile)

	writer := &tarArchiveWriter{writer: tarWriter, deployment: d}
	if err := d.createArchive(".tar", writer); err != nil {
		_ = tarWriter.Close()
		_ = tarFile.Close()
		return err
	}

	// Close tar writer to flush all data
	if err := tarWriter.Close(); err != nil {
		_ = tarFile.Close()
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Close the file
	if err := tarFile.Close(); err != nil {
		return fmt.Errorf("failed to close tar file: %w", err)
	}

	d.archive = tarFile
	return nil
}

func (d *Deployment) addFileToZip(zipWriter *zip.Writer, filePath string, headerName string) error {
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", headerName, err)
		}

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return fmt.Errorf("failed to create zip header: %w", err)
		}

		// Set the header name to the specified headerName
		if fileInfo.IsDir() {
			header.Name = headerName + "/" // Add trailing slash for directories
		} else {
			header.Name = headerName
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip writer: %w", err)
		}

		// If it's a file, write its content to the zip
		if !fileInfo.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", headerName, err)
			}
			defer func() { _ = file.Close() }()

			_, err = io.Copy(writer, file)
			if err != nil {
				return fmt.Errorf("failed to copy %s to zip: %w", headerName, err)
			}
		}
	}
	return nil
}

func (d *Deployment) addFileToTar(tarWriter *tar.Writer, filePath string, headerName string) error {
	if _, err := os.Lstat(filePath); err == nil {
		// Use Lstat instead of Stat to not follow symlinks
		fileInfo, err := os.Lstat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", headerName, err)
		}

		// For symlinks, we need to read the link target
		linkTarget := ""
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", headerName, err)
			}
		}

		header, err := tar.FileInfoHeader(fileInfo, linkTarget)
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Set the header name to the specified headerName
		if fileInfo.IsDir() {
			header.Name = headerName + "/" // Add trailing slash for directories
		} else {
			header.Name = headerName
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// If it's a regular file (not a directory or symlink), write its content to the tar
		if !fileInfo.IsDir() && fileInfo.Mode()&os.ModeSymlink == 0 {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", headerName, err)
			}
			defer func() { _ = file.Close() }()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return fmt.Errorf("failed to copy %s to tar: %w", headerName, err)
			}
		}
	}
	return nil
}

func (d *Deployment) Print(skipBuild bool) error {
	for _, deployment := range d.blaxelDeployments {
		fmt.Print(deployment.ToString())
		fmt.Println("---")
	}
	if !skipBuild {
		config := core.GetConfig()
		if core.IsVolumeTemplate(config.Type) {
			// Ensure archive is created before trying to print it
			if d.archive == nil {
				fmt.Println("Compressing volume template files for dry run...")
				err := d.Tar()
				if err != nil {
					return fmt.Errorf("failed to create tar: %w", err)
				}
				fmt.Println("Compression completed")
			}
			err := d.PrintTar()
			if err != nil {
				return fmt.Errorf("failed to print tar: %w", err)
			}
		} else {
			// Ensure archive is created before trying to print it
			if d.archive == nil {
				err := d.Zip()
				if err != nil {
					return fmt.Errorf("failed to create zip: %w", err)
				}
			}
			err := d.PrintZip()
			if err != nil {
				return fmt.Errorf("failed to print zip: %w", err)
			}
		}
	}
	return nil
}

func (d *Deployment) PrintZip() error {
	// Reopen the file to get the reader
	zipFile, err := os.Open(d.archive.Name())
	if err != nil {
		return fmt.Errorf("failed to reopen zip file: %w", err)
	}

	// Get the file size
	fileInfo, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Print the content of the zip file
	zipReader, err := zip.NewReader(zipFile, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	for _, file := range zipReader.File {
		fmt.Printf("File: %s, Size: %d bytes\n", file.Name, file.FileInfo().Size())
	}

	return nil
}

func (d *Deployment) PrintTar() error {
	// Reopen the file to get the reader
	tarFile, err := os.Open(d.archive.Name())
	if err != nil {
		return fmt.Errorf("failed to reopen tar file: %w", err)
	}
	defer func() { _ = tarFile.Close() }()

	// Print the content of the tar file
	tarReader := tar.NewReader(tarFile)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}
		fmt.Printf("File: %s, Size: %d bytes\n", header.Name, header.Size)
	}

	return nil
}

func deployPackage(dryRun bool, name string) bool {
	commands, err := getDeployCommands(dryRun, name)
	if err != nil {
		err = fmt.Errorf("failed to get package commands: %w", err)
		core.PrintError("Deploy", err)
		core.ExitWithError(err)
	}

	if len(commands) == 1 {
		return false
	}

	server.RunCommands(commands, true)
	return true
}

func getDeployCommands(dryRun bool, defaultName string) ([]server.PackageCommand, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %v", err)
	}
	command := server.PackageCommand{
		Name:    "root",
		Cwd:     pwd,
		Command: "bl",
		Args:    []string{"deploy", "--recursive=false", "--skip-version-warning"},
	}
	if dryRun {
		command.Args = append(command.Args, "--dryrun")
	}
	if defaultName != "" {
		command.Args = append(command.Args, "--name", defaultName)
	}
	commands := []server.PackageCommand{}
	config := core.GetConfig()
	if !config.SkipRoot {
		commands = append(commands, command)
	}
	packages := server.GetAllPackages(core.GetConfig())
	for name, pkg := range packages {
		command := server.PackageCommand{
			Name:    name,
			Cwd:     filepath.Join(pwd, pkg.Path),
			Command: "bl",
			Args: []string{
				"deploy",
				"--recursive=false",
				"--skip-version-warning",
			},
		}
		if dryRun {
			command.Args = append(command.Args, "--dryrun")
		}
		for _, envFile := range core.GetEnvFiles() {
			command.Args = append(command.Args, "--env-file", envFile)
		}
		for _, secret := range core.GetSecrets() {
			command.Args = append(command.Args, "-s", fmt.Sprintf("%s=%s", secret.Name, secret.Value))
		}
		commands = append(commands, command)
	}
	return commands, nil
}

// isBlaxelErrorDeploy checks if an error is a blaxel API error and sets the apiErr pointer
func isBlaxelErrorDeploy(err error, apiErr **blaxel.Error) bool {
	if e, ok := err.(*blaxel.Error); ok {
		*apiErr = e
		return true
	}
	return false
}
