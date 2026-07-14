package core

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/charmbracelet/huh"
)

const (
	sandboxScratchTemplate    = "template-sandbox-scratch"
	sandboxClaudeCodeTemplate = "template-sandbox-claude-code"
	sandboxCodexTemplate      = "template-sandbox-codex"
	sandboxCodegenTemplate    = "template-sandbox-codegen"

	sandboxCodegenTemplateURL    = "https://github.com/blaxel-templates/template-sandbox-codegen.git"
	sandboxClaudeCodeTemplateURL = "https://github.com/blaxel-templates/template-sandbox-claude-code.git"
)

func ensureSandboxTemplates(templates Templates) Templates {
	existing := map[string]bool{}
	for _, template := range templates {
		existing[template.Name] = true
	}

	baseURL := sandboxTemplateURL(templates, sandboxCodegenTemplate)
	if baseURL == "" {
		baseURL = sandboxCodegenTemplateURL
	}

	additions := Templates{}
	for _, template := range []Template{
		sandboxCatalogTemplate(sandboxScratchTemplate, "Scratch sandbox", "Start from a basic sandbox workspace.", baseURL),
		sandboxCatalogTemplate(sandboxClaudeCodeTemplate, "Claude Code sandbox", "Sandbox with Claude Code available as claude on PATH.", sandboxClaudeCodeTemplateURL),
		sandboxCatalogTemplate(sandboxCodexTemplate, "Codex sandbox", "Sandbox with Codex available as codex on PATH.", baseURL),
	} {
		if !existing[template.Name] {
			additions = append(additions, template)
		}
	}

	return append(additions, templates...)
}

func sandboxTemplateURL(templates Templates, name string) string {
	for _, template := range templates {
		if template.Name == name {
			return template.URL
		}
	}
	return ""
}

func sandboxCatalogTemplate(name string, description string, longDescription string, url string) Template {
	return Template{
		Template: blaxel.Template{
			Name:        name,
			Description: fmt.Sprintf("%s. %s", description, longDescription),
			URL:         url,
			Topics:      []string{"sandbox"},
		},
		Type: "sandbox",
	}
}

func sandboxTemplateAlias(templateNameFlag string) (string, bool) {
	key := strings.TrimPrefix(strings.ToLower(templateNameFlag), "template-")
	switch key {
	case "scratch", "sandbox-scratch":
		return sandboxScratchTemplate, true
	case "claude", "claude-code", "sandbox-claude-code":
		return sandboxClaudeCodeTemplate, true
	case "codex", "sandbox-codex":
		return sandboxCodexTemplate, true
	default:
		return "", false
	}
}

func printSandboxTemplateLine(t Template) {
	name := sandboxTemplateFlagName(t)
	if t.Description != "" {
		PrintDiagnostic(fmt.Sprintf("  - %-30s %s", name, t.Description))
	} else {
		PrintDiagnostic(fmt.Sprintf("  - %s", name))
	}
}

func sandboxTemplateFlagName(t Template) string {
	switch t.Name {
	case sandboxScratchTemplate:
		return "scratch"
	case sandboxClaudeCodeTemplate:
		return "claude-code"
	case sandboxCodexTemplate:
		return "codex"
	default:
		return strings.TrimPrefix(templateDisplayName(t), "template-")
	}
}

type sandboxTemplateVariant struct {
	title       string
	flagName    string
	description string
	imageName   string
	packageName string
	commandName string
}

func sandboxTemplateVariantFor(templateName string) (sandboxTemplateVariant, bool) {
	switch templateName {
	case sandboxScratchTemplate:
		return sandboxTemplateVariant{
			title:       "Scratch",
			flagName:    "scratch",
			description: "A basic Blaxel sandbox with a Next.js workspace and the sandbox API.",
			imageName:   "blaxel-sandbox-scratch",
		}, true
	case sandboxClaudeCodeTemplate:
		return sandboxTemplateVariant{
			title:       "Claude Code",
			flagName:    "claude-code",
			description: "A Blaxel sandbox with Claude Code installed and available on PATH as `claude`.",
			imageName:   "blaxel-sandbox-claude-code",
			packageName: "@anthropic-ai/claude-code",
			commandName: "claude",
		}, true
	case sandboxCodexTemplate:
		return sandboxTemplateVariant{
			title:       "Codex",
			flagName:    "codex",
			description: "A Blaxel sandbox with Codex installed and available on PATH as `codex`.",
			imageName:   "blaxel-sandbox-codex",
			packageName: "@openai/codex",
			commandName: "codex",
		}, true
	default:
		return sandboxTemplateVariant{}, false
	}
}

