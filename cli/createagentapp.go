package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"regexp"
	"slices"
	"strings"

	"github.com/beamlit/toolkit/sdk"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
)

// retrieveModels fetches and returns a list of available model deployments from the API.
// It filters the models to only include supported runtime types (openai, anthropic, mistral, etc.).
// Returns an error if the API calls fail or if there are parsing issues.
func retrieveModels(modelType string) ([]sdk.Model, error) {
	var modelDeployments []sdk.Model
	ctx := context.Background()
	res, err := client.ListModels(ctx)
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
		if model.Spec.Runtime != nil {
			runtimeType := *model.Spec.Runtime.Type
			modelName := *model.Spec.Runtime.Model
			if modelType == "model" {
				supportedRuntimes := []string{"openai", "anthropic", "mistral", "cohere", "xai", "vertex", "bedrock", "azure-ai-inference", "azure-marketplace", "gemini"}
				if slices.Contains(supportedRuntimes, runtimeType) && !strings.Contains(modelName, "realtime") {
					modelDeployments = append(modelDeployments, model)
				}
			} else if modelType == "realtime-model" {
				supportedRuntimes := []string{"openai", "azure-ai-inference", "azure-marketplace"}
				if slices.Contains(supportedRuntimes, runtimeType) && strings.Contains(modelName, "realtime") {
					modelDeployments = append(modelDeployments, model)
				}
			}
		}
	}
	return modelDeployments, nil
}

func promptAgentAppTemplateConfig(templates Templates, agentAppOptions *TemplateOptions) {
	templateConfig, err := templates.find(agentAppOptions.Language, agentAppOptions.Template.Name).getConfig()
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
		} else if variable.Type == "model" || variable.Type == "realtime-model" {
			values[variable.Name] = &value
			models, err := retrieveModels(variable.Type)
			if err == nil {
				if len(models) == 0 {
					value = "None"
				} else if len(models) == 1 {
					value = *models[0].Metadata.Name
				} else {
					options := []huh.Option[string]{}
					for _, model := range models {
						options = append(options, huh.NewOption(*model.Metadata.Name, *model.Metadata.Name))
					}
					options = append(options, huh.NewOption("None", ""))
					input := huh.NewSelect[string]().
						Title(title).
						Description(variable.Description).
						Height(5).
						Options(options...).
						Value(&value)
					fields = append(fields, input)
				}
			}
		}
	}

	if len(fields) > 0 {
		formTemplates := huh.NewForm(
			huh.NewGroup(fields...),
		)
		formTemplates.WithTheme(GetHuhTheme())
		err = formTemplates.Run()
		if err != nil {
			fmt.Println("Cancel create blaxel agent app")
			os.Exit(0)
		}
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
func promptCreateAgentApp(directory string) TemplateOptions {
	agentAppOptions := TemplateOptions{
		ProjectName: directory,
		Directory:   directory,
		IgnoreFiles: map[string]IgnoreFile{},
		IgnoreDirs:  map[string]IgnoreDir{},
	}
	currentUser, err := user.Current()
	if err == nil {
		agentAppOptions.Author = currentUser.Username
	} else {
		agentAppOptions.Author = "blaxel"
	}
	templates, err := RetrieveTemplates("agent")
	if err != nil {
		fmt.Println("Could not retrieve templates")
		os.Exit(0)
	}
	languagesOptions := []huh.Option[string]{}
	for _, language := range templates.getLanguages() {
		languagesOptions = append(languagesOptions, huh.NewOption(language, language))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your agent app").
				Value(&agentAppOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Language").
				Description("Language to use for your agent app").
				Height(5).
				Options(languagesOptions...).
				Value(&agentAppOptions.Language),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your agent app").
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					templates := templates.filterByLanguage(agentAppOptions.Language)
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(template.Name, "")
						options = append(options, huh.NewOption(key, template.Name))
					}
					return options
				}, &agentAppOptions).
				Value(&agentAppOptions.Template.Name),
		),
	)
	form.WithTheme(GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel agent app")
		os.Exit(0)
	}
	promptAgentAppTemplateConfig(templates, &agentAppOptions)

	return agentAppOptions
}

// CreateAgentAppCmd returns a cobra.Command that implements the 'create-agent-app' CLI command.
// The command creates a new Blaxel agent app in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-agent-app directory
func (r *Operations) CreateAgentAppCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "create-agent-app directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"ca", "caa"},
		Short:   "Create a new blaxel agent app",
		Long:    "Create a new blaxel agent app",
		Example: `bl create-agent-app my-agent-app`,
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
				Title("Creating your blaxel agent app...").
				Action(func() {
					err = opts.Template.Clone(opts)
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
			fmt.Printf(`Your blaxel agent app has been created. Start working on it:
cd %s;
bl serve --hotreload;
`, opts.Directory)
		},
	}
	return cmd
}
