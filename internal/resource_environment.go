package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceEnvironment() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEnvironmentCreate,
		ReadContext:   resourceEnvironmentRead,
		DeleteContext: resourceEnvironmentDelete,
		UpdateContext: resourceEnvironmentUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: resourceEnvironmentCustomizeDiff,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer environment (endpoint) as displayed in the UI.",
			},
			"environment_address": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Connection URL of the environment (e.g. tcp://host:port for Docker, https://host for Kubernetes/Agent). Used as the endpoint URL Portainer connects to.",
				// State written before Read kept the scheme holds the address
				// the way Portainer stores it for agents, without the scheme.
				DiffSuppressFunc: func(_, old, new string, _ *schema.ResourceData) bool {
					return old != "" && old == trimURLScheme(old) && old == trimURLScheme(new)
				},
			},
			"public_ip": {
				Type:     schema.TypeString,
				Optional: true,
				// Portainer fills the public URL in from the environment address
				// when none is given, which without Computed showed as drift on
				// every plan.
				Computed:    true,
				Description: "Public IP/URL used by Portainer for Published Ports (maps to PublicURL). When unset, Portainer uses the environment address.",
			},
			"type": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment type: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					t := val.(int)
					if t < 1 || t > 7 {
						errs = append(errs, fmt.Errorf("%q must be between 1 and 7", key))
					}
					return
				},
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					// Portainer converts the type once the agent connects: an Edge
					// Agent (4) on Kubernetes becomes a Kubernetes Edge Agent (7),
					// and an Agent (2) on Kubernetes becomes Kubernetes via agent
					// (6). Suppress this expected drift.
					return (old == "7" && new == "4") || (old == "6" && new == "2")
				},
			},
			"group_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
				Description: "ID of the Portainer endpoint group. Default is 1 (Unassigned).",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of tag IDs to assign to the environment.",
			},
			"tls_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether TLS is enabled for the connection to the environment.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether to skip server TLS certificate verification when connecting to the environment.",
			},
			"tls_ca_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded TLS CA certificate used to verify the environment's server certificate. Stored in state as a sensitive value.",
			},
			"tls_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded TLS client certificate presented by Portainer when connecting to the environment. Stored in state as a sensitive value.",
			},
			"tls_key": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ConflictsWith: []string{"tls_key_wo"},
				Description:   "PEM-encoded TLS client private key paired with tls_cert. Stored in state as a sensitive value; use `tls_key_wo` to keep it out of state.",
			},
			"tls_key_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ConflictsWith: []string{"tls_key"},
				RequiredWith:  []string{"tls_key_wo_version"},
				Description:   "Write-only PEM-encoded TLS client private key (supports ephemeral values; never stored in Terraform state). Uploaded on create, whenever another TLS attribute changes, and when `tls_key_wo_version` changes. Requires Terraform 1.11 or later.",
			},
			"tls_key_wo_version": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Version of `tls_key_wo`. Increment it to upload a new key.",
			},
			"tls_skip_client_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether to skip client TLS certificate verification when Portainer connects to the environment.",
			},
			"edge_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "Edge key generated by Portainer for type 4/7 (Edge Agent) environments. Used by the Edge Agent to register with Portainer.",
			},
			"edge_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Edge identifier (UUID) for type 4/7 (Edge Agent) environments, to pass to the Edge Agent as PORTAINER_EDGE_ID. Returned by Portainer when the EnforceEdgeID setting is enabled, otherwise generated by the provider at create time. Empty for all other environment types.",
			},
			"user_access_policies": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Description: "Map of user IDs to role IDs (e.g. userID -> roleID)",
			},
			"team_access_policies": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Description: "Map of team IDs to role IDs (e.g. teamID -> roleID)",
			},
			"container_engine": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "docker",
				Description:  "Container engine used by the environment(endpoint). Value must be one of: 'docker' or 'podman'",
				ValidateFunc: validation.StringInSlice([]string{"docker", "podman"}, false),
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					if envType := d.Get("type").(int); envType != 4 {
						// container_engine is not relevant unless type == 4
						return true
					}
					// State written before container_engine existed holds no
					// value; Portainer treats an empty engine as Docker, so this
					// must not plan a replacement. Never on create, where
					// suppressing the default would send no engine at all.
					return d.Id() != "" && normalizeContainerEngine(old) == normalizeContainerEngine(new)
				},
			},
		},
	}
}

