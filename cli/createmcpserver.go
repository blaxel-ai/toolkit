package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/beamlit/toolkit/sdk"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// CreateMCPServerOptions contains all the configuration options needed to create a new mcp server.
type CreateMCPServerOptions struct {
	Directory       string             // Target directory for the new mcp server
	ProjectName     string             // Name of the project
	ProjectPrompt   string             // Description of the project
	Language        string             // Language to use for the project
	Template        string             // Template to use for the project
	Author          string             // Author of the project
	TemplateOptions map[string]*string // Options for the template
	IgnoreFiles     map[string]IgnoreFile
	IgnoreDirs      map[string]IgnoreDir
}

// retrieveTemplates retrieves the list of available templates from the templates repository.
// It fetches the repository's tree structure and extracts the paths of all directories.
// Returns a list of template names or an error if the retrieval fails.
func retrieveMCPServerTemplates() ([]string, map[string][]string, error) {
	var scriptErr error
	languages := []string{}
	templates := map[string][]string{}
	spinnerErr := spinner.New().
		Title("Retrieving templates...").
		Action(func() {
			url := fmt.Sprintf("https://api.github.com/repos/beamlit/templates/git/trees/%s?recursive=1", getBranch())
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				scriptErr = err
				return
			}

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				scriptErr = err
				return
			}

			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				scriptErr = err
				return
			}
			var treeResponse GithubTreeResponse
			err = json.Unmarshal(body, &treeResponse)
			if err != nil {
				scriptErr = err
				return
			}
			for _, tree := range treeResponse.Tree {
				if strings.HasPrefix(tree.Path, "mcps/") && len(strings.Split(tree.Path, "/")) == 3 {
					language := strings.Split(tree.Path, "/")[1]
					if !slices.Contains(languages, language) {
						languages = append(languages, language)
					}
					if _, ok := templates[language]; !ok {
						templates[language] = []string{}
					}
					templates[language] = append(templates[language], strings.Split(tree.Path, "/")[2])
				}
			}
		}).
		Run()
	if spinnerErr != nil {
		return nil, nil, spinnerErr
	}
	if scriptErr != nil {
		return nil, nil, scriptErr
	}
	return languages, templates, nil
}

func retrieveMCPServerTemplateConfig(language string, template string) (*TemplateConfig, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/beamlit/templates/refs/heads/%s/mcps/%s/%s/template.yaml", getBranch(), language, template)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var templateConfig TemplateConfig
	err = yaml.Unmarshal(body, &templateConfig)
	if err != nil {
		return nil, err
	}
	return &templateConfig, nil
}

