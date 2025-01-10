package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/beamlit/toolkit/sdk"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type IgnoreFile struct {
	File string
	Skip string
}

type IgnoreDir struct {
	Folder string
	Skip   string
}

// CreateAgentAppOptions contains all the configuration options needed to create a new agent app.
type CreateAgentAppOptions struct {
	Directory          string             // Target directory for the new agent app
	ProjectName        string             // Name of the project
	ProjectDescription string             // Description of the project
	Template           string             // Template to use for the project
	Author             string             // Author of the project
	TemplateOptions    map[string]*string // Options for the template
	IgnoreFiles        map[string]IgnoreFile
	IgnoreDirs         map[string]IgnoreDir
}

type TemplateConfig struct {
	Variables []struct {
		Name        string  `yaml:"name"`
		Label       *string `yaml:"label"`
		Type        string  `yaml:"type"`
		Description string  `yaml:"description"`
		File        string  `yaml:"file"`
		Skip        string  `yaml:"skip"`
		Folder      string  `yaml:"folder"`
		Options     []struct {
			Label  string `yaml:"label"`
			Value  string `yaml:"value"`
			Name   string `yaml:"name"`
			File   string `yaml:"file"`
			Skip   string `yaml:"skip"`
			Folder string `yaml:"folder"`
		} `yaml:"options"`
	} `yaml:"variables"`
}

type GithubTreeResponse struct {
	Tree []struct {
		Path string `json:"path"`
	} `json:"tree"`
}

type GithubContentResponse struct {
	Content string `json:"content"`
}

// getTheme returns a custom theme configuration for the CLI interface using the Dracula color scheme.
// It customizes various UI elements like buttons, text inputs, and selection indicators.
func getTheme() *huh.Theme {
	t := huh.ThemeBase()
	var (
		background = lipgloss.AdaptiveColor{Dark: "#282a36"}
		selection  = lipgloss.AdaptiveColor{Dark: "#44475a"}
		foreground = lipgloss.AdaptiveColor{Dark: "#f8f8f2"}
		comment    = lipgloss.AdaptiveColor{Dark: "#6272a4"}
		green      = lipgloss.AdaptiveColor{Dark: "#50fa7b"}
		orange     = lipgloss.AdaptiveColor{Dark: "#fd7b35"}
		red        = lipgloss.AdaptiveColor{Dark: "#ff5555"}
		yellow     = lipgloss.AdaptiveColor{Dark: "#f1fa8c"}
	)

	t.Focused.Base = t.Focused.Base.BorderForeground(selection)
	t.Focused.Title = t.Focused.Title.Foreground(orange)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(orange)
	t.Focused.Description = t.Focused.Description.Foreground(comment)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
	t.Focused.Directory = t.Focused.Directory.Foreground(orange)
	t.Focused.File = t.Focused.File.Foreground(foreground)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(yellow)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(yellow)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(yellow)
	t.Focused.Option = t.Focused.Option.Foreground(foreground)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(yellow)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(green)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(green).SetString("[✓] ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(foreground)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(comment)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(yellow).Background(orange).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(foreground).Background(background)

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(yellow)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(comment)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(yellow)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	return t
}

// retrieveModels fetches and returns a list of available model deployments from the API.
// It filters the models to only include supported runtime types (openai, anthropic, mistral, etc.).
// Returns an error if the API calls fail or if there are parsing issues.
func retrieveModels() ([]sdk.Model, error) {
	var modelDeployments []sdk.Model
	ctx := context.Background()
	res, err := client.ListModels(ctx, &sdk.ListModelsParams{})
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var models []sdk.Model
	err = json.Unmarshal(body, &models)
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		environment := "production"
		res, err := client.GetModel(ctx, *model.Metadata.Name, &sdk.GetModelParams{
			Environment: &environment,
		})
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		var model sdk.Model
		err = json.Unmarshal(body, &model)
		if err != nil {
			return nil, err
		}
		if model.Spec.Runtime != nil {
			runtimeType := *model.Spec.Runtime.Type
			supportedRuntimes := []string{"openai", "anthropic", "mistral", "cohere", "xai", "vertex", "bedrock"}
			if slices.Contains(supportedRuntimes, runtimeType) {
				modelDeployments = append(modelDeployments, model)
			}
		}
	}
	return modelDeployments, nil
}

// retrieveTemplates retrieves the list of available templates from the templates repository.
// It fetches the repository's tree structure and extracts the paths of all directories.
// Returns a list of template names or an error if the retrieval fails.
func retrieveTemplates() ([]string, error) {

	url := "https://api.github.com/repos/beamlit/templates/git/trees/main?recursive=1"

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
	var treeResponse GithubTreeResponse
	err = json.Unmarshal(body, &treeResponse)
	if err != nil {
		return nil, err
	}
	templates := []string{}
	for _, tree := range treeResponse.Tree {
		if strings.HasPrefix(tree.Path, "agents/") && len(strings.Split(tree.Path, "/")) == 2 {
			templates = append(templates, strings.Split(tree.Path, "/")[1])
		}
	}
	return templates, nil
}

func retrieveTemplateConfig(template string) (*TemplateConfig, error) {
	url := fmt.Sprintf("https://api.github.com/repos/beamlit/templates/contents/agents/%s/template.yaml", template)

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
	var contentResponse GithubContentResponse
	err = json.Unmarshal(body, &contentResponse)
	if err != nil {
		return nil, err
	}
	var templateConfig TemplateConfig
	content, err := base64.StdEncoding.DecodeString(contentResponse.Content)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(content, &templateConfig)
	if err != nil {
		return nil, err
	}
	return &templateConfig, nil
}

