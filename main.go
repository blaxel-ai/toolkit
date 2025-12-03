package main

import (
	"fmt"
	"os"
	"time"

	"github.com/blaxel-ai/toolkit/cli"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
)

var (
	version   = "dev"
	commit    = "none"
	date      = "unknown"
	sentryDSN = ""
)

func main() {
	// Initialize Sentry for error tracking only if tracking is enabled
	if sdk.IsTrackingEnabled() {
		err := core.InitSentry(core.SentryConfig{
			DSN:     sentryDSN,
			Release: version,
		})
		if err != nil {
			// Log but don't fail if Sentry initialization fails
			if os.Getenv("BL_DEBUG") == "true" {
				os.Stderr.WriteString("Warning: Failed to initialize Sentry: " + err.Error() + "\n")
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
