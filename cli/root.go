package cli

import (
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/joho/godotenv"
)

func Execute(releaseVersion string, releaseCommit string, releaseDate string) error {
	godotenv.Load()
	return core.Execute(releaseVersion, releaseCommit, releaseDate)
}
