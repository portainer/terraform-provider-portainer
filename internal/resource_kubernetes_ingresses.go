package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespaceIngress() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesNamespaceIngressCreate,
		Read:   resourceKubernetesNamespaceIngressRead,
		Update: resourceKubernetesNamespaceIngressUpdate,
		Delete: resourceKubernetesNamespaceIngressDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the Ingress resource is managed.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Kubernetes namespace in which the Ingress is created.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Kubernetes Ingress resource.",
			},
			"class_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of the IngressClass that should handle this Ingress (sets `spec.ingressClassName`).",
			},
			"hosts": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of hostnames associated with the Ingress.",
			},
			"annotations": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Annotations applied to the Ingress resource as key/value strings.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels applied to the Ingress resource as key/value strings.",
			},
			"tls": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "TLS configuration blocks for the Ingress, each pairing a list of hosts with a TLS secret.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hosts": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "List of hostnames covered by the referenced TLS secret.",
						},
						"secret_name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Name of the Kubernetes Secret containing the TLS certificate and key.",
						},
					},
				},
			},
			"paths": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of routing rules mapping host/path combinations to backend Kubernetes services.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"host": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Hostname matched by this routing rule.",
						},
						"path": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "URL path matched by this routing rule.",
						},
						"path_type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Path matching strategy (`Exact`, `Prefix`, or `ImplementationSpecific`).",
						},
						"port": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Port on the backend Service that traffic is forwarded to.",
						},
						"service_name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Name of the Kubernetes Service that receives traffic for this rule.",
						},
					},
				},
			},
		},
	}
}

func resourceKubernetesNamespaceIngressCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	return createOrUpdateIngress(d, client, "POST")
}

func resourceKubernetesNamespaceIngressUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	return createOrUpdateIngress(d, client, "PUT")
}

func createOrUpdateIngress(d *schema.ResourceData, client *APIClient, method string) error {
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("name").(string)

	annotations := map[string]string{}
	if raw, ok := d.GetOk("annotations"); ok {
		for k, v := range raw.(map[string]interface{}) {
			annotations[k] = v.(string)
		}
	}

	labels := map[string]string{}
	if raw, ok := d.GetOk("labels"); ok {
		for k, v := range raw.(map[string]interface{}) {
			labels[k] = v.(string)
		}
	}

	tls := []map[string]interface{}{}
	if raw, ok := d.GetOk("tls"); ok {
		for _, item := range raw.([]interface{}) {
			m := item.(map[string]interface{})
			tls = append(tls, map[string]interface{}{
				"Hosts":      m["hosts"],
				"SecretName": m["secret_name"],
			})
		}
	}

	paths := []map[string]interface{}{}
	if raw, ok := d.GetOk("paths"); ok {
		for _, item := range raw.([]interface{}) {
			m := item.(map[string]interface{})
			paths = append(paths, map[string]interface{}{
				"HasService":  true,
				"Host":        m["host"],
				"IngressName": name,
				"Path":        m["path"],
				"PathType":    m["path_type"],
				"Port":        m["port"],
				"ServiceName": m["service_name"],
			})
		}
	}

	body := map[string]interface{}{
		"Name":        name,
		"Namespace":   namespace,
		"ClassName":   d.Get("class_name").(string),
		"Annotations": annotations,
		"Labels":      labels,
		"Hosts":       d.Get("hosts"),
		"TLS":         tls,
		"Paths":       paths,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresses", client.Endpoint, envID, namespace)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to %s ingress: %s", strings.ToLower(method), string(data))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", envID, namespace, name))
	return nil
}

func resourceKubernetesNamespaceIngressRead(d *schema.ResourceData, meta interface{}) error {
	return nil // No-op
}

func resourceKubernetesNamespaceIngressDelete(d *schema.ResourceData, meta interface{}) error {
	return nil // Not yet supported by API
}
