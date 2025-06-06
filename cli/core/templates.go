package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"slices"

	"github.com/blaxel-ai/toolkit/sdk"
)

type Templates []Template
type Template struct {
	sdk.Template
	Language string
	Type     string
}

type TemplateOptions struct {
	Directory     string // Target directory for the new agent app
	ProjectName   string // Name of the project
	ProjectPrompt string // Description of the project
	Language      string // Language to use for the project
	TemplateName  string // Name of the template to use for the project
	Author        string // Author of the project
}

type IgnoreFile struct {
	File string
	Skip string
}

type IgnoreDir struct {
	Folder string
	Skip   string
}

func RetrieveTemplates(templateType string) (Templates, error) {
	templates := Templates{}
	resp, err := client.ListTemplatesWithResponse(context.Background())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("error: received non-200 response code")
	}
	for _, template := range *resp.JSON200 {
		if template.Topics != nil {
			if slices.Contains(*template.Topics, templateType) {
				language := ""
				if slices.Contains(*template.Topics, "python") {
					language = "python"
				} else if slices.Contains(*template.Topics, "typescript") {
					language = "typescript"
				}
				templates = append(templates, Template{
					Template: template,
					Language: language,
					Type:     templateType,
				})
			}
		}
	}
	return templates, nil
}

func (t Templates) GetLanguages() []string {
	languages := []string{}
	for _, template := range t {
		if !slices.Contains(languages, template.Language) {
			languages = append(languages, template.Language)
		}
	}
	return languages
}

func (t Templates) FilterByLanguage(language string) Templates {
	filtered := Templates{}
	for _, template := range t {
		if template.Language == language {
			filtered = append(filtered, template)
		}
	}
	return filtered
}

func (t Templates) Find(name string) (Template, error) {
	for _, template := range t {
		if *template.Name == name {
			return template, nil
		}
	}
	return Template{}, fmt.Errorf("template not found")
}

func (t Template) Clone(opts TemplateOptions) error {
	// Create project directory
	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		return err
	}

	// We clone in a tmp dir, cause the template can contain variables and they will be evaluated
	cloneDirCmd := exec.Command("git", "clone", *t.Template.Url, opts.Directory)
	if err := cloneDirCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone templates repository: %w", err)
	}

	// Remove .git folder after cloning
	gitDir := opts.Directory + "/.git"
	if err := os.RemoveAll(gitDir); err != nil {
		return fmt.Errorf("failed to remove .git directory: %w", err)
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

// CreateDefaultTemplateOptions creates default options when template is specified via flag
func CreateDefaultTemplateOptions(directory, templateName string, templates Templates) TemplateOptions {
	// Find the template by name (supports both full name and display name)
	var foundTemplate *Template
	for _, template := range templates {
		key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
		if *template.Name == templateName || key == templateName {
			foundTemplate = &template
			break
		}
	}

	if foundTemplate == nil {
		return TemplateOptions{} // Empty options indicate template not found
	}

	currentUser, err := user.Current()
	author := "blaxel"
	if err == nil {
		author = currentUser.Username
	}

	return TemplateOptions{
		ProjectName:  directory, // Use directory name as default project name
		Directory:    directory,
		TemplateName: *foundTemplate.Name,
		Language:     foundTemplate.Language,
		Author:       author,
	}
}
