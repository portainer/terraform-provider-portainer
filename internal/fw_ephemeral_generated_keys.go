package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The ephemeral counterparts of portainer_endpoints_edge_generate_key and
// portainer_sshkeygen. Portainer only computes these values, it stores
// nothing for them, so there is nothing to close: the value simply stays out
// of the plan and the state, and a fresh one is generated on every run.

var (
	_ ephemeral.EphemeralResource              = (*edgeGenerateKeyEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*edgeGenerateKeyEphemeral)(nil)
	_ ephemeral.EphemeralResource              = (*sshKeygenEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*sshKeygenEphemeral)(nil)
)

func newEdgeGenerateKeyEphemeral() ephemeral.EphemeralResource { return &edgeGenerateKeyEphemeral{} }

type edgeGenerateKeyEphemeral struct{ ephemeralBase }

type edgeGenerateKeyEphemeralModel struct {
	EdgeKey types.String `tfsdk:"edge_key"`
}

func (e *edgeGenerateKeyEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoints_edge_generate_key"
}

func (e *edgeGenerateKeyEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates a general Edge key without writing it to the plan or the state.",
		Attributes: map[string]schema.Attribute{
			"edge_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The generated general Edge key, with which an Edge Agent joins the Portainer instance.",
			},
		},
	}
}

func (e *edgeGenerateKeyEphemeral) Open(ctx context.Context, _ ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	client := e.ephemeralClient(&resp.Diagnostics)
	if client == nil {
		return
	}
	// The same request as the resource.
	var result GenerateEdgeKeyResponse
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/endpoints/edge/generate-key", map[string]string{"edgeKey": ""}, &result); err != nil {
		resp.Diagnostics.AddError("Failed to generate edge key", fmt.Errorf("failed to generate edge key: %w", err).Error())
		return
	}
	resp.Diagnostics.Append(resp.Result.Set(ctx, &edgeGenerateKeyEphemeralModel{EdgeKey: types.StringValue(result.EdgeKey)})...)
}

func newSSHKeygenEphemeral() ephemeral.EphemeralResource { return &sshKeygenEphemeral{} }

type sshKeygenEphemeral struct{ ephemeralBase }

type sshKeygenEphemeralModel struct {
	Public  types.String `tfsdk:"public"`
	Private types.String `tfsdk:"private"`
}

func (e *sshKeygenEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sshkeygen"
}

func (e *sshKeygenEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Has Portainer generate an SSH key pair without writing the private key to the plan or the state. A new pair is generated on every run.",
		Attributes: map[string]schema.Attribute{
			"public": schema.StringAttribute{
				Computed:    true,
				Description: "Generated public key.",
			},
			"private": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Generated private key.",
			},
		},
	}
}

func (e *sshKeygenEphemeral) Open(ctx context.Context, _ ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	client := e.ephemeralClient(&resp.Diagnostics)
	if client == nil {
		return
	}
	var keypair struct {
		Public  string `json:"public"`
		Private string `json:"private"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/sshkeygen", nil, &keypair); err != nil {
		resp.Diagnostics.AddError("Failed to generate SSH key", fmt.Sprintf("failed to generate SSH key: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.Result.Set(ctx, &sshKeygenEphemeralModel{
		Public:  types.StringValue(keypair.Public),
		Private: types.StringValue(keypair.Private),
	})...)
}
