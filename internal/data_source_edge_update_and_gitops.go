package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceEdgeUpdateScheduleInfo folds the two argument-less endpoints that
// describe what an edge update schedule could do: the fleet's current state
// and the agent versions on offer.
func dataSourceEdgeUpdateScheduleInfo() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeUpdateScheduleInfoRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"agent_versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Agent versions an update schedule can move environments to, sorted.",
			},
			"minimum_agent_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Lowest agent version that can take part in an update schedule.",
			},
			"up_to_date_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments already on the newest agent.",
			},
			"outdated_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments on an older agent. This is the number an update schedule would act on.",
			},
			"has_local_time_zone": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether any environment reports a local timezone, which is what lets a schedule run in local time.",
			},
			"has_no_local_time_zone": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether any environment reports no local timezone. Those environments cannot be scheduled in local time.",
			},
		},
	}
}

func dataSourceEdgeUpdateScheduleInfoRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var info struct {
		MinAgentVersion    string `json:"MinAgentVersion"`
		UpToDateCount      int    `json:"UpToDateCount"`
		OutdatedCount      int    `json:"OutdatedCount"`
		HasLocalTimeZone   bool   `json:"HasLocalTimeZone"`
		HasNoLocalTimeZone bool   `json:"HasNoLocalTimeZone"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/edge_update_schedules/info", nil, &info); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the edge update schedule information: %w", err))
	}

	var versions []string
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/edge_update_schedules/agent_versions", nil, &versions); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the supported agent versions: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"agent_versions":         sortedStrings(versions),
		"minimum_agent_version":  info.MinAgentVersion,
		"up_to_date_count":       info.UpToDateCount,
		"outdated_count":         info.OutdatedCount,
		"has_local_time_zone":    info.HasLocalTimeZone,
		"has_no_local_time_zone": info.HasNoLocalTimeZone,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-edge-update-schedule-info")
	return nil
}

// dataSourceAgentVersions lists the agent versions running across the
// environments the token can see. It is a different question from
// portainer_edge_update_schedule_info, which reports the versions an update
// schedule can move environments to.
func dataSourceAgentVersions() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAgentVersionsRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Agent versions in use across the environments the token can see, sorted. More than one entry means the fleet is not on a single version.",
			},
		},
	}
}

func dataSourceAgentVersionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var versions []string
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/endpoints/agent_versions", nil, &versions); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the agent versions: %w", err))
	}

	if err := setFields(d, map[string]interface{}{"versions": sortedStrings(versions)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-agent-versions")
	return nil
}

func dataSourceEdgeUpdatePreviousVersions() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeUpdatePreviousVersionsRead,

		Schema: map[string]*schema.Schema{
			"environment_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Only report on these environments.",
			},
			"edge_group_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Only report on the environments in these edge groups.",
			},
			// Computed attributes
			"previous_versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The agent version each environment was on before its last update, which is what a rollback would return it to.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoint_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Identifier of the environment.",
						},
						"version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Agent version it ran before the update.",
						},
					},
				},
			},
		},
	}
}

func dataSourceEdgeUpdatePreviousVersionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	query := url.Values{}
	for field, key := range map[string]string{
		"environment_ids": "environmentIds", "edge_group_ids": "edgeGroupIds",
	} {
		ids, _ := d.Get(field).([]interface{})
		if len(ids) == 0 {
			continue
		}
		parts := make([]string, 0, len(ids))
		for _, id := range toIntSlice(ids) {
			parts = append(parts, strconv.Itoa(id))
		}
		query.Set(key, strings.Join(parts, ","))
	}

	listURL := client.Endpoint + "/edge_update_schedules/previous_versions"
	if len(query) > 0 {
		listURL += "?" + query.Encode()
	}

	// Portainer answers with a map of environment identifier to version, so
	// the keys are sorted to keep the output stable between two plans.
	var versions map[string]string
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &versions); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the previous agent versions: %w", err))
	}

	endpointIDs := make([]string, 0, len(versions))
	for endpointID := range versions {
		endpointIDs = append(endpointIDs, endpointID)
	}
	sort.Strings(endpointIDs)

	entries := make([]interface{}, 0, len(endpointIDs))
	for _, endpointID := range endpointIDs {
		entries = append(entries, map[string]interface{}{
			"endpoint_id": endpointID, "version": versions[endpointID],
		})
	}

	if err := setFields(d, map[string]interface{}{"previous_versions": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-edge-update-previous-versions")
	return nil
}

// dataSourceEdgeUpdateSchedulesActive reports which update schedules are
// currently in flight for a set of environments. Portainer takes the
// environment list in a POST body, but the call changes nothing.
func dataSourceEdgeUpdateSchedulesActive() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeUpdateSchedulesActiveRead,

		Schema: map[string]*schema.Schema{
			"environment_ids": {
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Environments to report active update schedules for.",
			},
			// Computed attributes
			"schedules": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The update schedules currently in flight. An empty list means none of the given environments is mid-update.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"schedule_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the update schedule.",
						},
						"endpoint_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Environment the schedule is updating.",
						},
						"edge_stack_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Edge stack carrying out the update.",
						},
						"target_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Agent version the schedule is moving the environment to.",
						},
					},
				},
			},
		},
	}
}

func dataSourceEdgeUpdateSchedulesActiveRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	ids, _ := d.Get("environment_ids").([]interface{})
	payload := map[string]interface{}{"EnvironmentIDs": toIntSlice(ids)}

	var relations []struct {
		ScheduleID    int    `json:"scheduleId"`
		EnvironmentID int    `json:"environmentId"`
		EdgeStackID   int    `json:"edgeStackId"`
		TargetVersion string `json:"targetVersion"`
	}
	activeURL := client.Endpoint + "/edge_update_schedules/active"
	if err := doJSON(ctx, client, http.MethodPost, activeURL, payload, &relations); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the active edge update schedules: %w", err))
	}

	entries := make([]interface{}, 0, len(relations))
	for _, relation := range relations {
		entries = append(entries, map[string]interface{}{
			"schedule_id": relation.ScheduleID, "endpoint_id": relation.EnvironmentID,
			"edge_stack_id": relation.EdgeStackID, "target_version": relation.TargetVersion,
		})
	}

	if err := setFields(d, map[string]interface{}{"schedules": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-edge-update-schedules-active")
	return nil
}

func dataSourceEdgeConfigurationFiles() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeConfigurationFilesRead,

		Schema: map[string]*schema.Schema{
			"edge_configuration_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the edge configuration whose files are read.",
			},
			// Computed attributes
			"files": {
				Type:     schema.TypeString,
				Computed: true,
				// The endpoint hands back the payload itself rather than a JSON
				// document, so it is carried through unchanged.
				Description: "The configuration's files as Portainer returns them. This is the payload itself, not a JSON document, so it is carried through unchanged.",
			},
		},
	}
}

func dataSourceEdgeConfigurationFilesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	configurationID := d.Get("edge_configuration_id").(int)

	filesURL := fmt.Sprintf("%s/edge_configurations/%d/files", client.Endpoint, configurationID)
	body, err := apiGETRaw(ctx, client, filesURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the files of edge configuration %d: %w", configurationID, err))
	}

	if err := setFields(d, map[string]interface{}{"files": string(body)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-edge-configuration-files-%d", configurationID))
	return nil
}

func dataSourceEdgeStackStaggerStatus() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeStackStaggerStatusRead,

		Schema: map[string]*schema.Schema{
			"edge_stack_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the edge stack whose staggered rollout is reported on.",
			},
			// Computed attributes
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "How the staggered rollout is going, as Portainer reports it.",
			},
		},
	}
}

func dataSourceEdgeStackStaggerStatusRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	stackID := d.Get("edge_stack_id").(int)

	var response struct {
		Status string `json:"status"`
	}
	statusURL := fmt.Sprintf("%s/edge_stacks/%d/stagger/status", client.Endpoint, stackID)
	if err := doJSON(ctx, client, http.MethodGet, statusURL, nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the stagger status of edge stack %d: %w", stackID, err))
	}

	if err := setFields(d, map[string]interface{}{"status": response.Status}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-edge-stack-stagger-status-%d", stackID))
	return nil
}

// gitRepositoryArguments are the fields both git helpers below take to reach a
// repository. They are written out in each schema rather than shared, because
// docsdriftlint and schemalint read the schemas statically.

// gitRepositoryPayload builds the repository half of the request.
func gitRepositoryPayload(d *schema.ResourceData) map[string]interface{} {
	payload := map[string]interface{}{
		"repository":    d.Get("repository").(string),
		"reference":     d.Get("reference").(string),
		"TLSSkipVerify": d.Get("tls_skip_verify").(bool),
	}
	// The credentials are omitted when unset so a public repository is not
	// cloned with a blank username and password.
	for field, key := range map[string]string{
		"username": "username", "password": "password",
	} {
		if v, ok := d.GetOk(field); ok && v.(string) != "" {
			payload[key] = v.(string)
		}
	}
	if v, ok := d.GetOk("source_id"); ok && v.(int) != 0 {
		payload["sourceId"] = v.(int)
	}
	return payload
}

func dataSourceGitopsRepoFileSearch() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsRepoFileSearchRead,

		Schema: map[string]*schema.Schema{
			"repository": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the git repository to search.",
			},
			"reference": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Git reference to search at, for example `refs/heads/main`.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username to authenticate to the repository with. Leave unset for a public repository or when `source_id` supplies the credentials.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password or token to authenticate with. Stored in state as a sensitive value.",
			},
			"source_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of a GitOps source to take the credentials from instead of giving them here.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the git server's TLS certificate.",
			},
			"keyword": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return paths matching this keyword.",
			},
			"include": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "File extensions to include, for example `yml,yaml`.",
			},
			"directories_only": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to return only directories rather than files.",
			},
			"force": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to re-clone the repository rather than answer from Portainer's cache.",
			},
			// Computed attributes
			"paths": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The paths the search matched, sorted.",
			},
		},
	}
}

func dataSourceGitopsRepoFileSearchRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := gitRepositoryPayload(d)
	payload["dirOnly"] = d.Get("directories_only").(bool)
	for field, key := range map[string]string{"keyword": "keyword", "include": "include"} {
		if v, ok := d.GetOk(field); ok && v.(string) != "" {
			payload[key] = v.(string)
		}
	}

	searchURL := client.Endpoint + "/gitops/repo/files/search"
	if d.Get("force").(bool) {
		searchURL += "?force=true"
	}

	var paths []string
	if err := doJSON(ctx, client, http.MethodPost, searchURL, payload, &paths); err != nil {
		return diag.FromErr(fmt.Errorf("failed to search the repository for files: %w", err))
	}

	if err := setFields(d, map[string]interface{}{"paths": sortedStrings(paths)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-gitops-repo-file-search-" + d.Get("repository").(string))
	return nil
}

func dataSourceGitopsHelmValues() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsHelmValuesRead,

		Schema: map[string]*schema.Schema{
			"repository": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the git repository holding the values files.",
			},
			"reference": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Git reference to read at, for example `refs/heads/main`.",
			},
			"values_files": {
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Paths of the values files to merge, in order. Later files win, the same way `helm -f` treats them.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username to authenticate to the repository with. Leave unset for a public repository or when `source_id` supplies the credentials.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password or token to authenticate with. Stored in state as a sensitive value.",
			},
			"source_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of a GitOps source to take the credentials from instead of giving them here.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the git server's TLS certificate.",
			},
			// Computed attributes
			"merged_values": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The merged values as YAML, which is what a Helm deployment from these files would receive.",
			},
			"files_processed": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The files that were actually merged, in the order they were applied. A file missing from this list was not found in the repository.",
			},
			"commit_hash": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Commit the values were read at. Watch this to notice the repository moving under a configuration.",
			},
		},
	}
}

func dataSourceGitopsHelmValuesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := gitRepositoryPayload(d)
	files, _ := d.Get("values_files").([]interface{})
	payload["valuesFiles"] = files

	var response struct {
		MergedValues   string          `json:"mergedValues"`
		FilesProcessed json.RawMessage `json:"filesProcessed"`
		CommitHash     string          `json:"commitHash"`
	}
	valuesURL := client.Endpoint + "/gitops/repo/helm/values"
	if err := doJSON(ctx, client, http.MethodPost, valuesURL, payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to preview the merged Helm values: %w", err))
	}

	// The processed list is kept in the order Portainer applied the files,
	// because that order is what decides which value wins.
	var processed []string
	if len(response.FilesProcessed) > 0 {
		if err := json.Unmarshal(response.FilesProcessed, &processed); err != nil {
			return diag.FromErr(fmt.Errorf("failed to parse the list of processed values files: %w", err))
		}
	}

	if err := setFields(d, map[string]interface{}{
		"merged_values":   response.MergedValues,
		"files_processed": processed,
		"commit_hash":     response.CommitHash,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-gitops-helm-values-" + d.Get("repository").(string))
	return nil
}
