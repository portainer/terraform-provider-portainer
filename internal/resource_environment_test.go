package internal

import (
	"net/http"
	"strings"
	"testing"
)

// resource_environment uses the generated SDK (client.Client.Endpoints.*).
//
// Notable SDK behaviors that shape these tests:
//   - EndpointCreate is POST /endpoints with Content-Type: multipart/form-data
//     (NOT JSON). To assert the create payload we inspect raw form fields in
//     the request body string, not DecodeJSON.
//   - findExistingEnvironmentByName(...) always runs first and calls
//     GET /endpoints. Every Create test must register this list mock.
//   - EndpointInspect is GET /endpoints/{id} returning a flat JSON model
//     with capitalized field names (Id, Name, Type, GroupId, URL, TagIds,
//     EdgeID, EdgeKey, PublicURL).
//   - EndpointUpdate is PUT /endpoints/{id} with JSON body using camelCase
//     fields (name, url, groupID, tls, tlsskipVerify, tagIDs, ...).
//   - EndpointDelete is DELETE /endpoints/{id} returning 204.
//
// Skipped from this suite (out of scope / would require harness changes):
//   - The full multipart form decode (multipart parsing of TLS file uploads).
//     We assert key form fields via substring match instead.
//   - Auth-policy update flows beyond the basic tag-update regression case.

// TestEnvironmentCreate_TypeDocker_HappyPath covers the most common path:
// type=1 (Docker), unique name, no existing endpoint by that name.
// Verifies Create → Read chain and that ID is set from the create response.
func TestEnvironmentCreate_TypeDocker_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingEnvironmentByName: empty list.
	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   5,
		"Name": "prod",
		"Type": 1,
	}))
	mock.On("GET", "/endpoints/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      5,
		"Name":    "prod",
		"Type":    1,
		"GroupId": 1,
		"URL":     "tcp://docker.example:2375",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "prod")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints") == nil {
		t.Error("expected POST /endpoints to be sent")
	}
	if mock.FindRequest("GET", "/endpoints/5") == nil {
		t.Error("expected Create to chain into Read at /endpoints/5")
	}
	if got := d.Get("name"); got != "prod" {
		t.Errorf("name: expected %q, got %v", "prod", got)
	}
	if got := d.Get("type"); got != 1 {
		t.Errorf("type: expected 1, got %v", got)
	}
}