// resourceEnvironmentCustomizeDiff forces replacement when an Edge Agent
// environment's address really changes.
//
// Portainer bakes the address into the edge key when the environment is created
// and never regenerates it, and the update endpoint does not accept a URL for
// edge types at all. Moving such an environment therefore requires a new
// environment with a new edge key and a redeployed agent — an in-place update
// cannot express it. Without this, the address written back from the edge key
// on every read left the plan showing the same change forever.
//
// A difference in the scheme alone is not a move: it comes from Portainer's own
// host-only normalisation of endpoint.URL (issue #136) for state written by
// older provider versions, and replacing a live environment over that would be
// destructive for no reason.
func resourceEnvironmentCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if d.Id() == "" {
		// Creation: there is nothing to replace.
		return nil
	}
	envType := d.Get("type").(int)
	if envType != 4 && envType != 7 {
		return nil
	}
	if !d.HasChange("environment_address") && !d.HasChange("container_engine") {
		return nil
	}

	old, new := d.GetChange("environment_address")
	if trimURLScheme(old.(string)) != trimURLScheme(new.(string)) {
		return d.ForceNew("environment_address")
	}

	// HasChange ignores DiffSuppressFunc, so compare the engines here: only a
	// Docker Edge Agent has one, and an empty engine is Docker.
	oldEngine, newEngine := d.GetChange("container_engine")
	if envType == 4 && normalizeContainerEngine(oldEngine.(string)) != normalizeContainerEngine(newEngine.(string)) {
		return d.ForceNew("container_engine")
	}

	return nil
}

// normalizeContainerEngine maps the empty engine that older Portainer versions
// store for Docker Edge Agent environments to "docker".
func normalizeContainerEngine(engine string) string {
	if engine == "" {
		return "docker"
	}
	return engine
}

// ensureEdgeID gives Edge Agent environments a usable edge_id when Portainer
// does not supply one. The server only assigns an Edge ID at creation while the
// EnforceEdgeID setting is enabled; otherwise the endpoint's EdgeID stays empty
// until an agent connects and reports one, leaving nothing to template
// PORTAINER_EDGE_ID from (issue #142). The Portainer UI covers the same gap by
// generating a UUID client-side for its deployment script, and the server adopts
// the ID the agent reports while its own is empty, so the generated value
// becomes the authoritative one.
//
// It must stay create-time only. Read carries an existing value forward from
// state, so minting one there would be stable on refresh — but a plan does not
// persist the state it refreshes, so a resource whose stored edge_id is empty
// (imported, or created by an earlier provider version) would mint a fresh UUID
// on every plan and show phantom drift to anything referencing it. Those cases
// stay empty until an agent associates; see docs/resources/environment.md.
//
// The type is read back from state (i.e. as reported by the API), because
// Portainer's creation enum stops at 5 and can hand back a non-edge environment
// for a type it does not recognise.
func ensureEdgeID(d *schema.ResourceData) error {
	envType := d.Get("type").(int)
	if envType != 4 && envType != 7 {
		return nil
	}
	if d.Get("edge_id").(string) != "" {
		return nil
	}
	return d.Set("edge_id", uuid.New().String())
}

func resourceEnvironmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	if existingID, err := findExistingEnvironmentByName(ctx, client, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing environment: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		if diags := resourceEnvironmentUpdate(ctx, d, meta); diags.HasError() {
			return diags
		}
		return diag.FromErr(ensureEdgeID(d))
	}

	envType := d.Get("type").(int)
	endpointCreationType := int64(envType)
	if envType == 6 {
		endpointCreationType = 2
	}

	// POST /endpoints is multipart/form-data. Empty string fields are left out,
	// as the generated SDK this replaced did.
	var fields [][2]string
	addField := func(name, value string) {
		if value != "" {
			fields = append(fields, [2]string{name, value})
		}
	}
	var files []multipartFile
	addField("Name", name)
	addField("EndpointCreationType", strconv.FormatInt(endpointCreationType, 10))

	// Edge Agent creation (type 4) is ambiguous on the Portainer side: without a
	// container engine hint Portainer provisions the endpoint as a Kubernetes
	// Edge Agent (type 7) instead of a Docker or Podman Edge Agent (type 4).
	// Sending ContainerEngine as podman or docker pins it to the edge type the
	// user asked for (issues #140 / #93). type 7 intentionally leaves it unset
	// so Portainer keeps the Kubernetes edge type.
	if envType == 4 {
		containerEngine := normalizeContainerEngine(d.Get("container_engine").(string))
		addField("ContainerEngine", containerEngine)
	}

	addField("URL", strings.TrimSpace(d.Get("environment_address").(string)))
	addField("GroupID", strconv.Itoa(d.Get("group_id").(int)))

	if v, ok := d.GetOk("public_ip"); ok && v.(string) != "" {
		addField("PublicURL", v.(string))
	}

	// Edge Agent environments (type 4, 7) do not support TagIds in the
	// multipart Create request. Tags are applied via a follow-up Update call.
	isEdgeAgent := envType == 4 || envType == 7
	//
	// Portainer accepts TagIds only as a single part holding a JSON array
	// ("[1,2,3]"); repeated TagIds parts, one per id, are rejected with
	// "Invalid TagIds parameter".
	if v, ok := d.GetOk("tag_ids"); ok && !isEdgeAgent {
		tagIDs := []string{}
		for _, id := range v.([]interface{}) {
			tagIDs = append(tagIDs, strconv.Itoa(id.(int)))
		}
		if len(tagIDs) > 0 {
			addField("TagIds", "["+strings.Join(tagIDs, ",")+"]")
		}
	}

	// Portainer refuses TLS outright for Edge Agent environments — since 2.43,
	// endpoint_create.go rejects the request with "TLS is not supported for Edge
	// Agent environments" as soon as payload.TLS is true. tls_enabled defaults to
	// true in this schema, so sending the TLS fields unconditionally made every
	// Edge Agent creation fail against a current Portainer. The agent reaches
	// Portainer through the edge tunnel and never presents these certificates
	// anyway, so they are simply not sent — the same exclusion Update already
	// applies for edge types.
	if !isEdgeAgent {
		tlsEnabled := d.Get("tls_enabled").(bool)
		addField("TLS", strconv.FormatBool(tlsEnabled))
		tlsSkipVerify := d.Get("tls_skip_verify").(bool)
		addField("TLSSkipVerify", strconv.FormatBool(tlsSkipVerify))
		tlsSkipClientVerify := d.Get("tls_skip_client_verify").(bool)
		addField("TLSSkipClientVerify", strconv.FormatBool(tlsSkipClientVerify))

		// Portainer reads the CA only when server verification is on and the
		// client certificate and key only when client verification is on.
		if tlsEnabled && !tlsSkipVerify {
			if v, ok := d.GetOk("tls_ca_cert"); ok && v.(string) != "" {
				files = append(files, multipartFile{Field: "TLSCACertFile", Filename: "ca.pem", Content: v.(string)})
			}
		}
		if tlsEnabled && !tlsSkipClientVerify {
			if v, ok := d.GetOk("tls_cert"); ok && v.(string) != "" {
				files = append(files, multipartFile{Field: "TLSCertFile", Filename: "cert.pem", Content: v.(string)})
			}
			key, err := environmentTLSKey(d)
			if err != nil {
				return diag.FromErr(err)
			}
			if key != "" {
				files = append(files, multipartFile{Field: "TLSKeyFile", Filename: "key.pem", Content: key})
			}
		}
	}

	var created portainerEndpoint
	if err := doMultipart(ctx, client, http.MethodPost, client.Endpoint+"/endpoints", fields, files, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create environment: %w", err))
	}

	d.SetId(strconv.FormatInt(created.ID, 10))

	state := map[string]interface{}{}
	if created.EdgeKey != "" {
		state["edge_key"] = created.EdgeKey
	}
	if created.EdgeID != "" {
		state["edge_id"] = created.EdgeID
	}
	if err := setFields(d, state); err != nil {
		return diag.FromErr(err)
	}

	// For edge agents, tags and the public URL are not honoured by the
	// multipart Create request and must be applied via a follow-up Update.
	if isEdgeAgent {
		_, hasTags := d.GetOk("tag_ids")
		publicIP, hasPublicIP := d.GetOk("public_ip")
		if hasTags || (hasPublicIP && publicIP.(string) != "") {
			if diags := resourceEnvironmentUpdate(ctx, d, meta); diags.HasError() {
				return diags
			}
		}
	}

	if _, ok := d.GetOk("user_access_policies"); ok {
		if diags := resourceEnvironmentUpdate(ctx, d, meta); diags.HasError() {
			return diags
		}
	}
	if _, ok := d.GetOk("team_access_policies"); ok {
		if diags := resourceEnvironmentUpdate(ctx, d, meta); diags.HasError() {
			return diags
		}
	}

	if diags := resourceEnvironmentRead(ctx, d, meta); diags.HasError() {
		return diags
	}
	return diag.FromErr(ensureEdgeID(d))
}

