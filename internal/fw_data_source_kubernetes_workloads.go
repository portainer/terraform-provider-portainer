package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesCronJobsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_cron_jobs", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id":    k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"include_system": k8sDSOptionalBoolDefault("Whether to include Kubernetes' own system cron jobs. They are filtered out by default because they are not something a configuration acts on."),
			// Computed attributes
			"cron_jobs": k8sDSObjectList("The cron jobs in the cluster.",
				k8sDSField{"id", types.StringType, "Kubernetes UID of the cron job."},
				k8sDSField{"name", types.StringType, "Name of the cron job."},
				k8sDSField{"namespace", types.StringType, "Namespace of the cron job."},
				k8sDSField{"schedule", types.StringType, "Cron expression the job runs on."},
				k8sDSField{"timezone", types.StringType, "Timezone the schedule is evaluated in, empty when it follows the cluster's."},
				k8sDSField{"suspend", types.BoolType, "Whether the cron job is suspended. A suspended job does not fire."},
				k8sDSField{"command", types.StringType, "Command the job runs."},
				k8sDSField{"is_system", types.BoolType, "Whether this is one of Kubernetes' own system cron jobs."},
			),
		},
	}, map[string]interface{}{"include_system": false}, kubernetesCronJobsRead)
}

func kubernetesCronJobsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to list the cron jobs of environment %d: %w", endpointID, err)
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

	if err := d.SetFields(map[string]interface{}{"cron_jobs": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-cron-jobs-%d", endpointID))
	return nil
}

func newKubernetesEndpointsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_endpoints", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			// Computed attributes
			"endpoints": k8sDSObjectList("The Kubernetes Endpoints objects in the cluster - the addresses behind each service.",
				k8sDSField{"uid", types.StringType, "Kubernetes UID of the Endpoints object."},
				k8sDSField{"name", types.StringType, "Name of the Endpoints object, which matches the service it backs."},
				k8sDSField{"namespace", types.StringType, "Namespace of the Endpoints object."},
				k8sDSField{"addresses", k8sDSStringList, "Addresses behind the service. An empty list means the service has no ready backends."},
				k8sDSField{"ports", types.ListType{ElemType: k8sDSObject(
					k8sDSField{"name", types.StringType, "Name of the port."},
					k8sDSField{"port", types.Int64Type, "Port number."},
					k8sDSField{"protocol", types.StringType, "Protocol of the port, for example `TCP`."},
				)}, "Ports the endpoints expose."},
			),
		},
	}, nil, kubernetesEndpointsRead)
}

func kubernetesEndpointsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to list the Kubernetes endpoints of environment %d: %w", endpointID, err)
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

	if err := d.SetFields(map[string]interface{}{"endpoints": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-endpoints-%d", endpointID))
	return nil
}

func newKubernetesServiceAccountDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_service_account", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"namespace":   k8sDSRequiredString("Namespace of the service account."),
			"name":        k8sDSRequiredString("Name of the service account to inspect."),
			// Computed attributes
			"uid":                             k8sDSComputedString("Kubernetes UID of the service account."),
			"is_system":                       k8sDSComputedBool("Whether this is one of Kubernetes' own system service accounts."),
			"automount_service_account_token": k8sDSComputedBool("Whether pods using the account get its token mounted automatically."),
			"image_pull_secrets":              k8sDSComputedStringList("Image pull secrets attached to the account."),
			"labels":                          k8sDSComputedStringMap("Labels on the service account."),
			"annotations":                     k8sDSComputedStringMap("Annotations on the service account."),
			"creation_date":                   k8sDSComputedString("When the service account was created."),
		},
	}, nil, kubernetesServiceAccountRead)
}

func kubernetesServiceAccountRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to read service account %s/%s: %w", namespace, name, err)
	}

	if err := d.SetFields(map[string]interface{}{
		"uid":                             account.UID,
		"is_system":                       account.IsSystem,
		"automount_service_account_token": account.AutomountServiceAccountToken,
		"image_pull_secrets":              account.ImagePullSecrets,
		"labels":                          stringMap(account.Labels),
		"annotations":                     stringMap(account.Annotations),
		"creation_date":                   account.CreationDate,
	}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-service-account-%d-%s-%s", endpointID, namespace, name))
	return nil
}

func newKubernetesApplicationDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_application", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id":   k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"namespace":     k8sDSRequiredString("Namespace of the application."),
			"name":          k8sDSRequiredString("Name of the application to inspect."),
			"resource_type": k8sDSOptionalString("Kind to look the application up as, for example `Deployment`. Leave unset to let Portainer work it out."),
			// Computed attributes
			"uid":               k8sDSComputedString("Kubernetes UID of the application."),
			"kind":              k8sDSComputedString("Kind of the workload, for example `Deployment` or `StatefulSet`."),
			"image":             k8sDSComputedString("Container image the application runs."),
			"status":            k8sDSComputedString("Status Portainer reports for the application."),
			"running_pods":      k8sDSComputedInt("Pods currently running."),
			"total_pods":        k8sDSComputedInt("Pods the application asks for. A gap between this and `running_pods` is what to watch."),
			"application_type":  k8sDSComputedString("How Portainer classifies the application."),
			"application_owner": k8sDSComputedString("Who Portainer records as the application's owner."),
			"deployment_type":   k8sDSComputedString("How the application was deployed."),
			"stack_id":          k8sDSComputedString("Stack the application belongs to, empty when it is not part of one."),
			"stack_name":        k8sDSComputedString("Name of that stack."),
			"service_name":      k8sDSComputedString("Service fronting the application, empty when it has none."),
			"service_type":      k8sDSComputedString("Type of that service, for example `ClusterIP` or `LoadBalancer`."),
			"load_balancer_ip":  k8sDSComputedString("External address of the load balancer, empty until one is assigned."),
			"labels":            k8sDSComputedStringMap("Labels on the application."),
			"creation_date":     k8sDSComputedString("When the application was created."),
			// Pods, containers, published ports and resource figures nest
			// several levels deep and differ by workload kind; the fields
			// above are the ones worth acting on, and this carries the rest.
			"details": k8sDSComputedString("The whole response as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for the pod, container and resource detail the attributes above do not cover."),
		},
	}, nil, kubernetesApplicationRead)
}

func kubernetesApplicationRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to read application %s/%s: %w", namespace, name, err)
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
		return fmt.Errorf("failed to parse application %s/%s: %w", namespace, name, err)
	}

	if err := d.SetFields(map[string]interface{}{
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
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-application-%d-%s-%s", endpointID, namespace, name))
	return nil
}

// newKubernetesResourceCountsDataSource folds Portainer's seven count
// endpoints into one data source. They are separate calls but a single
// question - how much is in this cluster - and pulling them apart would mean
// seven data sources that are each one number.
func newKubernetesResourceCountsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_resource_counts", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			// Computed attributes
			"applications": k8sDSComputedInt("Applications in the cluster."),
			"namespaces":   k8sDSComputedInt("Namespaces in the cluster."),
			"services":     k8sDSComputedInt("Services in the cluster."),
			"ingresses":    k8sDSComputedInt("Ingresses in the cluster."),
			"config_maps":  k8sDSComputedInt("ConfigMaps in the cluster."),
			"secrets":      k8sDSComputedInt("Secrets in the cluster."),
			"volumes":      k8sDSComputedInt("Volumes in the cluster."),
		},
	}, nil, kubernetesResourceCountsRead)
}

func kubernetesResourceCountsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
			return fmt.Errorf("failed to count the %s of environment %d: %w", count.path, endpointID, err)
		}
		fields[count.attribute] = value
	}

	if err := d.SetFields(fields); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-resource-counts-%d", endpointID))
	return nil
}