// TestEnvironmentCreate_TypeEdgeAgent_HappyPath verifies type=4 (Edge Agent)
// creates correctly and that the multipart payload carries the EndpointCreationType
// matching the user's type. Edge Agent creates skip TagIds in the multipart
// body (tags are applied via Update later).
func TestEnvironmentCreate_TypeEdgeAgent_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      9,
		"Name":    "edge-prod",
		"Type":    4,
		"EdgeID":  "abc",
		"EdgeKey": "edge-key-xyz",
	}))
	// Read after create: Portainer still reports Type=4 here (agent not yet
	// converted to 7).
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      9,
		"Name":    "edge-prod",
		"Type":    4,
		"GroupId": 1,
		"URL":     "",
		"EdgeID":  "abc",
		"EdgeKey": "edge-key-xyz",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-prod")
	_ = d.Set("environment_address", "")
	_ = d.Set("type", 4)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "9" {
		t.Errorf("expected ID %q, got %q", "9", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints")
	if post == nil {
		t.Fatal("expected POST /endpoints to be sent")
	}
	body := string(post.Body)
	// The SDK uses multipart/form-data; we substring-match the form fields.
	if !strings.Contains(body, "edge-prod") {
		t.Errorf("expected POST body to contain Name=edge-prod, body=%q", body)
	}
	// EndpointCreationType should be 4 for Edge Agent. The form field name
	// is "EndpointCreationType" in the multipart payload.
	if !strings.Contains(body, "EndpointCreationType") {
		t.Errorf("expected POST body to include EndpointCreationType form field")
	}
	// Sanity: a line with the value 4 should appear somewhere near the
	// EndpointCreationType field. (Multipart formatting makes a strict
	// regex brittle, so we do a coarse contains check.)
	if !strings.Contains(body, "\r\n\r\n4\r\n") {
		t.Errorf("expected EndpointCreationType value 4 in multipart body, body=%q", body)
	}

	// Computed edge fields should be populated from the create response.
	if got := d.Get("edge_id"); got != "abc" {
		t.Errorf("edge_id: expected %q, got %v", "abc", got)
	}
	if got := d.Get("edge_key"); got != "edge-key-xyz" {
		t.Errorf("edge_key: expected %q, got %v", "edge-key-xyz", got)
	}
}

// TestEnvironmentRead_EdgeAgent_TypeConversion is the KEY regression test.
// CLAUDE.md: "Portainer converts Edge Agent type 4 to type 7 after agent
// connects (handled via DiffSuppressFunc)".
//
// In resource_environment.go the Read function blindly writes the server-side
// Type into state. The DiffSuppressFunc on the schema field then masks the
// 7-vs-4 drift at plan time. This test pins down the actual Read behavior:
// after the server flips Type from 4 to 7, state reflects 7 (and the diff is
// suppressed elsewhere — see TestEnvironmentDiffSuppressFunc_EdgeAgentTypes).
//
// If a future refactor changes Read to preserve user input or removes the
// DiffSuppressFunc, this test plus the next one will both fail loudly.
func TestEnvironmentRead_EdgeAgent_TypeConversion(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      12,
		"Name":    "edge-k8s",
		"Type":    7, // server converted 4 → 7
		"GroupId": 1,
		"URL":     "",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("type", 4) // user wrote 4 in Terraform config
	d.SetId("12")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Whichever direction Read takes, the resource must NOT crash and the ID
	// must remain set.
	if d.Id() != "12" {
		t.Errorf("expected ID to remain %q, got %q", "12", d.Id())
	}
	// Document actual behavior: Read writes server's Type into state.
	if got := d.Get("type"); got != 7 {
		t.Errorf("type: expected Read to reflect server-side value 7 (DiffSuppressFunc masks the drift at plan time), got %v", got)
	}
}

// TestEnvironmentDiffSuppressFunc_EdgeAgentTypes is the companion regression
// test that pins down the schema's DiffSuppressFunc behavior. The function
// must suppress diffs when the prior state holds 7 and the user-supplied
// config still says 4 — but it MUST report diffs for any other transition.
func TestEnvironmentDiffSuppressFunc_EdgeAgentTypes(t *testing.T) {
	r := resourceEnvironment()
	schema := r.Schema["type"]
	if schema.DiffSuppressFunc == nil {
		t.Fatal("expected DiffSuppressFunc on 'type' schema field — regression: someone removed the 4↔7 drift mask")
	}

	cases := []struct {
		name           string
		old, new       string
		wantSuppressed bool
	}{
		{"4 stays 4", "4", "4", false},
		// Critical case: state has 7 (server-converted), config still says 4.
		{"7→4 (edge conversion masked)", "7", "4", true},
		// Reverse direction is real drift and must NOT be suppressed.
		{"4→7 (real change reported)", "4", "7", false},
		// Unrelated type changes must always surface.
		{"1→2 (Docker → Agent)", "1", "2", false},
		{"5→7 (Kubernetes → KubeEdge)", "5", "7", false},
		{"3→1 (Azure → Docker)", "3", "1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := schema.DiffSuppressFunc("type", tc.old, tc.new, nil)
			if got != tc.wantSuppressed {
				t.Errorf("DiffSuppressFunc(old=%q,new=%q) = %v, want %v",
					tc.old, tc.new, got, tc.wantSuppressed)
			}
		})
	}
}

// TestEnvironmentSchema_TypeValidation ensures the ValidateFunc rejects
// out-of-range values. The valid range is 1..7.
func TestEnvironmentSchema_TypeValidation(t *testing.T) {
	r := resourceEnvironment()
	validate := r.Schema["type"].ValidateFunc
	if validate == nil {
		t.Fatal("expected ValidateFunc on 'type' schema field")
	}

	valid := []int{1, 2, 3, 4, 5, 6, 7}
	for _, v := range valid {
		_, errs := validate(v, "type")
		if len(errs) != 0 {
			t.Errorf("expected type=%d to be valid, got errors: %v", v, errs)
		}
	}

	invalid := []int{0, -1, 8, 99}
	for _, v := range invalid {
		_, errs := validate(v, "type")
		if len(errs) == 0 {
			t.Errorf("expected type=%d to be rejected, got no errors", v)
		}
	}
}

