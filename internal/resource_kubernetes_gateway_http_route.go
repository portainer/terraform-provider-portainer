package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// k8sGatewayHTTPRoute is kubernetes.K8sGatewayHTTPRoute from the Portainer
// 3.0 API. See resource_kubernetes_gateway.go for how the Gateway API
// resources are managed.
type k8sGatewayHTTPRoute struct {
	Name            string                    `json:"Name"`
	Namespace       string                    `json:"Namespace"`
	Hostnames       []string                  `json:"Hostnames"`
	ParentRefs      []k8sGatewayParentRef     `json:"ParentRefs"`
	Rules           []k8sGatewayHTTPRouteRule `json:"Rules"`
	ResourceVersion string                    `json:"ResourceVersion,omitempty"`
}

type k8sGatewayParentRef struct {
	Group       string `json:"Group,omitempty"`
	Name        string `json:"Name"`
	Namespace   string `json:"Namespace,omitempty"`
	Port        int    `json:"Port,omitempty"`
	SectionName string `json:"SectionName,omitempty"`
}

type k8sGatewayHTTPRouteRule struct {
	Name        string                     `json:"Name,omitempty"`
	Matches     []k8sGatewayHTTPRouteMatch `json:"Matches"`
	Filters     []k8sGatewayHTTPFilter     `json:"Filters"`
	BackendRefs []k8sGatewayHTTPBackendRef `json:"BackendRefs"`
}

type k8sGatewayHTTPRouteMatch struct {
	Path        *k8sGatewayTypedMatch  `json:"Path,omitempty"`
	Method      string                 `json:"Method,omitempty"`
	Headers     []k8sGatewayTypedMatch `json:"Headers"`
	QueryParams []k8sGatewayTypedMatch `json:"QueryParams"`
}

// k8sGatewayTypedMatch is K8sGatewayHTTPPathMatch (no Name),
// K8sGatewayHTTPHeaderMatch and K8sGatewayHTTPQueryParamMatch.
type k8sGatewayTypedMatch struct {
	Name  string `json:"Name,omitempty"`
	Type  string `json:"Type,omitempty"`
	Value string `json:"Value"`
}

type k8sGatewayHTTPFilter struct {
	Type                  string                      `json:"Type"`
	RequestHeaderModifier *k8sGatewayHTTPHeaderFilter `json:"RequestHeaderModifier,omitempty"`
	RequestRedirect       *k8sGatewayHTTPRedirect     `json:"RequestRedirect,omitempty"`
}

type k8sGatewayHTTPHeaderFilter struct {
	Add    []k8sGatewayHTTPHeader `json:"Add"`
	Set    []k8sGatewayHTTPHeader `json:"Set"`
	Remove []string               `json:"Remove"`
}

type k8sGatewayHTTPHeader struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type k8sGatewayHTTPRedirect struct {
	Hostname   string `json:"Hostname,omitempty"`
	Port       int    `json:"Port,omitempty"`
	Scheme     string `json:"Scheme,omitempty"`
	StatusCode int    `json:"StatusCode,omitempty"`
}

type k8sGatewayHTTPBackendRef struct {
	BackendRef k8sGatewayBackendRef   `json:"BackendRef"`
	Filters    []k8sGatewayHTTPFilter `json:"Filters"`
}

type k8sGatewayBackendRef struct {
	BackendObjectReference k8sGatewayBackendObjectRef `json:"BackendObjectReference"`
	Weight                 int                        `json:"Weight"`
}

type k8sGatewayBackendObjectRef struct {
	Group     string `json:"Group,omitempty"`
	Kind      string `json:"Kind,omitempty"`
	Name      string `json:"Name"`
	Namespace string `json:"Namespace,omitempty"`
	Port      int    `json:"Port,omitempty"`
}

// gatewayTypedMatchSchema is a header or query parameter match.
func gatewayTypedMatchSchema(what string) *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the " + what + " to match.",
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "How the value is matched, `Exact` or `RegularExpression`. Kubernetes defaults it to `Exact`.",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Value of the " + what + " to match.",
			},
		},
	}
}

