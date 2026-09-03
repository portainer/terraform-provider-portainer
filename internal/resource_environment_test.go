package internal

import (
	"encoding/base64"
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
	// The issue #142 Edge ID fallback must stay scoped to Edge Agent types:
	// a Docker environment keeps an empty edge_id.
	if got := d.Get("edge_id"); got != "" {
		t.Errorf("edge_id: expected empty for a non-edge environment, got %v", got)
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
	// Issue #140: a type=4 (Docker Edge Agent) create must send
	// ContainerEngine=docker, otherwise Portainer provisions it as a Kubernetes
	// Edge Agent (type 7).
	if !strings.Contains(body, "ContainerEngine") || !strings.Contains(body, "\r\n\r\ndocker\r\n") {
		t.Errorf("expected ContainerEngine=docker in multipart body for type=4, body=%q", body)
	}

	// Computed edge fields should be populated from the create response.
	if got := d.Get("edge_id"); got != "abc" {
		t.Errorf("edge_id: expected %q, got %v", "abc", got)
	}
	if got := d.Get("edge_key"); got != "edge-key-xyz" {
		t.Errorf("edge_key: expected %q, got %v", "edge-key-xyz", got)
	}
}

// TestEnvironmentCreate_TypeEdgeAgent_GeneratesEdgeID is the regression test
// for issue #142: Portainer only generates an Edge ID at creation when the
// EnforceEdgeID setting is enabled, so both the create response and the
// follow-up inspect return an empty EdgeID until an agent associates. The
// provider must fill the gap by generating a UUID client-side (the same thing
// the Portainer UI does for its deployment script) so edge_id is usable as
// PORTAINER_EDGE_ID right after apply — and the chained Read must not wipe it
// back to "".
func TestEnvironmentCreate_TypeEdgeAgent_GeneratesEdgeID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      13,
		"Name":    "edge-no-enforce",
		"Type":    4,
		"EdgeKey": "edge-key-xyz",
		// EdgeID intentionally absent: EnforceEdgeID is off.
	}))
	mock.On("GET", "/endpoints/13", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      13,
		"Name":    "edge-no-enforce",
		"Type":    4,
		"GroupId": 1,
		"URL":     "",
		"EdgeKey": "edge-key-xyz",
		"EdgeID":  "",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-no-enforce")
	_ = d.Set("environment_address", "")
	_ = d.Set("type", 4)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	edgeID := d.Get("edge_id").(string)
	if edgeID == "" {
		t.Fatal("edge_id: expected a provider-generated UUID when Portainer returns an empty EdgeID, got \"\"")
	}
	// Coarse UUID shape check (8-4-4-4-12).
	if len(edgeID) != 36 || strings.Count(edgeID, "-") != 4 {
		t.Errorf("edge_id: expected UUID format, got %q", edgeID)
	}

	// A later Read while the agent is still unassociated (server EdgeID still
	// empty) must keep the generated value instead of wiping it.
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("edge_id"); got != edgeID {
		t.Errorf("edge_id: expected Read to preserve %q while server EdgeID is empty, got %v", edgeID, got)
	}
}

// TestEnvironmentCreate_ExistingEdgeAgent_GeneratesEdgeID covers the second
// producer of edge_id: when an environment with the requested name already
// exists, Create short-circuits into Update (see
// TestEnvironmentCreate_ExistingName_DelegatesToUpdate) and never sends a POST.
// Adopting an existing Edge Agent environment that way must still yield a
// usable edge_id (issue #142).
func TestEnvironmentCreate_ExistingEdgeAgent_GeneratesEdgeID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 21, "Name": "edge-existing", "Type": 4},
	}))
	mock.On("PUT", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 21, "Name": "edge-existing", "Type": 4,
	}))
	mock.On("GET", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      21,
		"Name":    "edge-existing",
		"Type":    4,
		"GroupId": 1,
		"URL":     "",
		"EdgeKey": "edge-key-xyz",
		"EdgeID":  "",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-existing")
	_ = d.Set("environment_address", "")
	_ = d.Set("type", 4)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create (existing-name path) failed: %v", err)
	}
	if mock.FindRequest("POST", "/endpoints") != nil {
		t.Error("expected NO POST /endpoints when name already exists")
	}
	if got := d.Get("edge_id").(string); got == "" {
		t.Error("edge_id: expected a generated UUID when adopting an existing edge environment, got \"\"")
	}
}