// TestEnvironmentDelete_HappyPath verifies the SDK Delete request reaches the
// expected path and method.
func TestEnvironmentDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/42", RespondString(http.StatusNoContent, "", ""))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("42")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/42") == nil {
		t.Error("expected DELETE /endpoints/42 to be sent")
	}
}

// TestEnvironmentDelete_404_NoError verifies a 404 on delete is swallowed
// (resource was already gone).
func TestEnvironmentDelete_404_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"endpoint not found"}`,
	))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}

// TestEnvironmentRead_404_ClearsID confirms that the Inspect-404 branch in
// Read removes the resource from state.
func TestEnvironmentRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/55", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"endpoint not found"}`,
	))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("55")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestEnvironmentRead_PopulatesState verifies that a successful inspect
// hydrates every relevant field from the response model.
func TestEnvironmentRead_PopulatesState(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":        3,
		"Name":      "k8s-prod",
		"Type":      5,
		"GroupId":   2,
		"URL":       "https://kube.example",
		"PublicURL": "kube.example:6443",
		"EdgeID":    "",
		"EdgeKey":   "",
		"TagIds":    []int{10, 20},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("3")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "k8s-prod" {
		t.Errorf("name: expected %q, got %v", "k8s-prod", got)
	}
	if got := d.Get("type"); got != 5 {
		t.Errorf("type: expected 5, got %v", got)
	}
	if got := d.Get("group_id"); got != 2 {
		t.Errorf("group_id: expected 2, got %v", got)
	}
	if got := d.Get("environment_address"); got != "https://kube.example" {
		t.Errorf("environment_address: expected %q, got %v", "https://kube.example", got)
	}
	if got := d.Get("public_ip"); got != "kube.example:6443" {
		t.Errorf("public_ip: expected %q, got %v", "kube.example:6443", got)
	}
	tagIDs := d.Get("tag_ids").([]interface{})
	if len(tagIDs) != 2 || tagIDs[0] != 10 || tagIDs[1] != 20 {
		t.Errorf("tag_ids: expected [10 20], got %v", tagIDs)
	}
}

// TestEnvironmentCreate_HTTPError ensures a 4xx response from the create
// endpoint propagates as an error and leaves the resource ID empty.
func TestEnvironmentCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid environment payload"}`,
	))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "broken")
	_ = d.Set("environment_address", "tcp://does-not-resolve:2375")
	_ = d.Set("type", 1)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestEnvironmentCreate_ExistingName_DelegatesToUpdate covers the
// findExistingEnvironmentByName short-circuit: when an environment with the
// requested name already exists, Create delegates to Update instead of POST.
//
// The expectation:
//   - GET /endpoints returns a record matching the requested name
//   - No POST /endpoints is sent
//   - A PUT /endpoints/{existingId} (the Update call) IS sent
//   - The ID is set to the existing record's ID
func TestEnvironmentCreate_ExistingName_DelegatesToUpdate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 21, "Name": "prod"},
	}))
	mock.On("PUT", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   21,
		"Name": "prod",
		"Type": 1,
	}))
	mock.On("GET", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      21,
		"Name":    "prod",
		"Type":    1,
		"GroupId": 1,
		"URL":     "tcp://docker.example:2375",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "prod")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create (existing-name path) failed: %v", err)
	}

	if d.Id() != "21" {
		t.Errorf("expected ID %q (reused from existing endpoint), got %q", "21", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints") != nil {
		t.Error("expected NO POST /endpoints when name already exists, but one was sent")
	}
	if mock.FindRequest("PUT", "/endpoints/21") == nil {
		t.Error("expected PUT /endpoints/21 (Update delegation) to be sent")
	}
}

// TestEnvironmentUpdate_NonEdgeSendsFullPayload verifies that for non-edge
// types (e.g. Docker = 1) the Update PUT body carries the connection fields
// (name, url, groupID, TLS flags) — i.e. the !isEdgeAgent branch is exercised.
// Camelcase fields match the SDK payload model (name, url, groupID, tls,
// tlsskipVerify, ...).
func TestEnvironmentUpdate_NonEdgeSendsFullPayload(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   7,
		"Name": "prod",
		"Type": 1,
	}))
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      7,
		"Name":    "prod",
		"Type":    1,
		"GroupId": 1,
		"URL":     "tcp://docker.example:2375",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "prod")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)
	_ = d.Set("group_id", 1)
	_ = d.Set("tls_enabled", true)
	_ = d.Set("tls_skip_verify", true)
	_ = d.Set("tls_skip_client_verify", true)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/7")
	if put == nil {
		t.Fatal("expected PUT /endpoints/7 to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["name"]; got != "prod" {
		t.Errorf("payload.name: expected %q, got %v", "prod", got)
	}
	if got := payload["url"]; got != "tcp://docker.example:2375" {
		t.Errorf("payload.url: expected the env address, got %v", got)
	}
	if got := payload["groupID"]; got != float64(1) {
		t.Errorf("payload.groupID: expected 1, got %v", got)
	}
	if got := payload["tls"]; got != true {
		t.Errorf("payload.tls: expected true, got %v", got)
	}
}

