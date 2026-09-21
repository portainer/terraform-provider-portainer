package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceLicensesInfo() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceLicensesInfoRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"valid": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the licensing on this instance is currently valid.",
			},
			"type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Portainer's numeric licence type.",
			},
			"company": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Company the licence is issued to.",
			},
			"nodes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Nodes the licence covers.",
			},
			"expires_at": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp the licence expires at.",
			},
			"enforced_at": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp enforcement begins at, zero when nothing is being enforced.",
			},
			"overuse_started_at": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp the instance started exceeding its node count, zero when it is within it. A non-zero value here is the one worth alerting on.",
			},
		},
	}
}

func dataSourceLicensesInfoRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var info struct {
		Valid                   bool   `json:"valid"`
		Type                    int    `json:"type"`
		Company                 string `json:"company"`
		Nodes                   int    `json:"nodes"`
		ExpiresAt               int64  `json:"expiresAt"`
		EnforcedAt              int64  `json:"enforcedAt"`
		OveruseStartedTimestamp int64  `json:"overuseStartedTimestamp"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/licenses/info", nil, &info); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the licence information: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"valid":              info.Valid,
		"type":               info.Type,
		"company":            info.Company,
		"nodes":              info.Nodes,
		"expires_at":         int(info.ExpiresAt),
		"enforced_at":        int(info.EnforcedAt),
		"overuse_started_at": int(info.OveruseStartedTimestamp),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-licenses-info")
	return nil
}

// dataSourceRecommendations folds Portainer's recommendation listing and its
// summary into one read: the counts are a roll-up of the same list, and
// fetching one without the other is rarely what is wanted.
func dataSourceRecommendations() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRecommendationsRead,

		Schema: map[string]*schema.Schema{
			"category": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only list recommendations in this category. Leave unset for all of them.",
			},
			"severity": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only list recommendations of this severity. Leave unset for all of them.",
			},
			// Computed attributes
			"recommendations": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The recommendations Portainer has for this instance.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Stable identifier of the recommendation type, which is what to match on rather than the title.",
						},
						"title": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Short title of the recommendation.",
						},
						"description": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "What the recommendation is about.",
						},
						"category": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Category the recommendation belongs to.",
						},
						"severity": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "How serious Portainer considers it.",
						},
						"action_label": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Label of the action Portainer's UI offers for it.",
						},
						"action_url": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Where that action leads.",
						},
					},
				},
			},
			"total": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total recommendations, from Portainer's own summary rather than the filtered listing above.",
			},
			"total_types": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Distinct recommendation types.",
			},
			"severity_counts": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Recommendation count per severity.",
			},
			"category_counts": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Recommendation count per category.",
			},
		},
	}
}

func dataSourceRecommendationsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	query := url.Values{}
	if v, ok := d.GetOk("category"); ok && v.(string) != "" {
		query.Set("category", v.(string))
	}
	if v, ok := d.GetOk("severity"); ok && v.(string) != "" {
		query.Set("severity", v.(string))
	}
	listURL := client.Endpoint + "/recommendations"
	if len(query) > 0 {
		listURL += "?" + query.Encode()
	}

	var list []struct {
		TypeID      string `json:"typeId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Severity    string `json:"severity"`
		ActionLabel string `json:"actionLabel"`
		ActionURL   string `json:"actionUrl"`
	}
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the recommendations: %w", err))
	}

	entries := make([]interface{}, 0, len(list))
	for _, item := range list {
		entries = append(entries, map[string]interface{}{
			"type_id": item.TypeID, "title": item.Title, "description": item.Description,
			"category": item.Category, "severity": item.Severity,
			"action_label": item.ActionLabel, "action_url": item.ActionURL,
		})
	}

	// The summary is over every recommendation, not the filtered listing, which
	// is why it is reported separately rather than derived from the list.
	var summary struct {
		Total          int            `json:"total"`
		TotalTypes     int            `json:"totalTypes"`
		SeverityCounts map[string]int `json:"severityCounts"`
		CategoryCounts map[string]int `json:"categoryCounts"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/recommendations/summary", nil, &summary); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the recommendation summary: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"recommendations": entries,
		"total":           summary.Total,
		"total_types":     summary.TotalTypes,
		"severity_counts": intMap(summary.SeverityCounts),
		"category_counts": intMap(summary.CategoryCounts),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-recommendations")
	return nil
}

// intMap turns a map of counts into the interface map the schema takes.
func intMap(source map[string]int) map[string]interface{} {
	out := make(map[string]interface{}, len(source))
	for k, v := range source {
		out[k] = v
	}
	return out
}

func dataSourcePolicyMetadata() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePolicyMetadataRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"minimum_agent_versions": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The lowest agent version each policy type needs, keyed by policy type. An environment on an older agent will not run that policy.",
			},
		},
	}
}

func dataSourcePolicyMetadataRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var metadata struct {
		MinimumAgentVersions map[string]string `json:"minimumAgentVersions"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/policies/metadata", nil, &metadata); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the policy metadata: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"minimum_agent_versions": stringMap(metadata.MinimumAgentVersions),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-policy-metadata")
	return nil
}

