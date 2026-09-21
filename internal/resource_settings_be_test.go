package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Business Edition settings endpoints. Each of these lives outside
// PUT /settings, which is why they are separate resources: folding them in
// would make every settings apply rewrite them too.
// =========================================================================

// TestSettingsDefaultRegistry_WritesAndReadsBack pins the payload key, which
// is PascalCase where most of the provider's payloads are camelCase, and the
// fact that the current value has to be read from the main settings object
// because the endpoint itself is write-only.
func TestSettingsDefaultRegistry_WritesAndReadsBack(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/settings/default_registry", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"DefaultRegistry": map[string]interface{}{"Hide": true},
	}))

	r := resourceSettingsDefaultRegistry()
	d := r.TestResourceData()
	_ = d.Set("hide", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	req := mock.FindRequest("PUT", "/settings/default_registry")
	if req == nil {
		t.Fatal("no default registry update was sent")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["Hide"] != true {
		t.Errorf("expected the PascalCase key Hide set to true, got %v", payload)
	}
	if got := d.Get("hide"); got != true {
		t.Errorf("hide: expected the value read back from /settings, got %v", got)
	}
}

// TestSettingsDefaultRegistry_DeleteLeavesSettingAlone pins the destroy
// semantics: there is no endpoint to clear the setting, and turning the
// built-in registry back on behind the operator's back would be a change they
// never asked for.
func TestSettingsDefaultRegistry_DeleteLeavesSettingAlone(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceSettingsDefaultRegistry()
	d := r.TestResourceData()
	d.SetId("portainer-settings-default-registry")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("destroy must not call Portainer, got %d request(s)", len(mock.Requests()))
	}
	if d.Id() != "" {
		t.Error("the resource must be removed from state")
	}
}

// TestSettingsAdditionalFunctionality_RoundTrips covers the policy engine
// toggle, whose response nests the value one level deeper than the payload.
func TestSettingsAdditionalFunctionality_RoundTrips(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/settings/additional_functionality", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/settings/additional_functionality", RespondJSON(http.StatusOK, map[string]interface{}{
		"additionalFunctionality": map[string]interface{}{"Policies": true},
	}))

	r := resourceSettingsAdditionalFunctionality()
	d := r.TestResourceData()
	_ = d.Set("policies", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/settings/additional_functionality").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["Policies"] != true {
		t.Errorf("expected the required key Policies set to true, got %v", payload)
	}
	if got := d.Get("policies"); got != true {
		t.Errorf("policies: expected true after the read, got %v", got)
	}
}

// TestSettings_SendsAddonsCatalogURL covers the one field Portainer 2.45 added
// to the main settings payload, and the omission that keeps it from reaching a
// CE instance that has never heard of it.
func TestSettings_SendsAddonsCatalogURL(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := r.TestResourceData()
	_ = d.Set("addons_catalog_url", "https://charts.example.com/addons.json")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/settings").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["AddonsCatalogURL"] != "https://charts.example.com/addons.json" {
		t.Errorf("AddonsCatalogURL was not sent: %v", payload["AddonsCatalogURL"])
	}

	// Unset, it must not appear at all: an empty value would clear a catalog
	// configured elsewhere.
	mock2 := NewMockServer(t)
	mock2.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	d2 := r.TestResourceData()
	if err := rcCreate(r, d2, mock2.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	var payload2 map[string]interface{}
	if err := mock2.FindRequest("PUT", "/settings").DecodeJSON(&payload2); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if _, ok := payload2["AddonsCatalogURL"]; ok {
		t.Error("an unset add-on catalog URL must be omitted, not sent empty")
	}
}

