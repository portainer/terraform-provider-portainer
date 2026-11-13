package internal

import (
	"net/http"
	"strings"
	"testing"
)

// d2kEndpointResponse is what GET /endpoints/{id} returns for a D2K
// environment, trimmed to what the resource reads.
func d2kEndpointResponse(id int, name string, swarm bool) map[string]interface{} {
	return map[string]interface{}{
		"Id":      id,
		"Name":    name,
		"GroupId": 2,
		"TagIds":  []int{4, 5},
		"D2K": map[string]interface{}{
			"Namespace":           "d2k-team-a",
			"ParentEnvironmentID": 3,
			"SwarmMode":           swarm,
		},
	}
}

// TestD2KEnvironmentCreate_HappyPath verifies the resource sends one instance
// and keeps the environment Portainer created for it.
func TestD2KEnvironmentCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Name":        "team-a",
			"Namespace":   "d2k-team-a",
			"Environment": map[string]interface{}{"Id": 12, "Name": "Team A"},
		},
	}))
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, d2kEndpointResponse(12, "Team A", true)))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	_ = d.Set("parent_environment_id", 3)
	_ = d.Set("name", "team-a")
	_ = d.Set("environment_name", "Team A")
	_ = d.Set("swarm_mode", true)
	_ = d.Set("group_id", 2)
	_ = d.Set("tag_ids", []interface{}{4, 5})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "12" {
		t.Errorf("ID: got %q", d.Id())
	}
	if got := d.Get("environment_id"); got != 12 {
		t.Errorf("environment_id: got %v", got)
	}
	if got := d.Get("namespace"); got != "d2k-team-a" {
		t.Errorf("namespace: got %v", got)
	}

	post := mock.FindRequest("POST", "/endpoints/d2k")
	if post == nil {
		t.Fatal("expected POST /endpoints/d2k")
	}
	var payload struct {
		ParentEnvironmentID int
		SwarmMode           bool
		GroupID             int
		TagIDs              []int
		Instances           []map[string]string
	}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.ParentEnvironmentID != 3 || !payload.SwarmMode || payload.GroupID != 2 || len(payload.TagIDs) != 2 {
		t.Errorf("payload: got %+v", payload)
	}
	if len(payload.Instances) != 1 || payload.Instances[0]["Name"] != "team-a" || payload.Instances[0]["EnvironmentName"] != "Team A" {
		t.Errorf("payload.Instances: expected one instance, got %+v", payload.Instances)
	}
}

// TestD2KEnvironmentCreate_InstanceError verifies a per-instance error fails
// the create, even though the request itself succeeded.
func TestD2KEnvironmentCreate_InstanceError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "team-a", "Error": "the agent is in async mode"},
	}))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	_ = d.Set("parent_environment_id", 3)
	_ = d.Set("name", "team-a")

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "the agent is in async mode") {
		t.Fatalf("expected the instance error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID, got %q", d.Id())
	}
}

// TestD2KEnvironmentCreate_ErrorKeepsCreatedEnvironment verifies an
// environment Portainer created despite an error stays in state, so it is
// tainted and replaced rather than leaked.
func TestD2KEnvironmentCreate_ErrorKeepsCreatedEnvironment(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "team-a", "Error": "install timed out", "Environment": map[string]interface{}{"Id": 13}},
	}))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	_ = d.Set("parent_environment_id", 3)
	_ = d.Set("name", "team-a")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected an error")
	}
	if d.Id() != "13" {
		t.Errorf("expected the created environment to stay in state, got %q", d.Id())
	}
}

// TestD2KEnvironmentCreate_NoResult verifies an answer that does not mention
// the instance is an error rather than an empty ID.
func TestD2KEnvironmentCreate_NoResult(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	_ = d.Set("parent_environment_id", 3)
	_ = d.Set("name", "team-a")

	if err := rcCreate(r, d, mock.Client()); err == nil || !strings.Contains(err.Error(), "no result") {
		t.Fatalf("expected a missing-result error, got %v", err)
	}
}

