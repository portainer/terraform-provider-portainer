package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// kubeSoloOfflineBundleName is the file name KubeSolo publishes offline
// tarballs under, which Portainer requires the upload to carry:
// kubesolo-<version>-linux-<arch>[-musl]-offline.tar.gz.
var kubeSoloOfflineBundleName = regexp.MustCompile(`^kubesolo-[^/]+-linux-[^/]+-offline\.tar\.gz$`)

var (
	_ resource.Resource                = (*kubeSoloOfflineBundleResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubeSoloOfflineBundleResource)(nil)
	_ resource.ResourceWithImportState = (*kubeSoloOfflineBundleResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*kubeSoloOfflineBundleResource)(nil)
)

func newKubeSoloOfflineBundleResource() resource.Resource { return &kubeSoloOfflineBundleResource{} }

// kubeSoloOfflineBundleResource uploads a KubeSolo offline tarball. PUT
// /kubesolo/offline-bundles/{name} stores the tarball or replaces the one
// stored under that name, so create and update run the same upload.
type kubeSoloOfflineBundleResource struct{ client *APIClient }

type kubeSoloOfflineBundleModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	FilePath   types.String `tfsdk:"file_path"`
	FileSHA256 types.String `tfsdk:"file_sha256"`
	Arch       types.String `tfsdk:"arch"`
	Libc       types.String `tfsdk:"libc"`
	Size       types.Int64  `tfsdk:"size"`
}

func (r *kubeSoloOfflineBundleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubesolo_offline_bundle"
}

func (r *kubeSoloOfflineBundleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "File name of the bundle, the same as `name`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(kubeSoloOfflineBundleName, "must be the published tarball name, kubesolo-<version>-linux-<arch>[-musl]-offline.tar.gz"),
				},
				Description: "File name of the tarball as published with the KubeSolo release, for example `kubesolo-v1.2.3-linux-amd64-offline.tar.gz`. It must be a tarball of the release bundled with Portainer (see `portainer_kubesolo_versions`).",
			},
			"file_path": schema.StringAttribute{
				Required:    true,
				Description: "Path on the local filesystem to the tarball to upload. Portainer keeps it only if it matches the release's SHA256SUMS.",
			},
			"file_sha256": schema.StringAttribute{
				Computed:    true,
				Description: "SHA256 hash of the uploaded tarball, used to upload it again when the file at `file_path` changes.",
			},
			"arch": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "CPU architecture the bundle installs on, for example `amd64`.",
			},
			"libc": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "C library of the machines the bundle installs on: `glibc` or `musl`.",
			},
			"size": schema.Int64Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description:   "Size of the stored tarball in bytes.",
			},
		},
	}
}

func (r *kubeSoloOfflineBundleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ModifyPlan plans an upload when the tarball at file_path was replaced
// without the path changing, the same way edge configurations do: Portainer
// exposes no digest of what it stores. A new tarball also has a new size.
func (r *kubeSoloOfflineBundleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	edgeFileHashPlan(ctx, &resp.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || req.State.Raw.IsNull() {
		return
	}
	var planned, prior types.String
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("file_sha256"), &planned)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("file_sha256"), &prior)...)
	if !planned.Equal(prior) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("size"), types.Int64Unknown())...)
	}
}

func kubeSoloOfflineBundleURL(client *APIClient, name string) string {
	return fmt.Sprintf("%s/kubesolo/offline-bundles/%s", client.Endpoint, url.PathEscape(name))
}

// uploadKubeSoloOfflineBundle streams the tarball as the raw request body
// (application/octet-stream). Bundles run to hundreds of megabytes, so the
// file is never read into memory; GetBody reopens it so the retry transport
// can replay the PUT.
func uploadKubeSoloOfflineBundle(ctx context.Context, client *APIClient, name, path string) (*kubeSoloOfflineBundle, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	open := func() (io.ReadCloser, error) { return os.Open(path) }
	body, err := open()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, kubeSoloOfflineBundleURL(client, name), body)
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	req.ContentLength = info.Size()
	req.GetBody = open
	req.Header.Set("Content-Type", "application/octet-stream")
	if err := setAuthHeader(req, client); err != nil {
		_ = body.Close()
		return nil, err
	}

	var bundle kubeSoloOfflineBundle
	if err := sendDecode(client, req, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

// upload hashes and uploads the tarball, then reads the bundle back. Only a
// first upload can 404 for want of the endpoint; on a replace the bundle's
// own name is in the path.
func (r *kubeSoloOfflineBundleResource) upload(ctx context.Context, m *kubeSoloOfflineBundleModel, first bool, state *tfsdk.State, diags *diag.Diagnostics) {
	name, filePath := m.Name.ValueString(), m.FilePath.ValueString()
	hash, err := sha256File(filePath)
	if err != nil {
		diags.AddError("Failed to upload KubeSolo offline bundle", fmt.Sprintf("failed to hash file_path %q: %s", filePath, err))
		return
	}
	if _, err := uploadKubeSoloOfflineBundle(ctx, r.client, name, filePath); err != nil {
		if first {
			err = explainUnsupported(ctx, r.client, err, "portainer_kubesolo_offline_bundle", requiresBE30)
		}
		diags.AddError("Failed to upload KubeSolo offline bundle", fmt.Sprintf("failed to upload KubeSolo offline bundle %q: %s", name, err))
		return
	}
	m.ID = types.StringValue(name)
	m.FileSHA256 = types.StringValue(hash)
	r.readInto(ctx, m, state, diags)
}

func (r *kubeSoloOfflineBundleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubeSoloOfflineBundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upload(ctx, &plan, true, &resp.State, &resp.Diagnostics)
	if !plan.ID.IsUnknown() {
		edgeKeepCreatedOnError(ctx, req.Plan, plan.ID.ValueString(), &resp.State, &resp.Diagnostics)
	}
}

func (r *kubeSoloOfflineBundleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubeSoloOfflineBundleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upload(ctx, &plan, false, &resp.State, &resp.Diagnostics)
}

func (r *kubeSoloOfflineBundleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubeSoloOfflineBundleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *kubeSoloOfflineBundleResource) readInto(ctx context.Context, m *kubeSoloOfflineBundleModel, state *tfsdk.State, diags *diag.Diagnostics) {
	// There is no endpoint for one bundle, so it is looked up in the list.
	bundles, err := listKubeSoloOfflineBundles(ctx, r.client)
	if err != nil {
		diags.AddError("Failed to read KubeSolo offline bundle", fmt.Sprintf("failed to list KubeSolo offline bundles: %s", err))
		return
	}
	for _, b := range bundles {
		if b.Name != m.ID.ValueString() {
			continue
		}
		m.Name = types.StringValue(b.Name)
		m.Arch = types.StringValue(b.Arch)
		m.Libc = types.StringValue(b.Libc)
		m.Size = types.Int64Value(b.Size)
		diags.Append(state.Set(ctx, m)...)
		return
	}
	state.RemoveResource(ctx)
}

func (r *kubeSoloOfflineBundleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubeSoloOfflineBundleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, kubeSoloOfflineBundleURL(r.client, state.ID.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete KubeSolo offline bundle", fmt.Sprintf("failed to delete KubeSolo offline bundle: %s", err))
	}
}

func (r *kubeSoloOfflineBundleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