// gatewayHeaderSchema is a header name/value pair a filter adds or sets.
func gatewayHeaderSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the header.",
			},
			"value": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Value of the header.",
			},
		},
	}
}

// gatewayHTTPFilterSchema is an HTTPRoute filter, applied to a whole rule or
// to one backend of it.
func gatewayHTTPFilterSchema() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Type of the filter, `RequestHeaderModifier` or `RequestRedirect`.",
			},
			"request_header_modifier": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Headers to change on the request, for `type = \"RequestHeaderModifier\"`.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"add": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        gatewayHeaderSchema(),
							Description: "Headers appended to the request.",
						},
						"set": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        gatewayHeaderSchema(),
							Description: "Headers set on the request, replacing any value it carries.",
						},
						"remove": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Names of headers removed from the request.",
						},
					},
				},
			},
			"request_redirect": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Redirect answered instead of forwarding the request, for `type = \"RequestRedirect\"`.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hostname": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Hostname the request is redirected to. Leave unset to keep the request's.",
						},
						"port": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Port the request is redirected to. Leave unset to keep the request's.",
						},
						"scheme": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Scheme the request is redirected to, `http` or `https`.",
						},
						"status_code": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "HTTP status code of the redirect, `301` or `302`.",
						},
					},
				},
			},
		},
	}
}

func resourceKubernetesGatewayHTTPRoute() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesGatewayHTTPRouteCreate,
		ReadContext:   resourceKubernetesGatewayHTTPRouteRead,
		UpdateContext: resourceKubernetesGatewayHTTPRouteUpdate,
		DeleteContext: resourceKubernetesGatewayHTTPRouteDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the HTTPRoute is managed.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace in which the HTTPRoute is created.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the HTTPRoute.",
			},
			"hostnames": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Hostnames the route matches against the request's Host header. Leave unset to match every hostname the parent listener accepts.",
			},
			"parent_ref": {
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				Description: "Gateways (or listeners of them) the route attaches to.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Name of the parent Gateway.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Namespace of the parent Gateway. Leave unset for the route's own namespace.",
						},
						"section_name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Name of the Gateway listener to attach to. Leave unset to attach to every listener that allows the route.",
						},
						"port": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Port of the Gateway listener to attach to. Leave unset to attach regardless of port.",
						},
						"group": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "API group of the parent. Kubernetes defaults it to `gateway.networking.k8s.io`.",
						},
					},
				},
			},
			"rule": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Routing rules: which requests match and which backends they are forwarded to.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Name of the rule.",
						},
						"match": {
							Type:        schema.TypeList,
							Optional:    true,
							Computed:    true,
							Description: "Conditions a request has to meet for the rule to apply; a request matching any one of them matches. Kubernetes defaults an empty list to a single `PathPrefix` match on `/`.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"path_type": {
										Type:        schema.TypeString,
										Optional:    true,
										Computed:    true,
										Description: "How `path_value` is matched: `Exact`, `PathPrefix` or `RegularExpression`. Kubernetes defaults it to `PathPrefix`.",
									},
									"path_value": {
										Type:        schema.TypeString,
										Optional:    true,
										Computed:    true,
										Description: "Path to match. Kubernetes defaults it to `/`.",
									},
									"method": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "HTTP method to match, for example `GET`. Leave unset to match every method.",
									},
									"header": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        gatewayTypedMatchSchema("header"),
										Description: "Request headers that must match.",
									},
									"query_param": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        gatewayTypedMatchSchema("query parameter"),
										Description: "Query parameters that must match.",
									},
								},
							},
						},
						"filter": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        gatewayHTTPFilterSchema(),
							Description: "Filters applied to every request the rule matches.",
						},
						"backend_ref": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Backends matching requests are forwarded to.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Name of the backend, usually a Service.",
									},
									"namespace": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Namespace of the backend. Leave unset for the route's own namespace; another namespace needs a ReferenceGrant there.",
									},
									"port": {
										Type:        schema.TypeInt,
										Optional:    true,
										Description: "Port of the backend Service traffic is sent to.",
									},
									"kind": {
										Type:        schema.TypeString,
										Optional:    true,
										Computed:    true,
										Description: "Kind of the backend. Kubernetes defaults it to `Service`.",
									},
									"group": {
										Type:        schema.TypeString,
										Optional:    true,
										Computed:    true,
										Description: "API group of the backend. Kubernetes defaults it to the core group (empty).",
									},
									"weight": {
										Type:        schema.TypeInt,
										Optional:    true,
										Default:     1,
										Description: "Share of the rule's traffic this backend receives, relative to the other backends. `0` sends it nothing. Defaults to `1`, the Kubernetes default.",
									},
									"filter": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        gatewayHTTPFilterSchema(),
										Description: "Filters applied only to requests forwarded to this backend.",
									},
								},
							},
						},
					},
				},
			},
			// Computed attributes
			"resource_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes resource version of the HTTPRoute, sent back on update so a concurrent change is rejected instead of overwritten.",
			},
		},
	}
}

