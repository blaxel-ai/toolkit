package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"

	"github.com/beamlit/toolkit/sdk"
	"github.com/spf13/cobra"
)

func executePythonGenerateBeamlitDeployment(tempDir string, module string, directory string) error {
	pythonCode := fmt.Sprintf(`
from beamlit.deploy import generate_beamlit_deployment
generate_beamlit_deployment("%s")
	`, tempDir)
	pythonCmd := "python"
	if _, err := os.Stat(".venv"); !os.IsNotExist(err) {
		pythonCmd = ".venv/bin/python"
	}
	cmd := exec.Command(pythonCmd, "-c", pythonCode)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, fmt.Sprintf("BL_SERVER_MODULE=%s", module))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BL_SERVER_DIRECTORY=%s", directory))
	if os.Getenv("BL_ENV") != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BL_ENV=%s", os.Getenv("BL_ENV")))
	}
	return cmd.Run()
}

func dockerLogin(registryURL string, apiUrl string) error {
	credentials := sdk.LoadCredentials(workspace)

	var password string
	if credentials.APIKey != "" {
		password = credentials.APIKey
	} else if credentials.AccessToken != "" {
		provider := sdk.NewBearerTokenProvider(credentials, workspace, apiUrl)
		err := provider.RefreshIfNeeded()
		if err != nil {
			return fmt.Errorf("failed to refresh credentials: %w", err)
		}
		password = provider.GetCredentials().AccessToken
	} else {
		return fmt.Errorf("no credentials found")
	}
	cmd := exec.Command(
		"docker",
		"login",
		"-u", "beamlit",
		"--password-stdin",
		registryURL,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		fmt.Printf("Could not login to beamlit registry (%s): %v", registryURL, err)
		return err
	}

	_, err = stdin.Write([]byte(password))
	if err != nil {
		return err
	}

	stdin.Close()

	return cmd.Wait()
}

