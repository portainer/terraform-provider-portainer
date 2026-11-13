package internal

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAddonsAvailability_DecodesBoolAndString covers the 3.0 breaking change
// of system/info's addonsAvailable from a bool (2.x) to a string.
func TestAddonsAvailability_DecodesBoolAndString(t *testing.T) {
	cases := map[string]string{
		`{"addonsAvailable": true}`:                      "available",
		`{"addonsAvailable": false}`:                     "unavailable",
		`{"addonsAvailable": "available"}`:               "available",
		`{"addonsAvailable": "fips-mode"}`:               "fips-mode",
		`{"addonsAvailable": "encryption-key-required"}`: "encryption-key-required",
		`{"addonsAvailable": null}`:                      "",
		`{}`:                                             "",
	}
	for raw, want := range cases {
		var info struct {
			AddonsAvailable addonsAvailability `json:"addonsAvailable"`
		}
		if err := json.Unmarshal([]byte(raw), &info); err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if string(info.AddonsAvailable) != want {
			t.Errorf("%s: got %q, want %q", raw, info.AddonsAvailable, want)
		}
	}

	var info struct {
		AddonsAvailable addonsAvailability `json:"addonsAvailable"`
	}
	if err := json.Unmarshal([]byte(`{"addonsAvailable": 1}`), &info); err == nil {
		t.Error("a number must be rejected")
	}
}

// TestDataSourceSystem_AddonsAvailable runs the data source against a 2.45
// (bool) and a 3.0 (string) system/info.
func TestDataSourceSystem_AddonsAvailable(t *testing.T) {
	cases := map[string]struct {
		value        interface{}
		available    bool
		availability string
	}{
		"2.45 true":   {true, true, "available"},
		"2.45 false":  {false, false, "unavailable"},
		"3.0 ok":      {"available", true, "available"},
		"3.0 not k8s": {"kubernetes-required", false, "kubernetes-required"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/system/status", RespondJSON(http.StatusOK, map[string]interface{}{"InstanceID": "i"}))
			mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "3.0.0"}))
			mock.On("GET", "/system/info", RespondJSON(http.StatusOK, map[string]interface{}{
				"platform": "kubernetes", "addonsAvailable": tc.value,
			}))
			mock.On("GET", "/system/nodes", RespondJSON(http.StatusOK, map[string]interface{}{"nodes": 1}))
			mock.On("GET", "/users/admin/check", RespondString(http.StatusNoContent, "", ""))

			ds := dataSourceSystem()
			d := ds.TestResourceData()
			if err := rcRead(ds, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if got := d.Get("addons_available"); got != tc.available {
				t.Errorf("addons_available: got %v", got)
			}
			if got := d.Get("addons_availability"); got != tc.availability {
				t.Errorf("addons_availability: got %v", got)
			}
		})
	}
}
