package internal

import (
	"net/http"
	"testing"
)

// TestDockerNetworkCov3_Read_FullNonConfigOnly covers the non-config_only Read
// branch with populated options, labels, ipam options, a resource control ID,
// and — importantly — the enable_ipv4/enable_ipv6 fallback path where the API
// reports both flags false but the prior config had enable_ipv4=true.
func TestDockerNetworkCov3_Read_FullNonConfigOnly(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/full1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         "full1",
		"Name":       "fullnet",
		"Driver":     "macvlan",
		"Scope":      "local",
		"Internal":   true,
		"Attachable": true,
		"Ingress":    false,
		"ConfigOnly": false,
		"EnableIPv4": false,
		"EnableIPv6": false,
		"Options":    map[string]interface{}{"parent": "eth0"},
		"Labels":     map[string]interface{}{"env": "prod"},
		"IPAM": map[string]interface{}{
			"Driver":  "default",
			"Options": map[string]interface{}{"foo": "bar"},
			"Config": []interface{}{
				map[string]interface{}{"Subnet": "10.0.0.0/24", "Gateway": "10.0.0.1"},
			},
		},
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 55},
		},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	// Prior config has enable_ipv4=true so the fallback branch flips it back on
	// even though the API returned false for both IPv4 and IPv6.
	_ = d.Set("enable_ipv4", true)
	d.SetId("full1")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "fullnet" {
		t.Errorf("name: expected %q, got %v", "fullnet", got)
	}
	if got := d.Get("internal"); got != true {
		t.Errorf("internal: expected true, got %v", got)
	}
	if got := d.Get("attachable"); got != true {
		t.Errorf("attachable: expected true, got %v", got)
	}
	// Fallback: API said false but config said true, so it is preserved as true.
	if got := d.Get("enable_ipv4"); got != true {
		t.Errorf("enable_ipv4: expected true (fallback), got %v", got)
	}
	opts := d.Get("options").(map[string]interface{})
	if opts["parent"] != "eth0" {
		t.Errorf("options.parent: expected eth0, got %v", opts["parent"])
	}
	labels := d.Get("labels").(map[string]interface{})
	if labels["env"] != "prod" {
		t.Errorf("labels.env: expected prod, got %v", labels["env"])
	}
	ipamOpts := d.Get("ipam_options").(map[string]interface{})
	if ipamOpts["foo"] != "bar" {
		t.Errorf("ipam_options.foo: expected bar, got %v", ipamOpts["foo"])
	}
	if got := d.Get("ipam_driver"); got != "default" {
		t.Errorf("ipam_driver: expected default, got %v", got)
	}
	if got := d.Get("resource_control_id"); got != 55 {
		t.Errorf("resource_control_id: expected 55, got %v", got)
	}
}

// TestDockerNetworkCov3_Read_EmptyOptionsLabelsFallToConfig covers the branches
// where the API returns empty Options/Labels/IPAM.Options and the resource
// falls back to the values already present in config.
func TestDockerNetworkCov3_Read_EmptyOptionsLabelsFallToConfig(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/empty1", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":     "empty1",
		"Name":   "emptynet",
		"Driver": "bridge",
		"Scope":  "local",
		"IPAM":   map[string]interface{}{"Driver": "default"},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("options", map[string]interface{}{"cfgopt": "1"})
	_ = d.Set("labels", map[string]interface{}{"cfglabel": "yes"})
	_ = d.Set("ipam_options", map[string]interface{}{"cfgipam": "on"})
	d.SetId("empty1")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	opts := d.Get("options").(map[string]interface{})
	if opts["cfgopt"] != "1" {
		t.Errorf("options should fall back to config, got %v", opts)
	}
	labels := d.Get("labels").(map[string]interface{})
	if labels["cfglabel"] != "yes" {
		t.Errorf("labels should fall back to config, got %v", labels)
	}
	ipamOpts := d.Get("ipam_options").(map[string]interface{})
	if ipamOpts["cfgipam"] != "on" {
		t.Errorf("ipam_options should fall back to config, got %v", ipamOpts)
	}
}
