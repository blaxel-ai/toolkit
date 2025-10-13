package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &sandboxResource{}
	_ resource.ResourceWithConfigure   = &sandboxResource{}
	_ resource.ResourceWithImportState = &sandboxResource{}
)

// NewSandboxResource is a helper function to simplify the provider implementation.
func NewSandboxResource() resource.Resource {
	return &sandboxResource{}
}

// sandboxResource is the resource implementation.
type sandboxResource struct {
	client *sdk.ClientWithResponses
}

// sandboxResourceModel maps the resource schema data.
type sandboxResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Image       types.String `tfsdk:"image"`
	Region      types.String `tfsdk:"region"`
	Memory      types.Int64  `tfsdk:"memory"`
	Generation  types.String `tfsdk:"generation"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Status      types.String `tfsdk:"status"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

// Metadata returns the resource type name.
func (r *sandboxResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sandbox"
}

// Schema defines the schema for the resource.
func (r *sandboxResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Blaxel sandbox.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier for the sandbox.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the sandbox.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Description: "Display name of the sandbox.",
				Optional:    true,
				Computed:    true,
			},
			"image": schema.StringAttribute{
				Description: "Container image for the sandbox.",
				Required:    true,
			},
			"region": schema.StringAttribute{
				Description: "Region where the sandbox will be deployed.",
				Optional:    true,
			},
			"memory": schema.Int64Attribute{
				Description: "Memory allocation in MB for the sandbox.",
				Optional:    true,
				Computed:    true,
			},
			"generation": schema.StringAttribute{
				Description: "Generation of the sandbox (e.g., mk3).",
				Optional:    true,
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the sandbox is enabled.",
				Optional:    true,
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "Current status of the sandbox.",
				Computed:    true,
			},
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *sandboxResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *sandboxResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sandboxResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the sandbox spec
	runtime := &sdk.Runtime{
		Image: plan.Image.ValueStringPointer(),
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

	displayName := plan.Name.ValueString()
	if !plan.DisplayName.IsNull() {
		displayName = plan.DisplayName.ValueString()
	}

	sandbox := sdk.Sandbox{
		Metadata: &sdk.Metadata{
			Name:        plan.Name.ValueStringPointer(),
			DisplayName: &displayName,
		},
		Spec: spec,
	}

	// Create sandbox
	tflog.Debug(ctx, "Creating sandbox", map[string]any{"name": plan.Name.ValueString()})
	createResp, err := r.client.CreateSandboxWithResponse(ctx, sandbox)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating sandbox",
			"Could not create sandbox, unexpected error: "+err.Error(),
		)
		return
	}

	if createResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error creating sandbox",
			fmt.Sprintf("API returned status %d: %s", createResp.StatusCode(), string(createResp.Body)),
		)
		return
	}

	// Wait for sandbox to be deployed
	if err := r.waitForSandboxDeployment(ctx, plan.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for sandbox deployment",
			"Could not wait for sandbox to be deployed: "+err.Error(),
		)
		return
	}

	// Read the created sandbox to get all attributes
	getResp, err := r.client.GetSandboxWithResponse(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading sandbox",
			"Could not read sandbox after creation: "+err.Error(),
		)
		return
	}

	if getResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading sandbox",
			fmt.Sprintf("API returned status %d: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}

	var createdSandbox sdk.Sandbox
	if err := json.Unmarshal(getResp.Body, &createdSandbox); err != nil {
		resp.Diagnostics.AddError(
			"Error parsing sandbox response",
			"Could not parse sandbox response: "+err.Error(),
		)
		return
	}

	// Update the state
	r.updateStateFromSandbox(ctx, &plan, &createdSandbox)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *sandboxResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sandboxResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get sandbox from API
	getResp, err := r.client.GetSandboxWithResponse(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading sandbox",
			"Could not read sandbox: "+err.Error(),
		)
		return
	}

	if getResp.StatusCode() == 404 {
		// Sandbox no longer exists, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	if getResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading sandbox",
			fmt.Sprintf("API returned status %d: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}

	var sandbox sdk.Sandbox
	if err := json.Unmarshal(getResp.Body, &sandbox); err != nil {
		resp.Diagnostics.AddError(
			"Error parsing sandbox response",
			"Could not parse sandbox response: "+err.Error(),
		)
		return
	}

	// Update state
	r.updateStateFromSandbox(ctx, &state, &sandbox)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *sandboxResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sandboxResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the sandbox spec
	runtime := &sdk.Runtime{
		Image: plan.Image.ValueStringPointer(),
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

	displayName := plan.Name.ValueString()
	if !plan.DisplayName.IsNull() {
		displayName = plan.DisplayName.ValueString()
	}

	sandbox := sdk.Sandbox{
		Metadata: &sdk.Metadata{
			Name:        plan.Name.ValueStringPointer(),
			DisplayName: &displayName,
		},
		Spec: spec,
	}

	// Update sandbox
	tflog.Debug(ctx, "Updating sandbox", map[string]any{"name": plan.Name.ValueString()})
	updateResp, err := r.client.UpdateSandboxWithResponse(ctx, plan.Name.ValueString(), sandbox)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating sandbox",
			"Could not update sandbox: "+err.Error(),
		)
		return
	}

	if updateResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error updating sandbox",
			fmt.Sprintf("API returned status %d: %s", updateResp.StatusCode(), string(updateResp.Body)),
		)
		return
	}

	// Wait for sandbox to be deployed
	if err := r.waitForSandboxDeployment(ctx, plan.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for sandbox deployment",
			"Could not wait for sandbox to be deployed: "+err.Error(),
		)
		return
	}

	// Read the updated sandbox
	getResp, err := r.client.GetSandboxWithResponse(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading sandbox",
			"Could not read sandbox after update: "+err.Error(),
		)
		return
	}

	if getResp.StatusCode() != 200 {
		resp.Diagnostics.AddError(
			"Error reading sandbox",
			fmt.Sprintf("API returned status %d: %s", getResp.StatusCode(), string(getResp.Body)),
		)
		return
	}

	var updatedSandbox sdk.Sandbox
	if err := json.Unmarshal(getResp.Body, &updatedSandbox); err != nil {
		resp.Diagnostics.AddError(
			"Error parsing sandbox response",
			"Could not parse sandbox response: "+err.Error(),
		)
		return
	}

	// Update the state
	r.updateStateFromSandbox(ctx, &plan, &updatedSandbox)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *sandboxResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sandboxResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete sandbox
	tflog.Debug(ctx, "Deleting sandbox", map[string]any{"name": state.Name.ValueString()})
	deleteResp, err := r.client.DeleteSandboxWithResponse(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting sandbox",
			"Could not delete sandbox: "+err.Error(),
		)
		return
	}

	if deleteResp.StatusCode() != 200 && deleteResp.StatusCode() != 404 {
		resp.Diagnostics.AddError(
			"Error deleting sandbox",
			fmt.Sprintf("API returned status %d: %s", deleteResp.StatusCode(), string(deleteResp.Body)),
		)
		return
	}
}

