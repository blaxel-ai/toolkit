package cli

import (
	"github.com/blaxel-ai/toolkit/cli/core"
)

func Execute(releaseVersion string, releaseCommit string, releaseDate string) error {
	if worker, err := runSkillsUpdateWorker(releaseVersion); worker {
		return err
	}
	refreshHomebrewSetup()
	startSkillsUpdateWorker(releaseVersion)
	return core.Execute(releaseVersion, releaseCommit, releaseDate)
}
