package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// sslCertificate is the parsed X.509 certificate Portainer reports for its
// edge mTLS material. Only the fields worth surfacing in a configuration are
// decoded; the full response also carries CRL and OCSP details that nothing
// downstream can act on.
type sslCertificate struct {
	Subject struct {
		CommonName   string   `json:"CommonName"`
		Organization []string `json:"Organization"`
	} `json:"Subject"`
	Issuer struct {
		CommonName string `json:"CommonName"`
	} `json:"Issuer"`
	SerialNumber           string   `json:"SerialNumber"`
	SHA256Fingerprint      string   `json:"SHA256Fingerprint"`
	SignatureAlgorithm     string   `json:"SignatureAlgorithm"`
	IsCertificateAuthority bool     `json:"IsCertificateAuthority"`
	ValidNotBefore         string   `json:"ValidNotBefore"`
	ValidNotAfter          string   `json:"ValidNotAfter"`
	Version                int      `json:"Version"`
	SubjectAltDNSNames     []string `json:"SubjectAltDNSNames"`
	SubjectAltIPAddresses  []string `json:"SubjectAltIpAddresses"`
	KeyUsages              []string `json:"KeyUsages"`
	ExtendedKeyUsages      []string `json:"ExtendedKeyUsages"`
	PublicKey              struct {
		Algorithm string `json:"Algorithm"`
		Size      int    `json:"Size"`
	} `json:"PublicKey"`
}

// certificateFields flattens a parsed certificate into the attributes both
// edge mTLS data sources expose. The two schemas are written out separately so
// docsdriftlint can read them, but the mapping itself is shared.
func certificateFields(cert sslCertificate, configured bool) map[string]interface{} {
	return map[string]interface{}{
		"configured":               configured,
		"common_name":              cert.Subject.CommonName,
		"organization":             cert.Subject.Organization,
		"issuer_common_name":       cert.Issuer.CommonName,
		"serial_number":            cert.SerialNumber,
		"sha256_fingerprint":       cert.SHA256Fingerprint,
		"signature_algorithm":      cert.SignatureAlgorithm,
		"is_certificate_authority": cert.IsCertificateAuthority,
		"valid_not_before":         cert.ValidNotBefore,
		"valid_not_after":          cert.ValidNotAfter,
		"version":                  cert.Version,
		"subject_alt_dns_names":    cert.SubjectAltDNSNames,
		"subject_alt_ip_addresses": cert.SubjectAltIPAddresses,
		"key_usages":               cert.KeyUsages,
		"extended_key_usages":      cert.ExtendedKeyUsages,
		"public_key_algorithm":     cert.PublicKey.Algorithm,
		"public_key_size":          cert.PublicKey.Size,
	}
}

func dataSourceEdgeMTLSCACertificate() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeMTLSCACertificateRead,

		Schema: map[string]*schema.Schema{
			"configured": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer has an edge mTLS CA certificate configured. Every other attribute is empty when it does not.",
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
				Description: "Common name of the certificate's issuer. It matches `common_name` for a self-signed CA.",
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
				Description: "Whether the certificate carries the CA basic constraint. A CA certificate that reports `false` here will not verify agent certificates.",
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

func dataSourceEdgeMTLSCACertificateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var response struct {
		Certificate *sslCertificate `json:"MTLSCACertificate"`
	}
	url := client.Endpoint + "/settings/edge/mtls_ca_certificate"
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &response); err != nil {
		// An instance with no edge mTLS configured has no certificate to parse.
		// That is a state worth reporting, not a failed plan.
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to read the edge mTLS CA certificate: %w", err))
		}
	}

	cert := sslCertificate{}
	configured := response.Certificate != nil
	if configured {
		cert = *response.Certificate
	}

	if err := setFields(d, certificateFields(cert, configured)); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-edge-mtls-ca-certificate")
	return nil
}

func dataSourceEdgeMTLSCertificate() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeMTLSCertificateRead,

		Schema: map[string]*schema.Schema{
			"configured": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer has an edge mTLS certificate configured. Every other attribute is empty when it does not.",
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
				Description: "Common name of the certificate's issuer, which is the edge mTLS CA.",
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
				Description: "Whether the certificate carries the CA basic constraint. Portainer's own edge mTLS certificate is a leaf, so this reports `false`.",
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

func dataSourceEdgeMTLSCertificateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var response struct {
		Certificate *sslCertificate `json:"MTLSCertificate"`
	}
	url := client.Endpoint + "/settings/edge/mtls_certificate"
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &response); err != nil {
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to read the edge mTLS certificate: %w", err))
		}
	}

	cert := sslCertificate{}
	configured := response.Certificate != nil
	if configured {
		cert = *response.Certificate
	}

	if err := setFields(d, certificateFields(cert, configured)); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-edge-mtls-certificate")
	return nil
}