// ImportState imports the resource state.
func (r *sandboxResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to name attribute
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// Helper function to update state from sandbox object
func (r *sandboxResource) updateStateFromSandbox(ctx context.Context, state *sandboxResourceModel, sandbox *sdk.Sandbox) {
	if sandbox.Metadata != nil {
		if sandbox.Metadata.Name != nil {
			state.Name = types.StringValue(*sandbox.Metadata.Name)
			state.ID = types.StringValue(*sandbox.Metadata.Name)
		}
		if sandbox.Metadata.DisplayName != nil {
			state.DisplayName = types.StringValue(*sandbox.Metadata.DisplayName)
		}
	}

	if sandbox.Spec != nil {
		if sandbox.Spec.Runtime != nil {
			if sandbox.Spec.Runtime.Image != nil {
				state.Image = types.StringValue(*sandbox.Spec.Runtime.Image)
			}
			if sandbox.Spec.Runtime.Memory != nil {
				state.Memory = types.Int64Value(int64(*sandbox.Spec.Runtime.Memory))
			}
			if sandbox.Spec.Runtime.Generation != nil {
				state.Generation = types.StringValue(*sandbox.Spec.Runtime.Generation)
			}
		}
		if sandbox.Spec.Region != nil {
			state.Region = types.StringValue(*sandbox.Spec.Region)
		}
		if sandbox.Spec.Enabled != nil {
			state.Enabled = types.BoolValue(*sandbox.Spec.Enabled)
		}
	}

	if sandbox.Status != nil {
		state.Status = types.StringValue(*sandbox.Status)
	}
}

// Helper function to wait for sandbox deployment
func (r *sandboxResource) waitForSandboxDeployment(ctx context.Context, name string) error {
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
				tflog.Debug(ctx, "Sandbox status", map[string]any{"status": status})

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
