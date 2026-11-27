package internal

import (
	"context"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The provider, served by terraform-plugin-framework (see server.go). Up to
// v3.0.0 it was served by terraform-plugin-sdk/v2; the provider block, every
// resource and data source schema, the state and the import IDs stayed the
// same (internal/schema_compat_test.go checks the schemas).

var (
	_ provider.Provider                       = (*frameworkProvider)(nil)
	_ provider.ProviderWithEphemeralResources = (*frameworkProvider)(nil)
	_ provider.ProviderWithActions            = (*frameworkProvider)(nil)
	_ provider.ProviderWithListResources      = (*frameworkProvider)(nil)
)

// NewFrameworkProvider returns the provider.
func NewFrameworkProvider() provider.Provider {
	return &frameworkProvider{}
}

type frameworkProvider struct{}

type frameworkProviderModel struct {
	Endpoint       types.String `tfsdk:"endpoint"`
	APIKey         types.String `tfsdk:"api_key"`
	APIUser        types.String `tfsdk:"api_user"`
	APIPassword    types.String `tfsdk:"api_password"`
	SkipSSLVerify  types.Bool   `tfsdk:"skip_ssl_verify"`
	CustomHeaders  types.Map    `tfsdk:"custom_headers"`
	MaxRetries     types.Int64  `tfsdk:"max_retries"`
	RequestTimeout types.String `tfsdk:"request_timeout"`
}

func (p *frameworkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "portainer"
}

// Schema is the provider block of v3.0.0. SDKv2 reported "endpoint", a
// required attribute with an environment default, as optional while
// PORTAINER_ENDPOINT is set; the framework schema does the same, and
// Configure takes the value from the variable.
func (p *frameworkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	endpointFromEnv := os.Getenv("PORTAINER_ENDPOINT") != ""
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Required:    !endpointFromEnv,
				Optional:    endpointFromEnv,
				Description: "URL of the Portainer instance (e.g. https://portainer.example.com). '/api' will be appended automatically if missing.",
				Validators:  []validator.String{httpURLValidator{}},
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "API key to authenticate with Portainer. Mutually exclusive with 'user' and 'password'.",
			},
			"api_user": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Username for authentication. Must be used together with 'password'.",
			},
			"api_password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password for authentication. Must be used together with 'user'.",
			},
			"skip_ssl_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Verify the SSL/TLS certificate for the Portainer endpoint",
			},
			"custom_headers": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Custom headers to add to all requests (e.g. for Cloudflare Access or other security proxies).",
			},
			"max_retries": schema.Int64Attribute{
				Optional:    true,
				Validators:  []validator.Int64{int64validator.Between(0, 10)},
				Description: "How many times a request that failed transiently is retried, with exponential backoff (1s, 2s, 4s, ...). Only retries that cannot apply a change twice are made: a refused connection for any method, 429/503 for GET/PUT/DELETE, and 502/504 or a dropped connection for GET only. 0 disables retries. Defaults to 3.",
			},
			"request_timeout": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{durationValidator{}},
				Description: "Time limit for a single HTTP request to Portainer, including reading the response, as a Go duration (e.g. \"2m\"). Empty (the default) sets no per-request limit; requests are still cancelled when the Terraform operation times out. Avoid short values when pulling large images or installing Helm charts, which Portainer handles in one request.",
			},
		},
	}
}

// Configure applies the environment defaults (PORTAINER_*) and hands every
// resource, data source, ephemeral resource, action and list resource the
// shared *APIClient.
func (p *frameworkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m frameworkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := providerConfig{
		Endpoint:       stringOrEnv(m.Endpoint, "PORTAINER_ENDPOINT", ""),
		APIKey:         stringOrEnv(m.APIKey, "PORTAINER_API_KEY", ""),
		User:           stringOrEnv(m.APIUser, "PORTAINER_USER", ""),
		Password:       stringOrEnv(m.APIPassword, "PORTAINER_PASSWORD", ""),
		RequestTimeout: stringOrEnv(m.RequestTimeout, "PORTAINER_REQUEST_TIMEOUT", ""),
		SkipSSLVerify:  m.SkipSSLVerify.ValueBool(),
		MaxRetries:     3,
		CustomHeaders:  map[string]string{},
	}
	if m.SkipSSLVerify.IsNull() {
		cfg.SkipSSLVerify, _ = strconv.ParseBool(os.Getenv("PORTAINER_SKIP_SSL_VERIFY"))
	}
	if !m.MaxRetries.IsNull() {
		cfg.MaxRetries = int(m.MaxRetries.ValueInt64())
	} else if v, err := strconv.Atoi(os.Getenv("PORTAINER_MAX_RETRIES")); err == nil {
		cfg.MaxRetries = v
	}
	if !m.CustomHeaders.IsNull() {
		resp.Diagnostics.Append(m.CustomHeaders.ElementsAs(ctx, &cfg.CustomHeaders, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if cfg.Endpoint == "" {
		resp.Diagnostics.AddError("Missing Portainer endpoint", "Set \"endpoint\" in the provider block or the PORTAINER_ENDPOINT environment variable.")
		return
	}

	client, err := newAPIClient(ctx, cfg)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure the Portainer client", err.Error())
		return
	}
	resp.ResourceData = client
	resp.DataSourceData = client
	resp.EphemeralResourceData = client
	resp.ActionData = client
	resp.ListResourceData = client
}

func stringOrEnv(v types.String, env, fallback string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	if e := os.Getenv(env); e != "" {
		return e
	}
	return fallback
}

// The registries below are filled by the init functions of the
// fw_register_*.go files.

func (p *frameworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return frameworkResources
}

func (p *frameworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return frameworkDataSources
}

func (p *frameworkProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return frameworkEphemeralResources
}

func (p *frameworkProvider) Actions(_ context.Context) []func() action.Action {
	return frameworkActions
}

func (p *frameworkProvider) ListResources(_ context.Context) []func() list.ListResource {
	return frameworkListResources
}

var (
	frameworkResources = []func() resource.Resource{
		newTagResource,
	}
	frameworkDataSources        = []func() datasource.DataSource{}
	frameworkEphemeralResources = []func() ephemeral.EphemeralResource{}
	frameworkActions            = []func() action.Action{}
	frameworkListResources      = []func() list.ListResource{}
)