func PromptSandboxTemplateOptions(directory string, templates Templates) TemplateOptions {
	options := TemplateOptions{
		ProjectName:  directory,
		Directory:    directory,
		TemplateName: "",
	}

	currentUser, err := user.Current()
	if err == nil {
		options.Author = currentUser.Username
	} else {
		options.Author = "blaxel"
	}

	fields := []huh.Field{}
	if directory == "" {
		fields = append(fields, huh.NewInput().
			Title("Name").
			Description("Name of your sandbox").
			Value(&options.Directory),
		)
	}

	templateOptions := []huh.Option[string]{}
	for _, t := range sandboxTemplatesForDisplay(templates) {
		templateOptions = append(templateOptions, huh.NewOption(sandboxTemplateLabel(t), t.Name))
	}

	tmplSelect := huh.NewSelect[string]().
		Title("Sandbox type").
		Description("Choose the sandbox to scaffold.").
		Options(templateOptions...).
		Value(&options.TemplateName)
	if len(templateOptions) > 5 {
		tmplSelect = tmplSelect.Height(5)
	}
	fields = append(fields, tmplSelect)

	form := huh.NewForm(huh.NewGroup(fields...))
	form.WithTheme(GetHuhTheme())
	if err := form.Run(); err != nil {
		os.Exit(0)
	}
	options.ProjectName = options.Directory
	return options
}

func sandboxTemplatesForDisplay(templates Templates) Templates {
	knownTemplates := Templates{}
	remainingTemplates := Templates{}

	for _, name := range []string{sandboxScratchTemplate, sandboxClaudeCodeTemplate, sandboxCodexTemplate} {
		for _, t := range templates {
			if t.Name == name {
				knownTemplates = append(knownTemplates, t)
				break
			}
		}
	}

	if len(knownTemplates) > 0 {
		return knownTemplates
	}

	remainingTemplates = append(remainingTemplates, templates...)
	return remainingTemplates
}

func sandboxTemplateLabel(t Template) string {
	switch t.Name {
	case sandboxScratchTemplate:
		return "Scratch"
	case sandboxClaudeCodeTemplate:
		return "Claude Code"
	case sandboxCodexTemplate:
		return "Codex"
	default:
		return strings.TrimPrefix(templateDisplayName(t), "template-")
	}
}

func FinalizeSandboxTemplate(opts TemplateOptions) error {
	variant, ok := sandboxTemplateVariantFor(opts.TemplateName)
	if !ok {
		return nil
	}

	if err := writeSandboxDockerfile(opts.Directory, variant); err != nil {
		return err
	}
	if err := writeSandboxMakefile(opts.Directory, variant); err != nil {
		return err
	}
	if err := writeSandboxEntrypoint(opts.Directory); err != nil {
		return err
	}
	return writeSandboxReadme(opts.Directory, variant)
}

func writeSandboxDockerfile(dir string, variant sandboxTemplateVariant) error {
	installLine := ""
	if variant.packageName != "" {
		installLine = fmt.Sprintf("\n\nRUN npm install -g %s@latest", variant.packageName)
	}

	content := fmt.Sprintf(`FROM node:22-alpine

RUN apk update && apk add --no-cache \
  bash \
  git \
  curl \
  netcat-openbsd \
  ripgrep \
  && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=ghcr.io/blaxel-ai/sandbox:latest /sandbox-api /usr/local/bin/sandbox-api

RUN mkdir -p /app \
  && npx create-next-app@latest /app --use-npm --typescript --eslint --tailwind --src-dir --app --import-alias "@/*" --no-git --yes --no-turbopack%s

EXPOSE 3000

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV PATH="/usr/local/bin:/app/node_modules/.bin:$PATH"

ENTRYPOINT ["/entrypoint.sh"]
`, installLine)

	target, stale := sandboxDockerfileTarget(dir)
	for _, path := range stale {
		if err := removeSandboxFile(dir, filepath.Base(path)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove duplicate Dockerfile %s: %w", path, err)
		}
	}
	if err := writeSandboxFile(dir, filepath.Base(target), []byte(content), 0644); err != nil {
		return err
	}
	return nil
}

