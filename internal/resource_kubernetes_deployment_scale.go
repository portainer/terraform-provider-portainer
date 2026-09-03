package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesDeploymentScale() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesDeploymentScaleCreate,
		ReadContext:   resourceKubernetesDeploymentScaleRead,
		UpdateContext: resourceKubernetesDeploymentScaleCreate,
		// Scaling has no inverse: dropping the resource stops managing the
		// replica count and leaves the deployment running at its current scale.
		// Scale to 0 explicitly to stop the workload.
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment the deployment belongs to.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Namespace the deployment lives in.",
			},
			"deployment_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the deployment to scale. The deployment must already exist; this resource manages only its replica count.",
			},
			"replicas": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Desired number of replicas. Changing this scales the deployment in place.",
			},
			// Computed attributes
			"ready_replicas": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of replicas passing their readiness checks, as reported after the scale request.",
			},
			"available_replicas": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of replicas available for at least the configured minimum ready seconds, as reported after the scale request.",
			},
		},
	}
}

// k8sDeployment is the subset of the appsv1.Deployment response the scale
// resource reads back.
type k8sDeployment struct {
	Spec struct {
		Replicas *int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

func resourceKubernetesDeploymentScaleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("deployment_name").(string)
	replicas := d.Get("replicas").(int)

	var deployment k8sDeployment
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/deployments/%s/scale", client.Endpoint, envID, namespace, name)
	if err := doJSON(ctx, client, http.MethodPut, url, map[string]interface{}{"replicas": replicas}, &deployment); err != nil {
		return diag.FromErr(fmt.Errorf("failed to scale deployment %q in namespace %q to %d replicas: %w", name, namespace, replicas, err))
	}

	if err := setFields(d, map[string]interface{}{
		"ready_replicas":     deployment.Status.ReadyReplicas,
		"available_replicas": deployment.Status.AvailableReplicas,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/%s/scale", envID, namespace, name))
	return nil
}

func resourceKubernetesDeploymentScaleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("deployment_name").(string)

	var deployment k8sDeployment
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/deployments/%s", client.Endpoint, envID, namespace, name)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &deployment); err != nil {
		// A deleted deployment has no replica count to manage any more, so the
		// resource drops out of state instead of failing every plan.
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read deployment %q in namespace %q: %w", name, namespace, err))
	}

	// spec.replicas is a pointer in the Kubernetes API: absent means the default
	// of 1, which is not the same as an explicit 0.
	replicas := 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	if err := setFields(d, map[string]interface{}{
		"replicas":           replicas,
		"ready_replicas":     deployment.Status.ReadyReplicas,
		"available_replicas": deployment.Status.AvailableReplicas,
	}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
