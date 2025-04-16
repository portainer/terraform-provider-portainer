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
				Type:     schema.TypeInt,
				Required: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"class_name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"hosts": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"annotations": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"tls": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hosts": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"secret_name": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"paths": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"host": {
							Type:     schema.TypeString,
							Required: true,
						},
						"path": {
							Type:     schema.TypeString,
							Required: true,
						},
						"path_type": {
							Type:     schema.TypeString,
							Required: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"service_name": {
							Type:     schema.TypeString,
							Required: true,
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
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
