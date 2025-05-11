package cli

import (
	"fmt"
	"os"
	"os/user"
	"regexp"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
)

func promptJobTemplateConfig(templates Templates, jobOptions *TemplateOptions) {
	templateConfig, err := templates.find(jobOptions.Language, jobOptions.Template.Name).getConfig()
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
				jobOptions.IgnoreFiles[variable.Name] = IgnoreFile{File: variable.File, Skip: variable.Skip}
			}
			if variable.Folder != "" {
				jobOptions.IgnoreDirs[variable.Name] = IgnoreDir{Folder: variable.Folder, Skip: variable.Skip}
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
					jobOptions.IgnoreFiles[option.Name] = IgnoreFile{File: option.File, Skip: option.Skip}
				}
				if option.Folder != "" {
					jobOptions.IgnoreDirs[option.Name] = IgnoreDir{Folder: option.Folder, Skip: option.Skip}
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
			fmt.Println("Cancel create blaxel job")
			os.Exit(0)
		}
	}
	jobOptions.TemplateOptions = values
	for _, array_value := range array_values {
		for _, value := range *array_value {
			k := mapped_values[value]
			jobOptions.TemplateOptions[k] = &value
		}
	}
}

// promptCreateJob displays an interactive form to collect user input for creating a new job.
// It prompts for project name, language selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a CreateJobOptions struct with the user's selections.
func promptCreateJob(directory string) TemplateOptions {
	jobOptions := TemplateOptions{
		ProjectName: directory,
		Directory:   directory,
		IgnoreFiles: map[string]IgnoreFile{},
		IgnoreDirs:  map[string]IgnoreDir{},
	}
	currentUser, err := user.Current()
	if err == nil {
		jobOptions.Author = currentUser.Username
	} else {
		jobOptions.Author = "blaxel"
	}
	templates, err := RetrieveTemplates("job")
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
				Description("Name of your job").
				Value(&jobOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Language").
				Description("Language to use for your job").
				Height(5).
				Options(languagesOptions...).
				Value(&jobOptions.Language),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your job").
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates.filterByLanguage(jobOptions.Language) {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(template.Name, "")
						options = append(options, huh.NewOption(key, template.Name))
					}
					return options
				}, &jobOptions).
				Value(&jobOptions.Template.Name),
		),
	)
	form.WithTheme(GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel job")
		os.Exit(0)
	}
	promptJobTemplateConfig(templates, &jobOptions)

	return jobOptions
}

// CreateJobCmd returns a cobra.Command that implements the 'create-job' CLI command.
// The command creates a new Blaxel job in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-job directory
func (r *Operations) CreateJobCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:     "create-job directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"cj", "cjob"},
		Short:   "Create a new blaxel job",
		Long:    "Create a new blaxel job",
		Example: `bl create-job my-job`,
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
			opts := promptCreateJob(args[0])

			var err error
			spinnerErr := spinner.New().
				Title("Creating your blaxel job...").
				Action(func() {
					err = opts.Template.Clone(opts)
				}).
				Run()
			if spinnerErr != nil {
				fmt.Println("Error creating job", spinnerErr)
				return
			}
			if err != nil {
				fmt.Println("Error creating job", err)
				os.RemoveAll(opts.Directory)
				return
			}

			fmt.Printf(`Your blaxel job has been created. Start working on it:
cd %s;
bl run job my-job --local --file batches/sample-batch.json;
`, opts.Directory)
		},
	}
	return cmd
}