func expandGatewayTypedMatches(raw []interface{}) []k8sGatewayTypedMatch {
	out := make([]k8sGatewayTypedMatch, 0, len(raw))
	for _, item := range raw {
		m := item.(map[string]interface{})
		out = append(out, k8sGatewayTypedMatch{
			Name:  m["name"].(string),
			Type:  m["type"].(string),
			Value: m["value"].(string),
		})
	}
	return out
}

func expandGatewayHeaders(raw []interface{}) []k8sGatewayHTTPHeader {
	out := make([]k8sGatewayHTTPHeader, 0, len(raw))
	for _, item := range raw {
		m := item.(map[string]interface{})
		out = append(out, k8sGatewayHTTPHeader{Name: m["name"].(string), Value: m["value"].(string)})
	}
	return out
}

func expandGatewayHTTPFilters(raw []interface{}) []k8sGatewayHTTPFilter {
	out := make([]k8sGatewayHTTPFilter, 0, len(raw))
	for _, item := range raw {
		m := item.(map[string]interface{})
		f := k8sGatewayHTTPFilter{Type: m["type"].(string)}
		if l := m["request_header_modifier"].([]interface{}); len(l) > 0 && l[0] != nil {
			hm := l[0].(map[string]interface{})
			f.RequestHeaderModifier = &k8sGatewayHTTPHeaderFilter{
				Add:    expandGatewayHeaders(hm["add"].([]interface{})),
				Set:    expandGatewayHeaders(hm["set"].([]interface{})),
				Remove: expandStringList(hm["remove"].([]interface{})),
			}
		}
		if l := m["request_redirect"].([]interface{}); len(l) > 0 && l[0] != nil {
			rm := l[0].(map[string]interface{})
			f.RequestRedirect = &k8sGatewayHTTPRedirect{
				Hostname:   rm["hostname"].(string),
				Port:       rm["port"].(int),
				Scheme:     rm["scheme"].(string),
				StatusCode: rm["status_code"].(int),
			}
		}
		out = append(out, f)
	}
	return out
}

