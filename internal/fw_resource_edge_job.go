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

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = (*edgeJobResource)(nil)
	_ resource.ResourceWithConfigure        = (*edgeJobResource)(nil)
	_ resource.ResourceWithImportState      = (*edgeJobResource)(nil)
	_ resource.ResourceWithConfigValidators = (*edgeJobResource)(nil)
)

func newEdgeJobResource() resource.Resource { return &edgeJobResource{} }

type edgeJobResource struct {
	fwIDIdentity
	client *APIClient
}

type edgeJobResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	CronExpression types.String `tfsdk:"cron_expression"`
	EdgeGroups     types.List   `tfsdk:"edge_groups"`
	Endpoints      types.List   `tfsdk:"endpoints"`
	Recurring      types.Bool   `tfsdk:"recurring"`
	FileContent    types.String `tfsdk:"file_content"`
	FilePath       types.String `tfsdk:"file_path"`
}

func (r *edgeJobResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_job"
}

func (r *edgeJobResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the edge job.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
				Description: "Name of the Portainer edge job.",
			},
			"cron_expression": schema.StringAttribute{
				Required:    true,
				Description: "Cron expression that schedules execution of the edge job.",
			},
			"edge_groups": schema.ListAttribute{
				ElementType: types.Int64Type,
				Required:    true,
				Description: "List of edge group identifiers targeted by the job.",
			},
			"endpoints": schema.ListAttribute{
				ElementType: types.Int64Type,
				Required:    true,
				Description: "List of edge endpoint identifiers explicitly targeted by the job (in addition to those in `edge_groups`).",
			},
			"recurring": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the edge job should run repeatedly according to the cron expression (true) or only once (false).",
			},
			"file_content": schema.StringAttribute{
				Optional:    true,
				Description: "Inline script executed by edge agents. Mutually exclusive with `file_path`.",
			},
			"file_path": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Path on the local filesystem to a script file uploaded as the job payload. Mutually exclusive with `file_content`.",
			},
		},
	}
}

func (r *edgeJobResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("file_content"), path.MatchRoot("file_path")),
	}
}

func (r *edgeJobResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func findExistingEdgeJobByName(ctx context.Context, client *APIClient, name string) (int, error) {
	var jobs []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/edge_jobs", client.Endpoint), nil, &jobs); err != nil {
		return 0, fmt.Errorf("failed to list edge jobs: %w", err)
	}

	for _, job := range jobs {
		if job["Name"] == name {
			if id, ok := job["Id"].(float64); ok {
				return int(id), nil
			}
		}
	}

	return 0, nil
}

