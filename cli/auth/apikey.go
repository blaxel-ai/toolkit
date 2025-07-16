package auth

import (
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/fatih/color"
)

func LoginApiKey(workspace string) {
	var apiKey string
	// Check if API key is provided via environment variable
	if apiKey = os.Getenv("BL_API_KEY"); apiKey != "" {
		core.PrintInfo("Using API key from environment variable BL_API_KEY")
	} else {
		fmt.Printf("%s %s",
			color.New(color.FgBlue, color.Bold).Sprint("ℹ"),
			color.New(color.FgBlue).Sprint("Enter your API key: "))
		for {
			_, err := fmt.Scanln(&apiKey)
			if err != nil {
				core.PrintWarning("API key is required. Please enter your API key")
			}

			if apiKey != "" {
				break
			}
		}
	}

	// Create credentials struct and marshal to JSON
	creds := sdk.Credentials{
		APIKey: apiKey,
	}

	err := validateWorkspace(workspace, creds)
	if err != nil {
		core.PrintError("Login", fmt.Errorf("failed to access workspace '%s': %w", workspace, err))
		os.Exit(1)
	}

	sdk.SaveCredentials(workspace, creds)
	sdk.SetCurrentWorkspace(workspace)
	core.PrintSuccess(fmt.Sprintf("Successfully logged in to workspace %s", workspace))
}
