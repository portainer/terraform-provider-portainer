package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// edgeMTLSModel is the state of both edge mTLS certificate data sources.
type edgeMTLSModel struct {
	ID                     types.String `tfsdk:"id"`
	Configured             types.Bool   `tfsdk:"configured"`
	CommonName             types.String `tfsdk:"common_name"`
	Organization           types.List   `tfsdk:"organization"`
	IssuerCommonName       types.String `tfsdk:"issuer_common_name"`
	SerialNumber           types.String `tfsdk:"serial_number"`
	SHA256Fingerprint      types.String `tfsdk:"sha256_fingerprint"`
	SignatureAlgorithm     types.String `tfsdk:"signature_algorithm"`
	IsCertificateAuthority types.Bool   `tfsdk:"is_certificate_authority"`
	ValidNotBefore         types.String `tfsdk:"valid_not_before"`
	ValidNotAfter          types.String `tfsdk:"valid_not_after"`
	Version                types.Int64  `tfsdk:"version"`
	SubjectAltDNSNames     types.List   `tfsdk:"subject_alt_dns_names"`
	SubjectAltIPAddresses  types.List   `tfsdk:"subject_alt_ip_addresses"`
	KeyUsages              types.List   `tfsdk:"key_usages"`
	ExtendedKeyUsages      types.List   `tfsdk:"extended_key_usages"`
	PublicKeyAlgorithm     types.String `tfsdk:"public_key_algorithm"`
	PublicKeySize          types.Int64  `tfsdk:"public_key_size"`
}

// edgeMTLSSchema is the schema both edge mTLS data sources share; only the
// descriptions of configured, issuer_common_name and is_certificate_authority
// differ.
func edgeMTLSSchema(id, configured, issuer, isCA string) schema.Schema {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc}
	}
	list := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: desc}
	}
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                       str(id),
			"configured":               schema.BoolAttribute{Computed: true, Description: configured},
			"common_name":              str("Common name of the certificate's subject."),
			"organization":             list("Organization of the certificate's subject."),
			"issuer_common_name":       str(issuer),
			"serial_number":            str("Serial number of the certificate."),
			"sha256_fingerprint":       str("SHA-256 fingerprint of the certificate, which is what to compare against when rotating it."),
			"signature_algorithm":      str("Algorithm the certificate is signed with."),
			"is_certificate_authority": schema.BoolAttribute{Computed: true, Description: isCA},
			"valid_not_before":         str("Start of the certificate's validity period."),
			"valid_not_after":          str("End of the certificate's validity period. Watch this to rotate before edge agents stop connecting."),
			"version":                  schema.Int64Attribute{Computed: true, Description: "X.509 version of the certificate."},
			"subject_alt_dns_names":    list("DNS names in the certificate's subject alternative name extension."),
			"subject_alt_ip_addresses": list("IP addresses in the certificate's subject alternative name extension."),
			"key_usages":               list("Key usages the certificate permits."),
			"extended_key_usages":      list("Extended key usages the certificate permits."),
			"public_key_algorithm":     str("Algorithm of the certificate's public key."),
			"public_key_size":          schema.Int64Attribute{Computed: true, Description: "Size of the certificate's public key, in bits."},
		},
	}
}

