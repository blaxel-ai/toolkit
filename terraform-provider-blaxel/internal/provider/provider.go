package provider

import (
	"context"
	"os"

	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &blaxelProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &blaxelProvider{
			version: version,
		}
	}
}

// blaxelProvider is the provider implementation.
type blaxelProvider struct {
	version string
}

// blaxelProviderModel maps provider schema data to a Go type.
type blaxelProviderModel struct {
	ApiKey    types.String `tfsdk:"api_key"`
	Workspace types.String `tfsdk:"workspace"`
	ApiUrl    types.String `tfsdk:"api_url"`
	RunUrl    types.String `tfsdk:"run_url"`
}

// Metadata returns the provider type name.
func (p *blaxelProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "blaxel"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *blaxelProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with Blaxel API.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "API key for Blaxel authentication. Can also be set via BL_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"workspace": schema.StringAttribute{
				Description: "Blaxel workspace name. Can also be set via BL_WORKSPACE environment variable.",
				Optional:    true,
			},
			"api_url": schema.StringAttribute{
				Description: "Blaxel API URL. Can also be set via BLAXEL_API_URL environment variable. Defaults to https://api.blaxel.ai/v0",
				Optional:    true,
			},
			"run_url": schema.StringAttribute{
				Description: "Blaxel Run URL. Can also be set via BLAXEL_RUN_URL environment variable. Defaults to https://run.blaxel.ai",
				Optional:    true,
			},
		},
	}
}

// Configure prepares a Blaxel API client for data sources and resources.
func (p *blaxelProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Blaxel client")

	var config blaxelProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.
	if config.ApiKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Blaxel API Key",
			"The provider cannot create the Blaxel API client as there is an unknown configuration value for the Blaxel API key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the BL_API_KEY environment variable.",
		)
	}

	if config.Workspace.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspace"),
			"Unknown Blaxel Workspace",
			"The provider cannot create the Blaxel API client as there is an unknown configuration value for the Blaxel workspace. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the BL_WORKSPACE environment variable.",
		)
	}

	if config.ApiUrl.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Unknown Blaxel API URL",
			"The provider cannot create the Blaxel API client as there is an unknown configuration value for the Blaxel API URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the BLAXEL_API_URL environment variable.",
		)
	}

	if config.RunUrl.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("run_url"),
			"Unknown Blaxel Run URL",
			"The provider cannot create the Blaxel API client as there is an unknown configuration value for the Blaxel Run URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the BLAXEL_RUN_URL environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values from environment variables
	apiKey := os.Getenv("BL_API_KEY")
	workspace := os.Getenv("BL_WORKSPACE")
	apiUrl := os.Getenv("BLAXEL_API_URL")
	runUrl := os.Getenv("BLAXEL_RUN_URL")

	// Override with provider configuration if set
	if !config.ApiKey.IsNull() {
		apiKey = config.ApiKey.ValueString()
	}

	if !config.Workspace.IsNull() {
		workspace = config.Workspace.ValueString()
	}

	if !config.ApiUrl.IsNull() {
		apiUrl = config.ApiUrl.ValueString()
	}

	if !config.RunUrl.IsNull() {
		runUrl = config.RunUrl.ValueString()
	}

	// Default URLs if not specified
	if apiUrl == "" {
		apiUrl = "https://api.blaxel.ai/v0"
	}

	if runUrl == "" {
		runUrl = "https://run.blaxel.ai"
	}
	// If any of the expected configurations are missing, return errors
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Blaxel API Key",
			"The provider cannot create the Blaxel API client as there is a missing or empty value for the Blaxel API key. "+
				"Set the api_key value in the configuration or use the BL_API_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if workspace == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspace"),
			"Missing Blaxel Workspace",
			"The provider cannot create the Blaxel API client as there is a missing or empty value for the Blaxel workspace. "+
				"Set the workspace value in the configuration or use the BL_WORKSPACE environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "BL_WORKSPACE", workspace)
	ctx = tflog.SetField(ctx, "blaxel_api_url", apiUrl)
	ctx = tflog.SetField(ctx, "blaxel_run_url", runUrl)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "BL_API_KEY")

	tflog.Debug(ctx, "Creating Blaxel client")

	// Create a new Blaxel client using the configuration values
	client, err := sdk.NewClientWithAuth(apiUrl, runUrl, workspace, apiKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Blaxel API Client",
			"An unexpected error occurred when creating the Blaxel API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Blaxel Client Error: "+err.Error(),
		)
		return
	}

	// Make the Blaxel client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured Blaxel client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *blaxelProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

// Resources defines the resources implemented in the provider.
func (p *blaxelProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSandboxResource,
		NewSandboxClusterResource,
	}
}
