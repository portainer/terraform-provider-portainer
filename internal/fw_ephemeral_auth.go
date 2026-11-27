package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = (*authEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*authEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithClose     = (*authEphemeral)(nil)
)

// authEphemeral logs in with a username and password for one Terraform run.
// Close logs the session out, which makes Portainer revoke the JWT, so the
// token is neither stored nor left valid once the run is over. Renew is not
// implemented: a Portainer JWT lasts hours, far longer than a run.
func newAuthEphemeral() ephemeral.EphemeralResource { return &authEphemeral{} }

type authEphemeral struct{ ephemeralBase }

type authEphemeralModel struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	JWT      types.String `tfsdk:"jwt"`
}

// authClose is what Close needs to log the session out. Private data stays
// in Terraform's memory: it is never written to the plan or the state.
type authClose struct {
	JWT string `json:"jwt"`
}

func (e *authEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth"
}

func (e *authEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Logs in to Portainer for the duration of one Terraform run and logs out when the run ends. The JWT is never written to the plan or the state.",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Username to log in with.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Password to log in with.",
			},
			"jwt": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "JWT bearer token Portainer issued. It is revoked when Terraform closes the ephemeral resource at the end of the run.",
			},
		},
	}
}

func (e *authEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var cfg authEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	client := e.ephemeralClient(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// /auth is public: doJSONPublic sends the provider's own credentials when
	// it has any, which Portainer ignores for a login.
	var response struct {
		JWT string `json:"jwt"`
	}
	creds := map[string]string{"username": cfg.Username.ValueString(), "password": cfg.Password.ValueString()}
	if err := doJSONPublic(ctx, client, http.MethodPost, client.Endpoint+"/auth", creds, &response, nil); err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", fmt.Sprintf("failed to authenticate: %s", err))
		return
	}
	if response.JWT == "" {
		resp.Diagnostics.AddError("Failed to authenticate", "failed to authenticate: Portainer returned no token")
		return
	}

	setEphemeralPrivate(ctx, resp.Private, authClose(response), &resp.Diagnostics)
	cfg.JWT = types.StringValue(response.JWT)
	resp.Diagnostics.Append(resp.Result.Set(ctx, &cfg)...)
}

// Close logs the session out. The request authenticates with the issued JWT
// itself, not with the provider's credentials, since it is that token
// Portainer revokes. A failure is only a warning: the token expires on its
// own, and Portainer versions without revocation accept the call anyway.
func (e *authEphemeral) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	var p authClose
	if !getEphemeralPrivate(ctx, req.Private, &p, &resp.Diagnostics) || p.JWT == "" {
		return
	}
	client := e.ephemeralClient(&resp.Diagnostics)
	if client == nil {
		return
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, client.Endpoint+"/auth/logout", nil)
	if err != nil {
		resp.Diagnostics.AddWarning("Failed to log out", err.Error())
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.JWT)
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddWarning("Failed to log out", fmt.Sprintf("the JWT stays valid until it expires: %s", err))
		return
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddWarning("Failed to log out", fmt.Sprintf("the JWT stays valid until it expires: status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body))))
	}
}
