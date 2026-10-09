package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
)

var (
	workspacePollInterval = 3 * time.Second
	workspaceWaitTimeout  = 5 * time.Minute
)

// WaitForLoginWorkspaces keeps a browser login in memory while the user creates
// or joins a workspace in the existing Console. Only a successful empty list
// opens the Console and starts polling; API errors stop the wait.
func WaitForLoginWorkspaces(ctx context.Context, creds blaxel.Credentials, note func(string)) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, workspaceWaitTimeout)
	defer cancel()
	consoleURL := blaxel.GetAppURL()
	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()
	for {
		workspaces, err := listWorkspacesWithContext(ctx, creds, defaultClientFactory)
		if ctx.Err() != nil {
			return nil, workspaceWaitError(ctx.Err(), consoleURL)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list workspaces: %w", err)
		}
		if len(workspaces) > 0 {
			names := make([]string, 0, len(workspaces))
			for _, workspace := range workspaces {
				names = append(names, workspace.Name)
			}
			return names, nil
		}
		if ticker == nil {
			_ = openBrowser(consoleURL)
			note("No workspaces are available for your account. Create or join a workspace in the Console:")
			note(consoleURL)
			note(fmt.Sprintf("Waiting up to %s for a workspace to become available. Press Ctrl+C to cancel; then run bl login again when ready.", describeWait(workspaceWaitTimeout)))
			ticker = time.NewTicker(workspacePollInterval)
		}
		select {
		case <-ctx.Done():
			return nil, workspaceWaitError(ctx.Err(), consoleURL)
		case <-ticker.C:
		}
	}
}

func workspaceWaitError(err error, consoleURL string) error {
	message := "cancelled waiting for a workspace"
	if errors.Is(err, context.DeadlineExceeded) {
		message = "timed out waiting for a workspace"
	}
	return core.MarkExpectedError(
		fmt.Errorf("%s. Create or join a workspace at %s, then run bl login again: %w", message, consoleURL, err),
		core.CLIErrorOperational,
	)
}