func buildBeamlitDeployment(dockerfile string, destination string) error {
	fmt.Printf("Building beamlit deployment from %s to %s\n", dockerfile, destination)
	cmd := exec.Command(
		"docker",
		"build",
		"-t",
		destination,
		"--platform",
		"linux/amd64",
		"-f",
		dockerfile,
		".",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Create a channel to catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Create a channel to receive command completion status
	done := make(chan error)

	// Run the command in a goroutine
	go func() {
		done <- cmd.Run()
	}()

	// Wait for command completion
	if err := <-done; err != nil {
		fmt.Printf("Error building beamlit deployment: %v\n", err)
		return err
	}
	fmt.Printf("Beamlit deployment from %s built successfully\n", dockerfile)
	return nil
}

func pushBeamlitDeployment(destination string) error {
	fmt.Printf("Pushing beamlit deployment to %s\n", destination)
	cmd := exec.Command(
		"docker",
		"push",
		destination,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Create a channel to catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Create a channel to receive command completion status
	done := make(chan error)

	// Run the command in a goroutine
	go func() {
		done <- cmd.Run()
	}()

	// Wait for command completion
	if err := <-done; err != nil {
		fmt.Printf("Error pushing beamlit deployment: %v\n", err)
		return err
	}
	fmt.Printf("Beamlit deployment pushed successfully: %s\n", destination)
	return nil
}

func (r *Operations) handleDeploymentFile(tempDir string, agents *[]string, applyResults *[]ApplyResult, path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	// Skip directories
	if info.IsDir() {
		return nil
	}

	isAgent := strings.Contains(path, "agents/")
	isFunction := strings.Contains(path, "functions/")
	resourceType := "agent"
	if isFunction {
		resourceType = "function"
	}
	// Get relative path from tempDir
	relPath, err := filepath.Rel(tempDir, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}
	name := strings.Split(relPath, "/")[1]
	if isAgent {
		if !slices.Contains(*agents, name) {
			*agents = append(*agents, name)
		}
	}

	// Check if file is a Dockerfile
	if filepath.Base(path) == "Dockerfile" {
		// Build the Docker image
		// Read destination from destination.txt
		destinationFile := filepath.Join(filepath.Dir(path), "destination.txt")
		destinationBytes, err := os.ReadFile(destinationFile)
		if err != nil {
			return fmt.Errorf("failed to read destination file: %w", err)
		}
		destination := strings.TrimSpace(string(destinationBytes))
		fmt.Printf("Building Docker image for %s at %s\n", name, destination)
		err = buildBeamlitDeployment(path, destination)
		if err != nil {
			return fmt.Errorf("failed to build Docker image: %w", err)
		}
		fmt.Printf("Pushing Docker image for %s at %s\n", name, destination)
		err = pushBeamlitDeployment(destination)
		if err != nil {
			return fmt.Errorf("failed to push Docker image: %w", err)
		}
	}
	if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
		fmt.Printf("Applying configuration for %s:%s -> file: %s\n", resourceType, name, filepath.Base(path))
		results, err := r.Apply(path, false)
		if err != nil {
			return fmt.Errorf("failed to apply configuration: %w", err)
		}
		*applyResults = append(*applyResults, results...)
	}
	return nil
}

func (r *Operations) DeployAgentAppCmd() *cobra.Command {
	var module string
	var directory string
	cmd := &cobra.Command{
		Use:     "deploy",
		Args:    cobra.ExactArgs(0),
		Aliases: []string{"d", "dp"},
		Short:   "Deploy a beamlit agent app",
		Long:    "Deploy a beamlit agent app, you must be in a beamlit agent app directory.",
		Example: `bl deploy`,
		Run: func(cmd *cobra.Command, args []string) {

			// Create a temporary directory for deployment files
			tempDir := ".beamlit"

			err := dockerLogin(r.GetRegistryURL(), r.BaseURL)
			if err != nil {
				fmt.Printf("Could not login to beamlit registry (%s): %v\n", r.GetRegistryURL(), err)
				os.Exit(1)
			}

			// Execute Python script using the Python interpreter
			err = executePythonGenerateBeamlitDeployment(tempDir, module, directory)
			if err != nil {
				fmt.Printf("Error executing Python script: %v\n", err)
				os.Exit(1)
			}

			agents := []string{}
			applyResults := []ApplyResult{}

			// Walk through the temporary directory recursively, we deploy everything except agents
			err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !strings.Contains(path, "agents/") {
					return r.handleDeploymentFile(tempDir, &agents, &applyResults, path, info, err)
				}
				return nil
			})
			if err != nil {
				fmt.Printf("Error deploying beamlit app: %v\n", err)
				os.Exit(1)
			}
			// Walk through the temporary directory recursively, we deploy agents last
			err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if strings.Contains(path, "agents/") {
					return r.handleDeploymentFile(tempDir, &agents, &applyResults, path, info, err)
				}
				return nil
			})
			if err != nil {
				fmt.Printf("Error deploying beamlit app: %v\n", err)
				os.Exit(1)
			}

			
			env := "production"
			if environment != "" {
				env = environment
			}
			// Print apply summary in table format
			if len(applyResults) > 0 {
				fmt.Print("\nSummary:\n\n")
				fmt.Printf("%-20s %-30s %-10s\n", "KIND", "NAME", "RESULT")
				fmt.Printf("%-20s %-30s %-10s\n", "----", "----", "------")
				for _, result := range applyResults {
					fmt.Printf("%-20s %-30s %-10s\n", result.Kind, result.Name, result.Result)
				}
				fmt.Println()
			}
			if len(agents) > 1 {
				fmt.Printf("Your beamlit agents are ready:\n")
			} else {
				fmt.Printf("Your beamlit agent is ready:\n")
			}
			for _, agent := range agents {
				fmt.Printf(
					"- Url: %s/%s/global-inference-network/agent/%s?environment=%s\n",
					r.AppURL,
					workspace,
					agent,
					env,
				)
				fmt.Printf("  Watch status: bl get agent %s --watch\n", agent)
				fmt.Printf("  Run: bl run agent %s --data '{\"inputs\": \"Hello world\"}'\n\n", agent)
			}
		},
	}
	cmd.Flags().StringVarP(&module, "module", "m", "agent.main", "Module to serve, can be an agent or a function")
	cmd.Flags().StringVarP(&directory, "directory", "d", "src", "Directory to deploy, defaults to current directory")
	return cmd
}
