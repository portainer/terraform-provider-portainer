package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const scopedKubeconfigIDFormat = "<environment_id>:<scoped_kubeconfig_id>"

// scopedKubeconfigMaxExpiry is the longest lifetime Portainer issues a scoped
// kubeconfig for: 7 days.
const scopedKubeconfigMaxExpiry = 7 * 24 * 60 * 60

// scopedKubeconfigStatus is kubernetes.scopedKubeconfigStatusResponse, and
// with UserID/Username/EndpointID also kubernetes.scopedKubeconfigListItem.
type scopedKubeconfigStatus struct {
	ID             int      `json:"id"`
	EndpointID     int      `json:"endpointId"`
	UserID         int      `json:"userId"`
	Username       string   `json:"username"`
	Description    string   `json:"description"`
	Namespaces     []string `json:"namespaces"`
	IncludeSecrets bool     `json:"includeSecrets"`
	ServiceAccount string   `json:"serviceAccount"`
	Active         bool     `json:"active"`
	CreatedAt      int      `json:"createdAt"`
	ExpiresAt      int      `json:"expiresAt"`
}

// resourceKubernetesScopedKubeconfig issues a read-only kubeconfig limited to
// some namespaces of one environment. Portainer has no update for it, so
// every argument forces a new one; a scoped kubeconfig that expired is dropped
// from state on refresh, so the next apply issues a fresh one.
//
// The credential is returned once, by the issuing call, and never again: it is
// kept in state as a sensitive attribute, since a Terraform output is the only
// way to hand it on. A write-only attribute would not help, as it is the
// provider, not the user, that receives the value.
func resourceKubernetesScopedKubeconfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesScopedKubeconfigCreate,
		ReadContext:   resourceKubernetesScopedKubeconfigRead,
		DeleteContext: resourceKubernetesScopedKubeconfigDelete,

		Importer: &schema.ResourceImporter{
			StateContext: importKubernetesScopedKubeconfig,
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment the kubeconfig gives access to.",
			},
			"expiry_seconds": {
				Type:         schema.TypeInt,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.IntBetween(1, scopedKubeconfigMaxExpiry),
				Description:  "Lifetime of the kubeconfig in seconds, at most 604800 (7 days). Once it has expired, the next plan issues a new one.",
			},
			"namespaces": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Namespaces the kubeconfig can read, within the caller's own access. Leave unset for every namespace the caller can view; the `portainer` and `portainer-addon-*` namespaces are covered only when named.",
			},
			"include_secrets": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Whether the kubeconfig can read Secrets. Granted only where the caller can view secrets, and never in the `portainer` and `portainer-addon-*` namespaces.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Free-text description shown in Portainer's list of scoped kubeconfigs.",
			},
			// Computed attributes
			"scoped_kubeconfig_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Portainer identifier of the scoped kubeconfig.",
			},
			"kubeconfig": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The kubeconfig document. Portainer returns it only when it is issued, so it is empty after an import.",
			},
			"service_account": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes service account the kubeconfig authenticates as.",
			},
			"covered_namespaces": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Namespaces the kubeconfig actually covers, as reported by Portainer.",
			},
			"active": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the kubeconfig still works. Always true in state: one that expired or was revoked is dropped on refresh.",
			},
			"created_at": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Time the kubeconfig was issued, as a Unix timestamp.",
			},
			"expires_at": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Time the kubeconfig stops working, as a Unix timestamp.",
			},
		},
	}
}

func parseScopedKubeconfigID(id string) (int, int, error) {
	parts, err := importIDParts(id, ":", 2, scopedKubeconfigIDFormat)
	if err != nil {
		return 0, 0, err
	}
	envID, err1 := strconv.Atoi(parts[0])
	kcID, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid ID %q: expected %s", id, scopedKubeconfigIDFormat)
	}
	return envID, kcID, nil
}

func scopedKubeconfigURL(client *APIClient, envID, kcID int) string {
	return fmt.Sprintf("%s/kubernetes/%d/scoped_kubeconfig/%d", client.Endpoint, envID, kcID)
}

func resourceKubernetesScopedKubeconfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	payload := map[string]interface{}{
		"expirySeconds":  d.Get("expiry_seconds").(int),
		"includeSecrets": d.Get("include_secrets").(bool),
		"description":    d.Get("description").(string),
	}
	if v, ok := d.GetOk("namespaces"); ok {
		payload["namespaces"] = expandStringList(v.([]interface{}))
	}

	var result struct {
		ID             int      `json:"id"`
		Kubeconfig     string   `json:"kubeconfig"`
		ServiceAccount string   `json:"serviceAccount"`
		Namespaces     []string `json:"namespaces"`
		ExpiresAt      int      `json:"expiresAt"`
	}
	url := fmt.Sprintf("%s/kubernetes/%d/scoped_kubeconfig", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &result); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_scoped_kubeconfig", requiresBE2452)
		return diag.FromErr(fmt.Errorf("failed to issue a scoped kubeconfig: %w", err))
	}
	if result.ID == 0 {
		return diag.Errorf("failed to issue a scoped kubeconfig: Portainer returned no identifier")
	}

	d.SetId(fmt.Sprintf("%d:%d", envID, result.ID))
	// The credential is never returned again, so it is stored before the read.
	if err := setFields(d, map[string]interface{}{
		"scoped_kubeconfig_id": result.ID,
		"kubeconfig":           result.Kubeconfig,
		"service_account":      result.ServiceAccount,
		"covered_namespaces":   result.Namespaces,
		"expires_at":           result.ExpiresAt,
	}); err != nil {
		return diag.FromErr(err)
	}
	return resourceKubernetesScopedKubeconfigRead(ctx, d, meta)
}

// readScopedKubeconfig returns the status of a scoped kubeconfig, or nil when
// Portainer no longer knows it.
func readScopedKubeconfig(ctx context.Context, client *APIClient, envID, kcID int) (*scopedKubeconfigStatus, error) {
	var status scopedKubeconfigStatus
	if err := doJSON(ctx, client, http.MethodGet, scopedKubeconfigURL(client, envID, kcID), nil, &status); err != nil {
		if isAPINotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read scoped kubeconfig %d: %w", kcID, err)
	}
	return &status, nil
}

func resourceKubernetesScopedKubeconfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, kcID, err := parseScopedKubeconfigID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	status, err := readScopedKubeconfig(ctx, client, envID, kcID)
	if err != nil {
		return diag.FromErr(err)
	}
	// An expired or revoked kubeconfig no longer works; dropping it lets the
	// next apply issue a replacement.
	if status == nil || !status.Active {
		log.Printf("[INFO] scoped kubeconfig %d on environment %d is gone or no longer active, removing it from state", kcID, envID)
		d.SetId("")
		return nil
	}

	if err := setFields(d, map[string]interface{}{
		"environment_id":       envID,
		"scoped_kubeconfig_id": status.ID,
		"include_secrets":      status.IncludeSecrets,
		"description":          status.Description,
		"service_account":      status.ServiceAccount,
		"covered_namespaces":   status.Namespaces,
		"active":               status.Active,
		"created_at":           status.CreatedAt,
		"expires_at":           status.ExpiresAt,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// resourceKubernetesScopedKubeconfigDelete revokes the kubeconfig. Portainer
// answers 500 when the credential was revoked but its service account and
// role bindings could not be removed from the cluster: the credential already
// stopped working, so that is reported as a warning, not a failed destroy.
func resourceKubernetesScopedKubeconfigDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, kcID, err := parseScopedKubeconfigID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = doJSON(ctx, client, http.MethodDelete, scopedKubeconfigURL(client, envID, kcID), nil, nil)
	switch {
	case err == nil || isAPINotFound(err):
	case isAPIStatus(err, http.StatusInternalServerError):
		d.SetId("")
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  fmt.Sprintf("Scoped kubeconfig %d was revoked, but its Kubernetes objects could not be removed", kcID),
			Detail:   fmt.Sprintf("The credential no longer works. Its service account and role bindings stay in the cluster until removed by hand: %v", err),
		}}
	default:
		return diag.FromErr(fmt.Errorf("failed to revoke scoped kubeconfig %d: %w", kcID, err))
	}
	d.SetId("")
	return nil
}

// importKubernetesScopedKubeconfig takes "<environment_id>:<scoped_kubeconfig_id>"
// and fills the arguments Read does not refresh from the issued kubeconfig,
// so the plan after an import matches a configuration written to match it.
// The credential itself cannot be recovered.
func importKubernetesScopedKubeconfig(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	envID, kcID, err := parseScopedKubeconfigID(d.Id())
	if err != nil {
		return nil, err
	}
	status, err := readScopedKubeconfig(ctx, meta.(*APIClient), envID, kcID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, fmt.Errorf("scoped kubeconfig %d not found on environment %d", kcID, envID)
	}
	var namespaces []string
	if len(status.Namespaces) > 0 {
		namespaces = status.Namespaces
	}
	if err := setFields(d, map[string]interface{}{
		"environment_id": envID,
		"namespaces":     namespaces,
		"expiry_seconds": status.ExpiresAt - status.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
