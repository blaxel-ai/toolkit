package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func init() {
	// Auto-register this command
	RegisterCommand("docs", func() *cobra.Command {
		return DocCmd()
	})
}

// fixCompletionDocs fixes the completion command's Long descriptions to use
// Mintlify-compatible syntax. The default Cobra completion docs use
// `source <(bl completion bash)` which breaks Mintlify's MDX parser.
func fixCompletionDocs(cmd *cobra.Command) {
	for _, child := range cmd.Commands() {
		if child.Name() == "completion" {
			for _, subCmd := range child.Commands() {
				// Replace `source <(bl completion X)` with `eval "$(bl completion X)"`
				// This syntax works in both bash/zsh and doesn't break Mintlify
				subCmd.Long = strings.ReplaceAll(subCmd.Long, "source <(bl completion", "eval \"$(bl completion")
				subCmd.Long = strings.ReplaceAll(subCmd.Long, "bash)", "bash)\"")
				subCmd.Long = strings.ReplaceAll(subCmd.Long, "zsh)", "zsh)\"")
				subCmd.Long = strings.ReplaceAll(subCmd.Long, "fish)", "fish)\"")
				subCmd.Long = strings.ReplaceAll(subCmd.Long, "powershell)", "powershell)\"")
			}
			break
		}
	}
}

func DocCmd() *cobra.Command {
	var format string
	var outputDir string

	docCmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate documentation for the CLI",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootCmd.DisableAutoGenTag = true

			// Fix completion docs to use Mintlify-compatible syntax
			fixCompletionDocs(rootCmd)

			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			switch format {
			case "markdown":
				const fmTemplate = `---
title: "%s"
slug: %s
---
`
				filePrepender := func(filename string) string {
					name := filepath.Base(filename)
					base := strings.TrimSuffix(name, path.Ext(name))
					return fmt.Sprintf(fmTemplate, strings.ReplaceAll(base, "_", " "), base)
				}
				linkHandler := func(name string) string {
					return name
				}
				return doc.GenMarkdownTreeCustom(rootCmd, outputDir, filePrepender, linkHandler)
			case "man":
				header := &doc.GenManHeader{
					Title:   "BLAXEL",
					Section: "1",
				}
				return doc.GenManTree(rootCmd, header, outputDir)
			case "rst":
				return doc.GenReSTTree(rootCmd, outputDir)
			case "yaml":
				return doc.GenYamlTree(rootCmd, outputDir)
			default:
				return fmt.Errorf("unknown format %s", format)
			}
		},
	}

	docCmd.Flags().StringVarP(&format, "format", "f", "markdown", "Documentation format (markdown, man, rst, yaml)")
	docCmd.Flags().StringVarP(&outputDir, "output", "o", "./docs", "Output directory for documentation")

	return docCmd
}
