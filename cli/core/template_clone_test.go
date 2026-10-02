package core

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// templateEnvs are the BL_ENV values bl runs under. Templates must come from
// main in all of them; dev and local used to clone develop.
var templateEnvs = []string{"", "prod", "dev", "local"}

// newTemplateRepo creates a local git repository with diverging main and
// develop branches. branch.txt holds the branch name on each one.
func newTemplateRepo(t *testing.T) string {
	t.Helper()
	if !isCommandAvailable("git") {
		t.Skip("git is not available")
	}

	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=bl-test", "-c", "user.email=bl-test@example.com",
			"-c", "commit.gpgsign=false",
		}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	write := func(content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(repo, "branch.txt"), []byte(content), 0644))
	}

	git("init", "-q", "-b", "main")
	write("main")
	git("add", "branch.txt")
	git("commit", "-q", "-m", "main")
	git("checkout", "-q", "-b", "develop")
	write("develop")
	git("commit", "-q", "-am", "develop")
	git("checkout", "-q", "main")

	return repo
}

func requireClonedFromMain(t *testing.T, dir string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, "branch.txt"))
	require.NoError(t, err)
	require.Equal(t, "main", string(content))
	require.NoDirExists(t, filepath.Join(dir, ".git"))
}

func TestTemplateCloneUsesMainInEveryEnv(t *testing.T) {
	repo := newTemplateRepo(t)
	template := Template{Template: blaxel.Template{Name: "fixture", URL: repo}}

	for _, env := range templateEnvs {
		t.Run("BL_ENV="+env, func(t *testing.T) {
			t.Setenv("BL_ENV", env)
			dir := filepath.Join(t.TempDir(), "project")

			require.NoError(t, template.Clone(TemplateOptions{Directory: dir}))
			requireClonedFromMain(t, dir)
		})
	}
}

func TestInstallationStepsCloneMainInEveryEnv(t *testing.T) {
	repo := newTemplateRepo(t)
	template := Template{Template: blaxel.Template{Name: "fixture", URL: repo}}

	for _, env := range templateEnvs {
		t.Run("BL_ENV="+env, func(t *testing.T) {
			t.Setenv("BL_ENV", env)
			dir := filepath.Join(t.TempDir(), "project")

			// Run the interactive installer headless.
			p := tea.NewProgram(NewInstallationModel(),
				tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())
			done := make(chan struct{})
			go func() {
				_, _ = p.Run()
				close(done)
			}()

			err := runInstallationSteps(p, template, TemplateOptions{Directory: dir})
			p.Quit()
			<-done

			require.NoError(t, err)
			requireClonedFromMain(t, dir)
		})
	}
}
