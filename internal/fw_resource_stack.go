package internal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// portainer_stack deploys a Docker standalone, Docker Swarm or Kubernetes
// stack from a string, a local file, a Git repository or (Kubernetes) a URL.
// The CRUD functions are in fw_resource_stack_ops.go.

var newStackResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_stack",
	schema:     stackSchema,
	create:     stackCreate,
	read:       stackRead,
	update:     stackApplyUpdate,
	delete:     stackDelete,
	importer:   stackImport,
	identity:   stackIdentity,
	modifyPlan: stackModifyPlan,
	timeouts: map[string]time.Duration{
		"create": 30 * time.Minute,
		"update": 30 * time.Minute,
		"delete": 10 * time.Minute,
	},
})

func stackSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the stack.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"active":                       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether the stack should be running. Set to false to stop the stack."},
			"additional_files":             schema.ListAttribute{ElementType: types.StringType, Optional: true, Description: "List of additional Compose file paths to use when deploying from Git repository.", PlanModifiers: []planmodifier.List{dkReplaceList()}},
			"additional_helm_values_files": schema.ListAttribute{ElementType: types.StringType, Optional: true, Description: "List of additional Helm values files (e.g. values-prod.yaml). Only used with helm_chart_path."},
			"atomic":                       schema.BoolAttribute{Optional: true, Description: "Helm chart stacks only (deployment_type 'kubernetes', method 'repository' with helm_chart_path), Portainer Business Edition 3.1 or later: whether a failed Helm deployment is rolled back automatically (Helm --rollback-on-failure). Leaving it out sends nothing and keeps the choice Portainer has recorded; it is read back only while configured. Changed in place."},
			"authorized_teams":             schema.SetAttribute{ElementType: types.Int64Type, Optional: true, Description: "List of team IDs authorized to access this stack (only if ownership is restricted)."},
			"authorized_users":             schema.SetAttribute{ElementType: types.Int64Type, Optional: true, Description: "List of user IDs authorized to access this stack (only if ownership is restricted)."},
			"compose_format":               schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the supplied content is in Docker Compose format (true) instead of native Kubernetes manifest (false). Only applies to Kubernetes stacks. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"deployment_type":              schema.StringAttribute{Required: true, Description: "Deployment mode: 'standalone', 'swarm', or 'kubernetes'", PlanModifiers: []planmodifier.String{dkReplaceString()}, Validators: []validator.String{dkStringInSlice("standalone", "swarm", "kubernetes")}},
			"endpoint_id":                  schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer environment (endpoint) where the stack will be deployed. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"env_wo_version":               schema.Int64Attribute{Optional: true, Description: "Version flag for env_wo; change it to redeploy the stack with the current env_wo values."},
			// SDKv2's Default "" left the attribute null in state, which a
			// framework default cannot; stackFilePathPlan plans it instead.
			"file_path_in_repository":       schema.StringAttribute{Optional: true, Computed: true, Description: "Path to Compose/manifest file in the repository. Defaults to docker-compose.yml for Docker/Swarm stacks. Not required when helm_chart_path is set."},
			"filesystem_path":               schema.StringAttribute{Optional: true, Description: "Local filesystem path on the host used when support_relative_path is true. Maps to the repository working directory."},
			"force_update":                  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether to prune unused services/networks during stack update (default: true)"},
			"git_repository_authentication": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the Git repository requires authentication. When true, repository_username and repository_password (or their write-only equivalents) are sent to Portainer."},
			"helm_chart_path":               schema.StringAttribute{Optional: true, Description: "Path to a Helm chart folder in the Git repository (must contain Chart.yaml). Only used when deployment_type is 'kubernetes' and method is 'repository'.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"kubernetes_force_apply":        schema.BoolAttribute{Optional: true, Computed: true, Description: "Kubernetes stacks only, Portainer 3.0 or later: whether a deployment deletes and recreates a Kubernetes resource whose immutable fields changed instead of failing on it. Recreating destroys the resource and everything tied to its identity. Portainer leaves it off when this is left out; leaving it out keeps the choice Portainer has recorded and sends nothing. Changed in place for method 'repository'; for the other methods a change replaces the stack.", PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
			"kubernetes_prune":              schema.BoolAttribute{Optional: true, Computed: true, Description: "Kubernetes stacks only, Portainer 3.0 or later: whether a deployment deletes the Kubernetes resources an earlier deployment applied that the manifest no longer declares. Portainer prunes when this is left out; leaving it out keeps the choice Portainer has recorded and sends nothing, so older Portainer versions are unaffected. Changed in place for method 'repository'; for the other methods Portainer can only set it on create, so a change replaces the stack.", PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
			"manifest_url":                  schema.StringAttribute{Optional: true, Description: "URL to a remote Kubernetes manifest used when deployment_type is 'kubernetes' and method is 'url'. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"method":                        schema.StringAttribute{Required: true, Description: "Creation method: 'string', 'file', 'repository', or 'url'", PlanModifiers: []planmodifier.String{dkReplaceString()}, Validators: []validator.String{dkStringInSlice("string", "file", "repository", "url")}},
			"name":                          schema.StringAttribute{Required: true, Description: "Name of the Portainer stack. Must be unique within the target endpoint. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"namespace":                     schema.StringAttribute{Optional: true, Description: "Kubernetes namespace used when deployment_type is 'kubernetes'. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"ownership":                     schema.StringAttribute{Optional: true, Computed: true, Description: "Ownership level: 'public', 'administrators' or 'restricted'.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Validators: []validator.String{stackOwnershipValidator{}}},
			"prune":                         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether to prune unused services/networks during stack update (default: false)"},
			"pull_image":                    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether to force pull latest images during stack update (default: true)"},
			"registries":                    schema.ListAttribute{ElementType: types.Int64Type, Optional: true, Description: "List of registry IDs allowed for this stack."},
			"repository_git_credential_id":  schema.Int64Attribute{Optional: true, Description: "ID of the shared Git credentials to use for authentication (Portainer < 2.43). Replaced by 'source_id' in Portainer 2.43 STS, which switched to the Sources model; on 2.43+ this field is ignored. Both are sent, so a single configuration stays compatible with old and new Portainer."},
			"repository_password":           schema.StringAttribute{Optional: true, Sensitive: true, Description: "Password or personal access token used to authenticate against the Git repository. Stored in state as a sensitive value.", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_password_wo"))}},
			"repository_password_wo":        schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Description: "Write-only repository password (supports ephemeral values; not stored in Terraform state).", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_password")), stringvalidator.AlsoRequires(path.MatchRoot("repository_wo_version"))}},
			"repository_reference_name":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("refs/heads/main"), Description: "Git reference (branch or tag) used by Portainer when deploying from the repository. Defaults to refs/heads/main."},
			"repository_url":                schema.StringAttribute{Optional: true, Description: "URL of the Git repository used to deploy the stack when method is 'repository'. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.String{dkReplaceString()}, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_url_wo"))}},
			"repository_url_wo":             schema.StringAttribute{Optional: true, WriteOnly: true, Description: "Write-only repository URL (supports ephemeral values; not stored in Terraform state).", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_url")), stringvalidator.AlsoRequires(path.MatchRoot("repository_wo_version"))}},
			"repository_username":           schema.StringAttribute{Optional: true, Description: "Username used to authenticate against the Git repository.", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_username_wo"))}},
			"repository_username_wo":        schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Description: "Write-only repository username (supports ephemeral values).", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_username")), stringvalidator.AlsoRequires(path.MatchRoot("repository_wo_version"))}},
			"repository_wo_version":         schema.Int64Attribute{Optional: true, Description: "Version flag for write-only repository credentials; increment to trigger recreation.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"resource_control_id":           schema.Int64Attribute{Computed: true, Description: "Identifier of the Portainer resource control entry associated with the stack. Computed by Portainer."},
			"source_id":                     schema.Int64Attribute{Optional: true, Description: "ID of a Portainer Source (Git source) providing the repository URL and credentials. Introduced in Portainer 2.43 STS, which replaced 'repository_git_credential_id'. When set (non-zero), Portainer resolves the repository URL/credentials from the referenced Source and ignores the inline repository_username/repository_password and repository_git_credential_id fields."},
			// Computed besides optional, which SDKv2 did not need: Read stores
			// the content Portainer deployed even when the configuration
			// leaves it to stack_file_path (method "file") or a URL, and the
			// plan keeps it unless the file changed (see stackModifyPlan).
			"stack_file_content":    schema.StringAttribute{Optional: true, Computed: true, Description: "Inline Compose or Kubernetes manifest content used to deploy the stack. Required when method is 'string'; populated from stack_file_path when method is 'file'."},
			"stack_file_path":       schema.StringAttribute{Optional: true, Description: "Local filesystem path to a Compose or manifest file. Contents are read and uploaded to Portainer when method is 'file'."},
			"stack_webhook":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Enable autoUpdate webhook (GitOps)."},
			"support_relative_path": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether Portainer should support relative paths inside the Compose file for bind mounts referencing repository contents. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"swarm_id":              schema.StringAttribute{Optional: true, Computed: true, Description: "Identifier of the Docker Swarm cluster used when deployment_type is 'swarm'. Automatically fetched from Portainer when not provided. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), dkReplaceString()}},
			"tlsskip_verify":        schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to skip TLS verification when Portainer connects to the Git repository. Changing this value forces resource recreation.", PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown(), dkReplaceBool()}},
			"update_interval":       schema.StringAttribute{Optional: true, Description: "GitOps auto-update polling interval (e.g. '5m', '1h'). When set, Portainer periodically checks the Git repository for changes and redeploys the stack."},
			"webhook_id":            schema.StringAttribute{Computed: true, Description: "UUID of the GitOps webhook (read-only)."},
			"webhook_url":           schema.StringAttribute{Computed: true, Sensitive: true, Description: "Full URL of the webhook trigger. Sensitive: anyone with the URL can redeploy the stack."},
		},
		Blocks: map[string]schema.Block{
			"env": schema.ListNestedBlock{
				Description: "List of environment variables injected into the stack at deploy time.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true, Description: "Name of the environment variable."},
						"value": schema.StringAttribute{Required: true, Description: "Value of the environment variable."},
					},
				},
			},
			"env_wo": schema.ListNestedBlock{
				Description: "Environment variables whose values are write-only (supports ephemeral values; the values are not stored in Terraform state, the names are). Sent together with env on every deploy. Write-only values cannot be compared, so increment env_wo_version to redeploy the stack with changed values.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true, Description: "Name of the environment variable."},
						"value": schema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true, Description: "Write-only value of the environment variable."},
					},
				},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

// stackOwnershipValidator accepts the ownership levels, with SDKv2's message.
type stackOwnershipValidator struct{}

func (stackOwnershipValidator) Description(context.Context) string {
	return "must be one of 'public', 'private', 'administrators', or 'restricted'"
}

func (v stackOwnershipValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (stackOwnershipValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	switch req.ConfigValue.ValueString() {
	case "public", "administrators", "restricted", "private":
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Invalid ownership", fmt.Sprintf("%q must be one of 'public', 'private', 'administrators', or 'restricted'", "ownership"))
}

// dkStringInSlice is validation.StringInSlice(valid, false), with its message.
func dkStringInSlice(valid ...string) validator.String {
	return dkStringInSliceValidator{valid: valid}
}

type dkStringInSliceValidator struct{ valid []string }

func (v dkStringInSliceValidator) Description(context.Context) string {
	return fmt.Sprintf("must be one of %q", v.valid)
}

func (v dkStringInSliceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v dkStringInSliceValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	for _, s := range v.valid {
		if s == value {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", fmt.Sprintf("expected %s to be one of %q, got %s", req.Path.String(), v.valid, value))
}

// stackModifyPlan is the SDKv2 CustomizeDiff of the stack and the
// DiffSuppressFunc of stack_file_content.
func stackModifyPlan(_ context.Context, p *dkPlan) error {
	if err := stackKubernetesOptionsPlan(p); err != nil {
		return err
	}
	if err := stackAtomicPlan(p); err != nil {
		return err
	}
	if err := stackEnvPlan(p); err != nil {
		return err
	}
	if err := stackFilePathPlan(p); err != nil {
		return err
	}
	return stackFileContentPlan(p)
}

// stackFilePathPlan keeps the path Portainer reports when the configuration
// leaves file_path_in_repository out. v3.0.0 (SDKv2, Default "") planned such a
// path back to "" on every run - a repository stack deploys docker-compose.yml
// by default and Portainer records it - which applied nothing and came back
// on the next plan.
func stackFilePathPlan(p *dkPlan) error {
	if p.creates || p.Configured("file_path_in_repository") {
		return nil
	}
	return p.SetPlanned("file_path_in_repository", p.PriorValue("file_path_in_repository"))
}

// stackEnvPlan rejects a variable declared in both env and env_wo, or twice
// in env_wo: the stack would get both values and Read could not tell which
// one belongs in state.
func stackEnvPlan(p *dkPlan) error {
	writeOnly := map[string]bool{}
	for _, name := range envNames(dkList(p.dkBag, "env_wo")) {
		if writeOnly[name] {
			return fmt.Errorf("env_wo: variable %q is declared more than once", name)
		}
		writeOnly[name] = true
	}
	for _, name := range envNames(dkList(p.dkBag, "env")) {
		if writeOnly[name] {
			return fmt.Errorf("variable %q is declared in both env and env_wo", name)
		}
	}
	return nil
}

// stackAtomicPlan rejects atomic outside a Helm chart stack deployed from
// Git, the only stack whose create and git settings payloads take it.
func stackAtomicPlan(p *dkPlan) error {
	if !p.Configured("atomic") {
		return nil
	}
	if d := dkString(p.dkBag, "deployment_type"); d != "" && d != "kubernetes" {
		return fmt.Errorf("atomic only applies to deployment_type = \"kubernetes\"")
	}
	if m := dkString(p.dkBag, "method"); m != "" && m != "repository" {
		return fmt.Errorf("atomic only applies to method = \"repository\"")
	}
	if !p.Configured("helm_chart_path") {
		return fmt.Errorf("atomic only applies to a Helm chart stack: set helm_chart_path")
	}
	return nil
}

// stackKubernetesOptionKeys are the Portainer 3.0 Kubernetes apply policies.
var stackKubernetesOptionKeys = []string{"kubernetes_prune", "kubernetes_force_apply"}

// stackKubernetesOptionsPlan rejects the Kubernetes apply policies on a
// Docker stack and replaces a stack whose policies change when Portainer
// cannot change them in place: only the git settings endpoint takes them, the
// update of a string, file or url stack does not.
func stackKubernetesOptionsPlan(p *dkPlan) error {
	deployment := dkString(p.dkBag, "deployment_type")
	for _, key := range stackKubernetesOptionKeys {
		if deployment != "" && deployment != "kubernetes" && p.Configured(key) {
			return fmt.Errorf("%s only applies to deployment_type = \"kubernetes\"", key)
		}
		// Left out of the configuration, the attribute keeps its state value,
		// so a change here is always a configured one.
		if !p.creates && deployment == "kubernetes" && dkString(p.dkBag, "method") != "repository" && p.HasChange(key) {
			p.ForceNew(key)
		}
	}
	return nil
}

// stackFileContentPlan plans stack_file_content when the configuration
// leaves it out. With method "file" it follows the file at stack_file_path:
// unchanged (ignoring surrounding whitespace, as SDKv2's DiffSuppressFunc did)
// when the file still holds what was deployed, the new content otherwise.
// Otherwise it keeps the content Read stored.
func stackFileContentPlan(p *dkPlan) error {
	if p.creates || p.Configured("stack_file_content") {
		return nil
	}
	prior := p.PriorValue("stack_file_content")
	if dkString(p.dkBag, "method") == "file" {
		content, err := os.ReadFile(dkString(p.dkBag, "stack_file_path"))
		if err != nil {
			// Unreadable at plan time: the apply reports it.
			return p.SetPlanned("stack_file_content", tftypes.NewValue(tftypes.String, tftypes.UnknownValue))
		}
		var old string
		if prior.IsKnown() && !prior.IsNull() {
			_ = prior.As(&old)
		}
		if strings.TrimSpace(old) == strings.TrimSpace(string(content)) {
			return p.SetPlanned("stack_file_content", prior)
		}
		return p.SetPlanned("stack_file_content", string(content))
	}
	return p.SetPlanned("stack_file_content", prior)
}

// stackImport takes "<endpoint_id>:<stack_id>:<deployment_type>[:<method>]";
// the resource ID is the stack ID alone. The parts used to be separated by
// "-", which is still accepted.
//
// method is required and forces a new stack, and Read depends on it, so it
// cannot stay empty: without the segment it is "repository" for a stack
// deployed from Git, and has to be given otherwise, since Portainer does not
// record whether a stack came from a string, a file or a URL.
//
// Terraform does not apply schema defaults to imported state, so each showed
// as a change on the first plan - and for a stack that change is a redeploy.
// The importer stores them; Read then overwrites whatever Portainer reports.
func stackImport(ctx context.Context, d *dkBag, client *APIClient) error {
	const format = "<endpoint_id>:<stack_id>:<deployment_type>[:<method>]"
	id := d.ID()
	sep := ":"
	if !strings.Contains(id, ":") {
		sep = "-"
	}
	parts := strings.Split(id, sep)
	if len(parts) < 3 || len(parts) > 4 {
		return fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("invalid import ID %q: expected %s", id, format)
		}
	}
	defaults := stackSchemaDefaults(ctx)
	defaults["file_path_in_repository"] = ""
	if err := d.SetFields(defaults); err != nil {
		return err
	}
	endpointID, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	stackID, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	if err := d.Set("deployment_type", parts[2]); err != nil {
		return err
	}
	var method string
	if len(parts) == 4 {
		method = parts[3]
	} else {
		if client == nil {
			return fmt.Errorf("invalid import ID %q: expected %s", id, format)
		}
		var stack struct {
			GitConfig *struct{} `json:"gitConfig"`
		}
		if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/stacks/%d", client.Endpoint, stackID), nil, &stack); err != nil {
			return fmt.Errorf("failed to read stack %d: %w", stackID, err)
		}
		if stack.GitConfig == nil {
			return fmt.Errorf("stack %d was not deployed from Git, so the import ID needs its method: %s%sstring, %s%sfile or %s%surl", stackID, id, sep, id, sep, id, sep)
		}
		method = "repository"
	}
	if err := d.Set("method", method); err != nil {
		return err
	}
	d.SetID(strconv.Itoa(stackID))
	return nil
}

// stackSchemaDefaults returns the default of every stack attribute that has
// one.
func stackSchemaDefaults(ctx context.Context) map[string]interface{} {
	out := map[string]interface{}{}
	for name, a := range stackSchema(ctx).Attributes {
		switch at := a.(type) {
		case schema.BoolAttribute:
			if at.Default != nil {
				resp := &defaults.BoolResponse{}
				at.Default.DefaultBool(ctx, defaults.BoolRequest{}, resp)
				out[name] = resp.PlanValue.ValueBool()
			}
		case schema.StringAttribute:
			if at.Default != nil {
				resp := &defaults.StringResponse{}
				at.Default.DefaultString(ctx, defaults.StringRequest{}, resp)
				out[name] = resp.PlanValue.ValueString()
			}
		}
	}
	return out
}
