package core

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envFrom(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestDetectCodingAgent(t *testing.T) {
	cases := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"none", map[string]string{}, ""},
		{"blank values", map[string]string{"CLAUDECODE": "", "AI_AGENT": "  "}, ""},
		{"claude code", map[string]string{"CLAUDECODE": "1"}, "claude-code"},
		{"codex thread", map[string]string{"CODEX_THREAD_ID": "thread"}, "codex"},
		{"codex sandbox", map[string]string{"CODEX_SANDBOX": "seatbelt"}, "codex"},
		{"cursor", map[string]string{"CURSOR_AGENT": "1"}, "cursor"},
		{"gemini cli", map[string]string{"GEMINI_CLI": "1"}, "gemini-cli"},
		{"qwen code", map[string]string{"QWEN_CODE": "1"}, "qwen-code"},
		{"opencode shell", map[string]string{"OPENCODE_TERMINAL": "1"}, "opencode"},
		{"opencode plugin shell", map[string]string{"OPENCODE": "1", "AGENT": "1"}, "opencode"},
		{"goose", map[string]string{"GOOSE_TERMINAL": "1"}, "goose"},
		{"augment", map[string]string{"AUGMENT_AGENT": "1"}, "augment"},
		{"marker wins over AI_AGENT", map[string]string{"AI_AGENT": "something-else", "CODEX_SANDBOX": "seatbelt"}, "codex"},
		{"AI_AGENT with version", map[string]string{"AI_AGENT": "claude-code_2-1-306_agent"}, "claude-code"},
		{"AI_AGENT mixed case", map[string]string{"AI_AGENT": "Cursor"}, "cursor"},
		{"AI_AGENT crush", map[string]string{"AI_AGENT": "crush", "AGENT": "crush"}, "crush"},
		{"AI_AGENT alias", map[string]string{"AI_AGENT": "github-copilot-cli"}, "github-copilot"},
		{"AI_AGENT unknown", map[string]string{"AI_AGENT": "some-new-agent"}, "other"},
		{"AI_AGENT free form", map[string]string{"AI_AGENT": "Jane Doe's laptop"}, "other"},
		{"AI_AGENT wins over AGENT", map[string]string{"AGENT": "goose", "AI_AGENT": "claude"}, "claude-code"},
		{"AGENT known", map[string]string{"AGENT": "goose"}, "goose"},
		{"AGENT flag", map[string]string{"AGENT": "1"}, ""},
		{"AGENT unknown", map[string]string{"AGENT": "build-runner"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, detectCodingAgent(envFrom(tc.vars)))
		})
	}
}

// isolateUsageConsent clears the opt-out variables and points HOME at an empty
// directory, so the machine running the tests does not change the result.
func isolateUsageConsent(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("BL_INSTALL_TRACKING", "")
	t.Setenv("BL_SKIP_TELEMETRY", "")
	return home
}

func TestCLIUserAgent(t *testing.T) {
	base := "blaxel/cli/golang/1.2.3 (" + runtime.GOOS + "/" + runtime.GOARCH + ") blaxel/abcdef1"
	claude := envFrom(map[string]string{"CLAUDECODE": "1"})

	t.Run("no coding agent", func(t *testing.T) {
		isolateUsageConsent(t)
		assert.Equal(t, base, cliUserAgent("1.2.3", "abcdef1234567", envFrom(nil)))
	})

	t.Run("coding agent", func(t *testing.T) {
		isolateUsageConsent(t)
		assert.Equal(t, base+" agent/claude-code", cliUserAgent("1.2.3", "abcdef1234567", claude))
	})

	t.Run("unknown commit", func(t *testing.T) {
		isolateUsageConsent(t)
		userAgent := cliUserAgent("1.2.3", "", claude)
		assert.Regexp(t, regexp.MustCompile(`^blaxel/cli/golang/1\.2\.3 \([^)]+\) blaxel/unknown agent/claude-code$`), userAgent)
	})

	t.Run("CI keeps the token", func(t *testing.T) {
		isolateUsageConsent(t)
		t.Setenv("CI", "true")
		assert.Equal(t, base+" agent/claude-code", cliUserAgent("1.2.3", "abcdef1", claude))
	})

	for _, optOut := range []struct{ key, value string }{
		{"DO_NOT_TRACK", "1"}, {"DO_NOT_TRACK", "true"}, {"BL_INSTALL_TRACKING", "false"}, {"BL_SKIP_TELEMETRY", "1"},
	} {
		t.Run(optOut.key+"="+optOut.value+" drops the token", func(t *testing.T) {
			isolateUsageConsent(t)
			t.Setenv(optOut.key, optOut.value)
			assert.Equal(t, base, cliUserAgent("1.2.3", "abcdef1", claude))
		})
	}

	t.Run("saved tracking false keeps the token", func(t *testing.T) {
		// bl login saves tracking: false until the user opts in to reports.
		home := isolateUsageConsent(t)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".blaxel"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".blaxel", "config.yaml"), []byte("tracking: false\n"), 0o600))
		assert.Equal(t, base+" agent/claude-code", cliUserAgent("1.2.3", "abcdef1", claude))
	})

	t.Run("DO_NOT_TRACK=0 keeps the token", func(t *testing.T) {
		isolateUsageConsent(t)
		t.Setenv("DO_NOT_TRACK", "0")
		assert.Equal(t, base+" agent/claude-code", cliUserAgent("1.2.3", "abcdef1", claude))
	})

	t.Run("unsaved tracking choice keeps the token", func(t *testing.T) {
		home := isolateUsageConsent(t)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".blaxel"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".blaxel", "config.yaml"), []byte("context:\n  workspace: test-ws\n"), 0o600))
		assert.Equal(t, base+" agent/claude-code", cliUserAgent("1.2.3", "abcdef1", claude))
	})
}

// UserAgent, which bl mcp sends too, has the shape the control plane reads
// (api/pkg/useragent): its CLI origin pattern and its coding agent token,
// without the integration token the CLI never sends.
func TestUserAgent(t *testing.T) {
	cliOrigin := regexp.MustCompile(`^blaxel\/cli\/[a-z]+\/\d+\.\d+\.\d+(?:\s+\([^)]+\)\s+blaxel\/[a-z0-9]+(?:\s+agent\/[a-z0-9][a-z0-9._-]{0,63})?)?$`)
	codingAgent := regexp.MustCompile(`^blaxel\/(?:cli|sdk)\/[a-z]+\/\d+\.\d+\.\d+\s+\([^)]+\)\s+blaxel\/[a-z0-9]+\s+agent\/([a-z0-9][a-z0-9._-]{0,63})$`)
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() { version, commit = originalVersion, originalCommit })
	version, commit = "1.2.3", "abcdef1234567"

	isolateUsageConsent(t)
	t.Setenv("CLAUDECODE", "1")
	userAgent := UserAgent()
	assert.Regexp(t, cliOrigin, userAgent)
	assert.Equal(t, []string{userAgent, "claude-code"}, codingAgent.FindStringSubmatch(userAgent))

	t.Setenv("DO_NOT_TRACK", "1")
	userAgent = UserAgent()
	assert.Regexp(t, cliOrigin, userAgent)
	assert.Nil(t, codingAgent.FindStringSubmatch(userAgent), "an opt-out leaves the agent token out")
}
