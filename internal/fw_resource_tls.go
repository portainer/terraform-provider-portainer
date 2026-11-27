package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*uploadTLSResource)(nil)
	_ resource.ResourceWithConfigure = (*uploadTLSResource)(nil)
)

func newUploadTLSResource() resource.Resource { return &uploadTLSResource{} }

type uploadTLSResource struct{ client *APIClient }

type uploadTLSModel struct {
	ID          types.String `tfsdk:"id"`
	Certificate types.String `tfsdk:"certificate"`
	Folder      types.String `tfsdk:"folder"`
	FilePath    types.String `tfsdk:"file_path"`
}

func (r *uploadTLSResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls"
}

func (r *uploadTLSResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"certificate": schema.StringAttribute{Required: true, Description: "Type of TLS certificate: one of 'ca', 'cert', 'key'"},
			"folder":      schema.StringAttribute{Required: true, Description: "Folder name where the TLS file will be stored"},
			"file_path":   schema.StringAttribute{Required: true, Description: "Path to the local TLS file to upload"},
		},
	}
}

func (r *uploadTLSResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *uploadTLSResource) upload(ctx context.Context, certType, folder, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open TLS file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("folder", folder)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}
	writer.Close()

	uploadURL := fmt.Sprintf("%s/upload/tls/%s", r.client.Endpoint, certType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, body)
	if err != nil {
		return err
	}
	if err := setAuthHeader(req, r.client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := r.client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", string(respBody))
	}
	return nil
}

func (r *uploadTLSResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan uploadTLSModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	certType, filePath := plan.Certificate.ValueString(), plan.FilePath.ValueString()
	if err := r.upload(ctx, certType, plan.Folder.ValueString(), filePath); err != nil {
		resp.Diagnostics.AddError("Failed to upload the TLS file", err.Error())
		return
	}
	plan.ID = types.StringValue(fmt.Sprintf("upload-%s-%s", certType, filepath.Base(filePath)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state: an uploaded file is not tracked.
func (r *uploadTLSResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only records the new values, as the SDKv2 version did: the file is
// uploaded on create only.
func (r *uploadTLSResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan uploadTLSModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *uploadTLSResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