// TestEnvironmentCreate_Type7_ServerReturnsDocker guards the scope of the
// generated Edge ID. Portainer's endpointCreationType enum only defines 1..5,
// so an unrecognised creation type falls through to the plain Docker branch and
// the created environment comes back as type 1 with no edge key. edge_id is
// keyed off the type Portainer reports, not the requested one, so no Edge ID is
// fabricated for what is actually a Docker environment.
func TestEnvironmentCreate_Type7_ServerReturnsDocker(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 31, "Name": "edge-k8s-req", "Type": 1,
	}))
	mock.On("GET", "/endpoints/31", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      31,
		"Name":    "edge-k8s-req",
		"Type":    1,
		"GroupId": 1,
		"URL":     "https://portainer.example.com",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-k8s-req")
	_ = d.Set("environment_address", "https://portainer.example.com")
	_ = d.Set("type", 7)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := d.Get("edge_id"); got != "" {
		t.Errorf("edge_id: expected empty when Portainer reports a non-edge type, got %v", got)
	}
}

// TestEnvironmentRead_EdgeAgent_ServerEdgeIDWins verifies the counterpart of
// the issue #142 fix: once the agent associates and Portainer reports a real
// EdgeID, Read must overwrite whatever the provider generated at create time.
func TestEnvironmentRead_EdgeAgent_ServerEdgeIDWins(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/14", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      14,
		"Name":    "edge-associated",
		"Type":    4,
		"GroupId": 1,
		"URL":     "",
		"EdgeKey": "edge-key-xyz",
		"EdgeID":  "server-edge-id",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("type", 4)
	_ = d.Set("edge_id", "provider-generated-id")
	d.SetId("14")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("edge_id"); got != "server-edge-id" {
		t.Errorf("edge_id: expected server-reported %q to win, got %v", "server-edge-id", got)
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

// TestEnvironmentRead_EdgeAgentKeepsURLScheme verifies that Read does not strip
// the scheme from environment_address for edge agents (issue #136): Portainer
// stores the Edge Agent URL scheme-less, which produced perpetual
// "portainer.example.com" -> "https://portainer.example.com" drift.
func TestEnvironmentRead_EdgeAgentKeepsURLScheme(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/20", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 20, "Name": "edge-prod", "Type": 4, "GroupId": 1,
		"URL": "portainer.example.com",
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("20")
	_ = d.Set("type", 4)
	_ = d.Set("environment_address", "https://portainer.example.com")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "https://portainer.example.com" {
		t.Errorf("environment_address: expected scheme to be kept, got %v", got)
	}
}

// TestEnvironmentRead_EdgeAgentAdoptsChangedURL verifies the scheme-preserving
// branch does not mask a genuine host change coming back from the API.
func TestEnvironmentRead_EdgeAgentAdoptsChangedURL(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 21, "Name": "edge-prod", "Type": 4, "GroupId": 1,
		"URL": "other.example.com",
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("21")
	_ = d.Set("type", 4)
	_ = d.Set("environment_address", "https://portainer.example.com")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "other.example.com" {
		t.Errorf("environment_address: expected API value, got %v", got)
	}
}

// TestEnvironmentRead_NonEdgeUsesAPIURL verifies the scheme-preserving branch is
// edge-agent-only: directly-connected environments keep the URL Portainer
// reports verbatim.
func TestEnvironmentRead_NonEdgeUsesAPIURL(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/22", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 22, "Name": "docker", "Type": 1, "GroupId": 1,
		"URL": "docker.example.com:2375",
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("22")
	_ = d.Set("type", 1)
	_ = d.Set("environment_address", "https://docker.example.com:2375")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "docker.example.com:2375" {
		t.Errorf("environment_address: expected API value, got %v", got)
	}
}

// TestEdgeAddressFromKey pins the edge key parsing used to recover
// environment_address: field 0 of "url|tunnel|fingerprint|id", accepting both
// the padded and the unpadded base64 Portainer emits.
func TestEdgeAddressFromKey(t *testing.T) {
	key := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	rawKey := func(s string) string { return base64.RawStdEncoding.EncodeToString([]byte(s)) }

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"internal address with port", key("http://portainer:9000|portainer:8000|fp|21"), "http://portainer:9000"},
		{"public address", key("https://portainer.example.com|portainer.example.com:8000|fp|4"), "https://portainer.example.com"},
		{"unpadded key", rawKey("http://portainer:9000|portainer:8000|fp|21"), "http://portainer:9000"},
		{"surrounding spaces", key(" https://portainer.example.com |host:8000|fp|7"), "https://portainer.example.com"},
		{"too few fields", key("https://portainer.example.com|host:8000"), ""},
		{"not base64", "not-a-key", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := edgeAddressFromKey(tc.in); got != tc.want {
			t.Errorf("%s: edgeAddressFromKey(%q): expected %q, got %q", tc.name, tc.in, tc.want, got)
		}
	}
}

// TestEnvironmentRead_EdgeAgentAddressFromEdgeKey verifies that Read recovers
// the full address from the edge key. Portainer stores the Edge Agent URL as a
// bare host, dropping the port as well as the scheme, so an agent reaching
// Portainer on a container-internal address such as "http://portainer:9000"
// came back as "portainer" and drifted on every plan even after the
// scheme-only fix for issue #136. The prior state here holds that lossy value,
// which is exactly the state such a drifting resource is in.
func TestEnvironmentRead_EdgeAgentAddressFromEdgeKey(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/23", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 23, "Name": "docker-node", "Type": 4, "GroupId": 1,
		"URL":     "portainer",
		"EdgeKey": base64.StdEncoding.EncodeToString([]byte("http://portainer:9000|portainer:8000|fp|23")),
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("23")
	_ = d.Set("type", 4)
	_ = d.Set("environment_address", "portainer")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "http://portainer:9000" {
		t.Errorf("environment_address: expected the address from the edge key, got %v", got)
	}
}

// TestEnvironmentRead_EdgeAgentKeyOverridesState verifies the edge key wins over
// the value in state, so an address the running agent no longer uses (Portainer
// rewrites the key's scheme and port when an endpoint is de-associated) shows up
// as a real diff instead of being preserved.
func TestEnvironmentRead_EdgeAgentKeyOverridesState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/24", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 24, "Name": "docker-node", "Type": 4, "GroupId": 1,
		"URL":     "portainer",
		"EdgeKey": base64.StdEncoding.EncodeToString([]byte("https://portainer:9443|portainer:8000|fp|24")),
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("24")
	_ = d.Set("type", 4)
	_ = d.Set("environment_address", "http://portainer:9000")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "https://portainer:9443" {
		t.Errorf("environment_address: expected the address from the edge key, got %v", got)
	}
}