// TestD2KEnvironmentCreate_UnsupportedServer verifies a CE server's 404 is
// explained.
func TestD2KEnvironmentCreate_UnsupportedServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.0", "ServerEdition": "CE",
	}))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	_ = d.Set("parent_environment_id", 3)
	_ = d.Set("name", "team-a")

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "portainer_d2k_environment requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected an explained error, got %v", err)
	}
}

// TestD2KEnvironmentRead_Import verifies Read fills everything from the
// environment, including the install name derived from the namespace.
func TestD2KEnvironmentRead_Import(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, d2kEndpointResponse(12, "Team A", false)))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	d.SetId("12")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	want := map[string]interface{}{
		"name":                  "team-a",
		"environment_name":      "Team A",
		"parent_environment_id": 3,
		"swarm_mode":            false,
		"group_id":              2,
		"environment_id":        12,
	}
	for k, v := range want {
		if got := d.Get(k); got != v {
			t.Errorf("%s: got %v, want %v", k, got, v)
		}
	}
	if got := d.Get("tag_ids").([]interface{}); len(got) != 2 {
		t.Errorf("tag_ids: got %v", got)
	}
}

// TestD2KEnvironmentRead_NotD2K verifies an environment without D2K data is
// rejected rather than silently managed.
func TestD2KEnvironmentRead_NotD2K(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 1, "Name": "local"}))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcRead(r, d, mock.Client()); err == nil || !strings.Contains(err.Error(), "not a D2K environment") {
		t.Fatalf("expected a not-D2K error, got %v", err)
	}
}

// TestD2KEnvironmentRead_404_ClearsID verifies drift handling on 404.
func TestD2KEnvironmentRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/99", RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestD2KEnvironmentUpdate_SendsNameGroupTags verifies PUT /endpoints/{id}
// carries only what can change in place.
func TestD2KEnvironmentUpdate_SendsNameGroupTags(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/12", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, d2kEndpointResponse(12, "Renamed", false)))

	r := resourceD2KEnvironment()
	d := r.TestResourceData()
	d.SetId("12")
	_ = d.Set("parent_environment_id", 3)
	_ = d.Set("name", "team-a")
	_ = d.Set("environment_name", "Renamed")
	_ = d.Set("group_id", 2)
	_ = d.Set("tag_ids", []interface{}{4, 5})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/endpoints/12")
	if put == nil {
		t.Fatal("expected PUT /endpoints/12")
	}
	var body map[string]interface{}
	if err := put.DecodeJSON(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["name"] != "Renamed" || body["groupID"] != float64(2) {
		t.Errorf("body: got %v", body)
	}
	if tags, _ := body["tagIDs"].([]interface{}); len(tags) != 2 {
		t.Errorf("tagIDs: got %v", body["tagIDs"])
	}
	if _, ok := body["url"]; ok {
		t.Errorf("a D2K update must not send a URL, got %v", body)
	}
}

// TestD2KEnvironmentDelete verifies DELETE and that a missing environment is
// not an error.
func TestD2KEnvironmentDelete(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/12", RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/endpoints/13", RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceD2KEnvironment()
	for _, id := range []string{"12", "13"} {
		d := r.TestResourceData()
		d.SetId(id)
		if err := rcDelete(r, d, mock.Client()); err != nil {
			t.Fatalf("Delete %s failed: %v", id, err)
		}
	}
	if mock.FindRequest("DELETE", "/endpoints/12") == nil {
		t.Error("expected DELETE /endpoints/12")
	}
}

// TestD2KEnvironmentSchema_ForceNew pins which arguments replace the install.
func TestD2KEnvironmentSchema_ForceNew(t *testing.T) {
	s := resourceD2KEnvironment().Schema
	for _, k := range []string{"parent_environment_id", "name", "swarm_mode"} {
		if !s[k].ForceNew {
			t.Errorf("%s must be ForceNew", k)
		}
	}
	for _, k := range []string{"environment_name", "group_id", "tag_ids"} {
		if s[k].ForceNew {
			t.Errorf("%s is updated in place and must not be ForceNew", k)
		}
	}
}
