package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"archive/tar"
	"archive/zip"
	"net/http"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/deploy"
	mon "github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/blaxel-ai/toolkit/cli/server"
	tea "github.com/charmbracelet/bubbletea"
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
			if folder != "" {
				recursive = false
				core.ReadSecrets("", envFiles)
				core.ReadConfigToml(folder)
			}
			core.SetInteractiveMode(!noTTY)
			if recursive {
				if deployPackage(dryRun, name) {
					return
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				core.PrintError("Deploy", fmt.Errorf("failed to get current working directory: %w", err))
				os.Exit(1)
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

			err = deployment.Generate(skipBuild)
			if err != nil {
				core.PrintError("Deploy", fmt.Errorf("error generating blaxel deployment: %w", err))
				os.Exit(1)
			}

			if dryRun {
				err := deployment.Print(skipBuild)
				if err != nil {
					core.PrintError("Deploy", fmt.Errorf("error printing blaxel deployment: %w", err))
					os.Exit(1)
				}
				return
			}

			if !noTTY {
				err = deployment.ApplyInteractive()
			} else {
				err = deployment.Apply()
			}
			if err != nil {
				core.PrintError("Deploy", fmt.Errorf("error applying blaxel deployment: %w", err))
				os.Exit(1)
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
	dir               string
	name              string
	folder            string
	blaxelDeployments []core.Result
	archive           *os.File
	cwd               string
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
			err = d.Tar()
			if err != nil {
				return fmt.Errorf("failed to tar file: %w", err)
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

func (d *Deployment) GenerateDeployment(skipBuild bool) core.Result {
	var Spec map[string]interface{}
	var Kind string

	runtime := make(map[string]interface{})
	config := core.GetConfig()
	if config.Runtime != nil {
		runtime = *config.Runtime
	}
	if config.Transport == "" {
		config.Transport = "websocket"
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
			os.Exit(1)
		}

		if spec, ok := resource["spec"].(map[string]interface{}); ok {
			if rt, ok := spec["runtime"].(map[string]interface{}); ok {
				if image, ok := rt["image"].(string); ok && image != "" {
					runtime["image"] = image
				} else {
					core.PrintError("Deployment", fmt.Errorf("no image found for %s. please deploy with a build first", d.name))
					os.Exit(1)
				}
			}
		}
	}

	switch config.Type {
	case "function":
		Kind = "Function"
		Spec = map[string]interface{}{
			"runtime":   runtime,
			"triggers":  config.Triggers,
			"transport": config.Transport,
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

	var body []byte
	var statusCode int
	var err error

	switch resourceType {
	case "agent":
		resp, errGet := client.GetAgentWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "function":
		resp, errGet := client.GetFunctionWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "job":
		resp, errGet := client.GetJobWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "sandbox":
		resp, errGet := client.GetSandboxWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "volume-template", "volumetemplate", "vt":
		resp, errGet := client.GetVolumeTemplateWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s %s not found. please deploy with a build first", resourceType, name)
	}

	if statusCode >= 400 {
		return nil, fmt.Errorf("error getting %s %s: %d", resourceType, name, statusCode)
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(body, &resource); err != nil {
		return nil, err
	}

	return resource, nil
}

func getResourceStatus(resourceType, name string) (string, error) {
	ctx := context.Background()
	client := core.GetClient()

	var body []byte
	var statusCode int
	var err error

	switch resourceType {
	case "agent":
		resp, errGet := client.GetAgentWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "function":
		resp, errGet := client.GetFunctionWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "job":
		resp, errGet := client.GetJobWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "sandbox":
		resp, errGet := client.GetSandboxWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	case "volume-template", "volumetemplate", "vt":
		resp, errGet := client.GetVolumeTemplateWithResponse(ctx, name)
		if errGet != nil {
			err = errGet
		} else {
			body = resp.Body
			statusCode = resp.StatusCode()
		}
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		return "", err
	}

	if statusCode >= 400 {
		return "", fmt.Errorf("error getting %s %s: %d", resourceType, name, statusCode)
	}

	var resource map[string]interface{}
	if err := json.Unmarshal(body, &resource); err != nil {
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

	for _, result := range applyResults {
		if result.Result.UploadURL != "" {
			err := d.Upload(result.Result.UploadURL)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}
		}
	}

	return nil
}

func (d *Deployment) ApplyInteractive() error {
	// Create resources for interactive UI
	resources := make([]*deploy.Resource, 0)

	// Add main deployment resources
	for i, deployment := range d.blaxelDeployments {
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
		_ = i // unused variable fix
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

	// Run deployment in background
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
	// Determine where main resources end and additional resources begin
	mainResourceCount := len(resources) - len(additionalResources)

	// Start all deployments in parallel
	var wg sync.WaitGroup

	// Deploy additional resources
	for i := mainResourceCount; i < len(resources); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

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
			d.deployResourceInteractive(resources[idx], model, idx, depl)
		}(i, d.blaxelDeployments[i])
	}

	wg.Wait()
	model.Complete()
}

func (d *Deployment) deployResourceInteractive(resource *deploy.Resource, model *deploy.InteractiveModel, idx int, deployment core.Result) {
	// Start deployment
	model.UpdateResource(idx, deploy.StatusDeploying, "Applying resource", nil)
	model.AddBuildLog(idx, fmt.Sprintf("Starting deployment of %s/%s", resource.Kind, resource.Name))

	// mock mode removed

	// Real deployment
	applyResults, err := ApplyResources([]core.Result{deployment})
	if err != nil {
		model.UpdateResource(idx, deploy.StatusFailed, "Failed to apply", err)
		model.AddBuildLog(idx, fmt.Sprintf("Failed to apply resource: %v", err))
		return
	}

	// Handle upload if there's an upload URL
	if len(applyResults) > 0 && applyResults[0].Result.UploadURL != "" {
		model.UpdateResource(idx, deploy.StatusUploading, "Uploading code", nil)
		model.AddBuildLog(idx, "Uploading code to registry...")

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

		// Start monitoring the resource status
		statusTicker := time.NewTicker(3 * time.Second)
		defer statusTicker.Stop()
		statusTimeout := time.After(15 * time.Minute) // 15 minute timeout for deployment

		var logWatcher interface{ Stop() }
		buildLogStarted := false
		lastStatus := ""           // Track last status to avoid duplicate logs
		sawBuildingStatus := false // Track if we've seen BUILDING status

		for {
			select {
			case <-statusTimeout:
				if logWatcher != nil {
					logWatcher.Stop()
				}
				model.UpdateResource(idx, deploy.StatusFailed, "Deployment timeout", fmt.Errorf("deployment timed out after 15 minutes"))
				return
			case <-statusTicker.C:
				status, err := getResourceStatus(strings.ToLower(resource.Kind), resource.Name)
				if err != nil {
					// Continue polling on temporary errors
					continue
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
		// For resources that don't need monitoring (Model, Policy, etc.), just mark as complete
		model.UpdateResource(idx, deploy.StatusComplete, "Applied successfully", nil)
		model.AddBuildLog(idx, "Resource applied successfully")
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
					_, err := ApplyResources([]core.Result{result})
					if err != nil {
						model.UpdateResource(idx, deploy.StatusFailed, "Failed to apply", err)
						model.AddBuildLog(idx, fmt.Sprintf("Failed to apply resource: %v", err))
						return
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
	appUrl := core.GetAppURL()
	availableAt := fmt.Sprintf("It is available at: %s/%s/global-agentic-network/%s/%s", appUrl, currentWorkspace, config.Type, d.name)
	core.PrintSuccess(fmt.Sprintf("Deployment applied successfully\n%s", availableAt))
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

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", url, archiveFile)
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
	return strings.Split(string(content), "\n")
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

	err := filepath.Walk(archiveRoot, func(path string, info os.FileInfo, err error) error {
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

		return writer.addFile(path, relPath)
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
	defer func() { _ = tarFile.Close() }()

	tarWriter := tar.NewWriter(tarFile)
	defer func() { _ = tarWriter.Close() }()

	writer := &tarArchiveWriter{writer: tarWriter, deployment: d}
	if err := d.createArchive(".tar", writer); err != nil {
		return err
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
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", headerName, err)
		}

		header, err := tar.FileInfoHeader(fileInfo, "")
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

		// If it's a file, write its content to the tar
		if !fileInfo.IsDir() {
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
			err := d.PrintTar()
			if err != nil {
				return fmt.Errorf("failed to print tar: %w", err)
			}
		} else {
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
		core.PrintError("Deploy", fmt.Errorf("failed to get package commands: %w", err))
		os.Exit(1)
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
