package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// policyJSONObjectValidator is the framework form of validateJSONObject: an
// empty or unknown value passes, anything else must decode as a JSON object.
type policyJSONObjectValidator struct{}

func (policyJSONObjectValidator) Description(context.Context) string {
	return "value must be a JSON object"
}

func (v policyJSONObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (policyJSONObjectValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &decoded); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON object",
			fmt.Errorf("%s must be a JSON object (use jsonencode): %w", req.Path.String(), err).Error())
	}
}

// policyAPIFields is what GET /policies/{id} returns, as the SDK version read
// it: each field is only present when the response carried it.
type policyAPIFields struct {
	Name, EnvironmentType, PolicyType, CreatedAt, UpdatedAt *string
	// EnvironmentGroups is nil when the response had no (or a null) list.
	EnvironmentGroups []int
	HasGroups         bool
	// Data is the policy data marshalled back to JSON, nil when absent.
	Data *string
	// EnvironmentTypes is what Portainer returns instead of EnvironmentType
	// (every published Business Edition spec, and 3.1.0 live): the types the
	// policy type applies to, e.g. ["docker","podman","swarm"].
	EnvironmentTypes []string
}

// environmentType is the policy's environment type: EnvironmentType when the
// response had it, else the only entry of EnvironmentTypes; nil when the
// response cannot tell (several types, as for every Docker policy type).
func (f policyAPIFields) environmentType() *string {
	if f.EnvironmentType != nil {
		return f.EnvironmentType
	}
	if len(f.EnvironmentTypes) == 1 {
		return &f.EnvironmentTypes[0]
	}
	return nil
}

// readPolicyAPI fetches one policy (GET /policies/{id}).
func readPolicyAPI(ctx context.Context, client *APIClient, id string) (policyAPIFields, error) {
	var policy map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/policies/%s", client.Endpoint, id), nil, &policy); err != nil {
		return policyAPIFields{}, err
	}
	var out policyAPIFields
	str := func(key string) *string {
		v, ok := policy[key]
		if !ok {
			return nil
		}
		s, _ := v.(string)
		return &s
	}
	out.Name = str("Name")
	out.EnvironmentType = str("EnvironmentType")
	out.PolicyType = str("Type")
	out.CreatedAt = str("CreatedAt")
	out.UpdatedAt = str("UpdatedAt")
	if groups, ok := policy["EnvironmentGroups"]; ok && groups != nil {
		if groupsList, ok := groups.([]interface{}); ok {
			out.HasGroups = true
			out.EnvironmentGroups = make([]int, 0, len(groupsList))
			for _, g := range groupsList {
				if gf, ok := g.(float64); ok {
					out.EnvironmentGroups = append(out.EnvironmentGroups, int(gf))
				}
			}
		}
	}
	if envTypes, ok := policy["EnvironmentTypes"].([]interface{}); ok {
		for _, t := range envTypes {
			if s, ok := t.(string); ok {
				out.EnvironmentTypes = append(out.EnvironmentTypes, s)
			}
		}
	}
	if data, ok := policy["Data"]; ok && data != nil {
		if dataJSON, err := json.Marshal(data); err == nil {
			s := string(dataJSON)
			out.Data = &s
		}
	}
	return out, nil
}

// policyStringOr returns the API value when present, else the current one.
func policyStringOr(v *string, current types.String) types.String {
	if v == nil {
		return current
	}
	return types.StringValue(*v)
}

// policyJSONEquivalent reports whether two JSON documents decode equal.
func policyJSONEquivalent(a, b string) bool {
	var va, vb interface{}
	if json.Unmarshal([]byte(a), &va) != nil || json.Unmarshal([]byte(b), &vb) != nil {
		return false
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return string(ja) == string(jb)
}

// policyInt64ListOrNull keeps a null list null when the API has none.
func policyInt64ListOrNull(values []int, current types.List) types.List {
	if len(values) == 0 && current.IsNull() {
		return types.ListNull(types.Int64Type)
	}
	return int64ListValue(values)
}

var policyConflictObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"environment_group_id":     types.Int64Type,
	"environment_group_name":   types.StringType,
	"environment_count":        types.Int64Type,
	"supported_environments":   types.Int64Type,
	"unsupported_environments": types.Int64Type,
	"existing_policy_id":       types.Int64Type,
	"existing_policy_name":     types.StringType,
}}

var policyNewGroupObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"environment_group_id":     types.Int64Type,
	"environment_group_name":   types.StringType,
	"environment_count":        types.Int64Type,
	"supported_environments":   types.Int64Type,
	"unsupported_environments": types.Int64Type,
}}