// TestEdgeMTLSCertificates_ParseResponses covers both certificate readers and
// the field names, which are PascalCase and differ from the provider's usual
// convention.
func TestEdgeMTLSCertificates_ParseResponses(t *testing.T) {
	cert := map[string]interface{}{
		"Subject":                map[string]interface{}{"CommonName": "portainer-edge-ca", "Organization": []string{"Example"}},
		"Issuer":                 map[string]interface{}{"CommonName": "portainer-edge-ca"},
		"SerialNumber":           "42",
		"SHA256Fingerprint":      "ab:cd",
		"SignatureAlgorithm":     "SHA256-RSA",
		"IsCertificateAuthority": true,
		"ValidNotBefore":         "2026-01-01T00:00:00Z",
		"ValidNotAfter":          "2027-01-01T00:00:00Z",
		"Version":                3,
		"SubjectAltDNSNames":     []string{"edge.example.com"},
		"SubjectAltIpAddresses":  []string{"10.0.0.1"},
		"KeyUsages":              []string{"CertSign"},
		"ExtendedKeyUsages":      []string{"ServerAuth"},
		"PublicKey":              map[string]interface{}{"Algorithm": "RSA", "Size": 4096},
	}

	mock := NewMockServer(t)
	mock.On("GET", "/settings/edge/mtls_ca_certificate", RespondJSON(http.StatusOK, map[string]interface{}{
		"MTLSCACertificate": cert,
	}))
	mock.On("GET", "/settings/edge/mtls_certificate", RespondJSON(http.StatusOK, map[string]interface{}{
		"MTLSCertificate": cert,
	}))

	ca := dataSourceEdgeMTLSCACertificate()
	dCA := ca.TestResourceData()
	if err := rcRead(ca, dCA, mock.Client()); err != nil {
		t.Fatalf("CA certificate read failed: %v", err)
	}
	leaf := dataSourceEdgeMTLSCertificate()
	dLeaf := leaf.TestResourceData()
	if err := rcRead(leaf, dLeaf, mock.Client()); err != nil {
		t.Fatalf("certificate read failed: %v", err)
	}

	for _, d := range []interface{ Get(string) interface{} }{dCA, dLeaf} {
		if got := d.Get("configured"); got != true {
			t.Errorf("configured: expected true when a certificate came back, got %v", got)
		}
		if got := d.Get("common_name"); got != "portainer-edge-ca" {
			t.Errorf("common_name: expected the subject CN, got %v", got)
		}
		if got := d.Get("sha256_fingerprint"); got != "ab:cd" {
			t.Errorf("sha256_fingerprint: expected ab:cd, got %v", got)
		}
		if got := d.Get("valid_not_after"); got != "2027-01-01T00:00:00Z" {
			t.Errorf("valid_not_after: expected the expiry, got %v", got)
		}
		if got := d.Get("public_key_size"); got != 4096 {
			t.Errorf("public_key_size: expected 4096, got %v", got)
		}
		if got := d.Get("subject_alt_dns_names").([]interface{}); len(got) != 1 || got[0] != "edge.example.com" {
			t.Errorf("subject_alt_dns_names: expected the SAN list, got %v", got)
		}
	}
}

// TestEdgeMTLSCertificates_NoneConfigured covers the common case on an
// instance that never set edge mTLS up: nothing to parse is a state to report,
// not a failed plan.
func TestEdgeMTLSCertificates_NoneConfigured(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/settings/edge/mtls_ca_certificate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceEdgeMTLSCACertificate()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("configured"); got != false {
		t.Errorf("configured: expected false with no certificate in the response, got %v", got)
	}
	if got := d.Get("common_name"); got != "" {
		t.Errorf("common_name: expected empty, got %v", got)
	}
}

// TestEdgeMTLSCertificates_SurfacesRealErrors makes sure the tolerance above
// is narrow: only a missing certificate is shrugged off, not a broken call.
func TestEdgeMTLSCertificates_SurfacesRealErrors(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/settings/edge/mtls_certificate", RespondString(http.StatusForbidden,
		"application/json", `{"message":"forbidden"}`))

	ds := dataSourceEdgeMTLSCertificate()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("a forbidden response must fail the read, not report an unconfigured certificate")
	}
}
