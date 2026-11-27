package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = (*kubernetesConfigEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*kubernetesConfigEphemeral)(nil)
)

// kubernetesConfigEphemeral is the ephemeral counterpart of the
// portainer_kubernetes_config data source: the same GET /kubernetes/config,
// whose result carries the caller's credential and so is kept out of the
// state. Portainer creates nothing for it, so there is nothing to close.
func newKubernetesConfigEphemeral() ephemeral.EphemeralResource {
	return &kubernetesConfigEphemeral{}
}

type kubernetesConfigEphemeral struct{ ephemeralBase }

type kubernetesConfigEphemeralModel struct {
	EnvironmentIDs        types.List   `tfsdk:"environment_ids"`
	ExcludeEnvironmentIDs types.List   `tfsdk:"exclude_environment_ids"`
	Kubeconfig            types.String `tfsdk:"kubeconfig"`
}

func (e *kubernetesConfigEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_config"
}

func (e *kubernetesConfigEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generates a kubeconfig for Portainer's Kubernetes environments without writing it to the plan or the state.",
		Attributes: map[string]schema.Attribute{
			"environment_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Environments to include in the generated kubeconfig. Leave unset to include every Kubernetes environment the caller can reach.",
			},
			"exclude_environment_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Environments to leave out of the generated kubeconfig.",
			},
			"kubeconfig": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The generated kubeconfig, as YAML. It carries the caller's credential.",
			},
		},
	}
}

func (e *kubernetesConfigEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var cfg kubernetesConfigEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	client := e.ephemeralClient(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	q := url.Values{}
	for param, list := range map[string]types.List{"ids": cfg.EnvironmentIDs, "excludeIds": cfg.ExcludeEnvironmentIDs} {
		if list.IsNull() || list.IsUnknown() {
			continue
		}
		var ids []int64
		resp.Diagnostics.Append(list.ElementsAs(ctx, &ids, false)...)
		for _, id := range ids {
			q.Add(param, strconv.FormatInt(id, 10))
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	target := client.Endpoint + "/kubernetes/config"
	if len(q) > 0 {
		target += "?" + q.Encode()
	}

	// YAML, not JSON: the raw helper the data source uses.
	body, status, err := apiGETWithCodeCtx(ctx, target, client.APIKey, client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to generate a kubeconfig", fmt.Sprintf("failed to generate a kubeconfig: %s", err))
		return
	}
	if status >= http.StatusBadRequest {
		resp.Diagnostics.AddError("Failed to generate a kubeconfig", fmt.Sprintf("failed to generate a kubeconfig: status %d: %s", status, strings.TrimSpace(string(body))))
		return
	}
	cfg.Kubeconfig = types.StringValue(string(body))
	resp.Diagnostics.Append(resp.Result.Set(ctx, &cfg)...)
}
