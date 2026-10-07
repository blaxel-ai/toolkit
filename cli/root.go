package cli

import (
	"github.com/blaxel-ai/toolkit/cli/core"
)

func Execute(releaseVersion string, releaseCommit string, releaseDate string) error {
	refreshHomebrewSetup()
	return core.Execute(releaseVersion, releaseCommit, releaseDate)
}
