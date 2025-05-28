package cli

import (
	"fmt"
	"os"

	"archive/zip"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/spf13/cobra"
)

func (r *Operations) DeployCmd() *cobra.Command {
	var directory string
	var name string
	var dryRun bool
	var recursive bool
	cmd := &cobra.Command{
		Use:     "deploy",
		Args:    cobra.ExactArgs(0),
		Aliases: []string{"d", "dp"},
		Short:   "Deploy on blaxel",
		Long:    "Deploy agent, mcp or job on blaxel, you must be in a blaxel directory.",
		Example: `bl deploy`,
		Run: func(cmd *cobra.Command, args []string) {

			if recursive {
				if deployPackage(dryRun, name) {
					return
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current working directory: %v\n", err)
				os.Exit(1)
			}

			// Additional deployment directory, for blaxel yaml files
			deployDir := ".blaxel"

			if config.Name != "" {
				name = config.Name
			}

			deployment := Deployment{
				dir:    deployDir,
				folder: folder,
				name:   name,
				cwd:    cwd,
				r:      r,
			}

			err = deployment.Generate()
			if err != nil {
				fmt.Printf("Error generating blaxel deployment: %v\n", err)
				os.Exit(1)
			}

			if dryRun {
				err := deployment.Print()
				if err != nil {
					fmt.Printf("Error printing blaxel deployment: %v\n", err)
					os.Exit(1)
				}
				return
			}

			err = deployment.Apply()
			if err != nil {
				fmt.Printf("Error applying blaxel deployment: %v\n", err)
				os.Exit(1)
			}

			deployment.Ready()
		},
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", "src", "Directory to deploy, defaults to current directory")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Optional name for the deployment")
	cmd.Flags().BoolVarP(&dryRun, "dryrun", "", false, "Dry run the deployment")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Deploy recursively")
	return cmd
}

type Deployment struct {
	dir               string
	name              string
	folder            string
	blaxelDeployments []Result
	zip               *os.File
	cwd               string
	r                 *Operations
}

func (d *Deployment) Generate() error {
	if d.name == "" {
		split := strings.Split(d.cwd, "/")
		d.name = split[len(split)-1]
	}

	err := d.r.SeedCache(d.cwd)
	if err != nil {
		return fmt.Errorf("failed to seed cache: %w", err)
	}

	// Generate the blaxel deployment yaml
	d.blaxelDeployments = []Result{d.GenerateDeployment()}

	// Zip the directory
	err = d.Zip()
	if err != nil {
		return fmt.Errorf("failed to zip file: %w", err)
	}

	return nil
}

func (d *Deployment) GenerateDeployment() Result {
	entrypoint, err := findRootCmdAsString(RootCmdConfig{
		Hotreload:  false,
		Production: true,
		Entrypoint: config.Entrypoint,
	})
	if err != nil {
		fmt.Printf("failed to find root cmd: %v", err)
	}
	if len(entrypoint) > 0 {
		entrypoint[0] = "/usr/bin/" + entrypoint[0]
	}
	if len(entrypoint) > 1 {
		entrypoint[1] = "/blaxel/" + entrypoint[1]
	}
	var Spec map[string]interface{}
	var Kind string

	runtime := make(map[string]interface{})
	if config.Runtime != nil {
		runtime = *config.Runtime
	}
	runtime["envs"] = GetUniqueEnvs()
	runtime["entrypoint"] = entrypoint
	if config.Type == "function" {
		runtime["type"] = "mcp"
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
	}
	if len(config.Policies) > 0 {
		Spec["policies"] = config.Policies
	}
	return Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       Kind,
		Metadata: map[string]interface{}{
			"name": d.name,
			"labels": map[string]string{
				"x-blaxel-auto-generated": "true",
			},
		},
		Spec: Spec,
	}
}

func (d *Deployment) Apply() error {
	blaxelDir := filepath.Join(d.cwd, ".blaxel")
	if _, err := os.Stat(blaxelDir); err == nil {
		fmt.Println("Applying additional resources from .blaxel directory...")
		_, err = d.r.Apply(blaxelDir, WithRecursive(true))
		if err != nil {
			return fmt.Errorf("failed to apply .blaxel directory: %w", err)
		}
	}
	applyResults, err := d.r.ApplyResources(d.blaxelDeployments)
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
	fmt.Println("Deployment applied successfully")
	currentWorkspace := sdk.CurrentContext().Workspace
	fmt.Println("Your deployment is available at: " + d.r.AppURL + "/" + currentWorkspace + "/global-agentic-network/" + config.Type + "/" + d.name)
}

func (d *Deployment) Upload(url string) error {
	// Open the zip file
	zipFile, err := os.Open(d.zip.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipFile.Close()

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
	defer resp.Body.Close()

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
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

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
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return fmt.Errorf("failed to copy %s to zip: %w", headerName, err)
			}
		}
	}
	return nil
}

func (d *Deployment) Print() error {
	for _, deployment := range d.blaxelDeployments {
		fmt.Print(deployment.ToString())
		fmt.Println("---")
	}
	err := d.PrintZip()
	if err != nil {
		return fmt.Errorf("failed to print zip: %w", err)
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
		fmt.Println("Error getting package commands:", err)
		os.Exit(1)
	}

	if len(commands) == 1 {
		return false
	}

	runCommands(commands, true)
	return true
}

func getDeployCommands(dryRun bool, defaultName string) ([]PackageCommand, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %v", err)
	}
	command := PackageCommand{
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
	commands := []PackageCommand{}
	if !config.SkipRoot {
		commands = append(commands, command)
	}
	packages := getAllPackages()
	for name, pkg := range packages {
		command := PackageCommand{
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
		commands = append(commands, command)
	}
	return commands, nil
}
