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

	"github.com/atotto/clipboard"
	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	mon "github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
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
	Image        string `json:"image,omitempty"`
}

// createImageResponse is the response body from POST /images.
type createImageResponse struct {
	Message      string `json:"message"`
	Name         string `json:"name"`
	ResourceType string `json:"resourceType"`
	Image        string `json:"image,omitempty"`
}

func PushCmd() *cobra.Command {
	var name string
	var folder string
	var resourceType string
	var noTTY bool
	var registryCreds []string
	var dockerConfigPath string
	var timeoutStr string

	cmd := &cobra.Command{
		Use:   "push",
		Args:  cobra.ExactArgs(0),
		Short: "Build and push a container image to the Blaxel registry",
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
  bl push --type agent

  # Push with a longer timeout for large images
  bl push --timeout 30m`,
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
				if noTTY {
					core.PrintError("Push", fmt.Errorf("resource type is required. Specify it with --type (-t) flag or set 'type' in blaxel.toml"))
					core.ExitWithError(fmt.Errorf("resource type is required"))
				}
				// Interactive prompt for resource type
				var selected string
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("What type of image are you building?").
							Options(
								huh.NewOption("Sandbox", "sandbox"),
								huh.NewOption("Agent", "agent"),
								huh.NewOption("Job", "job"),
								huh.NewOption("MCP server", "function"),
							).
							Value(&selected),
					),
				)
				form.WithTheme(core.GetHuhTheme())
				if err := form.Run(); err != nil {
					core.ExitWithError(err)
				}
				resourceType = selected
			}

			// Validate resource type
			validTypes := map[string]bool{"agent": true, "function": true, "sandbox": true, "job": true}
			if !validTypes[resourceType] {
				core.PrintError("Push", fmt.Errorf("invalid resource type %q: must be one of sandbox, agent, job, function", resourceType))
				core.ExitWithError(fmt.Errorf("invalid resource type"))
			}

			// Parse timeout early to fail fast before expensive upload
			buildTimeout := mon.DefaultBuildTimeout
			if timeoutStr != "" {
				parsed, parseErr := time.ParseDuration(timeoutStr)
				if parseErr != nil {
					core.PrintError("Push", fmt.Errorf("invalid timeout value %q: %w (use format like 30m, 1h)", timeoutStr, parseErr))
					core.ExitWithError(parseErr)
				}
				if parsed <= 0 {
					core.PrintError("Push", fmt.Errorf("timeout must be a positive duration, got %q", timeoutStr))
					core.ExitWithError(fmt.Errorf("invalid timeout"))
				}
				buildTimeout = parsed
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

			// Validate build configuration (Dockerfile, sandbox-api, entrypoint, etc.)
			config.Type = resourceType
			validationWarning := ValidateBuildConfig(cwd, folder, config)
			if validationWarning != "" {
				handleConfigWarning(validationWarning, noTTY)
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

			// Check if a pre-built image is specified in blaxel.toml
			image := config.Image

			if image != "" {
				// Direct image flow: skip packaging and upload, register the image directly
				fmt.Printf("Registering image %s for %s...\n", image, imageRef(resourceType, name))
				client := core.GetClient()
				ctx := context.Background()

				reqBody := createImageRequest{
					Name:         name,
					ResourceType: resourceType,
					Generation:   generation,
					Image:        image,
				}

				var respBody createImageResponse
				err = client.Post(ctx, "images", reqBody, &respBody)
				if err != nil {
					core.PrintError("Push", fmt.Errorf("failed to register image: %w", err))
					core.ExitWithError(err)
				}

				printPushSuccess(resourceType, name, noTTY)
			} else {
				// Standard flow: package source code and upload
				projectDir := filepath.Join(cwd, folder)
				dockerConfigJSON, dockerErr := core.ResolveDockerConfig(projectDir, registryCreds, dockerConfigPath)
				if dockerErr != nil {
					core.PrintError("Push", fmt.Errorf("failed to resolve Docker registry credentials: %w", dockerErr))
					core.ExitWithError(dockerErr)
				}

				deployment := Deployment{
					folder:           folder,
					name:             name,
					cwd:              cwd,
					dockerConfigJSON: dockerConfigJSON,
				}

				fmt.Printf("Packaging source code for %s...\n", imageRef(resourceType, name))
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
				err = deployment.UploadWithRetry(uploadURL, func() (string, error) {
					var retryResp *http.Response
					var retryBody createImageResponse
					retryErr := client.Post(ctx, "images", reqBody, &retryBody,
						option.WithResponseInto(&retryResp),
						option.WithQuery("upload", "true"),
					)
					if retryErr != nil {
						return "", retryErr
					}
					if retryResp != nil {
						if u := retryResp.Header.Get("X-Blaxel-Upload-Url"); u != "" {
							return u, nil
						}
					}
					return "", fmt.Errorf("no upload URL returned from server")
				})
				if err != nil {
					core.PrintError("Push", fmt.Errorf("failed to upload source code: %w", err))
					core.ExitWithError(err)
				}
				fmt.Println("Upload completed")

				// Monitor build logs
				err = watchBuildLogsNonInteractive(resourceType, name, noTTY, buildTimeout)
				if err != nil {
					core.PrintError("Push", err)
					core.ExitWithError(err)
				}
			}
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Name for the image (defaults to directory name)")
	cmd.Flags().StringVarP(&folder, "directory", "d", "", "Source directory path")
	cmd.Flags().StringVarP(&resourceType, "type", "t", "", "Resource type (agent, function, sandbox, job). Defaults to blaxel.toml type; required if not set")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive mode")
	cmd.Flags().StringArrayVarP(&registryCreds, "registry-cred", "c", []string{}, "Registry credentials (format: registry=username:password, repeatable)")
	cmd.Flags().StringVar(&dockerConfigPath, "docker-config", "", "Path to a Docker config.json file with registry credentials")
	cmd.Flags().StringVar(&timeoutStr, "timeout", "", "Timeout for build log monitoring (e.g. 30m, 1h). Defaults to 15m")

	return cmd
}

// watchBuildLogsNonInteractive monitors the build logs until the build succeeds or fails.
func watchBuildLogsNonInteractive(resourceType, name string, noTTY bool, buildTimeout time.Duration) error {
	client := core.GetClient()
	workspace := core.GetWorkspace()

	fmt.Println("\nMonitoring build logs (first logs may take up to 30 seconds to appear)...")

	// Set up signal handling for graceful cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	doneCh := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			fmt.Println("\nBuild monitoring cancelled")
			cancel()
		case <-doneCh:
		}
	}()
	defer func() {
		signal.Stop(sigCh)
		close(doneCh)
	}()

	// Use the BuildLogWatcher to stream logs
	logWatcher := mon.NewBuildLogWatcher(client, workspace, resourceType, name, func(msg string) {
		fmt.Println(msg)
	}, buildTimeout)
	logWatcher.Start()
	defer logWatcher.Stop()

	// Poll resource events for build completion
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(buildTimeout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("build monitoring cancelled")
		case <-timeout:
			return fmt.Errorf("build timed out after %s", buildTimeout)
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
				printPushSuccess(resourceType, name, noTTY)
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

// imageAPIResponse represents the API response for GET /images/{resourceType}/{imageName}.
type imageAPIResponse struct {
	Metadata struct {
		Name         string `json:"name"`
		ResourceType string `json:"resourceType"`
		Status       string `json:"status"`
	} `json:"metadata"`
}

// getImageBuildStatus checks the build status by querying the image API.
// Returns "succeeded" if the image is built, "failed" if the build failed,
// or empty string if the build is still in progress.
func getImageBuildStatus(resourceType, name string) (string, error) {
	ctx := context.Background()
	client := core.GetClient()

	var result imageAPIResponse
	path := fmt.Sprintf("images/%s/%s", resourceType, name)
	if resourceType == "" {
		path = fmt.Sprintf("images/_/%s", name)
	}
	err := client.Get(ctx, path, nil, &result)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
			return "", nil // Not found yet, build may still be in progress
		}
		return "", err
	}

	switch result.Metadata.Status {
	case "BUILT":
		return "succeeded", nil
	case "FAILED":
		return "failed", nil
	default:
		return "", nil // Still building (UPLOADING, BUILDING, or no status)
	}
}

func imageRef(resourceType, name string) string {
	if resourceType != "" {
		return resourceType + "/" + name
	}
	return name
}

func printPushSuccess(resourceType, name string, noTTY bool) {
	fmt.Printf("\nImage %s built and pushed successfully!\n", imageRef(resourceType, name))
	fmt.Println()
	core.PrintInfoWithCommand("List images:", "bl get images")
	core.PrintInfoWithCommand("Image detail:", fmt.Sprintf("bl get image %s", imageRef(resourceType, name)))
	fmt.Println()

	if noTTY {
		printDeploySampleCLI(resourceType, name)
	} else {
		printDeploySamplesInteractive(resourceType, name)
	}
}

func resourceKind(resourceType string) string {
	switch resourceType {
	case "agent":
		return "Agent"
	case "function":
		return "Function"
	case "sandbox":
		return "Sandbox"
	case "job":
		return "Job"
	default:
		return "Agent"
	}
}

var sampleLanguages = []string{"TypeScript", "Python", "Go", "CLI", "curl"}

func getSandboxSamplesMap(name string) map[string]string {
	imageTag := fmt.Sprintf("sandbox/%s:latest", name)
	instanceName := fmt.Sprintf("%s-%s", name, core.RandomString(5))
	baseUrl := blaxel.GetBaseURL()
	tsSample := fmt.Sprintf(`import { SandboxInstance } from "@blaxel/core";

const sandbox = await SandboxInstance.createIfNotExists({
  name: "%s",
  image: "%s",
  memory: 4096,
});`, instanceName, imageTag)

	pySample := fmt.Sprintf(`import asyncio
from blaxel.core import SandboxInstance

async def main():
    sandbox = await SandboxInstance.create_if_not_exists({
        "name": "%s",
        "image": "%s",
        "memory": 4096,
    })

asyncio.run(main())`, instanceName, imageTag)

	goSample := fmt.Sprintf(`package main

import (
	"context"
	"log"

	blaxel "github.com/blaxel-ai/sdk-go"
)

func main() {
	client, err := blaxel.NewDefaultClient()
	if err != nil {
		log.Fatal(err)
	}

	sandbox, err := client.Sandboxes.New(context.Background(), blaxel.SandboxNewParams{
		Metadata: blaxel.SandboxNewParamsMetadata{
			Name: blaxel.String("%s"),
		},
		Spec: blaxel.SandboxNewParamsSpec{
			Runtime: blaxel.SandboxNewParamsSpecRuntime{
				Image:  blaxel.String("%s"),
				Memory: blaxel.Int(4096),
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created sandbox: %%s", sandbox.Metadata.Name)
}`, instanceName, imageTag)

	cliSample := fmt.Sprintf(`bl apply -f - <<EOF
apiVersion: blaxel.ai/v1alpha1
kind: Sandbox
metadata:
  name: %s
spec:
  runtime:
    image: %s
    memory: 4096
EOF`, instanceName, imageTag)

	curlSample := fmt.Sprintf(`curl -X POST "%s/sandboxes" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(bl token)" \
	-H "X-Blaxel-Workspace: $(bl workspace --current)" \
  -d '{
    "metadata": {
      "name": "%s"
    },
    "spec": {
      "runtime": {
        "image": "%s",
        "memory": 4096
      }
    }
  }'`, baseUrl, instanceName, imageTag)

	return map[string]string{
		"TypeScript": tsSample,
		"Python":     pySample,
		"Go":         goSample,
		"CLI":        cliSample,
		"curl":       curlSample,
	}
}

func getResourceSamples(resourceType, name string) map[string]string {
	kind := resourceKind(resourceType)
	workspace := core.GetWorkspace()
	baseUrl := blaxel.GetBaseURL()
	imageTag := fmt.Sprintf("%s/%s:latest", resourceType, name)

	tsSample := fmt.Sprintf(`import Blaxel from "@blaxel/sdk";

const client = new Blaxel();

const result = await client.%ss.update("%s", {
  spec: {
    runtime: {
      image: "%s",
    },
  },
});

console.log("Deployed:", result.metadata?.name);`, resourceType, name, imageTag)

	pySample := fmt.Sprintf(`from blaxel.client import BlaxelClient

client = BlaxelClient()

result = client.%ss.update_%s("%s", body={
    "spec": {
        "runtime": {
            "image": "%s",
        },
    },
})

print("Deployed:", result.metadata.name)`, resourceType, resourceType, name, imageTag)

	goSample := fmt.Sprintf(`package main

import (
	"context"
	"log"

	blaxel "github.com/blaxel-ai/sdk-go"
)

func main() {
	client, err := blaxel.NewDefaultClient()
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.%ss.Update(
		context.Background(),
		"%s",
		blaxel.%sUpdateParams{
			%s: blaxel.%sParam{
				Spec: blaxel.%sSpecParam{
					Runtime: blaxel.%sRuntimeParam{
						Image: blaxel.String("%s"),
					},
				},
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Deployed: %%s", result.Metadata.Name)
}`, kind, name, kind, kind, kind, kind, kind, imageTag)

	cliSample := fmt.Sprintf(`bl apply -f  - <<EOF
apiVersion: blaxel.ai/v1alpha1
kind: %s
metadata:
  name: %s
spec:
  runtime:
    image: %s
EOF`, kind, name, imageTag)

	curlSample := fmt.Sprintf(`curl -X PUT "%s/%ss/%s" \
  -H "Authorization: Bearer $(bl token)" \
	-H "X-Blaxel-Workspace: %s" \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": { "name": "%s" },
    "spec": {
      "runtime": {
        "image": "%s"
      }
    }
  }'`, baseUrl, resourceType, name, workspace, name, imageTag)

	return map[string]string{
		"TypeScript": tsSample,
		"Python":     pySample,
		"Go":         goSample,
		"CLI":        cliSample,
		"curl":       curlSample,
	}
}

// renderCodeBlock returns a styled code block string for display
func renderCodeBlock(code string) string {
	codeColor := color.New(color.FgHiWhite)
	border := color.New(color.FgHiBlack)

	var b strings.Builder
	b.WriteString(border.Sprint("  ┌─────────────────────────────────────────────────────────") + "\n")
	for _, line := range strings.Split(code, "\n") {
		b.WriteString(fmt.Sprintf("  %s %s\n", border.Sprint("│"), codeColor.Sprint(line)))
	}
	b.WriteString(border.Sprint("  └─────────────────────────────────────────────────────────"))
	return b.String()
}

func printDeploySampleCLI(resourceType, name string) {
	var samples map[string]string
	if resourceType == "sandbox" {
		samples = getSandboxSamplesMap(name)
	} else {
		samples = getResourceSamples(resourceType, name)
	}
	fmt.Println("Deploy this image with:")
	fmt.Println()
	fmt.Println(samples["CLI"])
}

// codeSampleModel is the bubbletea model for the interactive code sample viewer.
// Layout: title + tabs + help at top (fixed), code block at bottom (scrollable).
type codeSampleModel struct {
	samples    map[string]string
	languages  []string
	activeIdx  int
	copied     bool
	copyFadeAt time.Time
	width      int
	height     int
	viewport   viewport.Model
	ready      bool
}

type tickMsg time.Time

func (m codeSampleModel) headerView() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	b.WriteString(titleStyle.Render("Deploy this image:"))
	b.WriteString("\n\n")

	// Tab bar
	activeTab := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("#fd7b35")).
		Padding(0, 1)

	inactiveTab := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	for i, lang := range m.languages {
		if i == m.activeIdx {
			b.WriteString(activeTab.Render(lang))
		} else {
			b.WriteString(inactiveTab.Render(lang))
		}
		if i < len(m.languages)-1 {
			b.WriteString(" ")
		}
	}

	// Help / copy status
	b.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if m.copied {
		copiedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		b.WriteString(copiedStyle.Render("  Copied to clipboard!"))
		b.WriteString(helpStyle.Render("  ←/→ switch  esc quit"))
	} else {
		b.WriteString(helpStyle.Render("  ←/→ switch  c copy  esc quit"))
	}
	b.WriteString("\n\n")

	return b.String()
}

func (m *codeSampleModel) updateViewport() {
	activeLang := m.languages[m.activeIdx]
	m.viewport.SetContent(renderCodeBlock(m.samples[activeLang]))
	m.viewport.GotoTop()
}

func (m codeSampleModel) Init() tea.Cmd {
	return nil
}

func (m codeSampleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := lipgloss.Height(m.headerView())
		viewportHeight := m.height - headerHeight - 1
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		if !m.ready {
			m.viewport = viewport.New(m.width, viewportHeight)
			m.updateViewport()
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = viewportHeight
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, tea.Quit
		case "left", "h":
			if m.activeIdx > 0 {
				m.activeIdx--
				m.copied = false
				m.updateViewport()
			}
		case "right", "l":
			if m.activeIdx < len(m.languages)-1 {
				m.activeIdx++
				m.copied = false
				m.updateViewport()
			}
		case "tab":
			m.activeIdx = (m.activeIdx + 1) % len(m.languages)
			m.copied = false
			m.updateViewport()
		case "shift+tab":
			m.activeIdx = (m.activeIdx - 1 + len(m.languages)) % len(m.languages)
			m.copied = false
			m.updateViewport()
		case "1":
			m.activeIdx = 0
			m.copied = false
			m.updateViewport()
		case "2":
			if len(m.languages) > 1 {
				m.activeIdx = 1
				m.copied = false
				m.updateViewport()
			}
		case "3":
			if len(m.languages) > 2 {
				m.activeIdx = 2
				m.copied = false
				m.updateViewport()
			}
		case "4":
			if len(m.languages) > 3 {
				m.activeIdx = 3
				m.copied = false
				m.updateViewport()
			}
		case "5":
			if len(m.languages) > 4 {
				m.activeIdx = 4
				m.copied = false
				m.updateViewport()
			}
		case "enter", "c":
			lang := m.languages[m.activeIdx]
			if err := clipboard.WriteAll(m.samples[lang]); err == nil {
				m.copied = true
				m.copyFadeAt = time.Now().Add(2 * time.Second)
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return tickMsg(t)
				})
			}
		default:
			// Pass through to viewport for up/down/pgup/pgdown scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tickMsg:
		if time.Now().After(m.copyFadeAt) {
			m.copied = false
		}
	}
	return m, nil
}

func (m codeSampleModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	return m.headerView() + m.viewport.View()
}

func printDeploySamplesInteractive(resourceType, name string) {
	var samples map[string]string
	if resourceType == "sandbox" {
		samples = getSandboxSamplesMap(name)
	} else {
		samples = getResourceSamples(resourceType, name)
	}

	m := codeSampleModel{
		samples:   samples,
		languages: sampleLanguages,
		activeIdx: 0, // TypeScript by default
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		// Fallback to non-interactive
		printDeploySampleCLI(resourceType, name)
	}
}
