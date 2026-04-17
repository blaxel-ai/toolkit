package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validatePendingShareID rejects ids that contain characters that could break
// out of the URL path (slashes, whitespace, etc). Pending share IDs are UUIDs
// so legitimate values never need escaping.
func validatePendingShareID(id string) error {
	if id == "" {
		return fmt.Errorf("pending share ID is required")
	}
	for _, r := range id {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("invalid pending share ID %q: only alphanumerics, '-' and '_' are allowed", id)
		}
	}
	return nil
}

func init() {
	core.RegisterCommand("accept", func() *cobra.Command {
		return AcceptCmd()
	})
	core.RegisterCommand("decline", func() *cobra.Command {
		return DeclineCmd()
	})
}

// AcceptCmd returns the root "accept" command for accepting pending resources.
func AcceptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept",
		Short: "Accept a pending resource share",
		Long: `Accept a pending resource share request from another workspace.
Currently supports pending image shares.`,
		Example: `  # Accept a pending image share by id
  bl accept image-share 01HW...`,
	}
	cmd.AddCommand(AcceptImageShareCmd())
	return cmd
}

// DeclineCmd returns the root "decline" command for declining pending resources.
func DeclineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decline",
		Short: "Decline a pending resource share",
		Long: `Decline a pending resource share request from another workspace.
Currently supports pending image shares.`,
		Example: `  # Decline a pending image share by id
  bl decline image-share 01HW...`,
	}
	cmd.AddCommand(DeclineImageShareCmd())
	return cmd
}

// AcceptImageShareCmd accepts a pending image share by id.
func AcceptImageShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "image-share pendingShareId",
		Aliases: []string{"image-shares"},
		Short:   "Accept a pending image share",
		Long: `Accept a pending image share by its id. You must be an admin of the
target workspace. On success, the image metadata is copied into your workspace.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			if err := validatePendingShareID(id); err != nil {
				fmt.Println(err)
				core.ExitWithError(err)
			}
			ctx := context.Background()
			client := core.GetClient()

			path := fmt.Sprintf("image-shares/pending/%s/accept", id)
			var resp map[string]interface{}
			if err := client.Post(ctx, path, struct{}{}, &resp); err != nil {
				err = fmt.Errorf("error accepting image share %s: %v", id, err)
				fmt.Println(err)
				core.ExitWithError(err)
			}
			fmt.Printf("Accepted image share %s.\n", id)
		},
	}
}

// DeclineImageShareCmd declines a pending image share by id.
func DeclineImageShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "image-share pendingShareId",
		Aliases: []string{"image-shares"},
		Short:   "Decline a pending image share",
		Long: `Decline a pending image share by its id. You must be an admin of the
target workspace. On success, the pending request is removed.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			if err := validatePendingShareID(id); err != nil {
				fmt.Println(err)
				core.ExitWithError(err)
			}
			ctx := context.Background()
			client := core.GetClient()

			path := fmt.Sprintf("image-shares/pending/%s/decline", id)
			if err := client.Post(ctx, path, struct{}{}, nil); err != nil {
				err = fmt.Errorf("error declining image share %s: %v", id, err)
				fmt.Println(err)
				core.ExitWithError(err)
			}
			fmt.Printf("Declined image share %s.\n", id)
		},
	}
}

// GetImageSharesCmd returns the cobra command for listing pending image shares.
func GetImageSharesCmd() *cobra.Command {
	var direction string
	cmd := &cobra.Command{
		Use:     "image-shares",
		Aliases: []string{"image-share"},
		Short:   "List pending image shares",
		Long: `List pending image shares for the current workspace.

Use --direction=incoming to list shares others have requested to this workspace
(default), or --direction=outgoing to list shares this workspace has requested
to workspaces in other accounts.`,
		Example: `  # List pending shares waiting for your approval
  bl get image-shares

  # List pending shares your workspace has requested to other accounts
  bl get image-shares --direction outgoing`,
		Run: func(cmd *cobra.Command, args []string) {
			if direction != "incoming" && direction != "outgoing" {
				err := fmt.Errorf("--direction must be either 'incoming' or 'outgoing'")
				fmt.Println(err)
				core.ExitWithError(err)
			}

			ctx := context.Background()
			client := core.GetClient()

			path := fmt.Sprintf("image-shares/pending?direction=%s", direction)

			var resp []map[string]interface{}
			if err := client.Get(ctx, path, nil, &resp); err != nil {
				err = fmt.Errorf("error listing pending image shares: %v", err)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			switch core.GetOutputFormat() {
			case "json", "pretty":
				data, err := json.MarshalIndent(resp, "", "  ")
				if err != nil {
					fmt.Println(err)
					core.ExitWithError(err)
				}
				fmt.Println(string(data))
				return
			case "yaml":
				data, err := yaml.Marshal(resp)
				if err != nil {
					fmt.Println(err)
					core.ExitWithError(err)
				}
				fmt.Print(string(data))
				return
			}

			if len(resp) == 0 {
				fmt.Println("No pending image shares.")
				return
			}
			fmt.Printf("%-28s  %-10s  %-28s  %-28s  %s\n", "ID", "TYPE", "IMAGE", "PEER WORKSPACE", "EXPIRES")
			for _, s := range resp {
				id, _ := s["id"].(string)
				rt, _ := s["resourceType"].(string)
				img, _ := s["imageName"].(string)
				peer, _ := s["sourceWorkspace"].(string)
				if direction == "outgoing" {
					peer, _ = s["targetWorkspace"].(string)
				}
				expires, _ := s["expiresAt"].(string)
				fmt.Printf("%-28s  %-10s  %-28s  %-28s  %s\n", id, rt, img, peer, expires)
			}
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "incoming", "Filter by direction: incoming or outgoing")
	return cmd
}
