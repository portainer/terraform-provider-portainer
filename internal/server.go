package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// NewProviderServer serves the provider over protocol 5: it keeps every
// Terraform and OpenTofu version that ran the SDKv2 provider (v3.0.0 and
// earlier) working, and protocol 5 carries ephemeral resources, actions,
// list resources and resource identity too.
func NewProviderServer(_ context.Context) (tfprotov5.ProviderServer, error) {
	return providerserver.NewProtocol5(NewFrameworkProvider())(), nil
}
