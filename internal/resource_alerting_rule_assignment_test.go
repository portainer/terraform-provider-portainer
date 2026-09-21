package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Alert rule assignment (Business Edition): which environments a rule
// covers, and how its severity tiers are configured. Both endpoints apply
// to Portainer's built-in rules as much as to rules Terraform created,
// which is why they are resources of their own.
// =========================================================================

// TestAlertingRuleGroups_ReplacesAttachments pins the payload and the fact
// that the current attachments are read from the rule itself, since the
// attachment endpoint only accepts writes.
func TestAlertingRuleGroups_ReplacesAttachments(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/observability/alerting/rules/7/groups", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))
	mock.On("GET", "/observability/alerting/rules/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 7, "endpointGroupIds": []int{2, 5},
	}))

	r := resourceAlertingRuleGroups()
	d := r.TestResourceData()
	_ = d.Set("rule_id", 7)
	_ = d.Set("endpoint_group_ids", []interface{}{2, 5})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		EndpointGroupIDs []int `json:"endpointGroupIds"`
	}
	if err := mock.FindRequest("PUT", "/observability/alerting/rules/7/groups").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.EndpointGroupIDs) != 2 || payload.EndpointGroupIDs[0] != 2 {
		t.Errorf("expected the configured groups in order, got %v", payload.EndpointGroupIDs)
	}
	if d.Id() != "7" {
		t.Errorf("expected the rule identifier as the resource ID, got %q", d.Id())
	}
	if got := d.Get("endpoint_group_ids").([]interface{}); len(got) != 2 {
		t.Errorf("the attachments must be read back from the rule, got %v", got)
	}
}

// TestAlertingRuleGroups_EmptyListIsSent is the opposite of the omission rule
// that applies elsewhere in the provider: this endpoint replaces the
// attachments, so an empty list is a real request to detach rather than
// something to leave out.
func TestAlertingRuleGroups_EmptyListIsSent(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/observability/alerting/rules/7/groups", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))
	mock.On("GET", "/observability/alerting/rules/7", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))

	r := resourceAlertingRuleGroups()
	d := r.TestResourceData()
	_ = d.Set("rule_id", 7)
	_ = d.Set("endpoint_group_ids", []interface{}{})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/observability/alerting/rules/7/groups").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	ids, ok := payload["endpointGroupIds"].([]interface{})
	if !ok || len(ids) != 0 {
		t.Errorf("expected an explicit empty list, got %v", payload["endpointGroupIds"])
	}
}

// TestAlertingRuleGroups_DeleteDetachesButKeepsRule pins the destroy
// semantics: the attachments this resource created go away, the rule does not,
// because it is very likely one of Portainer's built-in rules.
func TestAlertingRuleGroups_DeleteDetachesButKeepsRule(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/observability/alerting/rules/7/groups", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))

	r := resourceAlertingRuleGroups()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/observability/alerting/rules/7/groups").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if ids, ok := payload["endpointGroupIds"].([]interface{}); !ok || len(ids) != 0 {
		t.Errorf("destroy must detach every group, got %v", payload["endpointGroupIds"])
	}
	if mock.FindRequest("DELETE", "/observability/alerting/rules/7") != nil {
		t.Error("destroy must not delete the rule itself")
	}
	if d.Id() != "" {
		t.Error("the resource must be removed from state")
	}
}

// TestAlertingRuleGroups_MissingRuleLeavesState covers a rule deleted outside
// Terraform: there is nothing left to attach to.
func TestAlertingRuleGroups_MissingRuleLeavesState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules/7", RespondString(http.StatusNotFound,
		"application/json", `{"message":"alert rule not found"}`))

	r := resourceAlertingRuleGroups()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Error("a deleted rule must take its attachment resource out of state")
	}
}

