package internal

import (
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// alertingRuleTier is one severity step of a multi-severity alert rule.
type alertingRuleTier struct {
	Severity  string  `json:"severity"`
	Threshold float64 `json:"threshold"`
	Enabled   bool    `json:"enabled"`
	// Selector is part of a built-in rule's definition (BE 3.0+). Portainer
	// never accepts it from a client, so it is only ever decoded.
	Selector string `json:"selector,omitempty"`
}

// alertingRuleTierAttrTypes is the element of the computed `tiers` list of
// portainer_alerting_rule and of the portainer_alerting_rules data source.
var alertingRuleTierAttrTypes = map[string]attr.Type{
	"severity":  types.StringType,
	"threshold": types.Float64Type,
	"enabled":   types.BoolType,
	"selector":  types.StringType,
}

// alertingRuleTiersValue builds the computed `tiers` list; no tiers is an
// empty list, as SDKv2 stored it.
func alertingRuleTiersValue(tiers []alertingRuleTier) types.List {
	elemType := types.ObjectType{AttrTypes: alertingRuleTierAttrTypes}
	elems := make([]attr.Value, 0, len(tiers))
	for _, tier := range tiers {
		elems = append(elems, types.ObjectValueMust(alertingRuleTierAttrTypes, map[string]attr.Value{
			"severity":  types.StringValue(tier.Severity),
			"threshold": types.Float64Value(tier.Threshold),
			"enabled":   types.BoolValue(tier.Enabled),
			"selector":  types.StringValue(tier.Selector),
		}))
	}
	return types.ListValueMust(elemType, elems)
}

// alertingLabelsValue is the labels map Portainer returned. A rule without
// labels keeps an empty map where the state or plan already held one (a
// configured `labels = {}`), and null otherwise.
func alertingLabelsValue(labels map[string]string, prior types.Map) types.Map {
	if len(labels) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.MapNull(types.StringType)
	}
	return stringMapValue(labels)
}

// setAlertingAuthHeaders authenticates the hand-built requests of the
// alerting settings and silence resources.
func setAlertingAuthHeaders(req *http.Request, client *APIClient) {
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
}
