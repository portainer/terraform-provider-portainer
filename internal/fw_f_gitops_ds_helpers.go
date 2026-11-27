package internal

import (
	"os"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helpers of the GitOps data sources (portainer_gitops_*), served by the
// plugin framework.

// gitopsDSAddError reports err the way SDKv2's diag.FromErr did: the whole
// error text as the summary, so the message users see is unchanged.
func gitopsDSAddError(diags *diag.Diagnostics, err error) {
	diags.AddError(err.Error(), "")
}

// gitopsDSTimestamp is the suffix the list data sources have always put in
// their ID (the same value makeTimestamp gives).
func gitopsDSTimestamp() int64 {
	return int64(os.Getpid()) + int64(os.Getppid())
}

// gitopsDSBoolDefault reads an optional bool that SDKv2 defaulted to false:
// null in the configuration is stored as false, as SDKv2 stored it.
func gitopsDSBoolDefault(v types.Bool) types.Bool {
	if v.IsNull() || v.IsUnknown() {
		return types.BoolValue(false)
	}
	return v
}

// gitopsDSString is d.GetOk for a string: "" when null or empty.
func gitopsDSString(v types.String) (string, bool) {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return "", false
	}
	return v.ValueString(), true
}

// gitopsDSInt is d.GetOk for a number: zero counts as unset.
func gitopsDSInt(v types.Int64) (int, bool) {
	if v.IsNull() || v.IsUnknown() || v.ValueInt64() == 0 {
		return 0, false
	}
	return int(v.ValueInt64()), true
}

// gitopsDSStringList builds a list(string); nil gives an empty list.
func gitopsDSStringList(values []string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

// gitopsDSSortedStrings returns a sorted copy, never nil.
func gitopsDSSortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

// gitopsDSAuth is the authentication object of a source payload, nil when no
// credentials are given.
func gitopsDSAuth(username, password string) map[string]interface{} {
	if username == "" && password == "" {
		return nil
	}
	return map[string]interface{}{"username": username, "password": password}
}

// gitopsDSSourceDetail is the part of GET /gitops/sources/{id} the
// portainer_gitops_source data source reads.
type gitopsDSSourceDetail struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Interval   string `json:"interval"`
	LastSync   int    `json:"lastSync"`
	Error      string `json:"error"`
	Connection struct {
		TLSSkipVerify  bool `json:"tlsSkipVerify"`
		Authentication struct {
			Username string `json:"username"`
		} `json:"authentication"`
	} `json:"connection"`
	Access struct {
		Public bool  `json:"public"`
		Teams  []int `json:"teams"`
		Users  []int `json:"users"`
	} `json:"access"`
}

// gitopsStatusSummary is the workflows.StatusSummary object, returned by both
// the source and the workflow summary endpoints.
type gitopsStatusSummary struct {
	Healthy int `json:"healthy"`
	Syncing int `json:"syncing"`
	Error   int `json:"error"`
	Paused  int `json:"paused"`
	Unknown int `json:"unknown"`
	Drifted int `json:"drifted"`
}

var gitopsDSSummaryAttrTypes = map[string]attr.Type{
	"healthy": types.Int64Type, "syncing": types.Int64Type, "error": types.Int64Type,
	"paused": types.Int64Type, "unknown": types.Int64Type, "drifted": types.Int64Type,
}

// gitopsDSSummaryAttribute is the computed summary list. SDKv2 served a
// computed list of objects as an attribute, so it stays one.
func gitopsDSSummaryAttribute(what string) schema.ListAttribute {
	return schema.ListAttribute{
		Computed:    true,
		ElementType: types.ObjectType{AttrTypes: gitopsDSSummaryAttrTypes},
		Description: "Count of " + what + " per sync status, as reported by Portainer's summary endpoint. Always exactly one element.",
	}
}

// toList is the summary as its one-element list.
func (s gitopsStatusSummary) toList() types.List {
	obj := types.ObjectValueMust(gitopsDSSummaryAttrTypes, map[string]attr.Value{
		"healthy": types.Int64Value(int64(s.Healthy)), "syncing": types.Int64Value(int64(s.Syncing)),
		"error": types.Int64Value(int64(s.Error)), "paused": types.Int64Value(int64(s.Paused)),
		"unknown": types.Int64Value(int64(s.Unknown)), "drifted": types.Int64Value(int64(s.Drifted)),
	})
	return types.ListValueMust(types.ObjectType{AttrTypes: gitopsDSSummaryAttrTypes}, []attr.Value{obj})
}

// gitopsWorkflow is the workflows.Workflow payload. The three phase statuses
// are what the Portainer UI shows as the workflow's health: a workflow is only
// fully healthy when the source, the artifact and the target all are.
type gitopsWorkflow struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status struct {
		Source   gitopsPhaseStatus `json:"source"`
		Artifact gitopsPhaseStatus `json:"artifact"`
		Target   gitopsPhaseStatus `json:"target"`
	} `json:"status"`
	CreationDate int `json:"creationDate"`
	LastSyncDate int `json:"lastSyncDate"`
}

type gitopsPhaseStatus struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// gitopsWorkflowAttrTypes are the attributes of one workflow, shared by the
// elements of portainer_gitops_workflows and (without id) the top level of
// portainer_gitops_workflow.
var gitopsWorkflowAttrTypes = map[string]attr.Type{
	"id": types.Int64Type, "name": types.StringType,
	"source_status": types.StringType, "source_error": types.StringType,
	"artifact_status": types.StringType, "artifact_error": types.StringType,
	"target_status": types.StringType, "target_error": types.StringType,
	"healthy": types.BoolType, "creation_date": types.Int64Type, "last_sync_date": types.Int64Type,
}

func (w gitopsWorkflow) attrValues() map[string]attr.Value {
	return map[string]attr.Value{
		"id": types.Int64Value(int64(w.ID)), "name": types.StringValue(w.Name),
		"source_status": types.StringValue(w.Status.Source.Status), "source_error": types.StringValue(w.Status.Source.Error),
		"artifact_status": types.StringValue(w.Status.Artifact.Status), "artifact_error": types.StringValue(w.Status.Artifact.Error),
		"target_status": types.StringValue(w.Status.Target.Status), "target_error": types.StringValue(w.Status.Target.Error),
		"healthy": types.BoolValue(w.Status.Source.Status == "healthy" &&
			w.Status.Artifact.Status == "healthy" &&
			w.Status.Target.Status == "healthy"),
		"creation_date": types.Int64Value(int64(w.CreationDate)), "last_sync_date": types.Int64Value(int64(w.LastSyncDate)),
	}
}

// gitopsDSRepositoryPayload builds the repository half of the requests of
// portainer_gitops_repo_file_search and portainer_gitops_helm_values.
func gitopsDSRepositoryPayload(repository, reference types.String, tlsSkipVerify types.Bool, username, password types.String, sourceID types.Int64) map[string]interface{} {
	payload := map[string]interface{}{
		"repository":    repository.ValueString(),
		"reference":     reference.ValueString(),
		"TLSSkipVerify": tlsSkipVerify.ValueBool(),
	}
	// The credentials are omitted when unset so a public repository is not
	// cloned with a blank username and password.
	if v, ok := gitopsDSString(username); ok {
		payload["username"] = v
	}
	if v, ok := gitopsDSString(password); ok {
		payload["password"] = v
	}
	if v, ok := gitopsDSInt(sourceID); ok {
		payload["sourceId"] = v
	}
	return payload
}
