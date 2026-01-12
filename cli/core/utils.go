package core

import (
	"context"
	"encoding/json"
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
			_, err := client.Integrations.Connections.Get(context.Background(), name)
			if err == nil {
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
	pythonFiles := []string{
		"pyproject.toml",
		"requirements.txt",
	}
	for _, f := range pythonFiles {
		if _, err := os.Stat(filepath.Join(directory, f)); !os.IsNotExist(err) {
			return "python"
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "package.json")); !os.IsNotExist(err) {
		return "typescript"
	}
	if _, err := os.Stat(filepath.Join(directory, "go.mod")); !os.IsNotExist(err) {
		return "go"
	}
	// Check if Python entry files exist
	if HasPythonEntryFile(directory) {
		return "python"
	}
	return ""
}

// HasPythonEntryFile checks if common Python entry files exist in the given directory
func HasPythonEntryFile(directory string) bool {
	files := []string{
		"app.py",
		"main.py",
		"api.py",
		"app/main.py",
		"app/app.py",
		"app/api.py",
		"src/main.py",
		"src/app.py",
		"src/api.py",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(directory, f)); err == nil {
			return true
		}
	}
	return false
}

// HasGoEntryFile checks if common Go entry files exist in the given directory
func HasGoEntryFile(directory string) bool {
	files := []string{
		"main.go",
		"src/main.go",
		"cmd/main.go",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(directory, f)); err == nil {
			return true
		}
	}
	return false
}

// HasTypeScriptEntryFile checks if common TypeScript/JavaScript entry files exist in the given directory
// or if package.json has a start script defined
func HasTypeScriptEntryFile(directory string) bool {
	// Check for common entry files
	files := []string{
		"index.js",
		"app.js",
		"server.js",
		"src/index.js",
		"src/app.js",
		"src/server.js",
		"dist/index.js",
		"dist/app.js",
		"dist/server.js",
		"build/index.js",
		"build/app.js",
		"build/server.js",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(directory, f)); err == nil {
			return true
		}
	}

	// Check if package.json has a start script
	packageJsonPath := filepath.Join(directory, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		content, err := os.ReadFile(packageJsonPath)
		if err == nil {
			var packageJson map[string]interface{}
			if err := json.Unmarshal(content, &packageJson); err == nil {
				if scripts, ok := packageJson["scripts"].(map[string]interface{}); ok {
					if _, hasStart := scripts["start"]; hasStart {
						return true
					}
				}
			}
		}
	}

	return false
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

// PrintInfoWithCommand prints an info message followed by a command in white
func PrintInfoWithCommand(message string, command string) {
	Print(fmt.Sprintf("%s %s %s\n",
		color.New(color.FgBlue, color.Bold).Sprint("ℹ"),
		color.New(color.FgBlue).Sprint(message),
		color.New(color.FgWhite, color.Bold).Sprint(command)))
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

// GetDeployDocURL returns the documentation URL for deploying a specific resource type
func GetDeployDocURL(resourceType string) string {
	switch resourceType {
	case "agent":
		return "https://docs.blaxel.ai/Agents/Deploy-an-agent"
	case "function":
		return "https://docs.blaxel.ai/Functions/Deploy-a-function"
	case "job":
		return "https://docs.blaxel.ai/Jobs/Deploy-a-job"
	default:
		return "https://docs.blaxel.ai/Agents/Deploy-an-agent"
	}
}

// CheckServerEnvUsage scans project files to check if the code uses HOST/PORT
// or BL_SERVER_HOST/BL_SERVER_PORT environment variables.
// Returns true if any of these patterns are found in the code.
func CheckServerEnvUsage(folder string, language string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}

	projectDir := filepath.Join(cwd, folder)

	// Define file extensions based on language
	var extensions []string
	switch language {
	case "python":
		extensions = []string{".py"}
	case "typescript":
		extensions = []string{".ts", ".js", ".tsx", ".jsx"}
	case "go":
		extensions = []string{".go"}
	default:
		// If language is unknown, check all common extensions
		extensions = []string{
			".py",                                        // Python
			".ts", ".js", ".tsx", ".jsx", ".mjs", ".mts", // TypeScript/JavaScript
			".go",      // Go
			".rs",      // Rust
			".c", ".h", // C
			".cpp", ".cc", ".cxx", ".hpp", // C++
			".cs",         // C#
			".java",       // Java
			".kt", ".kts", // Kotlin
			".rb",         // Ruby
			".php",        // PHP
			".swift",      // Swift
			".scala",      // Scala
			".ex", ".exs", // Elixir
		}
	}

	// Patterns to search for (both current and deprecated env vars)
	patterns := []string{
		"HOST",
		"PORT",
		"BL_SERVER_HOST",
		"BL_SERVER_PORT",
	}

	found := false
	_ = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			// Skip common non-source directories
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == ".venv" || name == "venv" ||
				name == "__pycache__" || name == "dist" || name == "build" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file has a matching extension
		ext := filepath.Ext(path)
		hasMatchingExt := false
		for _, e := range extensions {
			if ext == e {
				hasMatchingExt = true
				break
			}
		}
		if !hasMatchingExt {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		// Check for patterns
		for _, pattern := range patterns {
			if strings.Contains(contentStr, pattern) {
				found = true
				return filepath.SkipAll // Stop walking once we find a match
			}
		}
		return nil
	})
	return found
}

