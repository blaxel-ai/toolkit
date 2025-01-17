package cli

import (
	"fmt"
	"os"

	"github.com/beamlit/toolkit/sdk"
)

func (r *Operations) ClientCredentialsLogin(workspace string, environment string, clientCredentials string) {
	// Create credentials struct and marshal to JSON
	creds := sdk.Credentials{
		ClientCredentials: clientCredentials,
	}

	if err := CheckWorkspaceAccess(workspace, creds); err != nil {
		fmt.Printf("Error accessing workspace %s : %s\n", workspace, err)
		os.Exit(1)
	}

	sdk.SaveCredentials(workspace, creds)
	sdk.SetCurrentWorkspace(workspace, environment)
	fmt.Println("Successfully stored client credentials")
}
