package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

func formatOperationId(operationId string) []string {
	// Regular expression to match capital letters
	re := regexp.MustCompile(`[A-Z][^A-Z]*`)

	// Find all matches and convert them to lowercase
	words := re.FindAllString(operationId, -1)
	for i, word := range words {
		words[i] = strings.ToLower(word)
	}

	return []string{words[0], strings.Join(words[1:], "")}
}

func getResults(action string, filePath string, recursive bool) ([]Result, error) {
	return getResultsWrapper(action, filePath, recursive, 0)
}

func handleSecret(filePath string, content string) (string, error) {
	// Check if content contains IntegrationConnection, this is to find if the resource already exists
	// If it exists then we skip the secrets handling
	if strings.Contains(content, "kind: IntegrationConnection") {
		// Find name in metadata
		nameRegex := regexp.MustCompile(`metadata:\n.*name:\s*([^\n]+)`)
		nameMatch := nameRegex.FindStringSubmatch(content)
		if len(nameMatch) > 1 {
			name := strings.TrimSpace(nameMatch[1])
			response, err := client.GetIntegrationConnectionWithResponse(context.Background(), name)
			if err == nil && response.StatusCode() == 200 {
				return content, nil
			}
		}
	}

	fileName := strings.Split(filePath, "/")[len(strings.Split(filePath, "/"))-1]
	re := regexp.MustCompile(`\$secrets.([A-Za-z0-9_]+)|\${\s?secrets.([A-Za-z0-9_]+)\s?}`)
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}
	values := map[string]*string{}
	fields := []huh.Field{}
	i := 0
	for _, match := range matches {
		var value string
		fullMatch := match[0]
		secretName := match[1]
		if secretName == "" {
			secretName = match[2]
		}
		values[fullMatch] = &value
		if envValue, exists := os.LookupEnv(secretName); exists {
			value = envValue
		} else if secretValue := LookupSecret(secretName); secretValue != "" {
			value = secretValue
		} else {
			title := fmt.Sprintf("name: %s", secretName)
			if i == 0 {
				title = fmt.Sprintf("Secrets for %s\nname: %s", fileName, secretName)
			}
			fields = append(fields, huh.NewInput().
				Title(title).
				Value(&value))
			i += 1
		}
	}
	if len(fields) > 0 {
		formTemplates := huh.NewForm(
			huh.NewGroup(fields...),
		)
		formTemplates.WithTheme(GetHuhTheme())
		err := formTemplates.Run()
		if err != nil {
			return content, fmt.Errorf("error handling secret: %v", err)
		}
	}
	for key, value := range values {
		content = strings.ReplaceAll(content, key, *value)
	}
	return content, nil
}

func getResultsWrapper(action string, filePath string, recursive bool, n int) ([]Result, error) {
	var reader io.Reader
	var results []Result
	// Choisir la source (stdin ou fichier)
	if filePath == "-" {
		reader = os.Stdin
	} else {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("error getting file info: %v", err)
		}
		// If the path is a directory, read all files in the directory
		if fileInfo.IsDir() {
			if n > 0 && !recursive && strings.Contains(filePath, "/") {
				return nil, nil
			}
			return handleDirectory(action, filePath, recursive, n)
		}
		// Skip non-YAML files
		if !strings.HasSuffix(strings.ToLower(filePath), ".yml") && !strings.HasSuffix(strings.ToLower(filePath), ".yaml") {
			return nil, nil
		}
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("error opening file: %v", err)
		}
		defer func() { _ = file.Close() }()
		reader = file
	}
	// Read the entire content as a string first
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading content: %v", err)
	}

	contentStr := string(content)
	if action == "apply" {
		// Replace env variables in the content
		re := regexp.MustCompile(`\$([A-Za-z0-9_]+)|\${([A-Za-z0-9_]+)}`)
		contentStr = re.ReplaceAllStringFunc(contentStr, func(match string) string {
			// Remove $, ${, and } to get the env var name
			varName := match
			varName = strings.TrimPrefix(varName, "$")
			varName = strings.TrimPrefix(varName, "{")
			varName = strings.TrimSuffix(varName, "}")

			if value, exists := os.LookupEnv(varName); exists {
				return value
			} else if secretValue := LookupSecret(varName); secretValue != "" {
				return secretValue
			}
			return match // Keep original if env var not found
		})
		contentStr, err = handleSecret(filePath, contentStr)
		if err != nil {
			return nil, fmt.Errorf("error handling secret: %v", err)
		}
	}
	// Lire et parser les documents YAML
	decoder := yaml.NewDecoder(strings.NewReader(contentStr))
	for {
		var result Result
		err := decoder.Decode(&result)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error decoding YAML: %v", err)
		}
		results = append(results, result)
	}
	return results, nil
}

