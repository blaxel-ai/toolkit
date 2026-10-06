package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

// deployDockerfile is opt-in state, never inferred by the shared archive writer.
// The paths are resolved once so validation and upload use the same files.
type deployDockerfile struct {
	path       string
	ignorePath string
}

func deployInputError(format string, args ...any) error {
	return core.MarkExpectedError(fmt.Errorf(format, args...), core.CLIErrorValidation)
}

func resolveDeployDockerfile(cwd, folder, flag string, config core.Config, skipBuild, recursive bool) (*deployDockerfile, error) {
	if !deployBuildsSource(config, skipBuild) {
		if flag != "" {
			return nil, deployInputError("--dockerfile requires a source build (not --skip-build, a prebuilt image, or a volume template)")
		}
		return nil, nil // TOML build settings are inert without a source build.
	}

	path := flag
	if path == "" && config.Build != nil {
		path = config.Build.Dockerfile
	}
	if path == "" {
		return nil, nil // Preserve legacy Dockerfile/language detection.
	}
	if recursive && folder == "" && (len(config.Agent) > 0 || len(config.Function) > 0 || len(config.Job) > 0) {
		return nil, deployInputError("Custom Dockerfile selection requires a single project; use --recursive=false or deploy one directory with -d")
	}

	projectDir := filepath.Join(cwd, folder)
	resolved, err := resolveProjectDockerfile(projectDir, path)
	if err != nil {
		return nil, err
	}
	selected := &deployDockerfile{path: resolved}
	companion := path + ".dockerignore"
	if _, err := os.Lstat(filepath.Join(projectDir, companion)); err == nil {
		selected.ignorePath, err = resolveProjectDockerfile(projectDir, companion)
		if err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, deployInputError("Dockerfile companion %q cannot be inspected: %v", companion, err)
	}
	return selected, nil
}

func resolveProjectDockerfile(projectDir, path string) (string, error) {
	return resolveProjectFile("Dockerfile", projectDir, path)
}

// resolveProjectFile validates a user-supplied path to a regular file inside
// projectDir. kind names the file in error messages.
func resolveProjectFile(kind, projectDir, path string) (string, error) {
	// Reject Windows-rooted/volume-qualified paths on every host as well as
	// native absolute/traversal paths. IsLocal uses path components, not prefixes.
	if !filepath.IsLocal(path) || strings.HasPrefix(path, "\\") || (len(path) >= 2 && path[1] == ':') {
		return "", deployInputError("%s %q must be a relative path inside the project directory", kind, path)
	}
	root, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		return "", deployInputError("%s %q: cannot resolve project directory %q: %v", kind, path, projectDir, err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(projectDir, path))
	if errors.Is(err, fs.ErrNotExist) {
		return "", deployInputError("%s %q not found in %s", kind, path, projectDir)
	}
	if err != nil {
		return "", deployInputError("%s %q: %v", kind, path, err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return "", deployInputError("%s %q must be inside the project directory", kind, path)
	}
	if info, err := os.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		return "", deployInputError("%s %q is not a regular file", kind, path)
	}
	return resolved, nil
}

func (d *deployDockerfile) usesServerEnv() bool {
	if d == nil {
		return false
	}
	content, err := os.ReadFile(d.path)
	// HOST and PORT also match BL_SERVER_HOST and BL_SERVER_PORT.
	return err == nil && (bytes.Contains(content, []byte("HOST")) || bytes.Contains(content, []byte("PORT")))
}
