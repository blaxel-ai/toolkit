package cli

import (
	"fmt"
	"os"

	"github.com/beamlit/toolkit/sdk"
)

func (r *Operations) ApiKeyLogin(workspace string) {
	fmt.Println("Enter your API key :")
	var apiKey string
	for {
		fmt.Scanln(&apiKey)

		if apiKey != "" {
			break
		}
		fmt.Println("API key cannot be empty. Please enter your API key")
	}

	// Create credentials struct and marshal to JSON
	creds := sdk.Credentials{
		APIKey: apiKey,
	}

	if err := CheckWorkspaceAccess(workspace, creds); err != nil {
		fmt.Printf("Error accessing workspace %s : %s\n", workspace, err)
		os.Exit(1)
	}

	sdk.SaveCredentials(workspace, creds)
	fmt.Println("Successfully stored API key")
}
