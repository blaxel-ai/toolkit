package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

func executeInstallDependencies(language string) error {
	if language == "python" {
		cmd := exec.Command("uv", "sync", "--refresh", "--force-reinstall", "--prerelease", "allow")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
		return cmd.Run()
	}
	if language == "typescript" {
		cmd := exec.Command("npx", "pnpm", "install", "--force")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

func executePythonGenerateBlaxelDeployment(deployDir string, module string, directory string, name string) error {
	if module == "" {
		module = "agent.main"
	}
	pythonCode := fmt.Sprintf(`
from blaxel.deploy import generate_blaxel_deployment
generate_blaxel_deployment("%s", "%s")
	`, deployDir, name)
	pythonCmd := "python"
	if _, err := os.Stat(".venv"); !os.IsNotExist(err) {
		pythonCmd = ".venv/bin/python"
	}
	cmd := exec.Command(pythonCmd, "-c", pythonCode)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, fmt.Sprintf("BL_SERVER_MODULE=%s", module))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BL_SERVER_DIRECTORY=%s", directory))
	cmd.Env = append(cmd.Env, "BL_DEPLOY=true")
	if os.Getenv("BL_ENV") != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BL_ENV=%s", os.Getenv("BL_ENV")))
	}
	cmd.Env = AddClientEnv(cmd.Env)
	return cmd.Run()
}

func executeTypescriptGenerateBlaxelDeployment(deployDir string, module string, directory string, name string) error {
	if module == "" {
		module = "agent.agent"
	}

	tsCode := fmt.Sprintf(`
import { generateBlaxelDeployment } from "@blaxel/sdk";

generateBlaxelDeployment("%s", "%s");
	`, deployDir, name)

	// Create temporary file in deployDir
	tmpFile, err := os.CreateTemp("./", "blaxel-*.ts")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", deployDir, err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up temp file when done

	// Write TypeScript code to temp file
	if _, err := tmpFile.WriteString(tsCode); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("npx", "tsx", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, fmt.Sprintf("BL_SERVER_MODULE=%s", module))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BL_SERVER_DIRECTORY=%s", directory))
	cmd.Env = append(cmd.Env, "BL_DEPLOY=true")
	if os.Getenv("BL_ENV") != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BL_ENV=%s", os.Getenv("BL_ENV")))
	}
	// Copy env from computer
	cmd.Env = AddClientEnv(cmd.Env)
	return cmd.Run()
}

func handleZipFile(zipWriter *zip.Writer, currentDir string, path string, info os.FileInfo) error {
	// Get relative path
	relPath, err := filepath.Rel(currentDir, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Skip if at root
	if relPath == "." {
		return nil
	}

	// Create zip header
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("failed to create zip header: %w", err)
	}
	header.Name = relPath

	if info.IsDir() {
		header.Name += "/"
	} else {
		header.Method = zip.Deflate
	}

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// If not a directory, write file content
	if !info.IsDir() {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("failed to write file %s to zip: %w", path, err)
		}
	}
	return nil
}

func createZip(currentDir string, path string) (*os.File, error) {
	// Create a temporary zip file
	zipFile, err := os.CreateTemp("", "blaxel-*.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp zip file: %w", err)
	}
	defer os.Remove(zipFile.Name())

	// Create zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through the directory
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .blaxel directory
		ignores := []string{".blaxel", ".venv", ".git", "node_modules"}
		for _, ignore := range ignores {
			if strings.Contains(path, ignore) {
				return nil
			}
		}

		return handleZipFile(zipWriter, currentDir, path, info)
	})

	// Write dockerfile if it exists
	deployDir := filepath.Dir(path)
	dockerfilePath := filepath.Join(deployDir, "Dockerfile")
	if fileInfo, err := os.Stat(dockerfilePath); err == nil {
		if err := handleZipFile(zipWriter, deployDir, dockerfilePath, fileInfo); err != nil {
			return nil, fmt.Errorf("failed to add Dockerfile to zip: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create zip archive: %w", err)
	}

	// Close zip writer before uploading
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}
	return zipFile, nil
}

func handleUpload(resourceType string, name string, path string, uploadUrl string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Printf("Uploading %s:%s path: %s\n", resourceType, name, path)

	zipFile, err := createZip(currentDir, path)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	_, err = zipFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to start of zip file: %w", err)
	}
	defer zipFile.Close()

	// Upload the zip file
	req, err := http.NewRequest("PUT", uploadUrl, zipFile)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	fileInfo, err := zipFile.Stat()
	if err != nil {
		return err
	}
	req.ContentLength = fileInfo.Size()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}
	return nil
}

