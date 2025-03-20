package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func (r *Operations) DeployAgentAppCmd() *cobra.Command {
	var directory string
	var name string
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "deploy",
		Args:    cobra.ExactArgs(0),
		Aliases: []string{"d", "dp"},
		Short:   "Deploy a blaxel agent app",
		Long:    "Deploy a blaxel agent app, you must be in a blaxel agent app directory.",
		Example: `bl deploy`,
		Run: func(cmd *cobra.Command, args []string) {

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current working directory: %v\n", err)
				os.Exit(1)
			}

			// Create a temporary directory for deployment files
			deployDir := ".blaxel"

			if config.Name != "" {
				name = config.Name
			}

			deployment := Deployment{
				dir:  deployDir,
				name: name,
				cwd:  cwd,
				r:    r,
			}

			err = deployment.Generate()
			if err != nil {
				fmt.Printf("Error generating blaxel deployment: %v\n", err)
				os.Exit(1)
			}

			if dryRun {
				err := deployment.Print()
				if err != nil {
					fmt.Printf("Error printing blaxel deployment: %v\n", err)
					os.Exit(1)
				}
				return
			}

			err = deployment.Apply()
			if err != nil {
				fmt.Printf("Error applying blaxel deployment: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Deployment applied successfully")
		},
	}
	cmd.Flags().StringVarP(&directory, "directory", "d", "src", "Directory to deploy, defaults to current directory")
	cmd.Flags().StringVarP(&name, "name", "n", "", "Optional name for the deployment")
	cmd.Flags().BoolVarP(&dryRun, "dryrun", "", false, "Dry run the deployment")
	return cmd
}
