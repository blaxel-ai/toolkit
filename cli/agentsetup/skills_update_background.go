package agentsetup

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
)

const SkillsUpdateWorkerEnv = "BL_INTERNAL_SKILLS_UPDATE"

// Ordinary commands spawn a bounded worker because a short-lived CLI would
// exit before a background goroutine finishes. This is not a persistent daemon.
func RunSkillsUpdateWorker(version string) (bool, error) {
	if os.Getenv(SkillsUpdateWorkerEnv) != "1" || len(os.Args) != 3 || os.Args[1] != "skills" || os.Args[2] != "update" {
		return false, nil
	}
	updater, err := DefaultSkillsUpdater(version)
	if err == nil {
		err = updater.check(context.Background(), false)
	}
	return true, err
}

func StartSkillsUpdateWorker(version string) {
	if skillsUpdateWorkerSkipped(os.Args[1:]) || SkillsInstallDisabled(os.Getenv) {
		return
	}
	updater, err := DefaultSkillsUpdater(version)
	if err != nil {
		return
	}
	state, err := ReadSkillsUpdateState(updater.home)
	if err != nil || SkillsUpdateDisabled(state, updater.env) != "" || !state.LastAttempt.IsZero() && time.Since(state.LastAttempt) < SkillsUpdateInterval {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	worker := exec.Command(executable, "skills", "update")
	worker.Env = skillsUpdateWorkerEnv(os.LookupEnv)
	detachSkillsUpdateWorker(worker)
	// nil streams use the null device; no terminal or MCP pipes are inherited.
	if worker.Start() == nil {
		_ = worker.Process.Release()
	}
}

// skillsUpdateWorkerSkipped reports commands that handle skills themselves or
// must not start a worker: shell completion, bl mcp, skills, setup, upgrade
// and docs. The command is resolved as cobra will, so a flag value or argument
// such as `bl get skills` or `-w mcp` still gets the scheduled check.
func skillsUpdateWorkerSkipped(args []string) bool {
	if core.IsShellCompletionRequest(args) {
		return true
	}
	switch core.ResolveStartupCommand(args, "mcp", "skills", "setup", "upgrade", "docs") {
	case "mcp", "skills", "setup", "upgrade", "docs":
		return true
	}
	return false
}

// skillsUpdateWorkerEnv passes only local paths (including where each agent
// lives) and networking settings, never Blaxel credentials.
func skillsUpdateWorkerEnv(lookup func(string) (string, bool)) []string {
	var env []string
	for _, name := range []string{"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "SystemRoot", "APPDATA", "TMPDIR", "TEMP", "TMP", "XDG_STATE_HOME", "XDG_CONFIG_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GOOSE_PATH_ROOT", "CONTINUE_GLOBAL_DIR", "CRUSH_GLOBAL_CONFIG", "OPENCLAW_CONFIG_PATH", "OPENCLAW_STATE_DIR", SkillsInstallEnv, "CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE", "HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE", "SSL_CERT_DIR"} {
		if value, ok := lookup(name); ok {
			env = append(env, name+"="+value)
		}
	}
	// Suppress main's tracking probe before the worker entrypoint runs.
	return append(env, "DO_NOT_TRACK=1", SkillsUpdateWorkerEnv+"=1")
}

func (updater SkillsUpdater) Run(ctx context.Context, interval time.Duration) {
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