func (r *Operations) handleDeploymentFile(deployDir string, agents *[]string, applyResults *[]ApplyResult, path string, autogenerated bool, info os.FileInfo, err error) error {
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
	// Get relative path from deployDir
	relPath, err := filepath.Rel(deployDir, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}
	tmpSplit := strings.Split(relPath, "/")
	if len(tmpSplit) < 2 {
		return nil
	}
	name := tmpSplit[1]
	if isAgent {
		if !slices.Contains(*agents, name) {
			*agents = append(*agents, name)
		}
	}

	if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
		results, err := r.Apply(path, WithUpload(true), WithAutogenerated(autogenerated))
		if err != nil {
			return fmt.Errorf("failed to apply configuration: %w", err)
		}
		if len(results) > 0 {
			result := results[0]
			if result.Result.UploadURL != "" {
				// HANDLE UPLOAD
				err := handleUpload(resourceType, name, path, result.Result.UploadURL)
				if err != nil {
					return fmt.Errorf("failed to upload file: %w", err)
				}
			}
		}
		*applyResults = append(*applyResults, results...)
	}
	return nil
}

func (r *Operations) DeployAgentAppCmd() *cobra.Command {
	var module string
	var directory string
	var dependencies bool
	var name string
	var dryRun bool
	var autogenerated bool

	cmd := &cobra.Command{
		Use:     "deploy",
		Args:    cobra.ExactArgs(0),
		Aliases: []string{"d", "dp"},
		Short:   "Deploy a blaxel agent app",
		Long:    "Deploy a blaxel agent app, you must be in a blaxel agent app directory.",
		Example: `bl deploy`,
		Run: func(cmd *cobra.Command, args []string) {

			// Create a temporary directory for deployment files
			deployDir := ".blaxel"

			language := moduleLanguage()

			if dependencies {
				err := executeInstallDependencies(language)
				if err != nil {
					fmt.Printf("Error installing dependencies: %v\n", err)
					os.Exit(1)
				}
			}

			switch language {
			case "python":
				err := executePythonGenerateBlaxelDeployment(deployDir, module, directory, name)
				if err != nil {
					fmt.Printf("Error executing Python script: %v\n", err)
					os.Exit(1)
				}
			case "typescript":
				err := executeTypescriptGenerateBlaxelDeployment(deployDir, module, directory, name)
				if err != nil {
					fmt.Printf("Error executing Typescript script: %v\n", err)
					os.Exit(1)
				}
			default:
				fmt.Println("Error: Neither pyproject.toml nor package.json found in current directory")
				os.Exit(1)
			}

			if dryRun {
				fmt.Println("Dry run complete, check folder: ", deployDir)
				return
			}

			agents := []string{}
			applyResults := []ApplyResult{}
			// Walk through the temporary directory recursively and collect files
			var filesToProcess []string
			err := filepath.Walk(deployDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				filesToProcess = append(filesToProcess, path)
				return nil
			})
			if err != nil {
				fmt.Printf("Error collecting deployment files: %v\n", err)
				os.Exit(1)
			}

			// Process files in parallel using a worker pool
			var wg sync.WaitGroup
			errChan := make(chan error, len(filesToProcess))

			for _, path := range filesToProcess {
				wg.Add(1)
				go func(p string) {
					defer wg.Done()
					info, err := os.Stat(p)
					if err != nil {
						errChan <- err
						return
					}

					err = r.handleDeploymentFile(deployDir, &agents, &applyResults, p, autogenerated, info, nil)
					if err != nil {
						errChan <- err
						return
					}
				}(path)
			}

			// Wait for all goroutines to complete
			wg.Wait()
			close(errChan)

			// Check for any errors
			for err := range errChan {
				if err != nil {
					fmt.Printf("Error deploying blaxel app: %v\n", err)
				}
			}
			if len(errChan) > 0 {
				os.Exit(1)
			}

			// Print apply summary in table format
			// if len(applyResults) > 0 {
			// 	fmt.Print("\nSummary:\n\n")
			// 	fmt.Printf("%-20s %-30s %-10s\n", "KIND", "NAME", "RESULT")
			// 	fmt.Printf("%-20s %-30s %-10s\n", "----", "----", "------")
			// 	for _, result := range applyResults {
			// 		fmt.Printf("%-20s %-30s %-10s\n", result.Kind, result.Name, result.Result.Status)
			// 	}
			// }
			fmt.Println()
			if len(agents) > 1 {
				fmt.Printf("Your blaxel agents are deploying:\n")
			} else {
				fmt.Printf("Your blaxel agent is deploying:\n")
			}
			for _, agent := range agents {
				fmt.Printf(
					"- Url: %s/%s/global-agentic-network/agent/%s\n",
					r.AppURL,
					workspace,
					agent,
				)
				fmt.Printf("  Watch status: bl get agent %s --watch\n", agent)
				fmt.Printf("  Run: bl run agent %s --data '{\"inputs\": \"Hello world\"}'\n\n", agent)
			}

		},
	}
	cmd.Flags().StringVarP(&module, "module", "m", "", "Module to serve, can be an agent or a function")
	cmd.Flags().StringVarP(&directory, "directory", "d", "src", "Directory to deploy, defaults to current directory")
	cmd.Flags().BoolVarP(&dependencies, "dependencies", "D", false, "Install dependencies")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Optional name for the deployment")
	cmd.Flags().BoolVarP(&dryRun, "dryrun", "", false, "Dry run the deployment")
	cmd.Flags().BoolVarP(&autogenerated, "autogenerated", "", false, "Only deploy autogenerated resources from you agent application")
	return cmd
}
