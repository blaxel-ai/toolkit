package auth

import (
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/spf13/cobra"
)

func LoginClientCredentialsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "client-credentials [workspace]",
		Short: "Login using client credentials",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			workspace := args[0]
			LoginClientCredentials(workspace)
		},
	}
}

func LoginClientCredentials(workspace string) {
	clientId := os.Getenv("BL_CLIENT_ID")
	clientSecret := os.Getenv("BL_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		fmt.Println("Please set the BL_CLIENT_ID and BL_CLIENT_SECRET environment variables")
		os.Exit(1)
	}

	// TODO: Implement client credentials flow
	fmt.Println("Client credentials login not fully implemented yet")

	credentials := sdk.Credentials{
		AccessToken: clientId, // This is temporary, should use proper OAuth flow
	}

	handleLoginSuccess(workspace, credentials)
}
