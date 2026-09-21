package internal

import (
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// Edge waiting room and endpoint mTLS (Business Edition).
// =========================================================================

// TestEndpointTrust_SendsBatchOfOne pins the payload: Portainer's endpoint
// takes a batch, and the relations are keyed by the environment identifier as
// a string rather than being part of the entry itself.
func TestEndpointTrust_SendsBatchOfOne(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/edge/trust", RespondJSON(http.StatusNoContent, nil))
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "Name": "shop-floor-3", "EdgeID": "abc-123", "UserTrusted": true,
	}))

	r := resourceEndpointTrust()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 12)
	_ = d.Set("group_id", 2)
	_ = d.Set("edge_group_ids", []interface{}{4, 5})
	_ = d.Set("tag_ids", []interface{}{7})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		EndpointIDs []int `json:"EndpointIDs"`
		Relations   map[string]struct {
			Group      int   `json:"Group"`
			EdgeGroups []int `json:"EdgeGroups"`
			Tags       []int `json:"Tags"`
		} `json:"Relations"`
	}
	if err := mock.FindRequest("POST", "/endpoints/edge/trust").DecodeJSON(&payload); err != nil {
		t.Fatalf("trust payload is not JSON: %v", err)
	}
	if len(payload.EndpointIDs) != 1 || payload.EndpointIDs[0] != 12 {
		t.Errorf("expected a batch holding just this environment, got %v", payload.EndpointIDs)
	}
	relation, ok := payload.Relations["12"]
	if !ok {
		t.Fatalf("the relations must be keyed by the environment identifier, got %v", payload.Relations)
	}
	if relation.Group != 2 || len(relation.EdgeGroups) != 2 || len(relation.Tags) != 1 {
		t.Errorf("the configured relations were not sent: %+v", relation)
	}

	if d.Id() != "12" {
		t.Errorf("expected the environment identifier as the resource ID, got %q", d.Id())
	}
	if got := d.Get("name"); got != "shop-floor-3" {
		t.Errorf("name: expected the environment's name to be read back, got %v", got)
	}
}

// TestEndpointTrust_OmitsUnconfiguredRelations is the destructive case:
// Portainer reads an empty array as "clear this", so sending one for a
// relation the configuration never mentioned would strip the tags or edge
// groups the environment already carries.
func TestEndpointTrust_OmitsUnconfiguredRelations(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/edge/trust", RespondJSON(http.StatusNoContent, nil))
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "UserTrusted": true,
	}))

	r := resourceEndpointTrust()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 12)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/endpoints/edge/trust").DecodeJSON(&payload); err != nil {
		t.Fatalf("trust payload is not JSON: %v", err)
	}
	if _, ok := payload["Relations"]; ok {
		t.Errorf("no relations were configured, so none must be sent: %v", payload["Relations"])
	}
}

// TestEndpointTrust_OmitsOnlyTheUnsetRelations checks the omission is per
// field, not all-or-nothing.
func TestEndpointTrust_OmitsOnlyTheUnsetRelations(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/edge/trust", RespondJSON(http.StatusNoContent, nil))
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "UserTrusted": true,
	}))

	r := resourceEndpointTrust()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 12)
	_ = d.Set("tag_ids", []interface{}{7})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Relations map[string]map[string]interface{} `json:"Relations"`
	}
	if err := mock.FindRequest("POST", "/endpoints/edge/trust").DecodeJSON(&payload); err != nil {
		t.Fatalf("trust payload is not JSON: %v", err)
	}
	relation := payload.Relations["12"]
	if _, ok := relation["Tags"]; !ok {
		t.Error("the configured tags must be sent")
	}
	for _, key := range []string{"Group", "EdgeGroups"} {
		if _, ok := relation[key]; ok {
			t.Errorf("%s was not configured, so it must not be sent: %v", key, relation[key])
		}
	}
}

// TestEndpointTrust_UntrustedLeavesState covers the drift an apply can fix:
// an environment put back in the waiting room is not a trusted environment,
// however the resource's own state remembers it.
func TestEndpointTrust_UntrustedLeavesState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "Name": "shop-floor-3", "UserTrusted": false,
	}))

	r := resourceEndpointTrust()
	d := r.TestResourceData()
	d.SetId("12")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Error("an untrusted environment must leave state so the next apply re-trusts it")
	}
}

// TestEndpointTrust_DeleteDoesNotCallPortainer pins the destroy semantics.
// There is no endpoint to revoke trust, and quietly doing nothing is the
// honest behaviour - deleting the environment is how it is untrusted.
func TestEndpointTrust_DeleteDoesNotCallPortainer(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceEndpointTrust()
	d := r.TestResourceData()
	d.SetId("12")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("destroy must not call Portainer, got %d request(s)", len(mock.Requests()))
	}
}

