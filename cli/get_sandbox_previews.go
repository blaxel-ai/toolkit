package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
)

// nestedResourceHint returns a usage hint for nested resources that cannot be accessed directly.
func nestedResourceHint(resource *core.Resource) string {
	switch resource.Kind {
	case "Preview":
		return " Use 'bl get sandbox <sandbox-name> previews' instead."
	case "PreviewToken":
		return " Use 'bl get sandbox <sandbox-name> preview <preview-name> tokens' instead."
	default:
		return ""
	}
}

// HandleSandboxPreviewNestedResource handles preview and preview token nested resources for sandboxes
// Supports:
//
//	bl get sandbox <name> previews
//	bl get sandbox <name> preview <preview-name>
//	bl get sandbox <name> preview <preview-name> tokens
//	bl get sandbox <name> preview <preview-name> token <token-name>
func HandleSandboxPreviewNestedResource(args []string) bool {
	if len(args) < 2 {
		return false
	}

	sandboxName := args[0]
	nestedResource := args[1]

	switch nestedResource {
	case "previews", "preview", "pv":
		if len(args) >= 4 {
			previewName := args[2]
			tokenResource := args[3]
			switch tokenResource {
			case "tokens", "token", "pvt":
				if len(args) >= 5 {
					tokenName := args[4]
					// bl get sandbox <name> preview <preview-name> token <token-name>
					getSandboxPreviewToken(sandboxName, previewName, tokenName)
				} else {
					// bl get sandbox <name> preview <preview-name> tokens
					listSandboxPreviewTokens(sandboxName, previewName)
				}
			default:
				core.PrintError("Get", fmt.Errorf("unknown nested resource '%s' for preview, supported: tokens", tokenResource))
				os.Exit(1)
			}
		} else if len(args) >= 3 {
			previewName := args[2]
			// bl get sandbox <name> preview <preview-name>
			getSandboxPreview(sandboxName, previewName)
		} else {
			// bl get sandbox <name> previews
			listSandboxPreviews(sandboxName)
		}
		return true

	default:
		return false
	}
}

// DeleteSandboxPreviewNestedResource handles delete for preview nested resources
// Supports:
//
//	bl delete sandbox <name> preview <preview-name>
//	bl delete sandbox <name> preview <preview-name> token <token-name>
func DeleteSandboxPreviewNestedResource(args []string) bool {
	if len(args) < 3 {
		return false
	}

	sandboxName := args[0]
	nestedResource := args[1]

	switch nestedResource {
	case "preview", "pv":
		previewName := args[2]
		if len(args) >= 5 {
			tokenResource := args[3]
			tokenName := args[4]
			switch tokenResource {
			case "token", "pvt":
				// bl delete sandbox <name> preview <preview-name> token <token-name>
				deleteSandboxPreviewToken(sandboxName, previewName, tokenName)
			default:
				core.PrintError("Delete", fmt.Errorf("unknown nested resource '%s' for preview, supported: token", tokenResource))
				os.Exit(1)
			}
		} else {
			// bl delete sandbox <name> preview <preview-name>
			deleteSandboxPreview(sandboxName, previewName)
		}
		return true

	default:
		return false
	}
}

func listSandboxPreviews(sandboxName string) {
	ctx := context.Background()
	client := core.GetClient()

	previews, err := client.Sandboxes.Previews.List(ctx, sandboxName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to list previews for sandbox '%s': %w", sandboxName, err))
		os.Exit(1)
	}

	if previews == nil || len(*previews) == 0 {
		core.PrintInfo(fmt.Sprintf("No previews found for sandbox '%s'", sandboxName))
		return
	}

	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		outputProcessData(previews, outputFormat)
		return
	}

	jsonData, err := json.Marshal(previews)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal previews: %w", err))
		os.Exit(1)
	}

	var slices []interface{}
	if err := json.Unmarshal(jsonData, &slices); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal previews: %w", err))
		os.Exit(1)
	}

	resource := core.Resource{
		Kind:     "Preview",
		Plural:   "previews",
		Singular: "preview",
		Fields: []core.Field{
			{Key: "NAME", Value: "metadata.name"},
			{Key: "TYPE", Value: "metadata.resourceType"},
			{Key: "RESOURCE", Value: "metadata.resourceName"},
			{Key: "PORT", Value: "spec.port"},
			{Key: "PUBLIC", Value: "spec.public"},
			{Key: "URL", Value: "spec.url"},
			{Key: "STATUS", Value: "status"},
		},
	}

	core.Output(resource, slices, outputFormat)
}

func getSandboxPreview(sandboxName, previewName string) {
	ctx := context.Background()
	client := core.GetClient()

	preview, err := client.Sandboxes.Previews.Get(ctx, previewName, blaxel.SandboxPreviewGetParams{
		SandboxName: sandboxName,
	})
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get preview '%s' for sandbox '%s': %w", previewName, sandboxName, err))
		os.Exit(1)
	}

	if preview == nil {
		core.PrintError("Get", fmt.Errorf("no preview data returned"))
		os.Exit(1)
	}

	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		outputProcessData(preview, outputFormat)
		return
	}

	jsonData, err := json.Marshal(preview)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal preview: %w", err))
		os.Exit(1)
	}

	var previewMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &previewMap); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal preview: %w", err))
		os.Exit(1)
	}

	resource := core.Resource{
		Kind:     "Preview",
		Plural:   "previews",
		Singular: "preview",
		Fields: []core.Field{
			{Key: "NAME", Value: "metadata.name"},
			{Key: "TYPE", Value: "metadata.resourceType"},
			{Key: "RESOURCE", Value: "metadata.resourceName"},
			{Key: "PORT", Value: "spec.port"},
			{Key: "PUBLIC", Value: "spec.public"},
			{Key: "URL", Value: "spec.url"},
			{Key: "TTL", Value: "spec.ttl"},
			{Key: "CUSTOM_DOMAIN", Value: "spec.customDomain"},
			{Key: "STATUS", Value: "status"},
		},
	}

	core.Output(resource, []interface{}{previewMap}, outputFormat)
}

