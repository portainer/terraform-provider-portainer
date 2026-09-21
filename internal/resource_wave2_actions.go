package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// resourceKubernetesClusterUpgrade upgrades a provisioned Kubernetes cluster
// to the next stable version. It is a one-shot action with no state of its own
// to read back, so `triggers` is how another upgrade is asked for.
func resourceKubernetesClusterUpgrade() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesClusterUpgradeCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the provisioned Kubernetes environment to upgrade.",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another upgrade when they change. An upgrade is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func resourceKubernetesClusterUpgradeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	upgradeURL := fmt.Sprintf("%s/cloud/endpoints/%d/upgrade", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPost, upgradeURL, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to upgrade the Kubernetes cluster of environment %d: %w", endpointID, err))
	}

	d.SetId(fmt.Sprintf("%d-upgrade-%d", endpointID, makeTimestamp()))
	return nil
}

// resourceUserMembershipsSync refreshes a user's team memberships from the
// configured LDAP/AD or OAuth provider. It is a one-shot action: the
// memberships it produces are read through the team membership resources.
func resourceUserMembershipsSync() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceUserMembershipsSyncCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the user whose team memberships are synchronised.",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another synchronisation when they change. A sync is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func resourceUserMembershipsSyncCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	syncURL := fmt.Sprintf("%s/users/%d/memberships/sync", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodPost, syncURL, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to synchronise the team memberships of user %d: %w", userID, err))
	}

	d.SetId(fmt.Sprintf("%d-memberships-sync-%d", userID, makeTimestamp()))
	return nil
}

// dataSourceStackConversion converts a Compose stack to Kubernetes manifests
// or a Helm chart and returns the result for preview. It is a data source
// rather than a resource because the endpoint changes nothing: it hands back
// files to look at, and deploying them is a separate decision.
func dataSourceStackConversion() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceStackConversionRead,

		Schema: map[string]*schema.Schema{
			"stack_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Compose stack to convert.",
			},
			"target_format": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"kubernetes", "helm"}, false),
				Description:  "What to convert to: `kubernetes` for manifests or `helm` for a Helm chart.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace to generate the Kubernetes resources into.",
			},
			// Computed attributes
			"files": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The converted files, keyed by file name.",
			},
		},
	}
}

func dataSourceStackConversionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	stackID := d.Get("stack_id").(int)
	targetFormat := d.Get("target_format").(string)

	payload := map[string]interface{}{"targetFormat": targetFormat}
	if v, ok := d.GetOk("namespace"); ok && v.(string) != "" {
		payload["namespace"] = v.(string)
	}

	var response struct {
		Files map[string]string `json:"files"`
	}
	convertURL := fmt.Sprintf("%s/stacks/%d/convert", client.Endpoint, stackID)
	if err := doJSON(ctx, client, http.MethodPost, convertURL, payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to convert stack %d to %s: %w", stackID, targetFormat, err))
	}

	files := make(map[string]interface{}, len(response.Files))
	for name, content := range response.Files {
		files[name] = content
	}

	if err := setFields(d, map[string]interface{}{"files": files}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-stack-conversion-%d-%s", stackID, targetFormat))
	return nil
}