func promptTemplateConfig(agentAppOptions *CreateAgentAppOptions) {
	templateConfig, err := retrieveTemplateConfig(agentAppOptions.Template)
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
				agentAppOptions.IgnoreFiles[variable.Name] = IgnoreFile{File: variable.File, Skip: variable.Skip}
			}
			if variable.Folder != "" {
				agentAppOptions.IgnoreDirs[variable.Name] = IgnoreDir{Folder: variable.Folder, Skip: variable.Skip}
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
					agentAppOptions.IgnoreFiles[option.Name] = IgnoreFile{File: option.File, Skip: option.Skip}
				}
				if option.Folder != "" {
					agentAppOptions.IgnoreDirs[option.Name] = IgnoreDir{Folder: option.Folder, Skip: option.Skip}
				}
				options = append(options, huh.NewOption(option.Label, option.Value))
			}
			input := huh.NewMultiSelect[string]().
				Title(title).
				Description(variable.Description).
				Options(options...).
				Value(&array_value)
			fields = append(fields, input)
		} else if variable.Type == "model" {
			values[variable.Name] = &value
			input := huh.NewSelect[string]().
				Title(title).
				Description(variable.Description).
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					models, err := retrieveModels()
					if err != nil {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, model := range models {
						options = append(options, huh.NewOption(*model.Metadata.Name, *model.Metadata.Name))
					}
					return options
				}, &agentAppOptions).
				Value(&value)
			fields = append(fields, input)
		}
	}

	formTemplates := huh.NewForm(
		huh.NewGroup(fields...),
	)
	formTemplates.WithTheme(getTheme())
	err = formTemplates.Run()
	if err != nil {
		fmt.Println("Cancel create beamlit agent app")
		os.Exit(0)
	}
	agentAppOptions.TemplateOptions = values
	for _, array_value := range array_values {
		for _, value := range *array_value {
			k := mapped_values[value]
			agentAppOptions.TemplateOptions[k] = &value
		}
	}
}

// promptCreateAgentApp displays an interactive form to collect user input for creating a new agent app.
// It prompts for project name, model selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a CreateAgentAppOptions struct with the user's selections.
func promptCreateAgentApp(directory string) CreateAgentAppOptions {
	agentAppOptions := CreateAgentAppOptions{
		ProjectName: directory,
		Directory:   directory,
		IgnoreFiles: map[string]IgnoreFile{},
		IgnoreDirs:  map[string]IgnoreDir{},
	}
	currentUser, err := user.Current()
	if err == nil {
		agentAppOptions.Author = currentUser.Username
	} else {
		agentAppOptions.Author = "beamlit"
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your agent app").
				Value(&agentAppOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your agent app").
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					templates, err := retrieveTemplates()
					if err != nil {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates {
						options = append(options, huh.NewOption(template, template))
					}
					return options
				}, &agentAppOptions).
				Value(&agentAppOptions.Template),
		),
	)
	form.WithTheme(getTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create beamlit agent app")
		os.Exit(0)
	}
	promptTemplateConfig(&agentAppOptions)

	return agentAppOptions
}

// createAgentApp handles the actual creation of the agent app based on the provided options.
// It performs the following steps:
// 1. Creates the project directory
// 2. Clones the templates repository
// 3. Processes template files
// 4. Installs dependencies using uv sync
// Returns an error if any step fails.
func createAgentApp(opts CreateAgentAppOptions) error {
	// Create project directory
	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		return err
	}

	// Clone templates repository
	cloneDir := filepath.Join(opts.Directory, "templates")
	cloneDirCmd := exec.Command("git", "clone", "https://github.com/beamlit/templates.git", cloneDir)
	if err := cloneDirCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone templates repository: %w", err)
	}

	templateOptions := map[string]string{
		"ProjectName":        opts.ProjectName,
		"ProjectDescription": opts.ProjectDescription,
		"Author":             opts.Author,
		"Workspace":          workspace,
		"Environment":        environment,
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
	templateDir := filepath.Join(cloneDir, "agents", opts.Template)
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
	// Run uv sync to install dependencies
	uvSyncCmd := exec.Command("uv", "sync", "--refresh")
	uvSyncCmd.Dir = opts.Directory
	if err := uvSyncCmd.Run(); err != nil {
		return fmt.Errorf("failed to run uv sync: %w", err)
	}
	return nil
}

// CreateAgentAppCmd returns a cobra.Command that implements the 'create-agent-app' CLI command.
// The command creates a new Beamlit agent app in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-agent-app directory
func (r *Operations) CreateAgentAppCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "create-agent-app directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"ca", "caa"},
		Short:   "Create a new beamlit agent app",
		Long:    "Create a new beamlit agent app",
		Example: `bl create-agent-app`,
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
			opts := promptCreateAgentApp(args[0])

			var err error
			spinnerErr := spinner.New().
				Title("Creating your beamlit agent app...").
				Action(func() {
					err = createAgentApp(opts)
				}).
				Run()
			if spinnerErr != nil {
				fmt.Println("Error creating agent app", spinnerErr)
				return
			}
			if err != nil {
				fmt.Println("Error creating agent app", err)
				os.RemoveAll(opts.Directory)
				return
			}
			fmt.Printf(`Your beamlit agent app has been created. Start working on it:
cd %s;
bl serve --hotreload;
`, opts.Directory)
		},
	}
	return cmd
}
