package cli

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
)

const skillsUpdateWorkerEnv = "BL_INTERNAL_SKILLS_UPDATE"

// Ordinary commands spawn a bounded worker because a short-lived CLI would
// exit before a background goroutine finishes. This is not a persistent daemon.
func runSkillsUpdateWorker(version string) (bool, error) {
	if os.Getenv(skillsUpdateWorkerEnv) != "1" || len(os.Args) != 3 || os.Args[1] != "skills" || os.Args[2] != "update" {
		return false, nil
	}
	updater, err := defaultSkillsUpdater(version)
	if err == nil {
		err = updater.check(context.Background(), false)
	}
	return true, err
}

func startSkillsUpdateWorker(version string) {
	args := os.Args[1:]
	if core.IsShellCompletionRequest(args) || skillsInstallDisabled(os.Getenv) {
		return
	}
	for _, arg := range args {
		if arg == "mcp" || arg == "skills" || arg == "setup" || arg == "upgrade" || arg == "docs" {
			return
		}
	}
	updater, err := defaultSkillsUpdater(version)
	if err != nil {
		return
	}
	state, err := readSkillsUpdateState(updater.home)
	if err != nil || skillsUpdateDisabled(state, updater.env) != "" || !state.LastAttempt.IsZero() && time.Since(state.LastAttempt) < skillsUpdateInterval {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	worker := exec.Command(executable, "skills", "update")
	// Pass only local paths and networking settings, never Blaxel credentials.
	for _, name := range []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "XDG_STATE_HOME", "XDG_CONFIG_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", skillsInstallEnv, "CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE", "HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE", "SSL_CERT_DIR"} {
		if value, ok := os.LookupEnv(name); ok {
			worker.Env = append(worker.Env, name+"="+value)
		}
	}
	// Suppress main's tracking probe before the worker entrypoint runs.
	worker.Env = append(worker.Env, "DO_NOT_TRACK=1", skillsUpdateWorkerEnv+"=1")
	detachSkillsUpdateWorker(worker)
	// nil streams use the null device; no terminal or MCP pipes are inherited.
	if worker.Start() == nil {
		_ = worker.Process.Release()
	}
}

func (updater skillsUpdater) run(ctx context.Context, interval time.Duration) {
	_ = updater.check(ctx, false)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = updater.check(ctx, false)
		}
	}
}
