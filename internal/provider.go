package internal

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Provider defines the Portainer Terraform provider schema and resources.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PORTAINER_ENDPOINT", nil),
				Description: "URL of the Portainer instance (e.g. https://portainer.example.com). '/api' will be appended automatically if missing.",
			},
			"api_key": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("PORTAINER_API_KEY", nil),
				Description: "API key to authenticate with Portainer. Only API keys are supported (not JWT tokens).",
			},
			"skip_ssl_verify": {
				Type:        schema.TypeBool,
				Required:    false,
				Optional:    true,
				Sensitive:   false,
				DefaultFunc: schema.EnvDefaultFunc("PORTAINER_SKIP_SSL_VERIFY", false),
				Description: "Verify the SSL/TLS certificate for the Portainer endpoint",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"portainer_user":                                    resourceUser(),
			"portainer_team":                                    resourceTeam(),
			"portainer_environment":                             resourceEnvironment(),
			"portainer_endpoint_group":                          resourceEndpointGroup(),
			"portainer_tag":                                     resourceTag(),
			"portainer_registry":                                resourceRegistry(),
			"portainer_backup":                                  resourceBackup(),
			"portainer_backup_s3":                               resourceBackupS3(),
			"portainer_edge_group":                              resourceEdgeGroup(),
			"portainer_edge_job":                                resourceEdgeJob(),
			"portainer_auth":                                    resourceAuth(),
			"portainer_edge_stack":                              resourceEdgeStack(),
			"portainer_custom_template":                         resourceCustomTemplate(),
			"portainer_stack":                                   resourcePortainerStack(),
			"portainer_container_exec":                          resourceContainerExec(),
			"portainer_docker_network":                          resourceDockerNetwork(),
			"portainer_docker_image":                            resourceDockerImage(),
			"portainer_docker_volume":                           resourceDockerVolume(),
			"portainer_open_amt":                                resourceOpenAMT(),
			"portainer_settings":                                resourceSettings(),
			"portainer_ssl":                                     resourceSSLSettings(),
			"portainer_team_membership":                         resourceTeamMembership(),
			"portainer_webhook":                                 resourceWebhook(),
			"portainer_webhook_execute":                         resourceWebhookExecute(),
			"portainer_licenses":                                resourceLicenses(),
			"portainer_cloud_credentials":                       resourceCloudCredentials(),
			"portainer_endpoint_settings":                       resourceEndpointSettings(),
			"portainer_endpoint_snapshot":                       resourceEndpointsSnapshot(),
			"portainer_endpoint_association":                    resourceEndpointAssociation(),
			"portainer_endpoint_service_update":                 resourceEndpointServiceUpdate(),
			"portainer_kubernetes_namespace":                    resourceKubernetesNamespace(),
			"portainer_kubernetes_helm":                         resourceKubernetesHelm(),
			"portainer_kubernetes_ingresscontrollers":           resourceKubernetesIngressControllers(),
			"portainer_kubernetes_namespace_ingresscontrollers": resourceKubernetesNamespaceIngressControllers(),
			"portainer_kubernetes_ingresses":                    resourceKubernetesNamespaceIngress(),
			"portainer_kubernetes_application":                  resourceKubernetesApplication(),
			"portainer_kubernetes_namespace_system":             resourceKubernetesNamespaceSystem(),
			"portainer_kubernetes_delete_object":                resourceKubernetesDeleteObject(),
			"portainer_resource_control":                        resourceResourceControl(),
			"portainer_docker_secret":                           resourceDockerSecret(),
			"portainer_docker_config":                           resourceDockerConfig(),
			"portainer_kubernetes_cronjob":                      resourceKubernetesCronJob(),
			"portainer_kubernetes_job":                          resourceKubernetesJob(),
			"portainer_kubernetes_serviceaccounts":              resourceKubernetesServiceAccounts(),
			"portainer_kubernetes_configmaps":                   resourceKubernetesConfigMaps(),
			"portainer_kubernetes_secret":                       resourceKubernetesSecrets(),
			"portainer_kubernetes_service":                      resourceKubernetesService(),
			"portainer_kubernetes_role":                         resourceKubernetesRoles(),
			"portainer_kubernetes_rolebinding":                  resourceKubernetesRoleBindings(),
			"portainer_kubernetes_clusterrole":                  resourceKubernetesClusterRoles(),
			"portainer_kubernetes_clusterrolebinding":           resourceKubernetesClusterRoleBindings(),
			"portainer_kubernetes_volume":                       resourceKubernetesVolumes(),
			"portainer_kubernetes_storage":                      resourceKubernetesStorage(),
		},
		ConfigureContextFunc: configureProvider,
	}
}

// APIClient is a simple client struct to store connection information.
type APIClient struct {
	Endpoint   string
	APIKey     string
	HTTPClient http.Client
}

// DoRequest is a reusable method for making API requests
func (c *APIClient) DoRequest(method, path string, headers map[string]string, body interface{}) (*http.Response, error) {
	var buf io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.Endpoint, path), buf)
	if err != nil {
		return nil, err
	}

	if _, ok := headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.HTTPClient.Do(req)
}

func (c *APIClient) DoMultipartRequest(method, url string, body *bytes.Buffer, headers map[string]string, out interface{}) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, data)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// configureProvider sets up the API client and appends '/api' if missing from the endpoint.
func configureProvider(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	endpoint := d.Get("endpoint").(string)
	apiKey := d.Get("api_key").(string)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: d.Get("skip_ssl_verify").(bool),
		},
	}
	http_client := &http.Client{
		Transport: transport,
	}

	if !strings.HasSuffix(endpoint, "/api") {
		endpoint = strings.TrimRight(endpoint, "/") + "/api"
	}

	client := &APIClient{
		Endpoint:   endpoint,
		APIKey:     apiKey,
		HTTPClient: *http_client,
	}

	var diags diag.Diagnostics
	return client, diags
}