// TestEnvironmentUpdate_EdgeAgentSkipsConnectionFields verifies the
// edge-agent guard in Update: when type is 4 or 7, the PUT body must NOT
// carry name/url/tls fields (sending them triggers a proxy/tunnel
// registration attempt that fails when the agent is not yet connected).
// Only tag_ids / access-policies metadata fields are allowed through.
func TestEnvironmentUpdate_EdgeAgentSkipsConnectionFields(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   8,
		"Name": "edge-prod",
		"Type": 4,
	}))
	mock.On("GET", "/endpoints/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      8,
		"Name":    "edge-prod",
		"Type":    4,
		"GroupId": 1,
		"TagIds":  []int{11, 12},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("8")
	_ = d.Set("name", "edge-prod")
	_ = d.Set("environment_address", "")
	_ = d.Set("type", 4)
	_ = d.Set("tag_ids", []interface{}{11, 12})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/8")
	if put == nil {
		t.Fatal("expected PUT /endpoints/8 to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	// The edge-agent branch must NOT include connection fields. The SDK
	// uses `omitempty` on these JSON tags so an empty/zero value is
	// elided from the body — that is exactly what we expect.
	if _, present := payload["name"]; present {
		t.Errorf("expected PUT body to OMIT 'name' for edge agent, but it was present: %v", payload["name"])
	}
	if _, present := payload["url"]; present {
		t.Errorf("expected PUT body to OMIT 'url' for edge agent, but it was present: %v", payload["url"])
	}
	if _, present := payload["tls"]; present {
		t.Errorf("expected PUT body to OMIT 'tls' for edge agent, but it was present")
	}
	// Tag IDs must still be sent (CLAUDE.md: tags applied via Update for
	// edge agents).
	rawTags, ok := payload["tagIDs"]
	if !ok {
		t.Fatal("expected PUT body to include tagIDs for edge agent")
	}
	tags := rawTags.([]interface{})
	if len(tags) != 2 || tags[0] != float64(11) || tags[1] != float64(12) {
		t.Errorf("payload.tagIDs: expected [11 12], got %v", tags)
	}
}

// TestEnvironmentCreate_Type6_RemapsToAgent verifies the EndpointCreationType
// remap: user requests type=6 (Kubernetes via agent), but the multipart form
// must carry EndpointCreationType=2 because Portainer's creation endpoint
// has no "6" code — it uses the Agent (2) code with later type promotion.
func TestEnvironmentCreate_Type6_RemapsToAgent(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   13,
		"Name": "k8s-agent",
		"Type": 6,
	}))
	mock.On("GET", "/endpoints/13", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      13,
		"Name":    "k8s-agent",
		"Type":    6,
		"GroupId": 1,
		"URL":     "tcp://agent.example:9001",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "k8s-agent")
	_ = d.Set("environment_address", "tcp://agent.example:9001")
	_ = d.Set("type", 6)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints")
	if post == nil {
		t.Fatal("expected POST /endpoints to be sent")
	}
	body := string(post.Body)
	if !strings.Contains(body, "EndpointCreationType") {
		t.Fatal("expected POST body to include EndpointCreationType form field")
	}
	// type=6 must be remapped to EndpointCreationType=2 in the form.
	if !strings.Contains(body, "\r\n\r\n2\r\n") {
		t.Errorf("expected EndpointCreationType value 2 (Agent) in multipart body for type=6 remap, body=%q", body)
	}
	// And conversely, the literal value "6" should NOT appear as the
	// EndpointCreationType value (it may appear elsewhere — group ID, etc. —
	// so we only check that the value field is 2, not that "6" is absent).
}
