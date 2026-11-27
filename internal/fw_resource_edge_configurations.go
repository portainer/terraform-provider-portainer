package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var edgeConfigTypeToString = map[int]string{
	1: "general",
}

// edgeConfigTypeEquivalent reports whether two values of `type` name the same
// edge configuration type: the textual form and the numeric one are
// interchangeable (what the SDKv2 DiffSuppressFunc accepted).
func edgeConfigTypeEquivalent(old, new string) bool {
	if oldInt, err := strconv.Atoi(old); err == nil {
		if name, ok := edgeConfigTypeToString[oldInt]; ok {
			return name == new
		}
	}
	if newInt, err := strconv.Atoi(new); err == nil {
		if name, ok := edgeConfigTypeToString[newInt]; ok {
			return name == old
		}
	}
	return old == new
}

type EdgeConfiguration struct {
	ID           int         `json:"id"`
	Name         string      `json:"name"`
	Type         int         `json:"type"`
	Category     string      `json:"category"`
	BaseDir      string      `json:"baseDir"`
	EdgeGroupIDs []int       `json:"edgeGroupIDs"`
	Created      int64       `json:"created"`
	CreatedBy    int         `json:"createdBy"`
	Updated      int64       `json:"updated"`
	UpdatedBy    int         `json:"updatedBy"`
	Prev         interface{} `json:"prev"`
}

var (
	_ resource.Resource                = (*edgeConfigurationsResource)(nil)
	_ resource.ResourceWithConfigure   = (*edgeConfigurationsResource)(nil)
	_ resource.ResourceWithImportState = (*edgeConfigurationsResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*edgeConfigurationsResource)(nil)
)

func newEdgeConfigurationsResource() resource.Resource { return &edgeConfigurationsResource{} }

type edgeConfigurationsResource struct {
	fwIDIdentity
	client *APIClient
}

type edgeConfigurationsModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	Category     types.String `tfsdk:"category"`
	BaseDir      types.String `tfsdk:"base_dir"`
	EdgeGroupIDs types.List   `tfsdk:"edge_group_ids"`
	FilePath     types.String `tfsdk:"file_path"`
	FileSHA256   types.String `tfsdk:"file_sha256"`
}

func (r *edgeConfigurationsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_configurations"
}

func (r *edgeConfigurationsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the edge configuration.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Name of the Portainer edge configuration.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Edge configuration type. Accepts the textual form `general` or the numeric form `1`.",
			},
			"category": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString(""),
				Validators:    []validator.String{stringvalidator.OneOf("configuration", "secret", "")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Category of the edge configuration. One of `configuration`, `secret`, or empty string.",
			},
			"base_dir": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Base directory on edge agents where the configuration files are written.",
			},
			"edge_group_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.Int64Type,
				Description: "List of edge group identifiers that should receive this configuration.",
			},
			"file_path": schema.StringAttribute{
				Required:    true,
				Description: "Path on the local filesystem to the file or archive uploaded as the edge configuration payload.",
			},
			"file_sha256": schema.StringAttribute{
				Computed:    true,
				Description: "SHA256 hash of the uploaded edge configuration file used to detect content changes.",
			},
		},
	}
}

func (r *edgeConfigurationsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ModifyPlan hashes the file at file_path during plan and writes it to
// file_sha256. If the new hash differs from the value in state, Terraform
// sees a diff and triggers an in-place Update - even when file_path didn't
// change (i.e. the file's contents were rewritten in place).
func (r *edgeConfigurationsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	edgeFileHashPlan(ctx, &resp.Plan, &resp.Diagnostics)
}

// listEdgeConfigurations fetches all edge configurations from Portainer.
func listEdgeConfigurations(ctx context.Context, client *APIClient) ([]EdgeConfiguration, error) {
	var configs []EdgeConfiguration
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/edge_configurations", client.Endpoint), nil, &configs); err != nil {
		return nil, fmt.Errorf("failed to list edge configurations: %w", err)
	}
	return configs, nil
}

