package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesCronJobs() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesCronJobsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"include_system": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to include Kubernetes' own system cron jobs. They are filtered out by default because they are not something a configuration acts on.",
			},
			// Computed attributes
			"cron_jobs": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The cron jobs in the cluster.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes UID of the cron job.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the cron job.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the cron job.",
						},
						"schedule": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Cron expression the job runs on.",
						},
						"timezone": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Timezone the schedule is evaluated in, empty when it follows the cluster's.",
						},
						"suspend": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the cron job is suspended. A suspended job does not fire.",
						},
						"command": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Command the job runs.",
						},
						"is_system": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether this is one of Kubernetes' own system cron jobs.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesCronJobsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	includeSystem := d.Get("include_system").(bool)

	var jobs []struct {
		ID        string `json:"Id"`
		Name      string `json:"Name"`
		Namespace string `json:"Namespace"`
		Schedule  string `json:"Schedule"`
		Timezone  string `json:"Timezone"`
		Suspend   bool   `json:"Suspend"`
		Command   string `json:"Command"`
		IsSystem  bool   `json:"IsSystem"`
	}
	listURL := fmt.Sprintf("%s/kubernetes/%d/cron_jobs", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &jobs); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the cron jobs of environment %d: %w", endpointID, err))
	}

	entries := make([]interface{}, 0, len(jobs))
	for _, job := range jobs {
		if job.IsSystem && !includeSystem {
			continue
		}
		entries = append(entries, map[string]interface{}{
			"id": job.ID, "name": job.Name, "namespace": job.Namespace,
			"schedule": job.Schedule, "timezone": job.Timezone,
			"suspend": job.Suspend, "command": job.Command, "is_system": job.IsSystem,
		})
	}

	if err := setFields(d, map[string]interface{}{"cron_jobs": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-cron-jobs-%d", endpointID))
	return nil
}

func dataSourceKubernetesEndpoints() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesEndpointsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"endpoints": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The Kubernetes Endpoints objects in the cluster - the addresses behind each service.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"uid": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes UID of the Endpoints object.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the Endpoints object, which matches the service it backs.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the Endpoints object.",
						},
						"addresses": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Addresses behind the service. An empty list means the service has no ready backends.",
						},
						"ports": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Ports the endpoints expose.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Name of the port.",
									},
									"port": {
										Type:        schema.TypeInt,
										Computed:    true,
										Description: "Port number.",
									},
									"protocol": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Protocol of the port, for example `TCP`.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesEndpointsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	var endpoints []struct {
		UID       string   `json:"uid"`
		Name      string   `json:"name"`
		Namespace string   `json:"namespace"`
		Addresses []string `json:"addresses"`
		Ports     []struct {
			Name     string `json:"name"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
	}
	listURL := fmt.Sprintf("%s/kubernetes/%d/endpoints", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &endpoints); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the Kubernetes endpoints of environment %d: %w", endpointID, err))
	}

	entries := make([]interface{}, 0, len(endpoints))
	for _, endpoint := range endpoints {
		ports := make([]interface{}, 0, len(endpoint.Ports))
		for _, port := range endpoint.Ports {
			ports = append(ports, map[string]interface{}{
				"name": port.Name, "port": port.Port, "protocol": port.Protocol,
			})
		}
		entries = append(entries, map[string]interface{}{
			"uid": endpoint.UID, "name": endpoint.Name, "namespace": endpoint.Namespace,
			"addresses": endpoint.Addresses, "ports": ports,
		})
	}

	if err := setFields(d, map[string]interface{}{"endpoints": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-endpoints-%d", endpointID))
	return nil
}

func dataSourceKubernetesServiceAccount() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesServiceAccountRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace of the service account.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the service account to inspect.",
			},
			// Computed attributes
			"uid": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes UID of the service account.",
			},
			"is_system": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether this is one of Kubernetes' own system service accounts.",
			},
			"automount_service_account_token": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether pods using the account get its token mounted automatically.",
			},
			"image_pull_secrets": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Image pull secrets attached to the account.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels on the service account.",
			},
			"annotations": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Annotations on the service account.",
			},
			"creation_date": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "When the service account was created.",
			},
		},
	}
}

func dataSourceKubernetesServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("name").(string)

	var account struct {
		UID                          string            `json:"uid"`
		IsSystem                     bool              `json:"isSystem"`
		AutomountServiceAccountToken bool              `json:"automountServiceAccountToken"`
		ImagePullSecrets             []string          `json:"imagePullSecrets"`
		Labels                       map[string]string `json:"labels"`
		Annotations                  map[string]string `json:"annotations"`
		CreationDate                 string            `json:"creationDate"`
	}
	accountURL := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/service_accounts/%s",
		client.Endpoint, endpointID, url.PathEscape(namespace), url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, accountURL, nil, &account); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read service account %s/%s: %w", namespace, name, err))
	}

	if err := setFields(d, map[string]interface{}{
		"uid":                             account.UID,
		"is_system":                       account.IsSystem,
		"automount_service_account_token": account.AutomountServiceAccountToken,
		"image_pull_secrets":              account.ImagePullSecrets,
		"labels":                          stringMap(account.Labels),
		"annotations":                     stringMap(account.Annotations),
		"creation_date":                   account.CreationDate,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-service-account-%d-%s-%s", endpointID, namespace, name))
	return nil
}

func dataSourceKubernetesApplication() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesApplicationRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace of the application.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the application to inspect.",
			},
			"resource_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kind to look the application up as, for example `Deployment`. Leave unset to let Portainer work it out.",
			},
			// Computed attributes
			"uid": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes UID of the application.",
			},
			"kind": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kind of the workload, for example `Deployment` or `StatefulSet`.",
			},
			"image": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Container image the application runs.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status Portainer reports for the application.",
			},
			"running_pods": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Pods currently running.",
			},
			"total_pods": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Pods the application asks for. A gap between this and `running_pods` is what to watch.",
			},
			"application_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "How Portainer classifies the application.",
			},
			"application_owner": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Who Portainer records as the application's owner.",
			},
			"deployment_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "How the application was deployed.",
			},
			"stack_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Stack the application belongs to, empty when it is not part of one.",
			},
			"stack_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of that stack.",
			},
			"service_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Service fronting the application, empty when it has none.",
			},
			"service_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Type of that service, for example `ClusterIP` or `LoadBalancer`.",
			},
			"load_balancer_ip": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "External address of the load balancer, empty until one is assigned.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels on the application.",
			},
			"creation_date": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "When the application was created.",
			},
			"details": {
				Type:     schema.TypeString,
				Computed: true,
				// Pods, containers, published ports and resource figures nest
				// several levels deep and differ by workload kind; the fields
				// above are the ones worth acting on, and this carries the rest.
				Description: "The whole response as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for the pod, container and resource detail the attributes above do not cover.",
			},
		},
	}
}

func dataSourceKubernetesApplicationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("name").(string)

	applicationURL := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/applications/%s",
		client.Endpoint, endpointID, url.PathEscape(namespace), url.PathEscape(name))
	if v, ok := d.GetOk("resource_type"); ok && v.(string) != "" {
		applicationURL += "?resourceType=" + url.QueryEscape(v.(string))
	}

	body, err := apiGETRaw(ctx, client, applicationURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read application %s/%s: %w", namespace, name, err))
	}

	var application struct {
		UID                   string            `json:"Uid"`
		Kind                  string            `json:"Kind"`
		Image                 string            `json:"Image"`
		Status                string            `json:"Status"`
		RunningPodsCount      int               `json:"RunningPodsCount"`
		TotalPodsCount        int               `json:"TotalPodsCount"`
		ApplicationType       string            `json:"ApplicationType"`
		ApplicationOwner      string            `json:"ApplicationOwner"`
		DeploymentType        string            `json:"DeploymentType"`
		StackID               string            `json:"StackId"`
		StackName             string            `json:"StackName"`
		ServiceName           string            `json:"ServiceName"`
		ServiceType           string            `json:"ServiceType"`
		LoadBalancerIPAddress string            `json:"LoadBalancerIPAddress"`
		Labels                map[string]string `json:"Labels"`
		CreationDate          string            `json:"CreationDate"`
	}
	if err := json.Unmarshal(body, &application); err != nil {
		return diag.FromErr(fmt.Errorf("failed to parse application %s/%s: %w", namespace, name, err))
	}

	if err := setFields(d, map[string]interface{}{
		"uid":               application.UID,
		"kind":              application.Kind,
		"image":             application.Image,
		"status":            application.Status,
		"running_pods":      application.RunningPodsCount,
		"total_pods":        application.TotalPodsCount,
		"application_type":  application.ApplicationType,
		"application_owner": application.ApplicationOwner,
		"deployment_type":   application.DeploymentType,
		"stack_id":          application.StackID,
		"stack_name":        application.StackName,
		"service_name":      application.ServiceName,
		"service_type":      application.ServiceType,
		"load_balancer_ip":  application.LoadBalancerIPAddress,
		"labels":            stringMap(application.Labels),
		"creation_date":     application.CreationDate,
		"details":           string(body),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-application-%d-%s-%s", endpointID, namespace, name))
	return nil
}

// dataSourceKubernetesResourceCounts folds Portainer's seven count endpoints
// into one data source. They are separate calls but a single question - how
// much is in this cluster - and pulling them apart would mean seven data
// sources that are each one number.
func dataSourceKubernetesResourceCounts() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesResourceCountsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"applications": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Applications in the cluster.",
			},
			"namespaces": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Namespaces in the cluster.",
			},
			"services": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Services in the cluster.",
			},
			"ingresses": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Ingresses in the cluster.",
			},
			"config_maps": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ConfigMaps in the cluster.",
			},
			"secrets": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Secrets in the cluster.",
			},
			"volumes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Volumes in the cluster.",
			},
		},
	}
}

func dataSourceKubernetesResourceCountsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	// Each count is its own endpoint answering with a bare number. They are
	// read in a fixed order so a failure names the same resource every time.
	counts := []struct {
		attribute string
		path      string
	}{
		{"applications", "applications"},
		{"namespaces", "namespaces"},
		{"services", "services"},
		{"ingresses", "ingresses"},
		{"config_maps", "configmaps"},
		{"secrets", "secrets"},
		{"volumes", "volumes"},
	}

	fields := make(map[string]interface{}, len(counts))
	for _, count := range counts {
		var value int
		countURL := fmt.Sprintf("%s/kubernetes/%d/%s/count", client.Endpoint, endpointID, count.path)
		if err := doJSON(ctx, client, http.MethodGet, countURL, nil, &value); err != nil {
			return diag.FromErr(fmt.Errorf("failed to count the %s of environment %d: %w", count.path, endpointID, err))
		}
		fields[count.attribute] = value
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-resource-counts-%d", endpointID))
	return nil
}