func promptMCPServerTemplateConfig(mcpserverOptions *CreateMCPServerOptions) {
	templateConfig, err := retrieveMCPServerTemplateConfig(mcpserverOptions.Language, mcpserverOptions.Template)
	if err != nil {
		fmt.Println("Could not retrieve template configuration")
		os.Exit(0)
	}
	fields := []huh.Field{}
	values := map[string]*string{}
	array_values := map[string]*[]string{}
	mapped_values := map[string]string{}
	for _, variable := range templateConfig.Variables {
		var value string
		var array_value []string

		title := variable.Name
		if variable.Label != nil {
			title = *variable.Label
		}
		if variable.Type == "select" {
			values[variable.Name] = &value
			options := []huh.Option[string]{}
			if variable.File != "" {
				mcpserverOptions.IgnoreFiles[variable.Name] = IgnoreFile{File: variable.File, Skip: variable.Skip}
			}
			if variable.Folder != "" {
				mcpserverOptions.IgnoreDirs[variable.Name] = IgnoreDir{Folder: variable.Folder, Skip: variable.Skip}
			}
			for _, option := range variable.Options {
				options = append(options, huh.NewOption(option.Label, option.Value))
			}
			input := huh.NewSelect[string]().
				Title(title).
				Description(variable.Description).
				Options(options...).
				Value(&value)
			fields = append(fields, input)
		} else if variable.Type == "input" {
			values[variable.Name] = &value
			input := huh.NewInput().
				Title(title).
				Description(variable.Description).
				Value(&value)
			fields = append(fields, input)
		} else if variable.Type == "multiselect" {
			array_values[variable.Name] = &array_value
			options := []huh.Option[string]{}
			for _, option := range variable.Options {
				mapped_values[option.Value] = option.Name
				if option.File != "" {
					mcpserverOptions.IgnoreFiles[option.Name] = IgnoreFile{File: option.File, Skip: option.Skip}
				}
				if option.Folder != "" {
					mcpserverOptions.IgnoreDirs[option.Name] = IgnoreDir{Folder: option.Folder, Skip: option.Skip}
				}
				options = append(options, huh.NewOption(option.Label, option.Value))
			}
			input := huh.NewMultiSelect[string]().
				Title(title).
				Description(variable.Description).
				Options(options...).
				Value(&array_value)
			fields = append(fields, input)
		}
	}

	if len(fields) > 0 {
		formTemplates := huh.NewForm(
			huh.NewGroup(fields...),
		)
		formTemplates.WithTheme(GetHuhTheme())
		err = formTemplates.Run()
		if err != nil {
			fmt.Println("Cancel create blaxel mcp server")
			os.Exit(0)
		}
	}
	mcpserverOptions.TemplateOptions = values
	for _, array_value := range array_values {
		for _, value := range *array_value {
			k := mapped_values[value]
			mcpserverOptions.TemplateOptions[k] = &value
		}
	}
}

// promptCreateMCPServer displays an interactive form to collect user input for creating a new mcp server.
// It prompts for project name, language selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a CreateMCPServerOptions struct with the user's selections.
func promptCreateMCPServer(directory string) CreateMCPServerOptions {
	mcpserverOptions := CreateMCPServerOptions{
		ProjectName: directory,
		Directory:   directory,
		IgnoreFiles: map[string]IgnoreFile{},
		IgnoreDirs:  map[string]IgnoreDir{},
	}
	currentUser, err := user.Current()
	if err == nil {
		mcpserverOptions.Author = currentUser.Username
	} else {
		mcpserverOptions.Author = "blaxel"
	}
	languages, templates, err := retrieveMCPServerTemplates()
	if err != nil {
		fmt.Println("Could not retrieve templates")
		os.Exit(0)
	}
	languagesOptions := []huh.Option[string]{}
	for _, language := range languages {
		languagesOptions = append(languagesOptions, huh.NewOption(language, language))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your mcp server").
				Value(&mcpserverOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Language").
				Description("Language to use for your mcp server").
				Height(5).
				Options(languagesOptions...).
				Value(&mcpserverOptions.Language),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your mcp server").
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					templates := templates[mcpserverOptions.Language]
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(template, "")
						options = append(options, huh.NewOption(key, template))
					}
					return options
				}, &mcpserverOptions).
				Value(&mcpserverOptions.Template),
		),
	)
	form.WithTheme(GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel mcp server")
		os.Exit(0)
	}
	promptMCPServerTemplateConfig(&mcpserverOptions)

	return mcpserverOptions
}

