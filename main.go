package main

import (
	"fmt"
	"os"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli"
	"github.com/blaxel-ai/toolkit/cli/core"
)

var (
	version    = "dev"
	commit     = "none"
	date       = "unknown"
	sentryDSN  = ""
	posthogKey = ""
)

func main() {
	// Configure PostHog before Execute prompts for first-run consent. Event
	// tracking still checks the saved consent before sending anything.
	core.PosthogAPIKey = posthogKey
	defer core.FlushPosthog()

	// Initialize Sentry for error tracking only if tracking is enabled
	if blaxel.IsTrackingEnabled() {
		err := core.InitSentry(core.SentryConfig{
			DSN:     sentryDSN,
			Release: version,
		})
		if err != nil {
			// Log but don't fail if Sentry initialization fails
			if os.Getenv("BL_DEBUG") == "true" {
				_, _ = os.Stderr.WriteString("Warning: Failed to initialize Sentry: " + err.Error() + "\n")
			}
		}
		defer core.FlushSentry(2 * time.Second)

		// Recover from panics and send to Sentry
		defer core.RecoverWithSentry()
	}

	err := cli.Execute(version, commit, date)
	if err != nil {
		fmt.Println("Error", err)
		core.ExitWithError(err)
	}
}