// resolveCreatedEdgeConfigID disambiguates the just-created edge configuration
// when the Portainer API returns an empty POST response. It diffs the post-create
// listing against a pre-create snapshot of IDs sharing the same name; if no new
// entry is found, it falls back to the most recently created matching config.
//
// Background: Portainer's POST /edge_configurations does not return the new ID
// (https://github.com/portainer/terraform-provider-portainer/issues/115). Using
// name alone causes the provider to bind to a pre-existing same-name config,
// later mutating or deleting it. The snapshot-based diff fixes that for any
// case where a same-name config already exists.
func resolveCreatedEdgeConfigID(configs []EdgeConfiguration, name string, preExistingIDs map[int]struct{}) (EdgeConfiguration, error) {
	var newMatches []EdgeConfiguration
	allMatches := make([]EdgeConfiguration, 0, len(configs))
	for _, c := range configs {
		if c.Name != name {
			continue
		}
		allMatches = append(allMatches, c)
		if _, existed := preExistingIDs[c.ID]; !existed {
			newMatches = append(newMatches, c)
		}
	}

	pickNewest := func(in []EdgeConfiguration) EdgeConfiguration {
		sort.Slice(in, func(i, j int) bool { return in[i].Created > in[j].Created })
		return in[0]
	}

	switch {
	case len(newMatches) >= 1:
		// Exactly the entry POST just produced; if a concurrent caller raced
		// us, prefer the most recently created one.
		return pickNewest(newMatches), nil
	case len(allMatches) >= 1:
		// Server returned no new entry (replication lag, server quirk). Best
		// effort: pick the most recently created matching name.
		return pickNewest(allMatches), nil
	default:
		return EdgeConfiguration{}, fmt.Errorf("edge configuration created but could not determine its ID")
	}
}

// edgeConfigurationForm builds the multipart body: the JSON payload in the
// "edgeConfiguration" field and the file in "file".
func edgeConfigurationForm(payload map[string]interface{}, filePath string) (*bytes.Buffer, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal edgeConfiguration payload: %w", err)
	}
	_ = writer.WriteField("edgeConfiguration", string(payloadBytes))
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("failed to copy file content: %w", err)
	}
	writer.Close()
	return body, writer.FormDataContentType(), nil
}