// TestEnvironmentRead_EdgeAgentUnparsableKeyFallsBack verifies that an edge key
// which is not in the expected shape leaves the issue-#136 behaviour in place
// rather than wiping the scheme out of state.
func TestEnvironmentRead_EdgeAgentUnparsableKeyFallsBack(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/25", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 25, "Name": "edge-prod", "Type": 4, "GroupId": 1,
		"URL":     "portainer.example.com",
		"EdgeKey": "not-a-key",
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("25")
	_ = d.Set("type", 4)
	_ = d.Set("environment_address", "https://portainer.example.com")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "https://portainer.example.com" {
		t.Errorf("environment_address: expected scheme to be kept, got %v", got)
	}
}

// TestEnvironmentRead_NonEdgeIgnoresEdgeKey verifies the edge key is only
// consulted for edge agent types: a directly-connected environment keeps
// reflecting the URL Portainer reports.
func TestEnvironmentRead_NonEdgeIgnoresEdgeKey(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/26", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 26, "Name": "docker", "Type": 1, "GroupId": 1,
		"URL":     "docker.example.com:2375",
		"EdgeKey": base64.StdEncoding.EncodeToString([]byte("http://portainer:9000|portainer:8000|fp|26")),
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("26")
	_ = d.Set("type", 1)
	_ = d.Set("environment_address", "tcp://docker.example.com:2375")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("environment_address"); got != "docker.example.com:2375" {
		t.Errorf("environment_address: expected API value, got %v", got)
	}
}

