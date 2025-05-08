package cli

import (
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/sdk"
)

func (r *Operations) ClientCredentialsLogin(workspace string, clientCredentials string) {
	// Create credentials struct and marshal to JSON
	creds := sdk.Credentials{
		ClientCredentials: clientCredentials,
	}

	_, err := CheckWorkspaceAccess(workspace, creds)
	if err != nil {
		fmt.Printf("Error accessing workspace %s : %s\n", workspace, err)
		os.Exit(1)
	}
	sdk.SaveCredentials(workspace, creds)
	sdk.SetCurrentWorkspace(workspace)
	fmt.Println("Successfully stored client credentials")
}