// trimURLScheme strips a leading http://, https:// or tcp:// from an
// environment address so values can be compared the way Portainer stores them
// for Edge Agents and Agents (scheme-less host).
func trimURLScheme(s string) string {
	for _, scheme := range []string{"https://", "http://", "tcp://"} {
		if trimmed, ok := strings.CutPrefix(s, scheme); ok {
			return trimmed
		}
	}
	return s
}

// edgeAddressFromKey recovers the Portainer address an Edge Agent environment
// was created with. Portainer stores endpoint.URL for edge types as a bare host
// (edge.ParseHostForEdge drops the scheme, the port and the path), but the edge
// key keeps the submitted URL verbatim in its first field:
//
//	base64("<portainer url>|<tunnel host:port>|<tls fingerprint>|<endpoint id>")
//
// so the key - not endpoint.URL - is the lossless source for
// environment_address. Returns "" when the key is absent or not in that shape,
// which leaves the caller on its endpoint.URL fallback.
func edgeAddressFromKey(edgeKey string) string {
	if edgeKey == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(edgeKey)
	if err != nil {
		// Portainer emits the key unpadded (base64.RawStdEncoding).
		decoded, err = base64.RawStdEncoding.DecodeString(edgeKey)
		if err != nil {
			return ""
		}
	}
	fields := strings.Split(string(decoded), "|")
	if len(fields) != 4 {
		return ""
	}
	return strings.TrimSpace(fields[0])
}

func findExistingEnvironmentByName(ctx context.Context, client *APIClient, name string) (int, error) {
	envs, err := listEnvironments(ctx, client)
	if err != nil {
		return 0, err
	}

	for _, e := range envs {
		if e.Name == name {
			return int(e.ID), nil
		}
	}
	return 0, nil
}

// portainerEndpoint is the part of a Portainer environment (endpoint) the
// provider reads.
type portainerEndpoint struct {
	ID              int64               `json:"Id"`
	Name            string              `json:"Name"`
	Type            int64               `json:"Type"`
	GroupID         int64               `json:"GroupId"`
	URL             string              `json:"URL"`
	PublicURL       string              `json:"PublicURL"`
	EdgeID          string              `json:"EdgeID"`
	EdgeKey         string              `json:"EdgeKey"`
	TagIds          []int64             `json:"TagIds"`
	TLSConfig       *portainerTLSConfig `json:"TLSConfig"`
	ContainerEngine string              `json:"ContainerEngine"`
}

type portainerTLSConfig struct {
	TLS           bool   `json:"TLS"`
	TLSSkipVerify bool   `json:"TLSSkipVerify"`
	TLSCert       string `json:"TLSCert"`
}

// listEnvironments returns every environment (GET /endpoints).
func listEnvironments(ctx context.Context, client *APIClient) ([]portainerEndpoint, error) {
	var envs []portainerEndpoint
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/endpoints", nil, &envs); err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	return envs, nil
}

func resourceEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	var env portainerEndpoint
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d", client.Endpoint, id), nil, &env); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read environment: %w", err))
	}

	if err := d.Set("name", env.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", int(env.Type)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("group_id", int(env.GroupID)); err != nil {
		return diag.FromErr(err)
	}
	envType := int(env.Type)
	// Until an agent associates, Portainer reports an empty EdgeID (unless the
	// EnforceEdgeID setting is enabled), so for Edge Agent environments keep the
	// ID generated at create time rather than wiping it (issue #142). A
	// server-reported ID always wins, and non-edge environments keep reflecting
	// the API verbatim.
	edgeID := env.EdgeID
	if edgeID == "" && (envType == 4 || envType == 7) {
		edgeID = d.Get("edge_id").(string)
	}
	if err := d.Set("edge_id", edgeID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("edge_key", env.EdgeKey); err != nil {
		return diag.FromErr(err)
	}
	// Portainer stores the Edge Agent URL as a bare host, so a configured
	// "https://portainer.example.com" comes back as "portainer.example.com" and
	// "http://portainer:9000" comes back as "portainer" - both show up as
	// perpetual drift (issue #136). The edge key still carries the address
	// verbatim, so prefer it and fall back to the endpoint URL only when the key
	// is missing or unparsable, keeping the scheme already in state in that
	// case. A genuine address change stays visible: it is baked into the key at
	// create time and needs the environment to be replaced.
	//
	// Agent environments lose the scheme the same way ("tcp://host:9001" comes
	// back as "host:9001"), so any environment keeps the address in state when
	// only the scheme is missing.
	address := env.URL
	fromKey := ""
	if envType == 4 || envType == 7 {
		fromKey = edgeAddressFromKey(env.EdgeKey)
	}
	if fromKey != "" {
		address = fromKey
	} else if prior, ok := d.GetOk("environment_address"); ok {
		if s := prior.(string); s != address && trimURLScheme(s) == trimURLScheme(address) {
			address = s
		}
	}
	if err := d.Set("environment_address", address); err != nil {
		return diag.FromErr(err)
	}

	if env.PublicURL != "" {
		if err := d.Set("public_ip", env.PublicURL); err != nil {
			return diag.FromErr(err)
		}
	} else {
		if err := d.Set("public_ip", ""); err != nil {
			return diag.FromErr(err)
		}
	}

	tagIDs := []int{}
	for _, tid := range env.TagIds {
		tagIDs = append(tagIDs, int(tid))
	}
	if err := d.Set("tag_ids", tagIDs); err != nil {
		return diag.FromErr(err)
	}

	// TLS used to be write-only, so switching it in the Portainer UI never
	// showed up in the plan. Edge environments are left alone: Portainer keeps
	// TLS off for them whatever the schema defaults say. The skip flags are
	// only meaningful with TLS on (Portainer clears them when it is off), and
	// client verification has no flag of its own: it is on when a client
	// certificate is stored. Azure environments (3) are created without a TLS
	// configuration at all, which would read as tls_enabled = false against
	// the default true on every plan.
	if envType != 3 && envType != 4 && envType != 7 && env.TLSConfig != nil {
		tls := env.TLSConfig
		fields := map[string]interface{}{"tls_enabled": tls.TLS}
		if tls.TLS {
			fields["tls_skip_verify"] = tls.TLSSkipVerify
			fields["tls_skip_client_verify"] = tls.TLSCert == ""
		}
		if err := setFields(d, fields); err != nil {
			return diag.FromErr(err)
		}
	}

	// Edge Agent environments created by an older Portainer store an empty
	// ContainerEngine for Docker, and no migration fills it in.
	if err := d.Set("container_engine", normalizeContainerEngine(env.ContainerEngine)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// environmentUpdateBody is the PUT /endpoints/{id} payload. The SDK model
// tags the TLS flags omitempty, so it can never switch TLS or verification off,
// and drops an empty tag list or access policy map, so it can never clear
// them. Portainer treats a missing (null) field as "keep the current value",
// so the pointers, slices and maps here are nil unless the field is meant to be
// written.
type environmentUpdateBody struct {
	Name                string                        `json:"name,omitempty"`
	GroupID             int                           `json:"groupID,omitempty"`
	URL                 *string                       `json:"url,omitempty"`
	PublicURL           string                        `json:"publicURL,omitempty"`
	TLS                 *bool                         `json:"tls,omitempty"`
	TLSSkipVerify       *bool                         `json:"tlsskipVerify,omitempty"`
	TLSSkipClientVerify *bool                         `json:"tlsskipClientVerify,omitempty"`
	TagIDs              []int                         `json:"tagIDs"`
	UserAccessPolicies  map[string]environmentRoleRef `json:"userAccessPolicies"`
	TeamAccessPolicies  map[string]environmentRoleRef `json:"teamAccessPolicies"`
}

type environmentRoleRef struct {
	RoleID int `json:"RoleId"`
}

func expandEnvironmentPolicies(d *schema.ResourceData, key string) map[string]environmentRoleRef {
	raw := d.Get(key).(map[string]interface{})
	if len(raw) == 0 && !d.HasChange(key) {
		return nil
	}
	policies := make(map[string]environmentRoleRef, len(raw))
	for id, role := range raw {
		policies[id] = environmentRoleRef{RoleID: role.(int)}
	}
	return policies
}

// uploadEnvironmentTLSFiles stores the configured CA, client certificate and
// key in the environment's TLS folder. Portainer only reads these files from
// disk: PUT /endpoints/{id} points the TLS config at them but cannot carry
// their content, so a rotated certificate has to be uploaded before the update
// that reloads the proxy.
func uploadEnvironmentTLSFiles(ctx context.Context, client *APIClient, d *schema.ResourceData) error {
	if !d.Get("tls_enabled").(bool) {
		return nil
	}
	if !d.HasChanges("tls_enabled", "tls_skip_verify", "tls_skip_client_verify", "tls_ca_cert", "tls_cert", "tls_key", "tls_key_wo_version") {
		return nil
	}
	files := []struct{ kind, key, skip string }{
		{"ca", "tls_ca_cert", "tls_skip_verify"},
		{"cert", "tls_cert", "tls_skip_client_verify"},
		{"key", "tls_key", "tls_skip_client_verify"},
	}
	for _, f := range files {
		content := d.Get(f.key).(string)
		if f.key == "tls_key" {
			key, err := environmentTLSKey(d)
			if err != nil {
				return err
			}
			content = key
		}
		if content == "" || d.Get(f.skip).(bool) {
			continue
		}
		if err := uploadTLSFile(ctx, client, d.Id(), f.kind, content); err != nil {
			return err
		}
	}
	return nil
}

// environmentTLSKey returns the client key from tls_key, or from the
// write-only tls_key_wo.
func environmentTLSKey(d *schema.ResourceData) (string, error) {
	return secretValue(d, "tls_key", "tls_key_wo", "tls_key_wo_version")
}

func uploadTLSFile(ctx context.Context, client *APIClient, folder, kind, content string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("folder", folder); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("file", kind+".pem")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(part, content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.Endpoint+"/upload/tls/"+kind, bytes.NewReader(body.Bytes()))
	if err != nil {
		return err
	}
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload TLS %s file: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return &apiStatusError{Method: http.MethodPost, URL: req.URL.String(), StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}

func resourceEnvironmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	body := environmentUpdateBody{
		Name:    d.Get("name").(string),
		GroupID: d.Get("group_id").(int),
	}

	// Edge Agent environments (type 4, 7) trigger proxy/tunnel registration on
	// update when URL or TLS fields are included, which fails when the agent is
	// not yet connected. Only send metadata fields for edge agents.
	envType := d.Get("type").(int)
	isEdgeAgent := envType == 4 || envType == 7
	if !isEdgeAgent {
		url := d.Get("environment_address").(string)
		body.URL = &url
		body.TLS = new(d.Get("tls_enabled").(bool))
		body.TLSSkipVerify = new(d.Get("tls_skip_verify").(bool))
		body.TLSSkipClientVerify = new(d.Get("tls_skip_client_verify").(bool))

		if err := uploadEnvironmentTLSFiles(ctx, client, d); err != nil {
			return diag.FromErr(fmt.Errorf("failed to upload TLS files for environment: %w", err))
		}
	}

	// public_ip is computed, so d.Get returns the URL in state even when the
	// configuration leaves it out; only a configured value is sent. Without
	// one, a new connection URL becomes the public URL as well, which is how
	// Portainer defaults it for directly-connected environments (edge agents
	// connect back to Portainer, so their address is no public URL).
	if v, ok := configuredValue(d, "public_ip"); ok && v.(string) != "" {
		body.PublicURL = v.(string)
	} else if !isEdgeAgent && d.HasChange("environment_address") {
		body.PublicURL = d.Get("environment_address").(string)
	}

	// An empty list clears the tags, but only when tag_ids itself changed, so
	// tags assigned outside Terraform survive when the attribute is not set.
	if tags := toIntSlice(d.Get("tag_ids").([]interface{})); len(tags) > 0 || d.HasChange("tag_ids") {
		body.TagIDs = tags
	}
	body.UserAccessPolicies = expandEnvironmentPolicies(d, "user_access_policies")
	body.TeamAccessPolicies = expandEnvironmentPolicies(d, "team_access_policies")

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoints/%s", client.Endpoint, d.Id()), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update environment: %w", err))
	}

	// A real address change on an edge environment never reaches Update:
	// resourceEnvironmentCustomizeDiff forces replacement for it, because
	// Portainer cannot apply one in place. What can still arrive here is a
	// scheme-only difference against state written by an older provider version,
	// which is not a move and needs no special handling.
	return resourceEnvironmentRead(ctx, d, meta)
}

func resourceEnvironmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/endpoints/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete environment: %w", err))
	}

	return nil
}