// BuildServerEnvWarning returns a formatted warning message with language-specific
// sample code for using HOST and PORT environment variables.
func BuildServerEnvWarning(language string, resourceType string) string {
	codeColor := color.New(color.FgCyan)

	var warningMsg strings.Builder
	warningMsg.WriteString("⚠️  Server Configuration Warning\n")
	warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	warningMsg.WriteString(fmt.Sprintf("Your code does not appear to use the %s and %s environment variables.\n",
		codeColor.Sprint("HOST"), codeColor.Sprint("PORT")))
	warningMsg.WriteString("Blaxel injects these variables at runtime to configure your server.\n\n")

	warningMsg.WriteString("Sample code to read these variables:\n\n")

	switch language {
	case "python":
		warningMsg.WriteString(codeColor.Sprint("import os\n\n"))
		warningMsg.WriteString(codeColor.Sprint("host = os.environ.get(\"HOST\", \"0.0.0.0\")\n"))
		warningMsg.WriteString(codeColor.Sprint("port = int(os.environ.get(\"PORT\", \"80\"))\n"))
	case "typescript":
		warningMsg.WriteString(codeColor.Sprint("const host = process.env.HOST || \"0.0.0.0\";\n"))
		warningMsg.WriteString(codeColor.Sprint("const port = parseInt(process.env.PORT || \"80\", 10);\n"))
	case "go":
		warningMsg.WriteString(codeColor.Sprint("import \"os\"\n\n"))
		warningMsg.WriteString(codeColor.Sprint("host := os.Getenv(\"HOST\")\n"))
		warningMsg.WriteString(codeColor.Sprint("if host == \"\" {\n"))
		warningMsg.WriteString(codeColor.Sprint("    host = \"0.0.0.0\"\n"))
		warningMsg.WriteString(codeColor.Sprint("}\n"))
		warningMsg.WriteString(codeColor.Sprint("port := os.Getenv(\"PORT\")\n"))
		warningMsg.WriteString(codeColor.Sprint("if port == \"\" {\n"))
		warningMsg.WriteString(codeColor.Sprint("    port = \"80\"\n"))
		warningMsg.WriteString(codeColor.Sprint("}\n"))
	default:
		warningMsg.WriteString(codeColor.Sprint("# Read HOST and PORT from environment variables\n"))
		warningMsg.WriteString(codeColor.Sprint("# HOST defaults to 0.0.0.0\n"))
		warningMsg.WriteString(codeColor.Sprint("# PORT defaults to 80\n"))
	}

	warningMsg.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	warningMsg.WriteString(fmt.Sprintf("Learn more: %s\n\n", GetDeployDocURL(resourceType)))
	warningMsg.WriteString("⚠️  Without reading these variables, your deployment may not bind to the correct address.\n")
	warningMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return warningMsg.String()
}