func listSandboxPreviewTokens(sandboxName, previewName string) {
	ctx := context.Background()
	client := core.GetClient()

	tokens, err := client.Sandboxes.Previews.Tokens.Get(ctx, previewName, blaxel.SandboxPreviewTokenGetParams{
		SandboxName: sandboxName,
	})
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to list tokens for preview '%s' in sandbox '%s': %w", previewName, sandboxName, err))
		os.Exit(1)
	}

	if tokens == nil || len(*tokens) == 0 {
		core.PrintInfo(fmt.Sprintf("No tokens found for preview '%s' in sandbox '%s'", previewName, sandboxName))
		return
	}

	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		outputProcessData(tokens, outputFormat)
		return
	}

	jsonData, err := json.Marshal(tokens)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal tokens: %w", err))
		os.Exit(1)
	}

	var slices []interface{}
	if err := json.Unmarshal(jsonData, &slices); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal tokens: %w", err))
		os.Exit(1)
	}

	resource := core.Resource{
		Kind:     "PreviewToken",
		Plural:   "previewtokens",
		Singular: "previewtoken",
		Fields: []core.Field{
			{Key: "NAME", Value: "metadata.name"},
			{Key: "PREVIEW", Value: "metadata.previewName"},
			{Key: "RESOURCE", Value: "metadata.resourceName"},
			{Key: "EXPIRED", Value: "spec.expired"},
			{Key: "EXPIRES_AT", Value: "spec.expiresAt"},
		},
	}

	core.Output(resource, slices, outputFormat)
}

func getSandboxPreviewToken(sandboxName, previewName, tokenName string) {
	ctx := context.Background()
	client := core.GetClient()

	tokens, err := client.Sandboxes.Previews.Tokens.Get(ctx, previewName, blaxel.SandboxPreviewTokenGetParams{
		SandboxName: sandboxName,
	})
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to list tokens for preview '%s' in sandbox '%s': %w", previewName, sandboxName, err))
		os.Exit(1)
	}

	if tokens == nil {
		core.PrintError("Get", fmt.Errorf("no tokens returned"))
		os.Exit(1)
	}

	// Find the token by name
	var found *blaxel.PreviewToken
	for i := range *tokens {
		if (*tokens)[i].Metadata.Name == tokenName {
			found = &(*tokens)[i]
			break
		}
	}

	if found == nil {
		core.PrintError("Get", fmt.Errorf("token '%s' not found in preview '%s' of sandbox '%s'", tokenName, previewName, sandboxName))
		os.Exit(1)
	}

	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		outputProcessData(found, outputFormat)
		return
	}

	jsonData, err := json.Marshal(found)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal token: %w", err))
		os.Exit(1)
	}

	var tokenMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &tokenMap); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal token: %w", err))
		os.Exit(1)
	}

	resource := core.Resource{
		Kind:     "PreviewToken",
		Plural:   "previewtokens",
		Singular: "previewtoken",
		Fields: []core.Field{
			{Key: "NAME", Value: "metadata.name"},
			{Key: "PREVIEW", Value: "metadata.previewName"},
			{Key: "RESOURCE", Value: "metadata.resourceName"},
			{Key: "EXPIRED", Value: "spec.expired"},
			{Key: "EXPIRES_AT", Value: "spec.expiresAt"},
		},
	}

	core.Output(resource, []interface{}{tokenMap}, outputFormat)
}

func deleteSandboxPreview(sandboxName, previewName string) {
	ctx := context.Background()
	client := core.GetClient()

	_, err := client.Sandboxes.Previews.Delete(ctx, previewName, blaxel.SandboxPreviewDeleteParams{
		SandboxName: sandboxName,
	})
	if err != nil {
		core.PrintError("Delete", fmt.Errorf("failed to delete preview '%s' from sandbox '%s': %w", previewName, sandboxName, err))
		os.Exit(1)
	}

	fmt.Printf("Resource Preview:%s deleted from sandbox %s\n", previewName, sandboxName)
}

func deleteSandboxPreviewToken(sandboxName, previewName, tokenName string) {
	ctx := context.Background()
	client := core.GetClient()

	_, err := client.Sandboxes.Previews.Tokens.Delete(ctx, tokenName, blaxel.SandboxPreviewTokenDeleteParams{
		SandboxName: sandboxName,
		PreviewName: previewName,
	})
	if err != nil {
		core.PrintError("Delete", fmt.Errorf("failed to delete token '%s' from preview '%s' in sandbox '%s': %w", tokenName, previewName, sandboxName, err))
		os.Exit(1)
	}

	fmt.Printf("Resource PreviewToken:%s deleted from preview %s in sandbox %s\n", tokenName, previewName, sandboxName)
}