func buildKubernetesGatewayHTTPRoute(d *schema.ResourceData) k8sGatewayHTTPRoute {
	route := k8sGatewayHTTPRoute{
		Name:            d.Get("name").(string),
		Namespace:       d.Get("namespace").(string),
		Hostnames:       expandStringList(d.Get("hostnames").([]interface{})),
		ParentRefs:      []k8sGatewayParentRef{},
		Rules:           []k8sGatewayHTTPRouteRule{},
		ResourceVersion: d.Get("resource_version").(string),
	}
	for _, item := range d.Get("parent_ref").([]interface{}) {
		m := item.(map[string]interface{})
		route.ParentRefs = append(route.ParentRefs, k8sGatewayParentRef{
			Group:       m["group"].(string),
			Name:        m["name"].(string),
			Namespace:   m["namespace"].(string),
			Port:        m["port"].(int),
			SectionName: m["section_name"].(string),
		})
	}
	for _, item := range d.Get("rule").([]interface{}) {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rule := k8sGatewayHTTPRouteRule{
			Name:        m["name"].(string),
			Matches:     []k8sGatewayHTTPRouteMatch{},
			Filters:     expandGatewayHTTPFilters(m["filter"].([]interface{})),
			BackendRefs: []k8sGatewayHTTPBackendRef{},
		}
		for _, rawMatch := range m["match"].([]interface{}) {
			// An empty match {} block comes back as nil: it matches
			// everything, which an empty match object says too.
			mm, ok := rawMatch.(map[string]interface{})
			if !ok {
				rule.Matches = append(rule.Matches, k8sGatewayHTTPRouteMatch{})
				continue
			}
			match := k8sGatewayHTTPRouteMatch{
				Method:      mm["method"].(string),
				Headers:     expandGatewayTypedMatches(mm["header"].([]interface{})),
				QueryParams: expandGatewayTypedMatches(mm["query_param"].([]interface{})),
			}
			if pt, pv := mm["path_type"].(string), mm["path_value"].(string); pt != "" || pv != "" {
				match.Path = &k8sGatewayTypedMatch{Type: pt, Value: pv}
			}
			rule.Matches = append(rule.Matches, match)
		}
		for _, rawBackend := range m["backend_ref"].([]interface{}) {
			bm, ok := rawBackend.(map[string]interface{})
			if !ok {
				continue
			}
			backend := k8sGatewayHTTPBackendRef{
				BackendRef: k8sGatewayBackendRef{
					BackendObjectReference: k8sGatewayBackendObjectRef{
						Group:     bm["group"].(string),
						Kind:      bm["kind"].(string),
						Name:      bm["name"].(string),
						Namespace: bm["namespace"].(string),
						Port:      bm["port"].(int),
					},
					// Always sent: zero is a valid weight (the backend
					// receives nothing), so it cannot stand for unset.
					Weight: bm["weight"].(int),
				},
				Filters: expandGatewayHTTPFilters(bm["filter"].([]interface{})),
			}
			rule.BackendRefs = append(rule.BackendRefs, backend)
		}
		route.Rules = append(route.Rules, rule)
	}
	return route
}

func flattenGatewayTypedMatches(matches []k8sGatewayTypedMatch) []interface{} {
	out := make([]interface{}, 0, len(matches))
	for _, m := range matches {
		out = append(out, map[string]interface{}{"name": m.Name, "type": m.Type, "value": m.Value})
	}
	return out
}

func flattenGatewayHeaders(headers []k8sGatewayHTTPHeader) []interface{} {
	out := make([]interface{}, 0, len(headers))
	for _, h := range headers {
		out = append(out, map[string]interface{}{"name": h.Name, "value": h.Value})
	}
	return out
}

func flattenGatewayHTTPFilters(filters []k8sGatewayHTTPFilter) []interface{} {
	out := make([]interface{}, 0, len(filters))
	for _, f := range filters {
		m := map[string]interface{}{
			"type":                    f.Type,
			"request_header_modifier": []interface{}{},
			"request_redirect":        []interface{}{},
		}
		if hm := f.RequestHeaderModifier; hm != nil {
			m["request_header_modifier"] = []interface{}{map[string]interface{}{
				"add":    flattenGatewayHeaders(hm.Add),
				"set":    flattenGatewayHeaders(hm.Set),
				"remove": hm.Remove,
			}}
		}
		if rr := f.RequestRedirect; rr != nil {
			m["request_redirect"] = []interface{}{map[string]interface{}{
				"hostname":    rr.Hostname,
				"port":        rr.Port,
				"scheme":      rr.Scheme,
				"status_code": rr.StatusCode,
			}}
		}
		out = append(out, m)
	}
	return out
}