// TestEdgeWaitingRoom_FiltersUntrusted pins the query that makes the listing a
// waiting room rather than every environment, and the flat list of identifiers
// that feeds portainer_endpoint_trust.
func TestEdgeWaitingRoom_FiltersUntrusted(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 12, "Name": "shop-floor-3", "Type": 4, "EdgeID": "abc", "GroupId": 1, "LastCheckInDate": 1756000000},
		{"Id": 13, "Name": "shop-floor-4", "Type": 7, "EdgeID": "def", "GroupId": 1, "LastCheckInDate": 1756000100},
	}))

	ds := dataSourceEdgeWaitingRoom()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("GET", "/endpoints")
	if req == nil || !strings.Contains(req.Query, "edgeDeviceUntrusted=true") {
		t.Fatalf("expected the untrusted filter in the query, got %q", req.Query)
	}

	ids := d.Get("endpoint_ids").([]interface{})
	if len(ids) != 2 || ids[0] != 12 || ids[1] != 13 {
		t.Errorf("endpoint_ids: expected the identifiers in order, got %v", ids)
	}
	environments := d.Get("environments").([]interface{})
	first := environments[0].(map[string]interface{})
	if first["name"] != "shop-floor-3" || first["edge_id"] != "abc" || first["last_check_in"] != 1756000000 {
		t.Errorf("the environment details were not read: %+v", first)
	}
}

// TestEdgeWaitingRoom_GroupFilter covers the optional narrowing, which matters
// on an instance where fleets are separated by endpoint group.
func TestEdgeWaitingRoom_GroupFilter(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []interface{}{}))

	ds := dataSourceEdgeWaitingRoom()
	d := ds.TestResourceData()
	_ = d.Set("group_id", 3)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	req := mock.FindRequest("GET", "/endpoints")
	if !strings.Contains(req.Query, "groupIds=3") {
		t.Errorf("expected the group filter in the query, got %q", req.Query)
	}
	if got := d.Get("endpoint_ids").([]interface{}); len(got) != 0 {
		t.Errorf("an empty waiting room must read as an empty list, got %v", got)
	}
}

// TestEndpointMTLSCertificates_ReadBothEndpoints covers the pair: the
// certificate an agent connects with, and the one whose rejection explains why
// it cannot.
func TestEndpointMTLSCertificates_ReadBothEndpoints(t *testing.T) {
	cert := map[string]interface{}{
		"Subject":            map[string]interface{}{"CommonName": "agent-12"},
		"Issuer":             map[string]interface{}{"CommonName": "portainer-edge-ca"},
		"SHA256Fingerprint":  "de:ad",
		"ValidNotAfter":      "2027-01-01T00:00:00Z",
		"SignatureAlgorithm": "SHA256-RSA",
		"PublicKey":          map[string]interface{}{"Algorithm": "ECDSA", "Size": 256},
	}

	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12/mtls_certificate", RespondJSON(http.StatusOK, map[string]interface{}{
		"MTLSCertificate": cert,
	}))
	mock.On("GET", "/endpoints/12/mtls_certificate_error", RespondJSON(http.StatusOK, map[string]interface{}{
		"MTLSCertificate": cert,
	}))

	active := dataSourceEndpointMTLSCertificate()
	dActive := active.TestResourceData()
	_ = dActive.Set("endpoint_id", 12)
	if err := rcRead(active, dActive, mock.Client()); err != nil {
		t.Fatalf("certificate read failed: %v", err)
	}
	if dActive.Id() != "portainer-endpoint-mtls-certificate-12" {
		t.Errorf("the ID must name the environment, got %q", dActive.Id())
	}

	rejected := dataSourceEndpointMTLSCertificateError()
	dRejected := rejected.TestResourceData()
	_ = dRejected.Set("endpoint_id", 12)
	if err := rcRead(rejected, dRejected, mock.Client()); err != nil {
		t.Fatalf("rejected certificate read failed: %v", err)
	}
	if dRejected.Id() != "portainer-endpoint-mtls-certificate-error-12" {
		t.Errorf("the ID must distinguish the rejected certificate, got %q", dRejected.Id())
	}

	for _, d := range []interface{ Get(string) interface{} }{dActive, dRejected} {
		if got := d.Get("configured"); got != true {
			t.Errorf("configured: expected true when a certificate came back, got %v", got)
		}
		if got := d.Get("common_name"); got != "agent-12" {
			t.Errorf("common_name: expected the agent's CN, got %v", got)
		}
		if got := d.Get("issuer_common_name"); got != "portainer-edge-ca" {
			t.Errorf("issuer_common_name: expected the signing CA, got %v", got)
		}
		if got := d.Get("public_key_size"); got != 256 {
			t.Errorf("public_key_size: expected 256, got %v", got)
		}
	}
}

// TestEndpointMTLSCertificate_NoneHeld covers an environment that never
// presented a certificate, which is the normal state without edge mTLS.
func TestEndpointMTLSCertificate_NoneHeld(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12/mtls_certificate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceEndpointMTLSCertificate()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 12)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("configured"); got != false {
		t.Errorf("configured: expected false with no certificate in the response, got %v", got)
	}
}
