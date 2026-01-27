package cli

import (
	"context"
	"os"
	"strings"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/spf13/cobra"
)

// completionTimeout is the maximum time to wait for API calls during completion
const completionTimeout = 3 * time.Second

// completionContext returns a context with a timeout for completion API calls
func completionContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), completionTimeout)
}

// getWorkspaceFromFlags parses os.Args to find -w or --workspace flag value
func getWorkspaceFromFlags() string {
	args := os.Args
	for i, arg := range args {
		// Check for --workspace=value or -w=value
		if strings.HasPrefix(arg, "--workspace=") {
			return strings.TrimPrefix(arg, "--workspace=")
		}
		if strings.HasPrefix(arg, "-w=") {
			return strings.TrimPrefix(arg, "-w=")
		}
		// Check for --workspace value or -w value
		if (arg == "--workspace" || arg == "-w") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// getClientForCompletion returns a client configured for the workspace specified in flags,
// or the default client if no workspace flag is set.
// Uses NewClientFromCredentials which handles token refresh properly.
// Also initializes the environment based on the workspace config (dev/prod).
func getClientForCompletion() *blaxel.Client {
	workspace := getWorkspaceFromFlags()
	if workspace == "" {
		// Use default workspace from context
		ctx, _ := blaxel.CurrentContext()
		workspace = ctx.Workspace
	}

	if workspace == "" {
		return nil
	}

	// Initialize environment for this workspace (sets correct URLs for dev/prod)
	blaxel.InitializeEnvironment(workspace)

	// Load credentials for the workspace
	credentials, err := blaxel.LoadCredentials(workspace)
	if err != nil || !credentials.IsValid() {
		return nil
	}

	// Use NewClientFromCredentials which handles token refresh properly
	// GetBaseURL() now returns the correct URL based on the workspace's environment
	client := blaxel.NewClientFromCredentials(credentials,
		option.WithWorkspace(workspace),
		option.WithBaseURL(blaxel.GetBaseURL()),
	)
	return &client
}

// CompleteWorkspaceNames returns a list of workspace names from the local config for shell completion
func CompleteWorkspaceNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Load config from ~/.blaxel/config.yaml
	config, err := blaxel.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, ws := range config.Workspaces {
		if ws.Name != "" {
			if toComplete == "" || strings.HasPrefix(ws.Name, toComplete) {
				names = append(names, ws.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// GetWorkspaceValidArgsFunction returns a ValidArgsFunction for the workspace command
func GetWorkspaceValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return CompleteWorkspaceNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// sandboxNestedResourceKeywords are the keywords that indicate nested resources for sandboxes (for matching user input)
var sandboxNestedResourceKeywords = []string{"processes", "process", "proc", "procs", "ps"}

// processLogsKeywords are the keywords for getting process logs (for matching user input)
var processLogsKeywords = []string{"logs", "log"}

// jobNestedResourceKeywords are the keywords that indicate nested resources for jobs
var jobNestedResourceKeywords = []string{"execution"}

// CompleteSandboxNames returns a list of sandbox names for shell completion
func CompleteSandboxNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	sandboxes, err := client.Sandboxes.List(ctx)
	if err != nil || sandboxes == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, sbx := range *sandboxes {
		if sbx.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(sbx.Metadata.Name, toComplete) {
				names = append(names, sbx.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteSandboxProcessNames returns a list of process names for a given sandbox
func CompleteSandboxProcessNames(sandboxName string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	sandboxInstance, err := client.Sandboxes.GetInstance(ctx, sandboxName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	processes, err := sandboxInstance.Process.List(ctx)
	if err != nil || processes == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, proc := range *processes {
		if proc.Name != "" {
			if toComplete == "" || strings.HasPrefix(proc.Name, toComplete) {
				names = append(names, proc.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteJobNames returns a list of job names for shell completion
func CompleteJobNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	jobs, err := client.Jobs.List(ctx)
	if err != nil || jobs == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, job := range *jobs {
		if job.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(job.Metadata.Name, toComplete) {
				names = append(names, job.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteJobExecutionIDs returns a list of execution IDs for a given job
func CompleteJobExecutionIDs(jobName string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	executions, err := client.Jobs.Executions.List(ctx, jobName, blaxel.JobExecutionListParams{})
	if err != nil || executions == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var ids []string
	for _, exec := range *executions {
		if exec.Metadata.ID != "" {
			if toComplete == "" || strings.HasPrefix(exec.Metadata.ID, toComplete) {
				ids = append(ids, exec.Metadata.ID)
			}
		}
	}

	return ids, cobra.ShellCompDirectiveNoFileComp
}

// CompleteAgentNames returns a list of agent names for shell completion
func CompleteAgentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	agents, err := client.Agents.List(ctx)
	if err != nil || agents == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, agent := range *agents {
		if agent.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(agent.Metadata.Name, toComplete) {
				names = append(names, agent.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteFunctionNames returns a list of function names for shell completion
func CompleteFunctionNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	functions, err := client.Functions.List(ctx)
	if err != nil || functions == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, fn := range *functions {
		if fn.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(fn.Metadata.Name, toComplete) {
				names = append(names, fn.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteModelNames returns a list of model names for shell completion
func CompleteModelNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	models, err := client.Models.List(ctx)
	if err != nil || models == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, model := range *models {
		if model.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(model.Metadata.Name, toComplete) {
				names = append(names, model.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompleteVolumeNames returns a list of volume names for shell completion
func CompleteVolumeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	volumes, err := client.Volumes.List(ctx)
	if err != nil || volumes == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, vol := range *volumes {
		if vol.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(vol.Metadata.Name, toComplete) {
				names = append(names, vol.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// CompletePolicyNames returns a list of policy names for shell completion
func CompletePolicyNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	policies, err := client.Policies.List(ctx)
	if err != nil || policies == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, pol := range *policies {
		if pol.Metadata.Name != "" {
			if toComplete == "" || strings.HasPrefix(pol.Metadata.Name, toComplete) {
				names = append(names, pol.Metadata.Name)
			}
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// GetSandboxValidArgsFunction returns a ValidArgsFunction for sandbox commands
// It handles completions for:
// - sandbox names (first arg)
// - nested resource keywords like "process" (second arg)
// - process names (third arg)
// - "logs" keyword (fourth arg)
func GetSandboxValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			// Complete sandbox names
			return CompleteSandboxNames(cmd, args, toComplete)

		case 1:
			// Complete nested resource keywords with description
			var completions []string
			keyword := "process"
			if toComplete == "" || strings.HasPrefix(keyword, toComplete) {
				completions = append(completions, keyword+"\tList or get sandbox processes")
			}
			return completions, cobra.ShellCompDirectiveNoFileComp

		case 2:
			// Check if the second arg is a process-related keyword
			sandboxName := args[0]
			keyword := strings.ToLower(args[1])
			// Accept all aliases for flexibility
			for _, k := range sandboxNestedResourceKeywords {
				if keyword == k {
					// Complete process names
					return CompleteSandboxProcessNames(sandboxName, toComplete)
				}
			}
			return nil, cobra.ShellCompDirectiveNoFileComp

		case 3:
			// Complete "logs" keyword for process with description
			var completions []string
			keyword := "logs"
			if toComplete == "" || strings.HasPrefix(keyword, toComplete) {
				completions = append(completions, keyword+"\tView process logs")
			}
			return completions, cobra.ShellCompDirectiveNoFileComp

		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// GetJobValidArgsFunction returns a ValidArgsFunction for job commands
// It handles completions for:
// - job names (first arg)
// - nested resource keywords like "execution" (second arg)
// - execution IDs (third arg)
func GetJobValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			// Complete job names
			return CompleteJobNames(cmd, args, toComplete)

		case 1:
			// Complete nested resource keywords with description
			var completions []string
			keyword := "execution"
			if toComplete == "" || strings.HasPrefix(keyword, toComplete) {
				completions = append(completions, keyword+"\tList or get job executions")
			}
			return completions, cobra.ShellCompDirectiveNoFileComp

		case 2:
			// Check if the second arg is an execution-related keyword
			jobName := args[0]
			keyword := strings.ToLower(args[1])
			// Accept both "execution" and "executions" for flexibility
			if keyword == "execution" || keyword == "executions" {
				// Complete execution IDs
				return CompleteJobExecutionIDs(jobName, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp

		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// GetResourceValidArgsFunction returns a ValidArgsFunction for a given resource kind
func GetResourceValidArgsFunction(kind string) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch kind {
	case "Sandbox":
		return GetSandboxValidArgsFunction()
	case "Job":
		return GetJobValidArgsFunction()
	case "Agent":
		return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return CompleteAgentNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	case "Function":
		return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return CompleteFunctionNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	case "Model":
		return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return CompleteModelNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	case "Volume":
		return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return CompleteVolumeNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	case "Policy":
		return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return CompletePolicyNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	default:
		return nil
	}
}

// CompleteImageNames returns a list of image names in the format resourceType/imageName for shell completion
func CompleteImageNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	images, err := client.Images.List(ctx)
	if err != nil || images == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, img := range *images {
		// Build the full image reference: resourceType/imageName
		resourceType := ""
		imageName := ""
		if img.Metadata.ResourceType != "" {
			resourceType = img.Metadata.ResourceType
		}
		if img.Metadata.Name != "" {
			imageName = img.Metadata.Name
		}

		if resourceType != "" && imageName != "" {
			fullRef := resourceType + "/" + imageName
			if toComplete == "" || strings.HasPrefix(fullRef, toComplete) {
				names = append(names, fullRef)
			}
		}
	}

	// Return with no space suffix to allow user to add :tag
	return names, cobra.ShellCompDirectiveNoSpace
}

// CompleteImageTags returns a list of tags for a given image
func CompleteImageTags(resourceType, imageName, tagPrefix string) ([]string, cobra.ShellCompDirective) {
	ctx, cancel := completionContext()
	defer cancel()
	client := getClientForCompletion()
	if client == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Get the image to retrieve its tags
	image, err := client.Images.Get(ctx, imageName, blaxel.ImageGetParams{ResourceType: resourceType})
	if err != nil || image == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var tags []string
	if image.Spec.Tags != nil {
		for _, tag := range image.Spec.Tags {
			if tag.Name != "" {
				fullRef := resourceType + "/" + imageName + ":" + tag.Name
				if tagPrefix == "" || strings.HasPrefix(tag.Name, tagPrefix) {
					tags = append(tags, fullRef)
				}
			}
		}
	}

	return tags, cobra.ShellCompDirectiveNoFileComp
}

// GetImageValidArgsFunction returns a ValidArgsFunction for image commands
func GetImageValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Check if user is completing a tag (contains :)
		if strings.Contains(toComplete, ":") {
			// Parse the image reference to get resourceType/imageName and tag prefix
			parts := strings.SplitN(toComplete, ":", 2)
			imageRef := parts[0]
			tagPrefix := ""
			if len(parts) == 2 {
				tagPrefix = parts[1]
			}

			// Parse resourceType/imageName
			imageParts := strings.SplitN(imageRef, "/", 2)
			if len(imageParts) == 2 {
				resourceType := imageParts[0]
				imageName := imageParts[1]
				return CompleteImageTags(resourceType, imageName, tagPrefix)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Otherwise, complete image names (resourceType/imageName)
		return CompleteImageNames(cmd, args, toComplete)
	}
}

// logsResourceTypesWithDesc are the valid resource types for logs command with descriptions
var logsResourceTypesWithDesc = []struct {
	name string
	desc string
}{
	{"sandbox", "Isolated execution environment"},
	{"job", "Batch processing task"},
	{"agent", "AI agent application"},
	{"function", "MCP server / function"},
}

// GetLogsValidArgsFunction returns a ValidArgsFunction for the logs command
// It handles completions for:
// - resource types (first arg)
// - resource names (second arg)
func GetLogsValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			// Complete resource types with descriptions
			var completions []string
			for _, rt := range logsResourceTypesWithDesc {
				if toComplete == "" || strings.HasPrefix(rt.name, toComplete) {
					completions = append(completions, rt.name+"\t"+rt.desc)
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp

		case 1:
			// Complete resource names based on type
			resourceType := strings.ToLower(args[0])
			switch resourceType {
			case "sandbox", "sbx", "sandboxes":
				return CompleteSandboxNames(cmd, args, toComplete)
			case "job", "j", "jb", "jobs":
				return CompleteJobNames(cmd, args, toComplete)
			case "agent", "ag", "agents":
				return CompleteAgentNames(cmd, args, toComplete)
			case "function", "fn", "mcp", "mcps", "functions":
				return CompleteFunctionNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp

		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// GetConnectSandboxValidArgsFunction returns a ValidArgsFunction for the connect sandbox command
func GetConnectSandboxValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return CompleteSandboxNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// GetChatValidArgsFunction returns a ValidArgsFunction for the chat command
func GetChatValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return CompleteAgentNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// runResourceTypesWithDesc are the valid resource types for run command with descriptions
var runResourceTypesWithDesc = []struct {
	name string
	desc string
}{
	{"agent", "AI agent application"},
	{"model", "AI model configuration"},
	{"job", "Batch processing task"},
	{"function", "MCP server / function"},
	{"sandbox", "Isolated execution environment"},
}

// GetRunValidArgsFunction returns a ValidArgsFunction for the run command
// It handles completions for:
// - resource types (first arg)
// - resource names (second arg)
func GetRunValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			// Complete resource types with descriptions
			var completions []string
			for _, rt := range runResourceTypesWithDesc {
				if toComplete == "" || strings.HasPrefix(rt.name, toComplete) {
					completions = append(completions, rt.name+"\t"+rt.desc)
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp

		case 1:
			// Complete resource names based on type
			resourceType := strings.ToLower(args[0])
			switch resourceType {
			case "agent", "agents", "ag":
				return CompleteAgentNames(cmd, args, toComplete)
			case "model", "models", "ml":
				return CompleteModelNames(cmd, args, toComplete)
			case "job", "jobs", "jb":
				return CompleteJobNames(cmd, args, toComplete)
			case "function", "functions", "fn", "mcp", "mcps":
				return CompleteFunctionNames(cmd, args, toComplete)
			case "sandbox", "sandboxes", "sbx", "sb":
				return CompleteSandboxNames(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp

		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// newResourceTypesWithDesc are the valid resource types for new command with descriptions
var newResourceTypesWithDesc = []struct {
	name string
	desc string
}{
	{"agent", "AI agent application"},
	{"mcp", "MCP server (Model Context Protocol)"},
	{"sandbox", "Isolated execution environment"},
	{"job", "Batch processing task"},
	{"volumetemplate", "Volume template for persistent storage"},
}

// GetNewValidArgsFunction returns a ValidArgsFunction for the new command
// It handles completions for:
// - resource types (first arg)
func GetNewValidArgsFunction() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			// Complete resource types with descriptions
			var completions []string
			for _, rt := range newResourceTypesWithDesc {
				if toComplete == "" || strings.HasPrefix(rt.name, toComplete) {
					completions = append(completions, rt.name+"\t"+rt.desc)
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		}
		// Second arg is directory name - use default file completion
		return nil, cobra.ShellCompDirectiveDefault
	}
}