func (r *edgeConfigurationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeConfigurationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()

	// Snapshot existing edge configuration IDs sharing the requested name.
	// Portainer's POST does not return the new ID, so after the create we
	// diff the listing against this snapshot to identify the new entry. This
	// is what prevents adopting a pre-existing same-name config (issue #115).
	preCreate, err := listEdgeConfigurations(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create edge configuration", err.Error())
		return
	}
	preExistingIDs := make(map[int]struct{})
	for _, c := range preCreate {
		if c.Name == name {
			preExistingIDs[c.ID] = struct{}{}
		}
	}

	groups := int64ListToInts(ctx, plan.EdgeGroupIDs, &resp.Diagnostics)
	if groups == nil {
		groups = []int{}
	}
	filePath := plan.FilePath.ValueString()
	body, contentType, err := edgeConfigurationForm(map[string]interface{}{
		"name":         name,
		"type":         plan.Type.ValueString(),
		"category":     plan.Category.ValueString(),
		"baseDir":      plan.BaseDir.ValueString(),
		"edgeGroupIDs": groups,
	}, filePath)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create edge configuration", err.Error())
		return
	}

	status, respBody, err := edgePostMultipart(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/edge_configurations", r.client.Endpoint), contentType, body, true)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create edge configuration", err.Error())
		return
	}
	if status >= 400 {
		resp.Diagnostics.AddError("Failed to create edge configuration", fmt.Sprintf("failed to create edge configuration: %s", string(respBody)))
		return
	}

	var created EdgeConfiguration
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &created); err != nil {
			resp.Diagnostics.AddError("Failed to create edge configuration", fmt.Sprintf("failed to decode create response: %s", err))
			return
		}
	}
	if created.ID == 0 {
		postCreate, err := listEdgeConfigurations(ctx, r.client)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create edge configuration", err.Error())
			return
		}
		created, err = resolveCreatedEdgeConfigID(postCreate, name, preExistingIDs)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create edge configuration", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(strconv.Itoa(created.ID))
	// Persist the file's SHA256 in state so future plans can detect content
	// changes even when file_path is unchanged (issue #116). ModifyPlan
	// already computed this for the plan, but it is re-computed here so the
	// value stored matches the bytes actually uploaded. Like the SDKv2
	// version, the configuration is not read back after creation.
	if hash, err := sha256File(filePath); err == nil {
		plan.FileSHA256 = types.StringValue(hash)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *edgeConfigurationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeConfigurationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rawID := filepath.Base(plan.ID.ValueString())

	groups := int64ListToInts(ctx, plan.EdgeGroupIDs, &resp.Diagnostics)
	if groups == nil {
		groups = []int{}
	}
	// Portainer API docs are incorrect. API expects form data as follows,
	// note lower case naming as well:
	// edgeConfiguration: {"edgeGroupIDs":[...],"type":"..."}
	// file:              (binary file)
	filePath := plan.FilePath.ValueString()
	body, contentType, err := edgeConfigurationForm(map[string]interface{}{
		"type":         plan.Type.ValueString(),
		"edgeGroupIDs": groups,
	}, filePath)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update edge configuration", err.Error())
		return
	}

	status, respBody, err := edgePostMultipart(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/edge_configurations/%s", r.client.Endpoint, rawID), contentType, body, true)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update edge configuration", err.Error())
		return
	}
	if status >= 400 {
		resp.Diagnostics.AddError("Failed to update edge configuration", fmt.Sprintf("failed to update edge configuration: %s", string(respBody)))
		return
	}

	if hash, err := sha256File(filePath); err == nil {
		plan.FileSHA256 = types.StringValue(hash)
	}
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *edgeConfigurationsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state edgeConfigurationsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *edgeConfigurationsResource) readInto(ctx context.Context, m *edgeConfigurationsModel, state *tfsdk.State, diags *diag.Diagnostics) {
	rawID := filepath.Base(m.ID.ValueString())

	var config EdgeConfiguration
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/edge_configurations/%s", r.client.Endpoint, rawID), nil, &config); err != nil {
		if isAPINotFound(err) {
			state.RemoveResource(ctx)
			return
		}
		diags.AddError("Failed to read edge configuration", fmt.Sprintf("failed to read edge configuration: %s", err))
		return
	}

	m.Name = types.StringValue(config.Name)
	m.Category = types.StringValue(config.Category)
	m.BaseDir = types.StringValue(config.BaseDir)
	m.EdgeGroupIDs = int64ListValue(config.EdgeGroupIDs)

	typeValue := strconv.Itoa(config.Type)
	if typeName, ok := edgeConfigTypeToString[config.Type]; ok {
		typeValue = typeName
	}
	// The numeric and the textual form are the same type: whichever form the
	// state holds is kept, as the SDKv2 DiffSuppressFunc kept it.
	if m.Type.IsNull() || m.Type.IsUnknown() || !edgeConfigTypeEquivalent(m.Type.ValueString(), typeValue) {
		m.Type = types.StringValue(typeValue)
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *edgeConfigurationsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeConfigurationsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rawID := filepath.Base(state.ID.ValueString())
	url := fmt.Sprintf("%s/edge_configurations/%s", r.client.Endpoint, rawID)
	if err := doJSON(ctx, r.client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete edge configuration", fmt.Sprintf("failed to delete edge configuration: %s", err))
	}
}

func (r *edgeConfigurationsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}
