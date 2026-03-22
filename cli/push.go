package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	mon "github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("push", func() *cobra.Command {
		return PushCmd()
	})
}

// createImageRequest is the request body for POST /images.
type createImageRequest struct {
	Name         string `json:"name"`
	ResourceType string `json:"resourceType"`
	Generation   string `json:"generation,omitempty"`
}

// createImageResponse is the response body from POST /images.
type createImageResponse struct {
	Message      string `json:"message"`
	Name         string `json:"name"`
	ResourceType string `json:"resourceType"`
}

func PushCmd() *cobra.Command {
	var name string
	var folder string
	var resourceType string
	var noTTY bool

	cmd := &cobra.Command{
		Use:   "push",
		Args:  cobra.ExactArgs(0),
		Short: "Build and push an image to the Blaxel registry",
		Long: `Build and push a container image to the Blaxel registry without creating a deployment.

This command packages your code, uploads it, and builds a container image that
is stored in the workspace registry. Unlike 'bl deploy', this command does NOT
create or update any resource (agent, function, sandbox, or job).

The process includes:
1. Reading configuration from blaxel.toml
2. Packaging source code (respects .blaxelignore)
3. Uploading to Blaxel's build system via presigned URL
4. Building container image
5. Streaming build logs until the image is ready

You must run this command from a directory containing a blaxel.toml file.`,
		Example: `  # Push current directory as an image
  bl push

  # Push with a custom name
  bl push --name my-image

  # Push a specific subdirectory
  bl push -d ./packages/my-agent

  # Push specifying a resource type
  bl push --type agent`,
		Run: func(cmd *cobra.Command, args []string) {
			core.ReadSecrets(folder, []string{".env"})

			// Determine interactive mode
			if !cmd.Flags().Changed("yes") {
				if core.IsTerminalInteractive() && !core.IsCIEnvironment() {
					noTTY = false
				} else {
					noTTY = true
				}
			}

			if folder != "" {
				core.ReadConfigToml(folder, false)
			} else {
				core.ReadConfigToml("", false)
			}

			config := core.GetConfig()

			// Determine resource type
			if resourceType == "" {
				resourceType = config.Type
			}
			if resourceType == "" {
				resourceType = "agent"
			}

			// Determine name
			if name == "" {
				name = config.Name
			}

			cwd, err := os.Getwd()
			if err != nil {
				core.PrintError("Push", fmt.Errorf("failed to get current working directory: %w", err))
				core.ExitWithError(err)
			}

			if name == "" {
				name = filepath.Base(filepath.Join(cwd, folder))
			}
			name = core.Slugify(name)

			// Check for blaxel.toml validation warnings
			blaxelTomlWarning := core.GetBlaxelTomlWarning()
			if blaxelTomlWarning != "" {
				fmt.Println(blaxelTomlWarning)
				core.ClearBlaxelTomlWarning()
			}

			// Determine generation from runtime config
			generation := ""
			if config.Runtime != nil {
				if gen, ok := (*config.Runtime)["generation"]; ok {
					if genStr, ok := gen.(string); ok {
						generation = genStr
					}
				}
			}

			// Create the deployment struct to reuse archive logic
			deployment := Deployment{
				folder: folder,
				name:   name,
				cwd:    cwd,
			}

			// Package the source code
			fmt.Printf("Packaging source code for %s/%s...\n", resourceType, name)
			err = deployment.Zip()
			if err != nil {
				core.PrintError("Push", fmt.Errorf("failed to package source code: %w", err))
				core.ExitWithError(err)
			}

			// Call POST /images to get the presigned URL
			fmt.Println("Requesting image build...")
			client := core.GetClient()
			ctx := context.Background()

			reqBody := createImageRequest{
				Name:         name,
				ResourceType: resourceType,
				Generation:   generation,
			}

			var httpResponse *http.Response
			var respBody createImageResponse
			err = client.Post(ctx, "images", reqBody, &respBody,
				option.WithResponseInto(&httpResponse),
				option.WithQuery("upload", "true"),
			)
			if err != nil {
				core.PrintError("Push", fmt.Errorf("failed to request image build: %w", err))
				core.ExitWithError(err)
			}

			uploadURL := ""
			if httpResponse != nil {
				uploadURL = httpResponse.Header.Get("X-Blaxel-Upload-Url")
			}
			if uploadURL == "" {
				err = fmt.Errorf("no upload URL returned from server")
				core.PrintError("Push", err)
				core.ExitWithError(err)
			}

			// Upload the archive to the presigned URL
			fmt.Println("Uploading source code...")
			err = deployment.Upload(uploadURL)
			if err != nil {
				core.PrintError("Push", fmt.Errorf("failed to upload source code: %w", err))
				core.ExitWithError(err)
			}
			fmt.Println("Upload completed")

			// Monitor build logs
			if noTTY {
				err = watchBuildLogsNonInteractive(resourceType, name)
			} else {
				err = watchBuildLogsNonInteractive(resourceType, name)
			}
			if err != nil {
				core.PrintError("Push", err)
				core.ExitWithError(err)
			}
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name for the image (defaults to directory name)")
	cmd.Flags().StringVarP(&folder, "directory", "d", "", "Source directory path")
	cmd.Flags().StringVarP(&resourceType, "type", "t", "", "Resource type (agent, function, sandbox, job). Defaults to blaxel.toml type or 'agent'")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive mode")

	return cmd
}

// watchBuildLogsNonInteractive monitors the build logs until the build succeeds or fails.
func watchBuildLogsNonInteractive(resourceType, name string) error {
	client := core.GetClient()
	workspace := core.GetWorkspace()

	fmt.Println("\nMonitoring build logs...")

	// Set up signal handling for graceful cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println("\nBuild monitoring cancelled")
		cancel()
	}()

	// Use the BuildLogWatcher to stream logs
	logWatcher := mon.NewBuildLogWatcher(client, workspace, resourceType, name, func(msg string) {
		fmt.Println(msg)
	})
	logWatcher.Start()
	defer logWatcher.Stop()

	// Poll resource events for build completion
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(15 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("build monitoring cancelled")
		case <-timeout:
			return fmt.Errorf("build timed out after 15 minutes")
		case <-ticker.C:
			// Check if the image exists in the registry (build completed)
			status, err := getImageBuildStatus(resourceType, name)
			if err != nil {
				// Image not found yet, continue waiting
				continue
			}
			if status == "succeeded" {
				logWatcher.Stop()
				time.Sleep(1 * time.Second) // Allow final logs to flush
				printPushSuccess(resourceType, name)
				return nil
			}
			if status == "failed" {
				logWatcher.Stop()
				time.Sleep(1 * time.Second)
				return fmt.Errorf("image build failed")
			}
		}
	}
}

// getImageBuildStatus checks the build status by looking for recent events on the image.
// Returns "succeeded" if the image exists (was built successfully), "failed" if there was
// an error, or empty string if the image is not yet available.
func getImageBuildStatus(resourceType, name string) (string, error) {
	ctx := context.Background()
	client := core.GetClient()

	// Try to get the image - if it exists, the build succeeded
	var result interface{}
	err := client.Get(ctx, fmt.Sprintf("images/%s/%s", resourceType, name), nil, &result)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
			return "", nil // Not found yet, build may still be in progress
		}
		return "", err
	}

	return "succeeded", nil
}

func printPushSuccess(resourceType, name string) {
	fmt.Printf("\nImage %s/%s built and pushed successfully!\n", resourceType, name)
	fmt.Println()
	core.PrintInfoWithCommand("List images:", "bl get images")
	core.PrintInfoWithCommand("Image detail:", fmt.Sprintf("bl get image %s --type %s", name, resourceType))
}
