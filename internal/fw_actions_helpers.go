package internal

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Terraform 1.14 actions for the provider's one-shot "action resources"
// (portainer_endpoint_snapshot, portainer_kubernetes_node_drain, ...).
//
// Each action carries the type name of the resource it mirrors (actions have
// their own namespace) and is derived from it rather than written twice:
//   - its schema is the resource's arguments - computed outputs, `triggers`
//     and the write-only twins left out, since an action has no state to
//     replace or keep a secret out of;
//   - Invoke reshapes the action's configuration into a plan of the resource
//     (schema defaults applied, outputs null) and runs the operation the
//     resource's Create runs, so both send the same requests and report the
//     same errors.
//
// The resources stay: OpenTofu and Terraform < 1.14 have no actions.

// actionInput is an action's configuration in the shape of its resource.
type actionInput struct {
	Schema schema.Schema
	// Plan is the configuration with the resource's schema defaults filled
	// in and every other unset or computed attribute null.
	Plan tfsdk.Plan
	// Config is the configuration alone, for code that tells "unset" from a
	// default or reads secrets from the configuration.
	Config tfsdk.Config
}

// decode reads the input into the resource's model.
func (in *actionInput) decode(ctx context.Context, target any, diags *diag.Diagnostics) bool {
	diags.Append(in.Plan.Get(ctx, target)...)
	return !diags.HasError()
}

// actionInvokeFunc runs a resource's operation for its action.
type actionInvokeFunc func(ctx context.Context, client *APIClient, in *actionInput, resp *action.InvokeResponse)

// resourceActionDef describes the action mirroring a one-shot resource.
type resourceActionDef struct {
	resource    func() resource.Resource
	description string
	// omit lists resource arguments the action leaves out, besides
	// `triggers` and the write-only twins (`*_wo`, `*_wo_version`).
	omit []string
	// required lists optional resource arguments the action requires: the
	// resource accepted them or their write-only twin.
	required []string
	// descriptions replaces resource descriptions that only make sense for a
	// resource (state, replacement).
	descriptions map[string]string
	// invoke runs the operation; nil runs the Create function of a
	// bag-backed (dkResource) resource.
	invoke actionInvokeFunc
}

type resourceAction struct {
	def    *resourceActionDef
	client *APIClient
}

var _ action.ActionWithConfigure = (*resourceAction)(nil)

// newResourceAction returns the constructor of the action mirroring a
// resource.
func newResourceAction(def *resourceActionDef) func() action.Action {
	return func() action.Action { return &resourceAction{def: def} }
}

// withModel decodes the action input into the resource's model M before
// running f, the operation shared with the resource's Create.
func withModel[M any](f func(ctx context.Context, client *APIClient, m *M, in *actionInput, resp *action.InvokeResponse)) actionInvokeFunc {
	return func(ctx context.Context, client *APIClient, in *actionInput, resp *action.InvokeResponse) {
		var m M
		if in.decode(ctx, &m, &resp.Diagnostics) {
			f(ctx, client, &m, in, resp)
		}
	}
}

func (a *resourceAction) resourceSchema(ctx context.Context) schema.Schema {
	var resp resource.SchemaResponse
	a.def.resource().Schema(ctx, resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func (a *resourceAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	var r resource.MetadataResponse
	a.def.resource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: req.ProviderTypeName}, &r)
	resp.TypeName = r.TypeName
}