// TestEnvironmentUpdate_EdgeAgentSendsPublicURL verifies that public_ip is
// pushed to Portainer for edge agents (issue #137): the field is metadata only
// and does not trigger proxy/tunnel registration, so it must survive the
// edge-agent branch that strips the connection fields. environment_address must
// NOT be used as a fallback here — for edge agents it is the Portainer URL.
func TestEnvironmentUpdate_EdgeAgentSendsPublicURL(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "memgraph-main", "Type": 4,
	}))
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "memgraph-main", "Type": 4, "GroupId": 1,
		"PublicURL": "memgraph-main.example.com",
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("name", "memgraph-main")
	_ = d.Set("environment_address", "https://portainer.example.com")
	_ = d.Set("type", 4)
	_ = d.Set("public_ip", "memgraph-main.example.com")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/9")
	if put == nil {
		t.Fatal("expected PUT /endpoints/9 to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["publicURL"]; got != "memgraph-main.example.com" {
		t.Errorf("payload.publicURL: expected %q, got %v", "memgraph-main.example.com", got)
	}
	if _, present := payload["url"]; present {
		t.Errorf("expected PUT body to OMIT 'url' for edge agent, got %v", payload["url"])
	}
}

// TestEnvironmentUpdate_EdgeAgentWithoutPublicIPOmitsPublicURL verifies the
// edge-agent branch does not fall back to environment_address, which would set
// the Portainer URL itself as the environment's Public IP.
func TestEnvironmentUpdate_EdgeAgentWithoutPublicIPOmitsPublicURL(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/10", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 10, "Name": "edge-nopublic", "Type": 4,
	}))
	mock.On("GET", "/endpoints/10", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 10, "Name": "edge-nopublic", "Type": 4, "GroupId": 1,
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("10")
	_ = d.Set("name", "edge-nopublic")
	_ = d.Set("environment_address", "https://portainer.example.com")
	_ = d.Set("type", 4)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/10")
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if _, present := payload["publicURL"]; present {
		t.Errorf("expected PUT body to OMIT 'publicURL', got %v", payload["publicURL"])
	}
}

// TestEnvironmentCreate_EdgeAgentAppliesPublicIP verifies that creating an edge
// agent with public_ip triggers the follow-up Update — the multipart Create
// form does not persist PublicURL for edge agents (issue #137).
func TestEnvironmentCreate_EdgeAgentAppliesPublicIP(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 11, "Name": "memgraph-main", "Type": 4, "EdgeKey": "ek", "EdgeID": "eid",
	}))
	mock.On("PUT", "/endpoints/11", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 11, "Name": "memgraph-main", "Type": 4,
	}))
	mock.On("GET", "/endpoints/11", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 11, "Name": "memgraph-main", "Type": 4, "GroupId": 1,
		"PublicURL": "memgraph-main.example.com",
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "memgraph-main")
	_ = d.Set("environment_address", "https://portainer.example.com")
	_ = d.Set("type", 4)
	_ = d.Set("public_ip", "memgraph-main.example.com")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/11")
	if put == nil {
		t.Fatal("expected follow-up PUT /endpoints/11 to apply public_ip")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["publicURL"]; got != "memgraph-main.example.com" {
		t.Errorf("payload.publicURL: expected %q, got %v", "memgraph-main.example.com", got)
	}
	if got := d.Get("public_ip"); got != "memgraph-main.example.com" {
		t.Errorf("public_ip in state: got %v", got)
	}
	// The Edge ID from the create response must survive the follow-up Update
	// and the Read it chains into, which reports an empty EdgeID (issue #142).
	if got := d.Get("edge_id"); got != "eid" {
		t.Errorf("edge_id: expected %q to survive the follow-up Update and Read, got %v", "eid", got)
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
