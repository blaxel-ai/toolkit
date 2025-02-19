package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/beamlit/toolkit/sdk"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var BASE_URL = "https://api.blaxel.ai/v0"
var APP_URL = "https://app.blaxel.ai"
var RUN_URL = "https://run.blaxel.ai"
var REGISTRY_URL = "https://us.registry.blaxel.ai"

func init() {
	err := godotenv.Load()
	// nolint:staticcheck
	if err != nil {
	}
	env := os.Getenv("BL_ENV")
	if env == "dev" {
		BASE_URL = "https://api.blaxel.dev/v0"
		APP_URL = "https://app.blaxel.dev"
		RUN_URL = "https://run.blaxel.dev"
		REGISTRY_URL = "https://eu.registry.blaxel.dev"
	} else if env == "local" {
		BASE_URL = "http://localhost:8080/v0"
		APP_URL = "http://localhost:3000"
		RUN_URL = "https://run.blaxel.dev"
		REGISTRY_URL = "https://eu.registry.blaxel.dev"
	}
}

var workspace string
var outputFormat string
var client *sdk.ClientWithResponses
var reg *Operations
var verbose bool
var version string
var commit string
var date string
var utc bool
var rootCmd = &cobra.Command{
	Use:   "bl",
	Short: "Blaxel CLI is a command line tool to interact with Blaxel APIs.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setEnvs()

		reg = &Operations{
			BaseURL:     BASE_URL,
			RunURL:      RUN_URL,
			AppURL:      APP_URL,
			RegistryURL: REGISTRY_URL,
		}
		credentials := sdk.LoadCredentials(workspace)
		if !credentials.IsValid() && workspace != "" {
			fmt.Printf("Invalid credentials for workspace %s\n", workspace)
			fmt.Printf("Please run `bl login %s` to fix it credentials.\n", workspace)
		}
		var err error
		os := runtime.GOOS
		arch := runtime.GOARCH
		commitShort := "unknown"
		if commit != "" && len(commit) > 7 {
			commitShort = commit[:7]
		}
		c, err := sdk.NewClientWithCredentials(
			sdk.RunClientWithCredentials{
				ApiURL:      BASE_URL,
				RunURL:      RUN_URL,
				Credentials: credentials,
				Workspace:   workspace,
				Headers: map[string]string{
					"User-Agent": fmt.Sprintf("blaxel/v%s (%s/%s) blaxel/%s", version, os, arch, commitShort),
				},
			},
		)
		if err != nil {
			return err
		}
		client = c
		ctx := context.Background()
		c.RegisterCliCommands(reg, ctx)
		return nil
	},
}

func setEnvs() {
	if url := os.Getenv("BL_API_URL"); url != "" {
		BASE_URL = url
	}
	if runUrl := os.Getenv("BL_RUN_URL"); runUrl != "" {
		RUN_URL = runUrl
	}
	if appUrl := os.Getenv("BL_APP_URL"); appUrl != "" {
		APP_URL = appUrl
	}
}

func Execute(releaseVersion string, releaseCommit string, releaseDate string) error {
	setEnvs()

	reg = &Operations{
		BaseURL:     BASE_URL,
		RunURL:      RUN_URL,
		AppURL:      APP_URL,
		RegistryURL: REGISTRY_URL,
	}

	rootCmd.AddCommand(reg.ListOrSetWorkspacesCmd())
	rootCmd.AddCommand(reg.LoginCmd())
	rootCmd.AddCommand(reg.LogoutCmd())
	rootCmd.AddCommand(reg.GetCmd())
	rootCmd.AddCommand(reg.ApplyCmd())
	rootCmd.AddCommand(reg.DeleteCmd())
	rootCmd.AddCommand(reg.RunCmd())
	rootCmd.AddCommand(reg.DocCmd())
	rootCmd.AddCommand(reg.ServeCmd())
	rootCmd.AddCommand(reg.CreateAgentAppCmd())
	rootCmd.AddCommand(reg.DeployAgentAppCmd())
	rootCmd.AddCommand(reg.ChatCmd())
	rootCmd.AddCommand(reg.VersionCmd())

	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "Specify the workspace name")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format. One of: pretty,yaml,json,table")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&utc, "utc", "u", false, "Enable UTC timezone")

	if workspace == "" {
		workspace = sdk.CurrentContext().Workspace
	}
	if version == "" {
		version = releaseVersion
	}
	if commit == "" {
		commit = releaseCommit
	}
	if date == "" {
		date = releaseDate
	}
	return rootCmd.Execute()
}
