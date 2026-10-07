package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/ui"
	"github.com/stretchr/testify/assert"
)

func TestSetupSummaryNamesRemainingSteps(t *testing.T) {
	for _, test := range []struct {
		name, loggedIn, workspace, title string
		skipLogin                        bool
		loginDisabled                    bool
		results                          map[string]ui.Result
	}{
		{name: "logged in", loggedIn: "main", title: "Blaxel is ready"},
		{name: "login succeeded", workspace: "main", title: "Blaxel is ready"},
		{name: "non-terminal", title: "1 step left: bl login"},
		{name: "login unchecked", title: "1 step left: bl login"},
		{name: "login skipped", results: map[string]ui.Result{"login": {Err: ui.ErrSkipped}}, title: "1 step left: bl login"},
		{name: "skip-login flag", skipLogin: true, title: "1 step left: bl login"},
		{name: "login disabled", loginDisabled: true, title: "1 step left: bl login"},
		{name: "login failed", results: map[string]ui.Result{"login": {Err: errors.New("login failed")}}, title: "1 step left: bl login"},
		{name: "skills failed", loggedIn: "main", results: map[string]ui.Result{"skills": {Err: errors.New("offline")}}, title: "1 step left: bl setup"},
		{name: "skills failed without login", results: map[string]ui.Result{"skills": {Err: errors.New("offline")}}, title: "2 steps left: bl login, bl setup"},
		{name: "stopped", results: map[string]ui.Result{"skills": {Err: context.Canceled}, "login": {Err: context.Canceled}}, title: "2 steps left: bl login, bl setup"},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{}
			if test.loginDisabled {
				env[loginInstallEnv] = "false"
			}
			options := testSetupOptions(t, t.TempDir(), env, &setupRecorder{})
			options.skipLogin = test.skipLogin
			summary := setupSummary(options, setupPlan{loggedIn: test.loggedIn},
				map[string]bool{"skills": true}, test.results, &setupOutcome{workspace: test.workspace})
			assert.Equal(t, test.title, summary.Title)
		})
	}
}
