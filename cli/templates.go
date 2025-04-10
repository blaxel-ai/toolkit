package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

var GH_ORGANIZATION = "beamlit"
var GH_BRANCH = "main"

type Templates []Template

type Template struct {
	Language string   `json:"language"`
	Name     string   `json:"name"`
	Topics   []string `json:"topics"`
	Id       int      `json:"id"`
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

type TemplateOptions struct {
	Directory       string             // Target directory for the new agent app
	ProjectName     string             // Name of the project
	ProjectPrompt   string             // Description of the project
	Language        string             // Language to use for the project
	Template        Template           // Template to use for the project
	Author          string             // Author of the project
	TemplateOptions map[string]*string // Options for the template
	IgnoreFiles     map[string]IgnoreFile
	IgnoreDirs      map[string]IgnoreDir
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
	url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=40", GH_ORGANIZATION)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating github request: %w", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error: received non-200 response code")
	}

	var repos Templates
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("error decoding github response: %w", err)
	}

	for _, repo := range repos {
		if slices.Contains(repo.Topics, templateType) {
			templates = append(templates, repo)
		}
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Id > templates[j].Id
	})
	return templates, nil
}

func (t Templates) getLanguages() []string {
	languages := []string{}
	for _, template := range t {
		if !slices.Contains(languages, template.Language) {
			languages = append(languages, template.Language)
		}
	}
	return languages
}

func (t Templates) filterByLanguage(language string) Templates {
	filtered := Templates{}
	for _, template := range t {
		if template.Language == language {
			filtered = append(filtered, template)
		}
	}
	return filtered
}

func (t Templates) find(language string, name string) Template {
	for _, template := range t {
		if template.Language == language && template.Name == name {
			return template
		}
	}
	return Template{}
}

func (t Template) getConfig() (TemplateConfig, error) {
	var templateConfig TemplateConfig
	url := fmt.Sprintf("https://raw.githubusercontent.com/beamlit/%s/refs/heads/%s/template.yaml", t.Name, GH_BRANCH)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return templateConfig, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return templateConfig, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return templateConfig, err
	}

	err = yaml.Unmarshal(body, &templateConfig)
	if err != nil {
		return templateConfig, err
	}
	return templateConfig, nil
}

func (t Template) getUrl() string {
	return fmt.Sprintf("https://github.com/beamlit/%s.git", t.Name)
}

func (t Template) Clone(opts TemplateOptions) error {
	// Create project directory
	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		return err
	}

	// We clone in a tmp dir, cause the template can contain variables and they will be evaluated
	cloneDir := filepath.Join(opts.Directory, "templates")
	cloneDirCmd := exec.Command("git", "clone", t.getUrl(), cloneDir)
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
	ignoreFolders := []string{".git"}
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

	// Apply the template variables to the template files
	err := filepath.Walk(cloneDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		rel, err := filepath.Rel(cloneDir, path)
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
	case "Python":
		if err := installPythonDependencies(opts.Directory); err != nil {
			return err
		}
	case "TypeScript":
		if err := installTypescriptDependencies(opts.Directory); err != nil {
			return err
		}
	}

	return nil
}