func flattenGatewayHTTPRouteRules(rules []k8sGatewayHTTPRouteRule) []interface{} {
	out := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		matches := make([]interface{}, 0, len(r.Matches))
		for _, m := range r.Matches {
			pathType, pathValue := "", ""
			if m.Path != nil {
				pathType, pathValue = m.Path.Type, m.Path.Value
			}
			matches = append(matches, map[string]interface{}{
				"path_type":   pathType,
				"path_value":  pathValue,
				"method":      m.Method,
				"header":      flattenGatewayTypedMatches(m.Headers),
				"query_param": flattenGatewayTypedMatches(m.QueryParams),
			})
		}
		backends := make([]interface{}, 0, len(r.BackendRefs))
		for _, b := range r.BackendRefs {
			ref := b.BackendRef.BackendObjectReference
			backends = append(backends, map[string]interface{}{
				"name":      ref.Name,
				"namespace": ref.Namespace,
				"port":      ref.Port,
				"kind":      ref.Kind,
				"group":     ref.Group,
				"weight":    b.BackendRef.Weight,
				"filter":    flattenGatewayHTTPFilters(b.Filters),
			})
		}
		out = append(out, map[string]interface{}{
			"name":        r.Name,
			"match":       matches,
			"filter":      flattenGatewayHTTPFilters(r.Filters),
			"backend_ref": backends,
		})
	}
	return out
}

func gatewayHTTPRoutesURL(client *APIClient, envID int, namespace string) string {
	return fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/gatewayhttproutes", client.Endpoint, envID, namespace)
}

func resourceKubernetesGatewayHTTPRouteCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	route := buildKubernetesGatewayHTTPRoute(d)
	route.ResourceVersion = ""

	if err := doJSON(ctx, client, http.MethodPost, gatewayHTTPRoutesURL(client, envID, route.Namespace), route, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_gateway_http_route", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to create HTTPRoute %s/%s: %w", route.Namespace, route.Name, err))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", envID, route.Namespace, route.Name))
	return resourceKubernetesGatewayHTTPRouteRead(ctx, d, meta)
}

func resourceKubernetesGatewayHTTPRouteRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, namespace, name, err := parseGatewayAPIID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var routes []k8sGatewayHTTPRoute
	if err := doJSON(ctx, client, http.MethodGet, gatewayHTTPRoutesURL(client, envID, namespace), nil, &routes); err != nil {
		// The environment itself is gone.
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read HTTPRoute %s/%s: %w", namespace, name, err))
	}

	var route *k8sGatewayHTTPRoute
	for i := range routes {
		if routes[i].Name == name {
			route = &routes[i]
			break
		}
	}
	if route == nil {
		d.SetId("")
		return nil
	}

	parents := make([]interface{}, 0, len(route.ParentRefs))
	for _, p := range route.ParentRefs {
		parents = append(parents, map[string]interface{}{
			"name":         p.Name,
			"namespace":    p.Namespace,
			"section_name": p.SectionName,
			"port":         p.Port,
			"group":        p.Group,
		})
	}

	if err := setFields(d, map[string]interface{}{
		"environment_id":   envID,
		"namespace":        namespace,
		"name":             name,
		"hostnames":        route.Hostnames,
		"parent_ref":       parents,
		"rule":             flattenGatewayHTTPRouteRules(route.Rules),
		"resource_version": route.ResourceVersion,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceKubernetesGatewayHTTPRouteUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, namespace, name, err := parseGatewayAPIID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := doJSON(ctx, client, http.MethodPut, gatewayHTTPRoutesURL(client, envID, namespace), buildKubernetesGatewayHTTPRoute(d), nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update HTTPRoute %s/%s: %w", namespace, name, err))
	}
	return resourceKubernetesGatewayHTTPRouteRead(ctx, d, meta)
}

// resourceKubernetesGatewayHTTPRouteDelete removes the route through the bulk
// delete, which takes the names to delete per namespace.
func resourceKubernetesGatewayHTTPRouteDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, namespace, name, err := parseGatewayAPIID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	body := map[string][]string{namespace: {name}}
	url := fmt.Sprintf("%s/kubernetes/%d/gatewayhttproutes/delete", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPost, url, body, nil); err != nil && !isAPINotFound(err) {
		return diag.FromErr(fmt.Errorf("failed to delete HTTPRoute %s/%s: %w", namespace, name, err))
	}
	d.SetId("")
	return nil
}
