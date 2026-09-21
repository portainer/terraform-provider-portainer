package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The two endpoint mTLS data sources mirror Portainer's two endpoints: the
// certificate an agent is connecting with, and the one whose rejection is
// keeping it from connecting. They share the parsing helpers in
// data_source_edge_mtls.go, but each schema is written out in full so
// docsdriftlint can read it.

func dataSourceEndpointMTLSCertificate() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEndpointMTLSCertificateRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the environment whose certificate is read.",
			},
			// Computed attributes
			"configured": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer holds an accepted mTLS certificate for the environment. Every other attribute is empty when it does not.",
			},
			"common_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Common name of the certificate's subject.",
			},
			"organization": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Organization of the certificate's subject.",
			},
			"issuer_common_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Common name of the certificate's issuer, which is the edge mTLS CA the agent certificate was signed by.",
			},
			"serial_number": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Serial number of the certificate.",
			},
			"sha256_fingerprint": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "SHA-256 fingerprint of the certificate, which is what to compare against when rotating it.",
			},
			"signature_algorithm": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Algorithm the certificate is signed with.",
			},
			"is_certificate_authority": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the certificate carries the CA basic constraint. An agent certificate is a leaf, so this reports `false`.",
			},
			"valid_not_before": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Start of the certificate's validity period.",
			},
			"valid_not_after": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "End of the certificate's validity period. Watch this to rotate before edge agents stop connecting.",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "X.509 version of the certificate.",
			},
			"subject_alt_dns_names": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "DNS names in the certificate's subject alternative name extension.",
			},
			"subject_alt_ip_addresses": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "IP addresses in the certificate's subject alternative name extension.",
			},
			"key_usages": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Key usages the certificate permits.",
			},
			"extended_key_usages": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Extended key usages the certificate permits.",
			},
			"public_key_algorithm": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Algorithm of the certificate's public key.",
			},
			"public_key_size": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Size of the certificate's public key, in bits.",
			},
		},
	}
}

func dataSourceEndpointMTLSCertificateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	var response struct {
		Certificate *sslCertificate `json:"MTLSCertificate"`
	}
	url := fmt.Sprintf("%s/endpoints/%d/mtls_certificate", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &response); err != nil {
		// An environment that has never presented the mTLS certificate has nothing to
		// parse. That is a state worth reporting, not a failed plan.
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to read the mTLS certificate of environment %d: %w", endpointID, err))
		}
	}

	cert := sslCertificate{}
	configured := response.Certificate != nil
	if configured {
		cert = *response.Certificate
	}

	fields := certificateFields(cert, configured)
	fields["endpoint_id"] = endpointID
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-endpoint-mtls-certificate-%d", endpointID))
	return nil
}

func dataSourceEndpointMTLSCertificateError() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEndpointMTLSCertificateErrorRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the environment whose certificate is read.",
			},
			// Computed attributes
			"configured": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer holds a rejected mTLS certificate for the environment. Every other attribute is empty when it does not.",
			},
			"common_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Common name of the certificate's subject.",
			},
			"organization": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Organization of the certificate's subject.",
			},
			"issuer_common_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Common name of the certificate's issuer, which is the edge mTLS CA the agent certificate was signed by.",
			},
			"serial_number": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Serial number of the certificate.",
			},
			"sha256_fingerprint": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "SHA-256 fingerprint of the certificate, which is what to compare against when rotating it.",
			},
			"signature_algorithm": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Algorithm the certificate is signed with.",
			},
			"is_certificate_authority": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the certificate carries the CA basic constraint. An agent certificate is a leaf, so this reports `false`.",
			},
			"valid_not_before": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Start of the certificate's validity period.",
			},
			"valid_not_after": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "End of the certificate's validity period. Watch this to rotate before edge agents stop connecting.",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "X.509 version of the certificate.",
			},
			"subject_alt_dns_names": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "DNS names in the certificate's subject alternative name extension.",
			},
			"subject_alt_ip_addresses": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "IP addresses in the certificate's subject alternative name extension.",
			},
			"key_usages": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Key usages the certificate permits.",
			},
			"extended_key_usages": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Extended key usages the certificate permits.",
			},
			"public_key_algorithm": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Algorithm of the certificate's public key.",
			},
			"public_key_size": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Size of the certificate's public key, in bits.",
			},
		},
	}
}

func dataSourceEndpointMTLSCertificateErrorRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	var response struct {
		Certificate *sslCertificate `json:"MTLSCertificate"`
	}
	url := fmt.Sprintf("%s/endpoints/%d/mtls_certificate_error", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &response); err != nil {
		// An environment that has never presented the rejected mTLS certificate has nothing to
		// parse. That is a state worth reporting, not a failed plan.
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to read the rejected mTLS certificate of environment %d: %w", endpointID, err))
		}
	}

	cert := sslCertificate{}
	configured := response.Certificate != nil
	if configured {
		cert = *response.Certificate
	}

	fields := certificateFields(cert, configured)
	fields["endpoint_id"] = endpointID
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-endpoint-mtls-certificate-error-%d", endpointID))
	return nil
}