// createMCPServer handles the actual creation of the mcp server based on the provided options.
// It performs the following steps:
// 1. Creates the project directory
// 2. Clones the templates repository
// 3. Processes template files
// 4. Installs dependencies using uv sync
// Returns an error if any step fails.
func createMCPServer(opts CreateMCPServerOptions) error {
	// Create project directory
	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		return err
	}

	// Clone templates repository
	cloneDir := filepath.Join(opts.Directory, "templates")
	branch := getBranch()
	cloneDirCmd := exec.Command("git", "clone", "https://github.com/beamlit/templates.git", cloneDir, "--branch", branch)
	if err := cloneDirCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone templates repository: %w", err)
	}

	templateOptions := map[string]string{
		"ProjectName":   opts.ProjectName,
		"ProjectPrompt": opts.ProjectPrompt,
		"Author":        opts.Author,
		"Workspace":     workspace,
	}
	for key, value := range opts.TemplateOptions {
		templateOptions[key] = *value
	}

	// Initialize ignore files and folders
	ignoreFiles := []string{"template.yaml"}
	ignoreFolders := []string{}
	for key, ignoreFile := range opts.IgnoreFiles {
		value, ok := templateOptions[key]
		if ok {
			if ignoreFile.Skip == value {
				ignoreFiles = append(ignoreFiles, ignoreFile.File)
			}
		} else {
			if ignoreFile.Skip == "" {
				ignoreFiles = append(ignoreFiles, ignoreFile.File)
			}
		}
	}
	for key, ignoreDir := range opts.IgnoreDirs {
		value, ok := templateOptions[key]
		if ok {
			if ignoreDir.Skip == value {
				ignoreFolders = append(ignoreFolders, ignoreDir.Folder)
			}
		} else {
			if ignoreDir.Skip == "" {
				ignoreFolders = append(ignoreFolders, ignoreDir.Folder)
			}
		}
	}
	templateDir := filepath.Join(cloneDir, "mcps", opts.Language, opts.Template)
	err := filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		rel, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}

		// Skip files based on config
		for _, ignoreFile := range ignoreFiles {
			if strings.HasSuffix(rel, ignoreFile) {
				return nil
			}
		}

		// Skip folders based on config
		for _, ignoreFolder := range ignoreFolders {
			if strings.HasPrefix(rel, ignoreFolder) {
				return nil
			}
		}

		// Process template
		tmpl, err := template.ParseFiles(path)
		if err != nil {
			return err
		}

		// Create output file
		outPath := filepath.Join(opts.Directory, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}

		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()

		// Execute template
		return tmpl.Execute(out, templateOptions)
	})
	if err != nil {
		return err
	}
	// Remove templates directory after processing
	if err := os.RemoveAll(cloneDir); err != nil {
		return fmt.Errorf("failed to remove templates directory: %w", err)
	}

	// Install dependencies based on language
	switch opts.Language {
	case "python":
		if err := installPythonDependencies(opts.Directory); err != nil {
			return err
		}
	case "typescript":
		if err := installTypescriptDependencies(opts.Directory); err != nil {
			return err
		}
	}
	return nil
}

// CreateMCPServerCmd returns a cobra.Command that implements the 'create-mcpserver' CLI command.
// The command creates a new Blaxel mcp server in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-mcpserver directory
func (r *Operations) CreateMCPServerCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "create-mcp-server directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"cm", "cms"},
		Short:   "Create a new blaxel mcp server",
		Long:    "Create a new blaxel mcp server",
		Example: `bl create-mcpserver my-mcp-server`,
		Run: func(cmd *cobra.Command, args []string) {

			if len(args) < 1 {
				fmt.Println("Please provide a directory name")
				return
			}
			// Check if directory already exists
			if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
				fmt.Printf("Error: %s already exists\n", args[0])
				return
			}
			opts := promptCreateMCPServer(args[0])

			var err error
			spinnerErr := spinner.New().
				Title("Creating your blaxel mcp server...").
				Action(func() {
					err = createMCPServer(opts)
				}).
				Run()
			if spinnerErr != nil {
				fmt.Println("Error creating mcp server", spinnerErr)
				return
			}
			if err != nil {
				fmt.Println("Error creating mcp server", err)
				os.RemoveAll(opts.Directory)
				return
			}
			res, err := client.ListModels(context.Background())
			if err != nil {
				return
			}

			body, err := io.ReadAll(res.Body)
			if err != nil {
				return
			}

			var models []sdk.Model
			err = json.Unmarshal(body, &models)
			if err != nil {
				return
			}
			fmt.Printf(`Your blaxel mcp server has been created. Start working on it:
cd %s;
bl serve --hotreload;
`, opts.Directory)
		},
	}
	return cmd
}
