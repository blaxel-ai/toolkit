package cli

import (
	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/blaxel-ai/toolkit/cli/core"
)

func Execute(releaseVersion string, releaseCommit string, releaseDate string) error {
	if worker, err := agentsetup.RunSkillsUpdateWorker(releaseVersion); worker {
		return err
	}
	refreshHomebrewSetup()
	agentsetup.StartSkillsUpdateWorker(releaseVersion)
	return core.Execute(releaseVersion, releaseCommit, releaseDate)
}
