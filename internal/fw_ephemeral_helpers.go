package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
)

// ephemeralBase is the configured client every ephemeral resource needs.
// Terraform configures an ephemeral resource before each Open, Renew and
// Close call, so Close has a client even in a run that opened nothing.
type ephemeralBase struct{ client *APIClient }

func (e *ephemeralBase) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	e.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ephemeralClient returns the configured client, or reports why there is
// none: Open can run while the provider configuration is still unknown.
func (e *ephemeralBase) ephemeralClient(diags *diag.Diagnostics) *APIClient {
	if e.client == nil {
		diags.AddError("Provider not configured", "The Portainer provider has not been configured yet, so the ephemeral resource cannot be opened.")
	}
	return e.client
}

// ephemeralPrivateKey is the private data key under which an ephemeral
// resource remembers, between Open and Close, what Close has to remove.
// Terraform keeps private data in memory only: it reaches neither the plan
// nor the state.
const ephemeralPrivateKey = "close"

// privateSetter and privateGetter are the methods of the framework's private
// data, whose type lives in an internal package.
type privateSetter interface {
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

type privateGetter interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
}

func setEphemeralPrivate(ctx context.Context, p privateSetter, v any, diags *diag.Diagnostics) {
	raw, err := json.Marshal(v)
	if err != nil {
		diags.AddError("Failed to save private data", err.Error())
		return
	}
	diags.Append(p.SetKey(ctx, ephemeralPrivateKey, raw)...)
}

// getEphemeralPrivate decodes what Open saved into v, reporting false when
// there is nothing to clean up (Open failed or was never called).
func getEphemeralPrivate(ctx context.Context, p privateGetter, v any, diags *diag.Diagnostics) bool {
	if p == nil {
		return false
	}
	raw, d := p.GetKey(ctx, ephemeralPrivateKey)
	diags.Append(d...)
	if d.HasError() || len(raw) == 0 {
		return false
	}
	if err := json.Unmarshal(raw, v); err != nil {
		diags.AddError("Failed to read private data", fmt.Sprintf("cannot decode the data Open saved for Close: %s", err))
		return false
	}
	return true
}
