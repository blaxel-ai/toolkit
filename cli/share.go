package cli

import (
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("share", func() *cobra.Command {
		return ShareCmd()
	})
	core.RegisterCommand("unshare", func() *cobra.Command {
		return UnshareCmd()
	})
}

func ShareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share",
		Short: "Share a resource with another workspace",
		Long: `Share Blaxel resources with other workspaces in your account.
Currently supports sharing container images.`,
		Example: `  # Share an image with another workspace
  bl share image agent/my-agent --workspace other-workspace`,
	}

	cmd.AddCommand(ShareImagesCmd())
	return cmd
}

func UnshareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unshare",
		Short: "Unshare a resource from another workspace",
		Long: `Remove shared Blaxel resources from other workspaces.
Currently supports unsharing container images.`,
		Example: `  # Unshare an image from another workspace
  bl unshare image agent/my-agent --workspace other-workspace`,
	}

	cmd.AddCommand(UnshareImagesCmd())
	return cmd
}