func handleDirectory(action string, filePath string, recursive bool, n int) ([]Result, error) {
	var results []Result
	files, err := os.ReadDir(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory %s: %v", filePath, err)
	}

	for _, file := range files {
		path := fmt.Sprintf("%s/%s", filePath, file.Name())
		fileResults, err := getResultsWrapper(action, path, recursive, n+1)
		if err != nil {
			Print(fmt.Sprintf("error getting results for file %s: %v", path, err))
			continue
		}
		results = append(results, fileResults...)
	}
	return results, nil
}

func ModuleLanguage(directory string) string {
	if _, err := os.Stat(filepath.Join(directory, "pyproject.toml")); !os.IsNotExist(err) {
		return "python"
	} else if _, err := os.Stat(filepath.Join(directory, "package.json")); !os.IsNotExist(err) {
		return "typescript"
	} else if _, err := os.Stat(filepath.Join(directory, "go.mod")); !os.IsNotExist(err) {
		return "go"
	}
	return ""
}

// getTheme returns a custom theme configuration for the CLI interface using the Dracula color scheme.
// It customizes various UI elements like buttons, text inputs, and selection indicators.
func GetHuhTheme() *huh.Theme {
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
	t.Blurred.Base = t.Blurred.Base.BorderForeground(comment)
	t.Blurred.NextIndicator = t.Blurred.NextIndicator.Foreground(comment)
	t.Blurred.PrevIndicator = t.Blurred.PrevIndicator.Foreground(comment)

	t.Blurred.TextInput.Prompt = t.Blurred.TextInput.Prompt.Foreground(comment)
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(foreground)

	t.Help.ShortKey = t.Help.ShortKey.Foreground(comment)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(foreground)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(comment)
	t.Help.FullKey = t.Help.FullKey.Foreground(comment)
	t.Help.FullDesc = t.Help.FullDesc.Foreground(foreground)
	t.Help.FullSeparator = t.Help.FullSeparator.Foreground(comment)

	return t
}

// PrintError prints a formatted error message with colors
func PrintError(operation string, err error) {
	// Print error header with red color and bold
	Print(fmt.Sprintf("%s %s\n",
		color.New(color.FgRed, color.Bold).Sprint("✗"),
		color.New(color.FgRed, color.Bold).Sprintf("%s failed", operation)))

	// Print reason with lighter red color
	Print(fmt.Sprintf("%s %s\n",
		color.New(color.FgRed).Sprint("Reason:"),
		color.New(color.FgWhite).Sprint(err.Error())))
}

// PrintWarning prints a formatted warning message with colors
func PrintWarning(message string) {
	Print(fmt.Sprintf("%s %s\n",
		color.New(color.FgYellow, color.Bold).Sprint("⚠"),
		color.New(color.FgYellow).Sprint(message)))
}

// PrintSuccess prints a formatted success message with colors
func PrintSuccess(message string) {
	Print(fmt.Sprintf("%s %s\n",
		color.New(color.FgGreen, color.Bold).Sprint("✓"),
		color.New(color.FgGreen).Sprint(message)))
}

func PrintInfo(message string) {
	Print(fmt.Sprintf("%s %s\n",
		color.New(color.FgBlue, color.Bold).Sprint("ℹ"),
		color.New(color.FgBlue).Sprint(message)))
}

func Print(message string) {
	if IsInteractiveMode() {
		return
	}
	message = strings.TrimSuffix(message, "\n")
	fmt.Println(message)
}

// Slugify converts a string to a URL-safe slug format
// Example: "My Agent 123!" -> "my-agent-123"
func Slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove any character that's not alphanumeric or hyphen
	re := regexp.MustCompile(`[^a-z0-9\-]+`)
	s = re.ReplaceAllString(s, "")

	// Remove consecutive hyphens
	re = regexp.MustCompile(`\-+`)
	s = re.ReplaceAllString(s, "-")

	// Trim hyphens from start and end
	s = strings.Trim(s, "-")

	// If empty after slugification, generate a default
	if s == "" {
		s = "resource"
	}

	return s
}

// Pluralize returns a basic English plural of a given singular noun
func Pluralize(word string) string {
	lower := strings.ToLower(word)

	switch {
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh"):
		return word + "es"
	case strings.HasSuffix(lower, "y") && !strings.HasSuffix(lower, "ay") && !strings.HasSuffix(lower, "ey") && !strings.HasSuffix(lower, "iy") && !strings.HasSuffix(lower, "oy") && !strings.HasSuffix(lower, "uy"):
		return word[:len(word)-1] + "ies"
	default:
		return word + "s"
	}
}

// GetResults is a public wrapper for getResults
func GetResults(action string, filePath string, recursive bool) ([]Result, error) {
	return getResults(action, filePath, recursive)
}

// GetResources returns the resources slice
func GetResources() []*Resource {
	return resources
}

func IsVolumeTemplate(resourceType string) bool {
	if resourceType == "volumetemplate" {
		return true
	}
	if resourceType == "volume-template" {
		return true
	}
	if resourceType == "vt" {
		return true
	}
	return false
}
