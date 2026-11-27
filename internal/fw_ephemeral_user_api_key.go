package internal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = (*userAPIKeyEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*userAPIKeyEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithClose     = (*userAPIKeyEphemeral)(nil)
)

// userAPIKeyEphemeral issues an API key that lives for one Terraform run:
// Open creates it and Close revokes it, so the raw key reaches neither the
// plan nor the state. Portainer keys never expire, so there is nothing to
// renew; the key is gone as soon as Terraform is done with it.
func newUserAPIKeyEphemeral() ephemeral.EphemeralResource { return &userAPIKeyEphemeral{} }

type userAPIKeyEphemeral struct{ ephemeralBase }

type userAPIKeyEphemeralModel struct {
	UserID      types.Int64  `tfsdk:"user_id"`
	Description types.String `tfsdk:"description"`
	Password    types.String `tfsdk:"password"`
	APIKeyID    types.Int64  `tfsdk:"api_key_id"`
	RawAPIKey   types.String `tfsdk:"raw_api_key"`
	Prefix      types.String `tfsdk:"prefix"`
	DateCreated types.Int64  `tfsdk:"date_created"`
}

// userAPIKeyClose is what Close needs to revoke the key.
type userAPIKeyClose struct {
	UserID int64 `json:"user_id"`
	KeyID  int   `json:"key_id"`
}

func (e *userAPIKeyEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_api_key"
}

func (e *userAPIKeyEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Issues a Portainer API key for the duration of one Terraform run and revokes it when the run ends. The key is never written to the plan or the state.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the user the API key is issued for. It must be the user the provider is logged in as.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the key. Portainer requires it to be unique per user; when unset, a unique `terraform-ephemeral-<timestamp>` is used, so overlapping runs do not collide.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Password of the user the key is issued for. Portainer requires it to authorize the request.",
			},
			"api_key_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Portainer identifier of the issued key.",
			},
			"raw_api_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The issued API key. It is valid until Terraform closes the ephemeral resource at the end of the run.",
			},
			"prefix": schema.StringAttribute{
				Computed:    true,
				Description: "Non-secret prefix of the key, which is what Portainer displays to identify it.",
			},
			"date_created": schema.Int64Attribute{
				Computed:    true,
				Description: "Unix timestamp at which the key was created.",
			},
		},
	}
}

func (e *userAPIKeyEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var cfg userAPIKeyEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	client := e.ephemeralClient(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	userID := cfg.UserID.ValueInt64()

	// Same restriction as portainer_user_api_key: Portainer issues keys only to
	// a session, and only for that session's own user.
	if client.APIKey != "" {
		resp.Diagnostics.AddError("Failed to create API key", fmt.Sprintf(
			"cannot create an API key for user %d: Portainer only accepts this call from a session, so the provider has to be configured with api_user and api_password rather than api_key - and the key can only be created for that same user", userID))
		return
	}

	if cfg.Description.IsNull() || cfg.Description.IsUnknown() || cfg.Description.ValueString() == "" {
		cfg.Description = types.StringValue(fmt.Sprintf("terraform-ephemeral-%d", time.Now().UnixNano()))
	}
	payload := map[string]interface{}{
		"description": cfg.Description.ValueString(),
		"password":    cfg.Password.ValueString(),
	}
	var response struct {
		RawAPIKey string          `json:"rawAPIKey"`
		APIKey    portainerAPIKey `json:"apiKey"`
	}
	url := fmt.Sprintf("%s/users/%d/tokens", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &response); err != nil {
		resp.Diagnostics.AddError("Failed to create API key", fmt.Sprintf("failed to create an API key for user %d: %s", userID, err))
		return
	}
	if response.APIKey.ID == 0 {
		resp.Diagnostics.AddError("Failed to create API key", fmt.Sprintf("failed to create an API key for user %d: Portainer returned no key ID", userID))
		return
	}

	// The private data is saved first: should setting the result fail, Close
	// still revokes the key.
	setEphemeralPrivate(ctx, resp.Private, userAPIKeyClose{UserID: userID, KeyID: response.APIKey.ID}, &resp.Diagnostics)
	cfg.APIKeyID = types.Int64Value(int64(response.APIKey.ID))
	cfg.RawAPIKey = types.StringValue(response.RawAPIKey)
	cfg.Prefix = types.StringValue(response.APIKey.Prefix)
	cfg.DateCreated = types.Int64Value(int64(response.APIKey.DateCreated))
	resp.Diagnostics.Append(resp.Result.Set(ctx, &cfg)...)
}

// Close revokes the key, so it does not outlive the run. A key already gone
// is not an error.
func (e *userAPIKeyEphemeral) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	var p userAPIKeyClose
	if !getEphemeralPrivate(ctx, req.Private, &p, &resp.Diagnostics) {
		return
	}
	client := e.ephemeralClient(&resp.Diagnostics)
	if client == nil {
		return
	}
	url := fmt.Sprintf("%s/users/%d/tokens/%d", client.Endpoint, p.UserID, p.KeyID)
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to revoke API key", fmt.Sprintf("failed to revoke API key %d of user %d, which stays valid until revoked by hand: %s", p.KeyID, p.UserID, err))
	}
}
