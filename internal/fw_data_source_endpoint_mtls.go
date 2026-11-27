package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The two endpoint mTLS data sources mirror Portainer's two endpoints: the
// certificate an agent is connecting with, and the one whose rejection is
// keeping it from connecting. They share the certificate parsing of
// data_source_edge_mtls.go and differ only in the URL, the ID and two texts.

var (
	_ datasource.DataSource              = (*endpointMTLSCertificateDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*endpointMTLSCertificateDataSource)(nil)
)

func newEndpointMTLSCertificateDataSource() datasource.DataSource {
	return &endpointMTLSCertificateDataSource{
		typeSuffix:  "_endpoint_mtls_certificate",
		urlSuffix:   "mtls_certificate",
		idPrefix:    "portainer-endpoint-mtls-certificate-",
		configured:  "Whether Portainer holds an accepted mTLS certificate for the environment. Every other attribute is empty when it does not.",
		errorFormat: "failed to read the mTLS certificate of environment %d: %w",
	}
}

func newEndpointMTLSCertificateErrorDataSource() datasource.DataSource {
	return &endpointMTLSCertificateDataSource{
		typeSuffix:  "_endpoint_mtls_certificate_error",
		urlSuffix:   "mtls_certificate_error",
		idPrefix:    "portainer-endpoint-mtls-certificate-error-",
		configured:  "Whether Portainer holds a rejected mTLS certificate for the environment. Every other attribute is empty when it does not.",
		errorFormat: "failed to read the rejected mTLS certificate of environment %d: %w",
	}
}

type endpointMTLSCertificateDataSource struct {
	client      *APIClient
	typeSuffix  string
	urlSuffix   string
	idPrefix    string
	configured  string
	errorFormat string
}

type endpointMTLSCertificateModel struct {
	ID                     types.String `tfsdk:"id"`
	EndpointID             types.Int64  `tfsdk:"endpoint_id"`
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

func (d *endpointMTLSCertificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + d.typeSuffix
}

func (d *endpointMTLSCertificateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(description string) schema.Attribute {
		return schema.StringAttribute{Computed: true, Description: description}
	}
	list := func(description string) schema.Attribute {
		return schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: description}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"endpoint_id":              schema.Int64Attribute{Required: true, Description: "Identifier of the environment whose certificate is read."},
			"configured":               schema.BoolAttribute{Computed: true, Description: d.configured},
			"common_name":              str("Common name of the certificate's subject."),
			"organization":             list("Organization of the certificate's subject."),
			"issuer_common_name":       str("Common name of the certificate's issuer, which is the edge mTLS CA the agent certificate was signed by."),
			"serial_number":            str("Serial number of the certificate."),
			"sha256_fingerprint":       str("SHA-256 fingerprint of the certificate, which is what to compare against when rotating it."),
			"signature_algorithm":      str("Algorithm the certificate is signed with."),
			"is_certificate_authority": schema.BoolAttribute{Computed: true, Description: "Whether the certificate carries the CA basic constraint. An agent certificate is a leaf, so this reports `false`."},
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

func (d *endpointMTLSCertificateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *endpointMTLSCertificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m endpointMTLSCertificateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := m.EndpointID.ValueInt64()

	var response struct {
		Certificate *sslCertificate `json:"MTLSCertificate"`
	}
	url := fmt.Sprintf("%s/endpoints/%d/%s", d.client.Endpoint, endpointID, d.urlSuffix)
	if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &response); err != nil {
		// An environment that has never presented the certificate has nothing
		// to parse. That is a state worth reporting, not a failed plan.
		if !isAPINotFound(err) {
			resp.Diagnostics.AddError("Failed to read the mTLS certificate", fmt.Errorf(d.errorFormat, endpointID, err).Error())
			return
		}
	}

	cert := sslCertificate{}
	configured := response.Certificate != nil
	if configured {
		cert = *response.Certificate
	}

	m.ID = types.StringValue(fmt.Sprintf("%s%d", d.idPrefix, endpointID))
	m.Configured = types.BoolValue(configured)
	m.CommonName = types.StringValue(cert.Subject.CommonName)
	m.Organization = endpointA2StringList(cert.Subject.Organization)
	m.IssuerCommonName = types.StringValue(cert.Issuer.CommonName)
	m.SerialNumber = types.StringValue(cert.SerialNumber)
	m.SHA256Fingerprint = types.StringValue(cert.SHA256Fingerprint)
	m.SignatureAlgorithm = types.StringValue(cert.SignatureAlgorithm)
	m.IsCertificateAuthority = types.BoolValue(cert.IsCertificateAuthority)
	m.ValidNotBefore = types.StringValue(cert.ValidNotBefore)
	m.ValidNotAfter = types.StringValue(cert.ValidNotAfter)
	m.Version = types.Int64Value(int64(cert.Version))
	m.SubjectAltDNSNames = endpointA2StringList(cert.SubjectAltDNSNames)
	m.SubjectAltIPAddresses = endpointA2StringList(cert.SubjectAltIPAddresses)
	m.KeyUsages = endpointA2StringList(cert.KeyUsages)
	m.ExtendedKeyUsages = endpointA2StringList(cert.ExtendedKeyUsages)
	m.PublicKeyAlgorithm = types.StringValue(cert.PublicKey.Algorithm)
	m.PublicKeySize = types.Int64Value(int64(cert.PublicKey.Size))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// endpointA2StringList builds a list(string); nil gives an empty list, as
// SDKv2 stored it.
func endpointA2StringList(values []string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}
