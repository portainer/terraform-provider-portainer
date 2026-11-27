package internal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// k8sDSVolatileIDs are the data sources whose ID embeds a timestamp or the
// process ID, so it differs between any two reads - and between two provider
// binaries. Their ID is checked by shape instead.
var k8sDSVolatileIDs = map[string]*regexp.Regexp{
	"data.portainer_kubernetes_crd.all":                     regexp.MustCompile(`^\d+/5$`),
	"data.portainer_kubernetes_manifest_dry_run.default":    regexp.MustCompile(`^\d+/7$`),
	"data.portainer_kubernetes_manifest_dry_run.namespaced": regexp.MustCompile(`^\d+/7$`),
	"data.portainer_kubernetes_config.all":                  regexp.MustCompile(`^kubernetes-config-\d+$`),
	"data.portainer_kubernetes_config.some":                 regexp.MustCompile(`^kubernetes-config-\d+$`),
	"data.portainer_kubernetes_events.prod":                 regexp.MustCompile(`^1/prod/events/\d+$`),
	"data.portainer_kubernetes_events.uid":                  regexp.MustCompile(`^1//events/\d+$`),
}

// TestKubernetesDataSources_SameStateAsV300 reads every data source with
// v3.0.0 (SDKv2) and with this build against the same canned API, and
// requires the same state from both, attribute for attribute: the same
// values, the same IDs, and the same empty-versus-null lists and maps.
func TestKubernetesDataSources_SameStateAsV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	config := mockProviderConfig(mock) + k8sDSParityConfig

	v300 := map[string]map[string]string{}
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{
				ExternalProviders: previousRelease,
				Config:            config,
				Check:             k8sDSCaptureState(v300),
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				Check: func(s *terraform.State) error {
					current := map[string]map[string]string{}
					if err := k8sDSCaptureState(current)(s); err != nil {
						return err
					}
					return k8sDSCompareStates(v300, current)
				},
			},
		},
	})
}

func k8sDSCaptureState(into map[string]map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if !strings.HasPrefix(name, "data.portainer_kubernetes_") {
				continue
			}
			attrs := make(map[string]string, len(rs.Primary.Attributes))
			for k, v := range rs.Primary.Attributes {
				attrs[k] = v
			}
			into[name] = attrs
		}
		if len(into) == 0 {
			return fmt.Errorf("no Kubernetes data source in state")
		}
		return nil
	}
}

func k8sDSCompareStates(want, have map[string]map[string]string) error {
	var problems []string
	for name, w := range want {
		h, ok := have[name]
		if !ok {
			problems = append(problems, name+": missing")
			continue
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range h {
			keys[k] = true
		}
		for k := range keys {
			wv, wok := w[k]
			hv, hok := h[k]
			if k == "id" {
				if re := k8sDSVolatileIDs[name]; re != nil {
					if !re.MatchString(wv) || !re.MatchString(hv) {
						problems = append(problems, fmt.Sprintf("%s.id: v3.0.0 %q, now %q, want %s", name, wv, hv, re))
					}
					continue
				}
			}
			if wok != hok || wv != hv {
				problems = append(problems, fmt.Sprintf("%s.%s: v3.0.0 %q (set %v), now %q (set %v)", name, k, wv, wok, hv, hok))
			}
		}
	}
	for name := range have {
		if _, ok := want[name]; !ok {
			problems = append(problems, name+": not read by v3.0.0")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("state differs from v3.0.0:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}
