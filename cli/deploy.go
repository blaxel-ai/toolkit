package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"archive/zip"
	"net/http"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/server"
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
	cmd := &cobra.Command{
		Use:     "deploy",
		Args:    cobra.ExactArgs(0),
		Aliases: []string{"d", "dp"},
		Short:   "Deploy on blaxel",
		Long:    "Deploy agent, mcp or job on blaxel, you must be in a blaxel directory.",
		Example: `bl deploy`,
		Run: func(cmd *cobra.Command, args []string) {
			core.LoadCommandSecrets(commandSecrets)
			core.ReadSecrets(folder, envFiles)
			if folder != "" {
				recursive = false
				core.ReadSecrets("", envFiles)
				core.ReadConfigToml(folder)
			}

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

			err = deployment.Apply()
			if err != nil {
				core.PrintError("Deploy", fmt.Errorf("error applying blaxel deployment: %w", err))
				os.Exit(1)
			}

			deployment.Ready()
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Optional name for the deployment")
	cmd.Flags().BoolVarP(&dryRun, "dryrun", "", false, "Dry run the deployment")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Deploy recursively")
	cmd.Flags().StringVarP(&folder, "directory", "d", "", "Deployment app path, can be a sub directory")
	cmd.Flags().StringSliceVarP(&envFiles, "env-file", "e", []string{".env"}, "Environment file to load")
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to deploy")
	cmd.Flags().BoolVarP(&skipBuild, "skip-build", "", false, "Skip the build step")
	return cmd
}

type Deployment struct {
	dir               string
	name              string
	folder            string
	blaxelDeployments []core.Result
	zip               *os.File
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

	if !skipBuild {
		// Zip the directory
		err = d.Zip()
		if err != nil {
			return fmt.Errorf("failed to zip file: %w", err)
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
	runtime["envs"] = core.GetUniqueEnvs()
	if config.Type == "function" {
		runtime["type"] = "mcp"
	}

	if skipBuild {
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
	}
	if len(config.Policies) > 0 {
		Spec["policies"] = config.Policies
	}
	labels := map[string]string{}
	if !skipBuild {
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

func (d *Deployment) Ready() {
	currentWorkspace := core.GetWorkspace()
	config := core.GetConfig()
	appUrl := core.GetAppURL()
	availableAt := fmt.Sprintf("It is available at: %s/%s/global-agentic-network/%s/%s", appUrl, currentWorkspace, config.Type, d.name)
	core.PrintSuccess(fmt.Sprintf("Deployment applied successfully\n%s", availableAt))
}

func (d *Deployment) Upload(url string) error {
	// Open the zip file
	zipFile, err := os.Open(d.zip.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	// Get the file size
	fileInfo, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", url, zipFile)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set the content length
	req.ContentLength = fileInfo.Size()

	// Set the content type to application/zip
	req.Header.Set("Content-Type", "application/zip")

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

func (d *Deployment) Zip() error {
	ignoredPaths := d.IgnoredPaths()
	zipFile, err := os.CreateTemp("", ".blaxel.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	err = filepath.Walk(d.cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		for _, ignoredPath := range ignoredPaths {
			if strings.HasPrefix(path, filepath.Join(d.cwd, ignoredPath)) {
				return nil
			}
			if strings.Contains(path, "/"+ignoredPath+"/") {
				return nil
			}
			if strings.HasSuffix(path, "/"+ignoredPath) {
				return nil
			}
		}
		if path == d.cwd {
			return nil
		}

		relPath, err := filepath.Rel(d.cwd, path)
		if err != nil {
			return err
		}

		err = d.addFileToZip(zipWriter, path, relPath)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to zip directory: %w", err)
	}

	if d.folder != "" {
		blaxelTomlPath := filepath.Join(d.cwd, d.folder, "blaxel.toml")
		err := d.addFileToZip(zipWriter, blaxelTomlPath, "blaxel.toml")
		if err != nil {
			return err
		}
		dockerfilePath := filepath.Join(d.cwd, d.folder, "Dockerfile")
		err = d.addFileToZip(zipWriter, dockerfilePath, "Dockerfile")
		if err != nil {
			return err
		}
	}

	d.zip = zipFile

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

func (d *Deployment) Print(skipBuild bool) error {
	for _, deployment := range d.blaxelDeployments {
		fmt.Print(deployment.ToString())
		fmt.Println("---")
	}
	if !skipBuild {
		err := d.PrintZip()
		if err != nil {
			return fmt.Errorf("failed to print zip: %w", err)
		}
	}
	return nil
}

func (d *Deployment) PrintZip() error {
	// Reopen the file to get the reader
	zipFile, err := os.Open(d.zip.Name())
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
