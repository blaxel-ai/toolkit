package auth

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	blaxel "github.com/stainless-sdks/blaxel-go"
)

func LoginClientCredentials(workspace string, clientCredentials string) {
	// Create credentials struct and marshal to JSON
	creds := blaxel.Credentials{
		ClientCredentials: clientCredentials,
	}

	err := validateWorkspace(workspace, creds)
	if err != nil {
		err = fmt.Errorf("failed to access workspace '%s': %s", workspace, err)
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}

	blaxel.SaveCredentials(workspace, creds)
	blaxel.SetCurrentWorkspace(workspace)
	fmt.Println("Successfully stored client credentials")
}