func (r *edgeJobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()

	existingID, err := findExistingEdgeJobByName(ctx, r.client, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create edge job", fmt.Sprintf("failed to check for existing edge job: %s", err))
		return
	}
	if existingID != 0 {
		// A job of that name is adopted and updated, as the SDKv2 version did.
		plan.ID = types.StringValue(strconv.Itoa(existingID))
		if r.update(ctx, &plan, &resp.Diagnostics) {
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		return
	}

	edgeGroups := int64ListToInts(ctx, plan.EdgeGroups, &resp.Diagnostics)
	endpoints := int64ListToInts(ctx, plan.Endpoints, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	cron := plan.CronExpression.ValueString()
	recurring := plan.Recurring.ValueBool()

	var result struct {
		ID int `json:"Id"`
	}
	switch {
	case plan.FileContent.ValueString() != "":
		body := map[string]interface{}{
			"name":           name,
			"cronExpression": cron,
			"edgeGroups":     edgeGroups,
			"endpoints":      endpoints,
			"recurring":      recurring,
			"fileContent":    plan.FileContent.ValueString(),
		}
		if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/edge_jobs/create/string", r.client.Endpoint), body, &result); err != nil {
			resp.Diagnostics.AddError("Failed to create edge job", fmt.Sprintf("failed to create edge job: %s", err))
			return
		}
	case plan.FilePath.ValueString() != "":
		filePath := plan.FilePath.ValueString()
		file, err := os.Open(filePath)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create edge job", fmt.Sprintf("cannot open file: %s", err))
			return
		}
		defer file.Close()

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("Name", name)
		_ = writer.WriteField("CronExpression", cron)
		_ = writer.WriteField("EdgeGroups", edgeIntsToJSON(edgeGroups))
		_ = writer.WriteField("Endpoints", edgeIntsToJSON(endpoints))
		_ = writer.WriteField("Recurring", strconv.FormatBool(recurring))
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			resp.Diagnostics.AddError("Failed to create edge job", err.Error())
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			resp.Diagnostics.AddError("Failed to create edge job", err.Error())
			return
		}
		writer.Close()

		status, data, err := edgePostMultipart(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/edge_jobs/create/file", r.client.Endpoint), writer.FormDataContentType(), &body, false)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create edge job", err.Error())
			return
		}
		if status != http.StatusOK {
			resp.Diagnostics.AddError("Failed to create edge job", fmt.Sprintf("failed to create edge job from file: %s", string(data)))
			return
		}
		if err := json.Unmarshal(data, &result); err != nil {
			resp.Diagnostics.AddError("Failed to create edge job", err.Error())
			return
		}
	default:
		resp.Diagnostics.AddError("Failed to create edge job", "either file_content or file_path must be provided")
		return
	}

	// Like the SDKv2 version, the job is not read back after creation.
	plan.ID = types.StringValue(strconv.Itoa(result.ID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *edgeJobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state edgeJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result struct {
		Name           string                 `json:"Name"`
		CronExpression string                 `json:"CronExpression"`
		EdgeGroups     []int                  `json:"EdgeGroups"`
		Endpoints      map[string]interface{} `json:"Endpoints"`
		Recurring      bool                   `json:"Recurring"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/edge_jobs/%s", r.client.Endpoint, state.ID.ValueString()), nil, &result); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read edge job", fmt.Sprintf("failed to read edge job: %s", err))
		return
	}

	endpointIDs := make([]int, 0, len(result.Endpoints))
	for k := range result.Endpoints {
		if id, err := strconv.Atoi(k); err == nil {
			endpointIDs = append(endpointIDs, id)
		}
	}
	// Portainer keeps the endpoints in a map, so they come back unordered;
	// the configured order is kept when the same endpoints come back.
	endpointIDs = edgeJobOrderLike(ctx, state.Endpoints, endpointIDs, &resp.Diagnostics)

	state.Name = types.StringValue(result.Name)
	state.CronExpression = types.StringValue(result.CronExpression)
	state.EdgeGroups = int64ListValue(result.EdgeGroups)
	state.Recurring = types.BoolValue(result.Recurring)
	state.Endpoints = int64ListValue(endpointIDs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// edgeJobOrderLike returns ids in the order of prior when both hold the same
// identifiers, and sorted otherwise.
func edgeJobOrderLike(ctx context.Context, prior types.List, ids []int, diags *diag.Diagnostics) []int {
	previous := int64ListToInts(ctx, prior, diags)
	if len(previous) == len(ids) {
		seen := make(map[int]int, len(ids))
		for _, id := range ids {
			seen[id]++
		}
		same := true
		for _, id := range previous {
			if seen[id] == 0 {
				same = false
				break
			}
			seen[id]--
		}
		if same {
			return previous
		}
	}
	sort.Ints(ids)
	return ids
}

// update sends PUT /edge_jobs/{id}; the job is not read back, as in SDKv2.
func (r *edgeJobResource) update(ctx context.Context, plan *edgeJobResourceModel, diags *diag.Diagnostics) bool {
	edgeGroups := int64ListToInts(ctx, plan.EdgeGroups, diags)
	endpoints := int64ListToInts(ctx, plan.Endpoints, diags)
	if diags.HasError() {
		return false
	}
	payload := map[string]interface{}{
		"name":           plan.Name.ValueString(),
		"cronExpression": plan.CronExpression.ValueString(),
		"edgeGroups":     edgeGroups,
		"endpoints":      endpoints,
		"recurring":      plan.Recurring.ValueBool(),
	}
	if v := plan.FileContent.ValueString(); v != "" {
		payload["fileContent"] = v
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/edge_jobs/%s", r.client.Endpoint, plan.ID.ValueString()), payload, nil); err != nil {
		diags.AddError("Failed to update edge job", fmt.Sprintf("failed to update edge job: %s", err))
		return false
	}
	return true
}

func (r *edgeJobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeJobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.update(ctx, &plan, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *edgeJobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeJobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/edge_jobs/%s", r.client.Endpoint, state.ID.ValueString()), nil)
	if err := setAuthHeader(httpReq, r.client); err != nil {
		resp.Diagnostics.AddError("Failed to delete edge job", err.Error())
		return
	}
	httpResp, err := r.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete edge job", err.Error())
		return
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("Failed to delete edge job", "failed to delete edge job")
	}
}

func (r *edgeJobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}
