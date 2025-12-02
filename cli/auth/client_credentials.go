package auth

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
)

func LoginClientCredentials(workspace string, clientCredentials string) {
	// Create credentials struct and marshal to JSON
	creds := sdk.Credentials{
		ClientCredentials: clientCredentials,
	}

	err := validateWorkspace(workspace, creds)
	if err != nil {
		err = fmt.Errorf("failed to access workspace '%s': %s", workspace, err)
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}

	sdk.SaveCredentials(workspace, creds)
	sdk.SetCurrentWorkspace(workspace)
	fmt.Println("Successfully stored client credentials")
}
