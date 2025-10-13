package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &sandboxClusterResource{}
	_ resource.ResourceWithConfigure   = &sandboxClusterResource{}
	_ resource.ResourceWithImportState = &sandboxClusterResource{}
)

// NewSandboxClusterResource is a helper function to simplify the provider implementation.
func NewSandboxClusterResource() resource.Resource {
	return &sandboxClusterResource{}
}

// sandboxClusterResource is the resource implementation.
type sandboxClusterResource struct {
	client *sdk.ClientWithResponses
}

// sandboxClusterResourceModel maps the resource schema data.
type sandboxClusterResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	TemplateSandbox   types.String `tfsdk:"template_sandbox"`
	TemplateImage     types.String `tfsdk:"template_image"`
	Replicas          types.Int64  `tfsdk:"replicas"`
	SandboxNamePrefix types.String `tfsdk:"sandbox_name_prefix"`
	Region            types.String `tfsdk:"region"`
	Memory            types.Int64  `tfsdk:"memory"`
	Generation        types.String `tfsdk:"generation"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	DeployedSandboxes types.List   `tfsdk:"deployed_sandboxes"`
	LastUpdated       types.String `tfsdk:"last_updated"`
}

// Metadata returns the resource type name.
func (r *sandboxClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox_cluster"
}

// Schema defines the schema for the resource.
func (r *sandboxClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a cluster of Blaxel sandboxes deployed from a template sandbox.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier for the sandbox cluster.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the sandbox cluster.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_sandbox": schema.StringAttribute{
				Description: "Name of the template sandbox to deploy from. The template will be deployed first if it doesn't exist.",
				Required:    true,
			},
			"template_image": schema.StringAttribute{
				Description: "Initial image for the template sandbox. This will be used to create the template if it doesn't exist.",
				Required:    true,
			},
			"replicas": schema.Int64Attribute{
				Description: "Number of sandboxes to deploy in the cluster.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"sandbox_name_prefix": schema.StringAttribute{
				Description: "Prefix for the sandbox names. Sandboxes will be named as <prefix>-0, <prefix>-1, etc.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Description: "Region where the sandboxes will be deployed.",
				Optional:    true,
			},
			"memory": schema.Int64Attribute{
				Description: "Memory allocation in MB for each sandbox.",
				Optional:    true,
				Computed:    true,
			},
			"generation": schema.StringAttribute{
				Description: "Generation of the sandboxes (e.g., mk3).",
				Optional:    true,
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the sandboxes are enabled.",
				Optional:    true,
				Computed:    true,
			},
			"deployed_sandboxes": schema.ListAttribute{
				Description: "List of deployed sandbox names.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *sandboxClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*sdk.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *sdk.ClientWithResponses, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *sandboxClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sandboxClusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Step 1: Deploy or get the template sandbox
	tflog.Info(ctx, "Deploying template sandbox", map[string]any{"template": plan.TemplateSandbox.ValueString()})
	templateImage, err := r.ensureTemplateSandbox(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deploying template sandbox",
			"Could not deploy template sandbox: "+err.Error(),
		)
		return
	}

	tflog.Info(ctx, "Template sandbox deployed", map[string]any{"image": templateImage})

	// Step 2: Deploy N sandboxes from the template image
	replicas := plan.Replicas.ValueInt64()
	deployedSandboxes := make([]attr.Value, 0, replicas)

	for i := int64(0); i < replicas; i++ {
		sandboxName := fmt.Sprintf("%s-%d", plan.SandboxNamePrefix.ValueString(), i)
		tflog.Info(ctx, "Deploying sandbox from template", map[string]any{
			"name":  sandboxName,
			"image": templateImage,
			"index": i,
		})

		if err := r.deploySandboxFromTemplate(ctx, sandboxName, templateImage, plan); err != nil {
			resp.Diagnostics.AddError(
				"Error deploying sandbox",
				fmt.Sprintf("Could not deploy sandbox %s: %s", sandboxName, err.Error()),
			)
			// Clean up any sandboxes that were created
			r.cleanupSandboxes(ctx, deployedSandboxes)
			return
		}

		deployedSandboxes = append(deployedSandboxes, types.StringValue(sandboxName))
		tflog.Info(ctx, "Sandbox deployed successfully", map[string]any{"name": sandboxName})
	}

	// Update the state
	plan.ID = types.StringValue(plan.Name.ValueString())
	plan.DeployedSandboxes, _ = types.ListValue(types.StringType, deployedSandboxes)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *sandboxClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sandboxClusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Verify that all deployed sandboxes still exist
	var deployedSandboxes []attr.Value
	state.DeployedSandboxes.ElementsAs(ctx, &deployedSandboxes, false)

	stillExists := make([]attr.Value, 0)
	for _, sandboxName := range deployedSandboxes {
		name := sandboxName.(types.String).ValueString()
		getResp, err := r.client.GetSandboxWithResponse(ctx, name)
		if err != nil {
			tflog.Warn(ctx, "Error reading sandbox", map[string]any{"name": name, "error": err.Error()})
			continue
		}

		if getResp.StatusCode() == 200 {
			stillExists = append(stillExists, sandboxName)
		}
	}

	if len(stillExists) == 0 {
		// All sandboxes are gone, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	state.DeployedSandboxes, _ = types.ListValue(types.StringType, stillExists)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *sandboxClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sandboxClusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sandboxClusterResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the template sandbox first
	tflog.Info(ctx, "Updating template sandbox", map[string]any{"template": plan.TemplateSandbox.ValueString()})
	templateImage, err := r.ensureTemplateSandbox(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating template sandbox",
			"Could not update template sandbox: "+err.Error(),
		)
		return
	}

	// Update all deployed sandboxes
	var deployedSandboxes []attr.Value
	state.DeployedSandboxes.ElementsAs(ctx, &deployedSandboxes, false)

	for _, sandboxName := range deployedSandboxes {
		name := sandboxName.(types.String).ValueString()
		tflog.Info(ctx, "Updating sandbox", map[string]any{"name": name, "image": templateImage})

		if err := r.updateSandbox(ctx, name, templateImage, plan); err != nil {
			resp.Diagnostics.AddError(
				"Error updating sandbox",
				fmt.Sprintf("Could not update sandbox %s: %s", name, err.Error()),
			)
			return
		}
	}

	// Update the state
	plan.ID = state.ID
	plan.DeployedSandboxes = state.DeployedSandboxes
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *sandboxClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sandboxClusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete all deployed sandboxes
	var deployedSandboxes []attr.Value
	state.DeployedSandboxes.ElementsAs(ctx, &deployedSandboxes, false)

	r.cleanupSandboxes(ctx, deployedSandboxes)

	// Delete the template sandbox
	templateName := state.TemplateSandbox.ValueString()
	tflog.Info(ctx, "Deleting template sandbox", map[string]any{"template": templateName})
	deleteResp, err := r.client.DeleteSandboxWithResponse(ctx, templateName)
	if err != nil {
		tflog.Warn(ctx, "Error deleting template sandbox", map[string]any{"template": templateName, "error": err.Error()})
	} else if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 404 {
		tflog.Warn(ctx, "Non-200 status deleting template sandbox", map[string]any{"template": templateName, "status": deleteResp.StatusCode()})
	}
}

// ImportState imports the resource state.
func (r *sandboxClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to name attribute
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// Helper function to ensure template sandbox is deployed and return its image
func (r *sandboxClusterResource) ensureTemplateSandbox(ctx context.Context, plan sandboxClusterResourceModel) (string, error) {
	templateName := plan.TemplateSandbox.ValueString()

	// Check if template sandbox exists
	getResp, err := r.client.GetSandboxWithResponse(ctx, templateName)
	if err != nil {
		return "", fmt.Errorf("error checking template sandbox: %w", err)
	}

	if getResp.StatusCode() == 404 {
		// Template doesn't exist, create it
		tflog.Info(ctx, "Template sandbox not found, creating", map[string]any{"template": templateName})
		if err := r.createTemplateSandbox(ctx, templateName, plan); err != nil {
			return "", fmt.Errorf("error creating template sandbox: %w", err)
		}

		// Wait for template to be deployed
		if err := r.waitForSandboxDeployment(ctx, templateName); err != nil {
			return "", fmt.Errorf("error waiting for template deployment: %w", err)
		}

		// Get the template sandbox again to extract the image
		getResp, err = r.client.GetSandboxWithResponse(ctx, templateName)
		if err != nil {
			return "", fmt.Errorf("error reading template sandbox after creation: %w", err)
		}
	} else if getResp.StatusCode() != 200 {
		return "", fmt.Errorf("API returned status %d: %s", getResp.StatusCode(), string(getResp.Body))
	}

	// Extract the image from the template sandbox
	var sandbox sdk.Sandbox
	if err := json.Unmarshal(getResp.Body, &sandbox); err != nil {
		return "", fmt.Errorf("error parsing template sandbox response: %w", err)
	}

	if sandbox.Spec == nil || sandbox.Spec.Runtime == nil || sandbox.Spec.Runtime.Image == nil {
		return "", fmt.Errorf("template sandbox has no image specified")
	}

	return *sandbox.Spec.Runtime.Image, nil
}

// Helper function to create template sandbox
func (r *sandboxClusterResource) createTemplateSandbox(ctx context.Context, templateName string, plan sandboxClusterResourceModel) error {
	runtime := &sdk.Runtime{
		Image: plan.TemplateImage.ValueStringPointer(),
	}

	if !plan.Memory.IsNull() {
		memory := int(plan.Memory.ValueInt64())
		runtime.Memory = &memory
	}

	if !plan.Generation.IsNull() {
		generation := plan.Generation.ValueString()
		runtime.Generation = &generation
	}

	enabled := true
	if !plan.Enabled.IsNull() {
		enabled = plan.Enabled.ValueBool()
	}

	spec := &sdk.SandboxSpec{
		Runtime: runtime,
		Enabled: &enabled,
	}

	if !plan.Region.IsNull() {
		region := plan.Region.ValueString()
		spec.Region = &region
	}

	sandbox := sdk.Sandbox{
		Metadata: &sdk.Metadata{
			Name:        &templateName,
			DisplayName: &templateName,
		},
		Spec: spec,
	}

	createResp, err := r.client.CreateSandboxWithResponse(ctx, sandbox)
	if err != nil {
		return fmt.Errorf("error creating template sandbox: %w", err)
	}

	if createResp.StatusCode() != 200 {
		return fmt.Errorf("API returned status %d: %s", createResp.StatusCode(), string(createResp.Body))
	}

	return nil
}

// Helper function to deploy a sandbox from a template image
func (r *sandboxClusterResource) deploySandboxFromTemplate(ctx context.Context, name string, image string, plan sandboxClusterResourceModel) error {
	runtime := &sdk.Runtime{
		Image: &image,
	}

	if !plan.Memory.IsNull() {
		memory := int(plan.Memory.ValueInt64())
		runtime.Memory = &memory
	}

	if !plan.Generation.IsNull() {
		generation := plan.Generation.ValueString()
		runtime.Generation = &generation
	}

	enabled := true
	if !plan.Enabled.IsNull() {
		enabled = plan.Enabled.ValueBool()
	}

	spec := &sdk.SandboxSpec{
		Runtime: runtime,
		Enabled: &enabled,
	}

	if !plan.Region.IsNull() {
		region := plan.Region.ValueString()
		spec.Region = &region
	}

	sandbox := sdk.Sandbox{
		Metadata: &sdk.Metadata{
			Name:        &name,
			DisplayName: &name,
		},
		Spec: spec,
	}

	createResp, err := r.client.CreateSandboxWithResponse(ctx, sandbox)
	if err != nil {
		return fmt.Errorf("error creating sandbox: %w", err)
	}

	if createResp.StatusCode() != 200 {
		return fmt.Errorf("API returned status %d: %s", createResp.StatusCode(), string(createResp.Body))
	}

	// Wait for sandbox to be deployed
	if err := r.waitForSandboxDeployment(ctx, name); err != nil {
		return fmt.Errorf("error waiting for sandbox deployment: %w", err)
	}

	return nil
}

// Helper function to update a sandbox
func (r *sandboxClusterResource) updateSandbox(ctx context.Context, name string, image string, plan sandboxClusterResourceModel) error {
	runtime := &sdk.Runtime{
		Image: &image,
	}

	if !plan.Memory.IsNull() {
		memory := int(plan.Memory.ValueInt64())
		runtime.Memory = &memory
	}

	if !plan.Generation.IsNull() {
		generation := plan.Generation.ValueString()
		runtime.Generation = &generation
	}

	enabled := true
	if !plan.Enabled.IsNull() {
		enabled = plan.Enabled.ValueBool()
	}

	spec := &sdk.SandboxSpec{
		Runtime: runtime,
		Enabled: &enabled,
	}

	if !plan.Region.IsNull() {
		region := plan.Region.ValueString()
		spec.Region = &region
	}

	sandbox := sdk.Sandbox{
		Metadata: &sdk.Metadata{
			Name:        &name,
			DisplayName: &name,
		},
		Spec: spec,
	}

	updateResp, err := r.client.UpdateSandboxWithResponse(ctx, name, sandbox)
	if err != nil {
		return fmt.Errorf("error updating sandbox: %w", err)
	}

	if updateResp.StatusCode() != 200 {
		return fmt.Errorf("API returned status %d: %s", updateResp.StatusCode(), string(updateResp.Body))
	}

	// Wait for sandbox to be deployed
	if err := r.waitForSandboxDeployment(ctx, name); err != nil {
		return fmt.Errorf("error waiting for sandbox deployment: %w", err)
	}

	return nil
}

// Helper function to wait for sandbox deployment
func (r *sandboxClusterResource) waitForSandboxDeployment(ctx context.Context, name string) error {
	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for sandbox deployment")
		case <-ticker.C:
			getResp, err := r.client.GetSandboxWithResponse(ctx, name)
			if err != nil {
				tflog.Debug(ctx, "Error getting sandbox status", map[string]any{"error": err.Error()})
				continue
			}

			if getResp.StatusCode() != 200 {
				tflog.Debug(ctx, "Non-200 status getting sandbox", map[string]any{"status": getResp.StatusCode()})
				continue
			}

			var sandbox sdk.Sandbox
			if err := json.Unmarshal(getResp.Body, &sandbox); err != nil {
				tflog.Debug(ctx, "Error unmarshaling sandbox", map[string]any{"error": err.Error()})
				continue
			}

			if sandbox.Status != nil {
				status := *sandbox.Status
				tflog.Debug(ctx, "Sandbox status", map[string]any{"name": name, "status": status})

				switch status {
				case "DEPLOYED":
					return nil
				case "FAILED", "DEACTIVATED", "DEACTIVATING", "DELETING":
					return fmt.Errorf("sandbox deployment failed with status: %s", status)
				}
			}
		}
	}
}

// Helper function to cleanup sandboxes
func (r *sandboxClusterResource) cleanupSandboxes(ctx context.Context, sandboxes []attr.Value) {
	for _, sandboxName := range sandboxes {
		name := sandboxName.(types.String).ValueString()
		tflog.Info(ctx, "Deleting sandbox", map[string]any{"name": name})

		deleteResp, err := r.client.DeleteSandboxWithResponse(ctx, name)
		if err != nil {
			tflog.Warn(ctx, "Error deleting sandbox", map[string]any{"name": name, "error": err.Error()})
		} else if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 404 {
			tflog.Warn(ctx, "Non-200 status deleting sandbox", map[string]any{"name": name, "status": deleteResp.StatusCode()})
		}
	}
}
