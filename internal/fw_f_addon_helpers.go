package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonListItem is the shape Portainer returns for a single addon, both from
// the list endpoint and from an inspect. Record is admin-only and simply
// absent for anyone else, so every field read out of it has to tolerate a
// zero value rather than assume it was populated.
type addonListItem struct {
	ID                     string `json:"id"`
	DisplayName            string `json:"displayName"`
	Description            string `json:"description"`
	ShortDescription       string `json:"shortDescription"`
	Icon                   string `json:"icon"`
	Path                   string `json:"path"`
	Enabled                bool   `json:"enabled"`
	HealthStatus           string `json:"healthStatus"`
	HealthMessage          string `json:"healthMessage"`
	LifecycleStatus        string `json:"lifecycleStatus"`
	LifecycleStatusMessage string `json:"lifecycleStatusMessage"`
	Record                 *struct {
		ChartVersion        string                    `json:"chartVersion"`
		ChartPath           string                    `json:"chartPath"`
		HelmChartRepository string                    `json:"helmChartRepository"`
		RegistryID          int                       `json:"registryId"`
		ImageRegistryID     int                       `json:"imageRegistryId"`
		AvailableVersions   []string                  `json:"availableVersions"`
		UpgradeAvailable    bool                      `json:"upgradeAvailable"`
		VersionCheckError   string                    `json:"versionCheckError"`
		PublicURL           string                    `json:"publicUrl"`
		UserAccessPolicies  map[string]map[string]int `json:"userAccessPolicies"`
		TeamAccessPolicies  map[string]map[string]int `json:"teamAccessPolicies"`
	} `json:"record"`
}

// addonURL builds the base URL for one addon. Addon identifiers are catalog
// slugs rather than numbers, so they are escaped rather than formatted in.
func addonURL(client *APIClient, addonID string) string {
	return fmt.Sprintf("%s/addons/%s", client.Endpoint, url.PathEscape(addonID))
}

// readAddon inspects a single addon. Callers use the nil result to mean the
// addon is not in the catalog at all.
func readAddon(ctx context.Context, client *APIClient, addonID string) (*addonListItem, error) {
	var addon addonListItem
	if err := doJSON(ctx, client, http.MethodGet, addonURL(client, addonID), nil, &addon); err != nil {
		if isAPINotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &addon, nil
}

// validateJSONObject accepts only a JSON object, which is what Portainer's
// Helm value overrides have to be. A bare array or scalar parses as JSON but
// would be rejected by the server with a much less helpful message. SDKv2
// resources outside the addon family still use it as a ValidateFunc.
func validateJSONObject(v interface{}, key string) ([]string, []error) {
	raw, ok := v.(string)
	if !ok || raw == "" {
		return nil, nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, []error{fmt.Errorf("%s must be a JSON object (use jsonencode): %w", key, err)}
	}
	return nil, nil
}

// addonJSONObjectValidator is the framework form of validateJSONObject, with
// the same message.
type addonJSONObjectValidator struct{}

func (addonJSONObjectValidator) Description(context.Context) string {
	return "value must be a JSON object"
}

func (v addonJSONObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (addonJSONObjectValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	key := req.Path.String()
	if _, errs := validateJSONObject(req.ConfigValue.ValueString(), key); len(errs) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON object", errs[0].Error())
	}
}

// explainAddonsUnavailable turns the 404 of an add-on endpoint into an error
// that names why the add-on lifecycle is unavailable when GET /system/info
// says so (addonsAvailable): "kubernetes-required" means Portainer itself
// does not run on Kubernetes, which add-ons need. Other errors, and a 404 on
// an instance where add-ons are available, are returned unchanged.
func explainAddonsUnavailable(ctx context.Context, client *APIClient, err error) error {
	if !isAPINotFound(err) {
		return err
	}
	var info struct {
		AddonsAvailable addonsAvailability `json:"addonsAvailable"`
	}
	if ierr := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/info", nil, &info); ierr != nil {
		return err
	}
	switch info.AddonsAvailable {
	case "", addonsAvailable:
		return err
	case "kubernetes-required":
		return fmt.Errorf("add-ons need Portainer running on Kubernetes; this Portainer does not run on Kubernetes "+
			"(GET /system/info reports addonsAvailable = %q): %w", info.AddonsAvailable, err)
	default:
		return fmt.Errorf("add-ons are not available on this Portainer (GET /system/info reports addonsAvailable = %q): %w",
			info.AddonsAvailable, err)
	}
}

// addonDiagError reports an error the way SDKv2's diag.FromErr did: the
// whole message as the summary.
func addonDiagError(diags *diag.Diagnostics, err error) {
	diags.AddError(err.Error(), "")
}

// addonStringList builds a list(string); nil gives an empty list, as SDKv2
// stored it.
func addonStringList(values []string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

// addonTriggersRequiresReplace forces a new one-shot run when `triggers`
// changes. SDKv2 treated an unset map and an empty one as the same value, so
// moving between the two is not a change.
type addonTriggersRequiresReplace struct{}

func (addonTriggersRequiresReplace) Description(context.Context) string {
	return "Changing the triggers forces a new run."
}

func (m addonTriggersRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (addonTriggersRequiresReplace) PlanModifyMap(_ context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // create or destroy
	}
	if req.PlanValue.IsUnknown() {
		resp.RequiresReplace = true
		return
	}
	if len(req.PlanValue.Elements()) == 0 && len(req.StateValue.Elements()) == 0 {
		return
	}
	if !req.PlanValue.Equal(req.StateValue) {
		resp.RequiresReplace = true
	}
}