// TestAlertingRuleTiers_SendsOrderedLadder pins the tier payload. The tiers
// are a list rather than a set because they are an ordered ladder of
// severities and Portainer stores them in the order given.
func TestAlertingRuleTiers_SendsOrderedLadder(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/observability/alerting/rules/7/tiers", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))
	mock.On("GET", "/observability/alerting/rules/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 7, "enabled": true, "useTiers": true, "conditionOperator": ">", "duration": 5,
		"description": "CPU saturation",
		"tiers": []map[string]interface{}{
			{"severity": "critical", "threshold": 90.0, "enabled": true},
			{"severity": "warning", "threshold": 75.0, "enabled": true},
		},
	}))

	r := resourceAlertingRuleTiers()
	d := r.TestResourceData()
	_ = d.Set("rule_id", 7)
	_ = d.Set("enabled", true)
	_ = d.Set("condition_operator", ">")
	_ = d.Set("duration", 5)
	_ = d.Set("tier", []interface{}{
		map[string]interface{}{"severity": "critical", "threshold": 90.0, "enabled": true},
		map[string]interface{}{"severity": "warning", "threshold": 75.0, "enabled": true},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Enabled           bool   `json:"enabled"`
		ConditionOperator string `json:"conditionOperator"`
		Duration          int    `json:"duration"`
		Tiers             []struct {
			Severity  string  `json:"severity"`
			Threshold float64 `json:"threshold"`
			Enabled   bool    `json:"enabled"`
		} `json:"tiers"`
	}
	if err := mock.FindRequest("PUT", "/observability/alerting/rules/7/tiers").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if !payload.Enabled || payload.ConditionOperator != ">" || payload.Duration != 5 {
		t.Errorf("the scalar fields were not sent: %+v", payload)
	}
	if len(payload.Tiers) != 2 {
		t.Fatalf("expected two tiers, got %d", len(payload.Tiers))
	}
	if payload.Tiers[0].Severity != "critical" || payload.Tiers[0].Threshold != 90 {
		t.Errorf("the ladder order was not preserved: %+v", payload.Tiers)
	}

	if got := d.Get("use_tiers"); got != true {
		t.Errorf("use_tiers: expected true after the read, got %v", got)
	}
	if got := d.Get("description"); got != "CPU saturation" {
		t.Errorf("description: expected the rule's own value to be read back, got %v", got)
	}
}

// TestAlertingRuleTiers_OmitsUnknownOptionals covers the first write against a
// built-in rule: the fields the configuration never set must not reach the
// payload, because Portainer keeps its own value for an omitted field and
// blanking a built-in rule's description is not what an apply about thresholds
// should do.
func TestAlertingRuleTiers_OmitsUnknownOptionals(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/observability/alerting/rules/7/tiers", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))
	mock.On("GET", "/observability/alerting/rules/7", RespondJSON(http.StatusOK, map[string]interface{}{"id": 7}))

	r := resourceAlertingRuleTiers()
	d := r.TestResourceData()
	_ = d.Set("rule_id", 7)
	_ = d.Set("enabled", true)
	_ = d.Set("tier", []interface{}{
		map[string]interface{}{"severity": "warning", "threshold": 75.0, "enabled": true},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/observability/alerting/rules/7/tiers").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	for _, key := range []string{"description", "conditionOperator", "duration"} {
		if _, ok := payload[key]; ok {
			t.Errorf("%s was not configured, so it must not be sent: %v", key, payload[key])
		}
	}
	if _, ok := payload["tiers"]; !ok {
		t.Error("the tiers themselves must always be sent")
	}
}

// TestAlertingRuleTiers_DeleteDoesNotCallPortainer pins the destroy: there is
// no endpoint to clear a tier configuration, and guessing at a payload that
// might disable the rule would be worse than doing nothing.
func TestAlertingRuleTiers_DeleteDoesNotCallPortainer(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceAlertingRuleTiers()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("destroy must not call Portainer, got %d request(s)", len(mock.Requests()))
	}
}

// TestAlertingRuleEnvironments_ReadsDeliveryStatus covers the data source that
// answers the question an attachment cannot: whether the rule is actually
// being evaluated where it was attached.
func TestAlertingRuleEnvironments_ReadsDeliveryStatus(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules/7/environments", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 3, "active": 1, "pending": 1, "error": 1, "excluded": 0, "notSupported": 0,
		"groups": []map[string]interface{}{
			{
				"endpointGroupId": 2, "endpointGroupName": "shops", "size": 3,
				"environments": []map[string]interface{}{
					{"endpointId": 12, "name": "shop-3", "status": "active", "reasonCode": "", "message": "", "updatedAt": 1756000000},
					{"endpointId": 13, "name": "shop-4", "status": "error", "reasonCode": "agent_unreachable", "message": "agent did not respond", "updatedAt": 1756000100},
				},
			},
		},
	}))

	ds := dataSourceAlertingRuleEnvironments()
	d := ds.TestResourceData()
	_ = d.Set("rule_id", 7)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("total"); got != 3 {
		t.Errorf("total: expected 3, got %v", got)
	}
	if got := d.Get("error"); got != 1 {
		t.Errorf("error: expected 1, got %v", got)
	}

	groups := d.Get("groups").([]interface{})
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %d", len(groups))
	}
	group := groups[0].(map[string]interface{})
	if group["endpoint_group_name"] != "shops" || group["size"] != 3 {
		t.Errorf("the group details were not read: %+v", group)
	}

	environments := group["environments"].([]interface{})
	if len(environments) != 2 {
		t.Fatalf("expected two environments, got %d", len(environments))
	}
	failed := environments[1].(map[string]interface{})
	if failed["status"] != "error" || failed["reason_code"] != "agent_unreachable" {
		t.Errorf("the failing environment was not read: %+v", failed)
	}
	if d.Id() != "portainer-alerting-rule-environments-7" {
		t.Errorf("the ID must name the rule, got %q", d.Id())
	}
}
