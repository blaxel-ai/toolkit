package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/charmbracelet/huh/spinner"
)

type Templates []Template
type Template struct {
	blaxel.Template
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
	client := GetClient()
	if client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	resp, err := client.Templates.List(context.Background())
	if err != nil {
		// Check if it's an authentication error
		errMsg := err.Error()
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") {
			return nil, fmt.Errorf("Authentication required. Please log in to your workspace using 'bl login'.\nIf you don't have a workspace yet, visit https://app.blaxel.ai to create one")
		}
		return nil, err
	}
	if resp == nil {
		return templates, nil
	}
	for _, template := range *resp {
		if len(template.Topics) > 0 {
			topics := template.Topics
			if slices.Contains(topics, templateType) {
				language := ""
				if slices.Contains(topics, "python") {
					language = "python"
				} else if slices.Contains(topics, "typescript") {
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

// RetrieveTemplatesWithSpinner retrieves templates with optional spinner based on noTTY flag
func RetrieveTemplatesWithSpinner(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
	var templateError error
	var templates Templates

	if noTTY {
		templates, templateError = RetrieveTemplates(templateType)
		if templateError != nil {
			PrintError(errorPrefix, templateError)
			return nil, templateError
		}
	} else {
		spinnerErr := spinner.New().
			Title("Retrieving templates...").
			Action(func() {
				templates, templateError = RetrieveTemplates(templateType)
			}).
			Run()
		if spinnerErr != nil {
			PrintError(errorPrefix, spinnerErr)
			return nil, spinnerErr
		}
		if templateError != nil {
			PrintError(errorPrefix, templateError)
			return nil, templateError
		}
	}

	if len(templates) == 0 {
		err := fmt.Errorf("no %s templates available. Please contact support", templateType)
		PrintError(errorPrefix, err)
		return nil, err
	}

	return templates, nil
}

// CloneTemplateWithSpinner clones a template with optional spinner based on noTTY flag
func CloneTemplateWithSpinner(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
	template, err := templates.Find(opts.TemplateName)
	if err != nil {
		PrintError(errorPrefix, fmt.Errorf("template not found: %w", err))
		return err
	}

	// Use different installation methods based on TTY
	if noTTY {
		// Use the simple Clone method for non-interactive environments
		if err := template.Clone(opts); err != nil {
			PrintError(errorPrefix, err)
			return err
		}
	} else {
		// Use the new Bubble Tea installation UI for interactive environments
		if err := RunInstallationWithTea(template, opts); err != nil {
			PrintError(errorPrefix, err)
			return err
		}
	}

	return nil
}

func CleanTemplate(dir string) {
	// List of files and folders to remove
	itemsToRemove := []string{
		".github",
		"icon.png",
		"icon-dark.png",
		"LICENSE",
		".devcontainer",
	}

	for _, item := range itemsToRemove {
		itemPath := filepath.Join(dir, item)
		_ = os.RemoveAll(itemPath)
	}
}

// editBlaxelTomlInCurrentDir checks if blaxel.toml exists in the current working directory
// and appends text to it. If the file doesn't exist, it returns without error.
func EditBlaxelTomlInCurrentDir(resourceType string, resourceName string, resourcePath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	blaxelTomlPath := filepath.Join(cwd, "blaxel.toml")

	// Check if blaxel.toml exists
	if _, err := os.Stat(blaxelTomlPath); os.IsNotExist(err) {
		// File doesn't exist, nothing to edit
		return nil
	}

	// Read existing file to find used ports
	existingContent, err := os.ReadFile(blaxelTomlPath)
	if err != nil {
		return fmt.Errorf("failed to read blaxel.toml: %w", err)
	}

	// Find the next available port starting from 1340
	nextPort := findNextAvailablePort(string(existingContent))

	// Open file for appending
	file, err := os.OpenFile(blaxelTomlPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open blaxel.toml for appending: %w", err)
	}
	defer file.Close() //nolint:errcheck

	// Append the new resource configuration
	textToAppend := fmt.Sprintf("\n[%s.%s]\npath = \"%s\"\nport = %d\n", resourceType, resourceName, resourcePath, nextPort)
	_, err = file.WriteString(textToAppend)
	if err != nil {
		return fmt.Errorf("failed to append to blaxel.toml: %w", err)
	}

	return nil
}

// findNextAvailablePort parses the TOML content and finds the next available port starting from 1340
func findNextAvailablePort(content string) int {
	usedPorts := make(map[int]bool)

	// Use regex to find all port assignments
	portRegex := regexp.MustCompile(`port\s*=\s*(\d+)`)
	matches := portRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			var port int
			_, _ = fmt.Sscanf(match[1], "%d", &port)
			usedPorts[port] = true
		}
	}

	// Find the next available port starting from 1340
	port := 1339
	for usedPorts[port] {
		port++
	}

	return port
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
		if template.Name == name {
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
	env := os.Getenv("BL_ENV")
	branch := "main"
	if env == "dev" || env == "local" {
		branch = "develop"
	}
	if !isCommandAvailable("git") {
		return fmt.Errorf("git is not available on your system. Please install git and try again")
	}
	// We clone in a tmp dir, cause the template can contain variables and they will be evaluated
	cloneDirCmd := exec.Command("git", "clone", "-b", branch, t.URL, opts.Directory)
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
		key := regexp.MustCompile(`^\d+-`).ReplaceAllString(template.Name, "")
		if template.Name == templateName || key == templateName {
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
		TemplateName: foundTemplate.Name,
		Language:     foundTemplate.Language,
		Author:       author,
	}
}