// readEdgeMTLSCertificate reads one of the certificates. An instance with no
// edge mTLS configured has no certificate to parse; that is a state worth
// reporting, not a failed plan.
func readEdgeMTLSCertificate(ctx context.Context, client *APIClient, urlPath, jsonField, what, id string, resp *datasource.ReadResponse) {
	var response map[string]json.RawMessage
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+urlPath, nil, &response); err != nil {
		if !isAPINotFound(err) {
			resp.Diagnostics.AddError("Failed to read the "+what, fmt.Sprintf("failed to read the %s: %s", what, err))
			return
		}
	}
	var parsed *sslCertificate
	if raw, ok := response[jsonField]; ok {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			resp.Diagnostics.AddError("Failed to read the "+what, fmt.Sprintf("failed to read the %s: %s", what, err))
			return
		}
	}
	cert := sslCertificate{}
	configured := parsed != nil
	if configured {
		cert = *parsed
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &edgeMTLSModel{
		ID:                     types.StringValue(id),
		Configured:             types.BoolValue(configured),
		CommonName:             types.StringValue(cert.Subject.CommonName),
		Organization:           edgeStringList(cert.Subject.Organization),
		IssuerCommonName:       types.StringValue(cert.Issuer.CommonName),
		SerialNumber:           types.StringValue(cert.SerialNumber),
		SHA256Fingerprint:      types.StringValue(cert.SHA256Fingerprint),
		SignatureAlgorithm:     types.StringValue(cert.SignatureAlgorithm),
		IsCertificateAuthority: types.BoolValue(cert.IsCertificateAuthority),
		ValidNotBefore:         types.StringValue(cert.ValidNotBefore),
		ValidNotAfter:          types.StringValue(cert.ValidNotAfter),
		Version:                types.Int64Value(int64(cert.Version)),
		SubjectAltDNSNames:     edgeStringList(cert.SubjectAltDNSNames),
		SubjectAltIPAddresses:  edgeStringList(cert.SubjectAltIPAddresses),
		KeyUsages:              edgeStringList(cert.KeyUsages),
		ExtendedKeyUsages:      edgeStringList(cert.ExtendedKeyUsages),
		PublicKeyAlgorithm:     types.StringValue(cert.PublicKey.Algorithm),
		PublicKeySize:          types.Int64Value(int64(cert.PublicKey.Size)),
	})...)
}

// --- portainer_edge_mtls_ca_certificate ---

var _ datasource.DataSourceWithConfigure = (*edgeMTLSCACertificateDataSource)(nil)

func newEdgeMTLSCACertificateDataSource() datasource.DataSource {
	return &edgeMTLSCACertificateDataSource{}
}

type edgeMTLSCACertificateDataSource struct{ edgeDataSourceBase }

func (d *edgeMTLSCACertificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_mtls_ca_certificate"
}

func (d *edgeMTLSCACertificateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = edgeMTLSSchema(
		"Always `portainer-edge-mtls-ca-certificate`.",
		"Whether Portainer has an edge mTLS CA certificate configured. Every other attribute is empty when it does not.",
		"Common name of the certificate's issuer. It matches `common_name` for a self-signed CA.",
		"Whether the certificate carries the CA basic constraint. A CA certificate that reports `false` here will not verify agent certificates.",
	)
}

func (d *edgeMTLSCACertificateDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	readEdgeMTLSCertificate(ctx, d.client, "/settings/edge/mtls_ca_certificate", "MTLSCACertificate", "edge mTLS CA certificate", "portainer-edge-mtls-ca-certificate", resp)
}

// --- portainer_edge_mtls_certificate ---

var _ datasource.DataSourceWithConfigure = (*edgeMTLSCertificateDataSource)(nil)

func newEdgeMTLSCertificateDataSource() datasource.DataSource {
	return &edgeMTLSCertificateDataSource{}
}

type edgeMTLSCertificateDataSource struct{ edgeDataSourceBase }

func (d *edgeMTLSCertificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_mtls_certificate"
}

func (d *edgeMTLSCertificateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = edgeMTLSSchema(
		"Always `portainer-edge-mtls-certificate`.",
		"Whether Portainer has an edge mTLS certificate configured. Every other attribute is empty when it does not.",
		"Common name of the certificate's issuer, which is the edge mTLS CA.",
		"Whether the certificate carries the CA basic constraint. Portainer's own edge mTLS certificate is a leaf, so this reports `false`.",
	)
}

func (d *edgeMTLSCertificateDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	readEdgeMTLSCertificate(ctx, d.client, "/settings/edge/mtls_certificate", "MTLSCertificate", "edge mTLS certificate", "portainer-edge-mtls-certificate", resp)
}