func sandboxDockerfileTarget(dir string) (string, []string) {
	upper := filepath.Join(dir, "Dockerfile")
	lower := filepath.Join(dir, "dockerfile")
	upperInfo, upperExists := fileInfo(upper)
	lowerInfo, lowerExists := fileInfo(lower)

	switch {
	case upperExists:
		if lowerExists {
			if os.SameFile(upperInfo, lowerInfo) {
				return upper, nil
			}
			return upper, []string{lower}
		}
		return upper, nil
	case lowerExists:
		return lower, nil
	default:
		return upper, nil
	}
}

func fileInfo(path string) (os.FileInfo, bool) {
	info, err := os.Lstat(path)
	return info, err == nil && !info.IsDir()
}

func removeSandboxFile(dir, name string) error {
	if err := validateSandboxFileName(name); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to open sandbox directory: %w", err)
	}
	defer func() { _ = root.Close() }()
	return root.Remove(name)
}

func writeSandboxFile(dir, name string, data []byte, perm os.FileMode) error {
	if err := validateSandboxFileName(name); err != nil {
		return err
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to open sandbox directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	info, err := root.Lstat(name)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write %s: target is a symlink", name)
		}
		if info.IsDir() {
			return fmt.Errorf("refusing to write %s: target is a directory", name)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect %s: %w", name, err)
	}

	if err := root.WriteFile(name, data, perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", name, err)
	}
	return nil
}

func validateSandboxFileName(name string) error {
	if name == "" || name == "." || filepath.IsAbs(name) || name != filepath.Base(name) {
		return fmt.Errorf("invalid sandbox file target %q", name)
	}
	return nil
}

func writeSandboxMakefile(dir string, variant sandboxTemplateVariant) error {
	content := fmt.Sprintf(`build:
	docker build --platform=linux/amd64 -t %[1]s .

run:
	docker rm -f %[1]s || true
	docker run --platform=linux/amd64 -p 8080:8080 -p 3000:3000 --name %[1]s %[1]s
`, variant.imageName)

	if err := writeSandboxFile(dir, "Makefile", []byte(content), 0644); err != nil {
		return err
	}
	return nil
}

func writeSandboxEntrypoint(dir string) error {
	content := `#!/bin/sh

export PATH="/usr/local/bin:/app/node_modules/.bin:$PATH"

/usr/local/bin/sandbox-api &

wait_for_port() {
    port=$1
    timeout=30
    count=0

    echo "Waiting for port $port to be available..."

    while ! nc -z 127.0.0.1 "$port"; do
        sleep 1
        count=$((count + 1))
        if [ "$count" -gt "$timeout" ]; then
            echo "Timeout waiting for port $port"
            exit 1
        fi
    done

    echo "Port $port is now available"
}

wait_for_port 8080

echo "Running Next.js dev server..."
curl http://localhost:8080/process -X POST -d '{"workingDir": "/app", "command": "npm run dev -- --port 3000", "waitForCompletion": false}' -H "Content-Type: application/json"

wait
`

	path := filepath.Join(dir, "entrypoint.sh")
	if err := writeSandboxFile(dir, filepath.Base(path), []byte(content), 0755); err != nil {
		return err
	}
	return nil
}

func writeSandboxReadme(dir string, variant sandboxTemplateVariant) error {
	commandCheck := ""
	if variant.commandName != "" {
		commandCheck = fmt.Sprintf("\n# Check the agent CLI inside the sandbox image or connected sandbox\n%s --version\n", variant.commandName)
	}

	content := fmt.Sprintf(`# Blaxel %s Sandbox

%s

## Quick Start

`+"```bash"+`
bl new sandbox my-sandbox -t %s -y
cd my-sandbox
bl deploy
bl connect sandbox my-sandbox%s
`+"```"+`

## Local Docker

`+"```bash"+`
make build
make run
`+"```"+`

## Project Files

- `+"`Dockerfile`"+` builds the sandbox image.
- `+"`entrypoint.sh`"+` starts the Blaxel sandbox API and the Next.js dev server.
- `+"`blaxel.toml`"+` configures the Blaxel sandbox runtime.
`, variant.title, variant.description, variant.flagName, commandCheck)

	if err := writeSandboxFile(dir, "README.md", []byte(content), 0644); err != nil {
		return err
	}
	return nil
}
