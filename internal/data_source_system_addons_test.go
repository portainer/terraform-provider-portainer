package internal

import (
	"encoding/json"
	"testing"
)

// TestAddonsAvailability_DecodesBoolAndString covers the 3.0 breaking change
// of system/info's addonsAvailable from a bool (2.x) to a string.
func TestAddonsAvailability_DecodesBoolAndString(t *testing.T) {
	t.Parallel()
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