func (a *resourceAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	a.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// argument reports whether a resource attribute is an argument of the action.
func (a *resourceAction) argument(name string, attr schema.Attribute) bool {
	if name == "id" || name == "triggers" || attr.IsWriteOnly() || strings.HasSuffix(name, "_wo_version") {
		return false
	}
	if attr.IsComputed() && !attr.IsOptional() && !attr.IsRequired() {
		return false
	}
	return !slices.Contains(a.def.omit, name)
}

func (a *resourceAction) Schema(ctx context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	rs := a.resourceSchema(ctx)
	attrs := map[string]actionschema.Attribute{}
	for name, ra := range rs.Attributes {
		if !a.argument(name, ra) {
			continue
		}
		desc := ra.GetDescription()
		if ra.IsSensitive() {
			desc = actionSentence(actionStateNote.ReplaceAllString(desc, "")) + " " + actionSecretNote
		}
		if d, ok := a.def.descriptions[name]; ok {
			desc = d
		} else if def := actionDefaultText(ctx, ra); def != "" && !strings.Contains(strings.ToLower(desc), "default") {
			desc = actionSentence(desc) + " Defaults to `" + def + "`."
		}
		required := ra.IsRequired() || slices.Contains(a.def.required, name)
		aa, err := actionAttribute(ra, required, desc)
		if err != nil {
			resp.Diagnostics.AddError("Invalid action schema", fmt.Sprintf("%s: %s", name, err))
			continue
		}
		attrs[name] = aa
	}
	resp.Schema = actionschema.Schema{Description: a.def.description, Attributes: attrs}
}

// actionSentence ends a description with a period, so another sentence can
// follow it.
func actionSentence(desc string) string {
	desc = strings.TrimSpace(desc)
	if desc != "" && !strings.HasSuffix(desc, ".") {
		desc += "."
	}
	return desc
}

// actionStateNote is the sentence of a secret's resource description that
// points to its write-only twin, which the action does not need.
var actionStateNote = regexp.MustCompile(`\s*Stored in state as a sensitive value; use ` + "`" + `\w+_wo` + "`" + ` to keep it out of state\.`)

// actionSecretNote ends the description of a secret. Action attributes
// cannot be marked sensitive, and the framework rejects write-only ones, so
// the value is only as hidden as the expression that sets it.
const actionSecretNote = "Action attributes cannot be marked sensitive: set it from a sensitive variable so Terraform redacts it in its output."

// actionAttribute converts a resource argument into an action attribute.
func actionAttribute(ra schema.Attribute, required bool, desc string) (actionschema.Attribute, error) {
	optional := !required
	switch t := ra.(type) {
	case schema.StringAttribute:
		return actionschema.StringAttribute{Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	case schema.Int64Attribute:
		return actionschema.Int64Attribute{Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	case schema.BoolAttribute:
		return actionschema.BoolAttribute{Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	case schema.Float64Attribute:
		return actionschema.Float64Attribute{Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	case schema.ListAttribute:
		return actionschema.ListAttribute{ElementType: t.ElementType, Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	case schema.SetAttribute:
		return actionschema.SetAttribute{ElementType: t.ElementType, Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	case schema.MapAttribute:
		return actionschema.MapAttribute{ElementType: t.ElementType, Required: required, Optional: optional, Description: desc, Validators: t.Validators}, nil
	}
	return nil, fmt.Errorf("unsupported attribute type %T", ra)
}

// actionDefault returns the schema default of a resource attribute (nil when
// it has none), which the action applies itself: action schemas have no
// defaults.
func actionDefault(ctx context.Context, ra schema.Attribute) attr.Value {
	p := path.Root("default")
	switch t := ra.(type) {
	case schema.StringAttribute:
		if t.Default != nil {
			var r defaults.StringResponse
			t.Default.DefaultString(ctx, defaults.StringRequest{Path: p}, &r)
			return r.PlanValue
		}
	case schema.Int64Attribute:
		if t.Default != nil {
			var r defaults.Int64Response
			t.Default.DefaultInt64(ctx, defaults.Int64Request{Path: p}, &r)
			return r.PlanValue
		}
	case schema.BoolAttribute:
		if t.Default != nil {
			var r defaults.BoolResponse
			t.Default.DefaultBool(ctx, defaults.BoolRequest{Path: p}, &r)
			return r.PlanValue
		}
	case schema.Float64Attribute:
		if t.Default != nil {
			var r defaults.Float64Response
			t.Default.DefaultFloat64(ctx, defaults.Float64Request{Path: p}, &r)
			return r.PlanValue
		}
	}
	return nil
}

// actionDefaultText is the default of an attribute as the documentation
// shows it, "" when it has none.
func actionDefaultText(ctx context.Context, ra schema.Attribute) string {
	v := actionDefault(ctx, ra)
	if v == nil || v.IsNull() {
		return ""
	}
	return strings.Trim(v.String(), `"`)
}

// input reshapes the action's configuration into the resource's plan and
// configuration.
func (a *resourceAction) input(ctx context.Context, config tfsdk.Config) (*actionInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	rs := a.resourceSchema(ctx)
	objType, ok := rs.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		diags.AddError("Invalid resource schema", "the schema of the resource is not an object")
		return nil, diags
	}

	var configured map[string]tftypes.Value
	if err := config.Raw.As(&configured); err != nil {
		diags.AddError("Invalid action configuration", err.Error())
		return nil, diags
	}

	planVals := map[string]tftypes.Value{}
	configVals := map[string]tftypes.Value{}
	for name, t := range objType.AttributeTypes {
		null := tftypes.NewValue(t, nil)
		planVals[name], configVals[name] = null, null
		if v, ok := configured[name]; ok && !v.IsNull() {
			planVals[name], configVals[name] = v, v
			continue
		}
		ra, ok := rs.Attributes[name]
		if !ok {
			continue // a block, such as timeouts
		}
		if def := actionDefault(ctx, ra); def != nil {
			tv, err := def.ToTerraformValue(ctx)
			if err != nil {
				diags.AddError("Invalid schema default", fmt.Sprintf("%s: %s", name, err))
				return nil, diags
			}
			planVals[name] = tv
		}
	}

	return &actionInput{
		Schema: rs,
		Plan:   tfsdk.Plan{Schema: rs, Raw: tftypes.NewValue(objType, planVals)},
		Config: tfsdk.Config{Schema: rs, Raw: tftypes.NewValue(objType, configVals)},
	}, diags
}

func (a *resourceAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	if a.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The Portainer provider was not configured before the action was invoked.")
		return
	}
	in, diags := a.input(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if a.def.invoke != nil {
		a.def.invoke(ctx, a.client, in, resp)
		return
	}
	dkInvoke(ctx, a.def.resource(), a.client, in, resp)
}

// dkInvoke runs the Create function of a bag-backed resource with the
// timeout its resource gives it, for actions that leave invoke unset. A
// resource with an `output` sends it as progress: the action has no other
// way to show it.
func dkInvoke(ctx context.Context, r resource.Resource, client *APIClient, in *actionInput, resp *action.InvokeResponse) {
	var dr *dkResource
	switch t := r.(type) {
	case *dkResource:
		dr = t
	case *dkImportableResource:
		dr = &t.dkResource
	default:
		resp.Diagnostics.AddError("Invalid action", fmt.Sprintf("%T is not a bag-backed resource; the action needs an invoke function", r))
		return
	}
	objType := in.Schema.Type().TerraformType(ctx)
	d := newDkBag(dkResourceFields(in.Schema.Attributes, in.Schema.Blocks), objType)
	d.loadValues(in.Plan.Raw, false)
	d.loadConfig(in.Config.Raw)

	ctx, cancel := context.WithTimeout(ctx, dr.timeout(d, "create"))
	defer cancel()
	if err := dr.def.create(ctx, d, client); err != nil {
		dkAddError(&resp.Diagnostics, err)
		return
	}
	if out, ok := d.vals["output"].(string); ok && d.set["output"] && out != "" {
		sendProgress(resp, out)
	}
}

// sendProgress reports a message while an action runs.
func sendProgress(resp *action.InvokeResponse, msg string) {
	if resp.SendProgress != nil && strings.TrimSpace(msg) != "" {
		resp.SendProgress(action.InvokeProgressEvent{Message: strings.TrimRight(msg, "\n")})
	}
}

// progressWriter sends every write as a progress message: the operations
// that log their steps write one line at a time. Lines starting with skip
// (debug detail) stay in the output only.
type progressWriter struct {
	resp *action.InvokeResponse
	skip string
}

func (w progressWriter) Write(p []byte) (int, error) {
	if w.skip == "" || !strings.HasPrefix(string(p), w.skip) {
		sendProgress(w.resp, string(p))
	}
	return len(p), nil
}