// dataSourcePolicyConflicts previews what a policy would collide with before
// it is created. It is a data source rather than a resource because the
// endpoint changes nothing: it answers a question about a candidate policy.
func dataSourcePolicyConflicts() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePolicyConflictsRead,

		Schema: map[string]*schema.Schema{
			"policy_json": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateJSONObject,
				// Portainer does not give this payload a shape in its own
				// specification, so it is passed through as JSON rather than
				// flattened into arguments that could not be kept accurate.
				Description: "The candidate policy, as a JSON object. Use `jsonencode({...})`. Portainer does not describe this payload's shape in its API specification, so it is passed through as given.",
			},
			// Computed attributes
			"total_environments": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments the candidate policy would reach.",
			},
			"supported_environments": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Of those, the ones that can run it.",
			},
			"unsupported_environments": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Of those, the ones that cannot - usually an agent too old for the policy type. Check `portainer_policy_metadata` for the versions each type needs.",
			},
			"conflicts": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Environment groups where an existing policy already applies.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"environment_group_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the environment group.",
						},
						"environment_group_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the environment group.",
						},
						"environment_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Environments in that group.",
						},
						"supported_environments": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Of those, the ones that can run the policy.",
						},
						"unsupported_environments": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Of those, the ones that cannot.",
						},
						"existing_policy_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the policy already applying there.",
						},
						"existing_policy_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of that policy.",
						},
					},
				},
			},
			"new_groups": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Environment groups the candidate policy would newly cover, with no existing policy in the way.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"environment_group_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the environment group.",
						},
						"environment_group_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the environment group.",
						},
						"environment_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Environments in that group.",
						},
						"supported_environments": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Of those, the ones that can run the policy.",
						},
						"unsupported_environments": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Of those, the ones that cannot.",
						},
					},
				},
			},
		},
	}
}

func dataSourcePolicyConflictsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(d.Get("policy_json").(string)), &payload); err != nil {
		return diag.FromErr(fmt.Errorf("failed to parse the candidate policy as a JSON object: %w", err))
	}

	var response struct {
		TotalEnvironments       int `json:"totalEnvironments"`
		SupportedEnvironments   int `json:"supportedEnvironments"`
		UnsupportedEnvironments int `json:"unsupportedEnvironments"`
		Conflicts               []struct {
			EnvironmentGroupID      int    `json:"environmentGroupId"`
			EnvironmentGroupName    string `json:"environmentGroupName"`
			EnvironmentCount        int    `json:"environmentCount"`
			SupportedEnvironments   int    `json:"supportedEnvironments"`
			UnsupportedEnvironments int    `json:"unsupportedEnvironments"`
			ExistingPolicyID        int    `json:"existingPolicyId"`
			ExistingPolicyName      string `json:"existingPolicyName"`
		} `json:"conflicts"`
		NewGroups []struct {
			EnvironmentGroupID      int    `json:"environmentGroupId"`
			EnvironmentGroupName    string `json:"environmentGroupName"`
			EnvironmentCount        int    `json:"environmentCount"`
			SupportedEnvironments   int    `json:"supportedEnvironments"`
			UnsupportedEnvironments int    `json:"unsupportedEnvironments"`
		} `json:"newGroups"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/policies/conflicts", payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to preview the policy conflicts: %w", err))
	}

	conflicts := make([]interface{}, 0, len(response.Conflicts))
	for _, conflict := range response.Conflicts {
		conflicts = append(conflicts, map[string]interface{}{
			"environment_group_id":     conflict.EnvironmentGroupID,
			"environment_group_name":   conflict.EnvironmentGroupName,
			"environment_count":        conflict.EnvironmentCount,
			"supported_environments":   conflict.SupportedEnvironments,
			"unsupported_environments": conflict.UnsupportedEnvironments,
			"existing_policy_id":       conflict.ExistingPolicyID,
			"existing_policy_name":     conflict.ExistingPolicyName,
		})
	}

	newGroups := make([]interface{}, 0, len(response.NewGroups))
	for _, group := range response.NewGroups {
		newGroups = append(newGroups, map[string]interface{}{
			"environment_group_id":     group.EnvironmentGroupID,
			"environment_group_name":   group.EnvironmentGroupName,
			"environment_count":        group.EnvironmentCount,
			"supported_environments":   group.SupportedEnvironments,
			"unsupported_environments": group.UnsupportedEnvironments,
		})
	}

	if err := setFields(d, map[string]interface{}{
		"total_environments":       response.TotalEnvironments,
		"supported_environments":   response.SupportedEnvironments,
		"unsupported_environments": response.UnsupportedEnvironments,
		"conflicts":                conflicts,
		"new_groups":               newGroups,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-policy-conflicts")
	return nil
}

func dataSourcePolicyObservabilityTest() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePolicyObservabilityTestRead,

		Schema: map[string]*schema.Schema{
			"one_uptime_url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the OneUptime instance to test against.",
			},
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "OneUptime API key to authenticate with.",
			},
			"project_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "OneUptime project the policy would report into.",
			},
			"policy_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Existing policy to test the connection for, when there is one.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the OneUptime server's TLS certificate.",
			},
			"auto_provision_ingestion_key": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the test should provision an ingestion key as part of the check.",
			},
			"auto_rotate_key": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the test should rotate the key as part of the check.",
			},
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether a failed connection fails the plan. Defaults to `true`, which is the point of a pre-flight check; set it to `false` to branch on `success` instead.",
			},
			// Computed attributes
			"success": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer could reach OneUptime with these details.",
			},
			"message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "What Portainer reported about the attempt.",
			},
		},
	}
}

func dataSourcePolicyObservabilityTestRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"oneUptimeURL":              d.Get("one_uptime_url").(string),
		"tlsSkipVerify":             d.Get("tls_skip_verify").(bool),
		"autoProvisionIngestionKey": d.Get("auto_provision_ingestion_key").(bool),
		"autoRotateKey":             d.Get("auto_rotate_key").(bool),
	}
	// The credentials are omitted when unset so the test never sends a blank
	// key that OneUptime would reject for the wrong reason.
	if v, ok := d.GetOk("api_key"); ok && v.(string) != "" {
		payload["apiKey"] = v.(string)
	}
	if v, ok := d.GetOk("project_id"); ok && v.(string) != "" {
		payload["projectID"] = v.(string)
	}
	if v, ok := d.GetOk("policy_id"); ok && v.(int) != 0 {
		payload["policyId"] = v.(int)
	}

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	testURL := client.Endpoint + "/policies/observability-k8s/test"
	if err := doJSON(ctx, client, http.MethodPost, testURL, payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to test the OneUptime connection: %w", err))
	}

	if !response.Success && d.Get("fail_on_error").(bool) {
		return diag.FromErr(fmt.Errorf("the OneUptime connection test failed: %s (set fail_on_error = false to read the result instead)", response.Message))
	}

	if err := setFields(d, map[string]interface{}{
		"success": response.Success,
		"message": response.Message,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-policy-observability-test")
	return nil
}

// sortedStrings returns a sorted copy, so a data source built from a map
// produces the same output on two identical plans.
func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
