package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// k8sGatewayHTTPRoute is kubernetes.K8sGatewayHTTPRoute from the Portainer
// 3.0 API. See fw_resource_kubernetes_gateway.go for how the Gateway API
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

var (
	_ resource.Resource                = (*kubernetesGatewayHTTPRouteResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesGatewayHTTPRouteResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesGatewayHTTPRouteResource)(nil)
)

func newKubernetesGatewayHTTPRouteResource() resource.Resource {
	return &kubernetesGatewayHTTPRouteResource{}
}

type kubernetesGatewayHTTPRouteResource struct{ k8sfwBase }

func (r *kubernetesGatewayHTTPRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_gateway_http_route"
}

// gatewayTypedMatchBlock is a header or query parameter match.
func gatewayTypedMatchBlock(what, description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					Required:    true,
					Description: "Name of the " + what + " to match.",
				},
				"type": schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Description: "How the value is matched, `Exact` or `RegularExpression`. Kubernetes defaults it to `Exact`.",
				},
				"value": schema.StringAttribute{
					Required:    true,
					Description: "Value of the " + what + " to match.",
				},
			},
		},
	}
}

// gatewayHeaderBlock is a header name/value pair a filter adds or sets.
func gatewayHeaderBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name":  schema.StringAttribute{Required: true, Description: "Name of the header."},
				"value": schema.StringAttribute{Required: true, Description: "Value of the header."},
			},
		},
	}
}

// gatewayHTTPFilterBlock is an HTTPRoute filter, applied to a whole rule or
// to one backend of it.
func gatewayHTTPFilterBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{
					Required:    true,
					Description: "Type of the filter, `RequestHeaderModifier` or `RequestRedirect`.",
				},
			},
			Blocks: map[string]schema.Block{
				"request_header_modifier": schema.ListNestedBlock{
					Description: "Headers to change on the request, for `type = \"RequestHeaderModifier\"`.",
					Validators:  []validator.List{listvalidator.SizeAtMost(1)},
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"remove": schema.ListAttribute{
								ElementType: types.StringType,
								Optional:    true,
								Description: "Names of headers removed from the request.",
							},
						},
						Blocks: map[string]schema.Block{
							"add": gatewayHeaderBlock("Headers appended to the request."),
							"set": gatewayHeaderBlock("Headers set on the request, replacing any value it carries."),
						},
					},
				},
				"request_redirect": schema.ListNestedBlock{
					Description: "Redirect answered instead of forwarding the request, for `type = \"RequestRedirect\"`.",
					Validators:  []validator.List{listvalidator.SizeAtMost(1)},
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"hostname": schema.StringAttribute{
								Optional:    true,
								Description: "Hostname the request is redirected to. Leave unset to keep the request's.",
							},
							"port": schema.Int64Attribute{
								Optional:    true,
								Description: "Port the request is redirected to. Leave unset to keep the request's.",
							},
							"scheme": schema.StringAttribute{
								Optional:    true,
								Description: "Scheme the request is redirected to, `http` or `https`.",
							},
							"status_code": schema.Int64Attribute{
								Optional:    true,
								Description: "HTTP status code of the redirect, `301` or `302`.",
							},
						},
					},
				},
			},
		},
	}
}

func (r *kubernetesGatewayHTTPRouteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the HTTPRoute, `" + gatewayAPIIDFormat + "`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment where the HTTPRoute is managed.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"namespace": schema.StringAttribute{
				Required:      true,
				Description:   "Kubernetes namespace in which the HTTPRoute is created.",
				PlanModifiers: replace,
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the HTTPRoute.",
				PlanModifiers: replace,
			},
			"hostnames": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Hostnames the route matches against the request's Host header. Leave unset to match every hostname the parent listener accepts.",
			},
			"resource_version": schema.StringAttribute{
				Computed:    true,
				Description: "Kubernetes resource version of the HTTPRoute, sent back on update so a concurrent change is rejected instead of overwritten.",
			},
		},
		Blocks: map[string]schema.Block{
			"parent_ref": schema.ListNestedBlock{
				Description: "Gateways (or listeners of them) the route attaches to.",
				Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the parent Gateway.",
						},
						"namespace": schema.StringAttribute{
							Optional:    true,
							Description: "Namespace of the parent Gateway. Leave unset for the route's own namespace.",
						},
						"section_name": schema.StringAttribute{
							Optional:    true,
							Description: "Name of the Gateway listener to attach to. Leave unset to attach to every listener that allows the route.",
						},
						"port": schema.Int64Attribute{
							Optional:    true,
							Description: "Port of the Gateway listener to attach to. Leave unset to attach regardless of port.",
						},
						"group": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "API group of the parent. Kubernetes defaults it to `gateway.networking.k8s.io`.",
						},
					},
				},
			},
			"rule": schema.ListNestedBlock{
				Description: "Routing rules: which requests match and which backends they are forwarded to.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional:    true,
							Description: "Name of the rule.",
						},
					},
					Blocks: map[string]schema.Block{
						// Optional and computed in the SDKv2 version; a block
						// cannot be computed in the framework, so Read leaves out
						// the single default match Kubernetes adds to a rule
						// configured without one (see gatewayHTTPRouteMatches).
						"match": schema.ListNestedBlock{
							Description: "Conditions a request has to meet for the rule to apply; a request matching any one of them matches. Kubernetes defaults an empty list to a single `PathPrefix` match on `/`.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"path_type": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "How `path_value` is matched: `Exact`, `PathPrefix` or `RegularExpression`. Kubernetes defaults it to `PathPrefix`.",
									},
									"path_value": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Path to match. Kubernetes defaults it to `/`.",
									},
									"method": schema.StringAttribute{
										Optional:    true,
										Description: "HTTP method to match, for example `GET`. Leave unset to match every method.",
									},
								},
								Blocks: map[string]schema.Block{
									"header":      gatewayTypedMatchBlock("header", "Request headers that must match."),
									"query_param": gatewayTypedMatchBlock("query parameter", "Query parameters that must match."),
								},
							},
						},
						"filter": gatewayHTTPFilterBlock("Filters applied to every request the rule matches."),
						"backend_ref": schema.ListNestedBlock{
							Description: "Backends matching requests are forwarded to.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:    true,
										Description: "Name of the backend, usually a Service.",
									},
									"namespace": schema.StringAttribute{
										Optional:    true,
										Description: "Namespace of the backend. Leave unset for the route's own namespace; another namespace needs a ReferenceGrant there.",
									},
									"port": schema.Int64Attribute{
										Optional:    true,
										Description: "Port of the backend Service traffic is sent to.",
									},
									"kind": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Kind of the backend. Kubernetes defaults it to `Service`.",
									},
									"group": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "API group of the backend. Kubernetes defaults it to the core group (empty).",
									},
									"weight": schema.Int64Attribute{
										Optional:      true,
										Computed:      true,
										Default:       int64default.StaticInt64(1),
										PlanModifiers: []planmodifier.Int64{k8sfwKeepNull{}},
										Description:   "Share of the rule's traffic this backend receives, relative to the other backends. `0` sends it nothing. Defaults to `1`, the Kubernetes default.",
									},
								},
								Blocks: map[string]schema.Block{
									"filter": gatewayHTTPFilterBlock("Filters applied only to requests forwarded to this backend."),
								},
							},
						},
					},
				},
			},
		},
	}
}

func gatewayMap(raw interface{}) map[string]interface{} {
	m, _ := raw.(map[string]interface{})
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

func gatewayStr(m map[string]interface{}, k string) string {
	s, _ := m[k].(string)
	return s
}

func gatewayInt(m map[string]interface{}, k string) int {
	n, _ := m[k].(int)
	return n
}

func gatewayList(m map[string]interface{}, k string) []interface{} {
	l, _ := m[k].([]interface{})
	return l
}

func expandGatewayTypedMatches(raw []interface{}) []k8sGatewayTypedMatch {
	out := make([]k8sGatewayTypedMatch, 0, len(raw))
	for _, item := range raw {
		m := gatewayMap(item)
		out = append(out, k8sGatewayTypedMatch{
			Name:  gatewayStr(m, "name"),
			Type:  gatewayStr(m, "type"),
			Value: gatewayStr(m, "value"),
		})
	}
	return out
}

func expandGatewayHeaders(raw []interface{}) []k8sGatewayHTTPHeader {
	out := make([]k8sGatewayHTTPHeader, 0, len(raw))
	for _, item := range raw {
		m := gatewayMap(item)
		out = append(out, k8sGatewayHTTPHeader{Name: gatewayStr(m, "name"), Value: gatewayStr(m, "value")})
	}
	return out
}

func expandGatewayHTTPFilters(raw []interface{}) []k8sGatewayHTTPFilter {
	out := make([]k8sGatewayHTTPFilter, 0, len(raw))
	for _, item := range raw {
		m := gatewayMap(item)
		f := k8sGatewayHTTPFilter{Type: gatewayStr(m, "type")}
		if l := gatewayList(m, "request_header_modifier"); len(l) > 0 && l[0] != nil {
			hm := gatewayMap(l[0])
			f.RequestHeaderModifier = &k8sGatewayHTTPHeaderFilter{
				Add:    expandGatewayHeaders(gatewayList(hm, "add")),
				Set:    expandGatewayHeaders(gatewayList(hm, "set")),
				Remove: k8sfwStrings(hm["remove"]),
			}
		}
		if l := gatewayList(m, "request_redirect"); len(l) > 0 && l[0] != nil {
			rm := gatewayMap(l[0])
			f.RequestRedirect = &k8sGatewayHTTPRedirect{
				Hostname:   gatewayStr(rm, "hostname"),
				Port:       gatewayInt(rm, "port"),
				Scheme:     gatewayStr(rm, "scheme"),
				StatusCode: gatewayInt(rm, "status_code"),
			}
		}
		out = append(out, f)
	}
	return out
}

func buildKubernetesGatewayHTTPRoute(data map[string]interface{}) k8sGatewayHTTPRoute {
	route := k8sGatewayHTTPRoute{
		Name:            gatewayStr(data, "name"),
		Namespace:       gatewayStr(data, "namespace"),
		Hostnames:       k8sfwStrings(data["hostnames"]),
		ParentRefs:      []k8sGatewayParentRef{},
		Rules:           []k8sGatewayHTTPRouteRule{},
		ResourceVersion: gatewayStr(data, "resource_version"),
	}
	for _, item := range gatewayList(data, "parent_ref") {
		m := gatewayMap(item)
		route.ParentRefs = append(route.ParentRefs, k8sGatewayParentRef{
			Group:       gatewayStr(m, "group"),
			Name:        gatewayStr(m, "name"),
			Namespace:   gatewayStr(m, "namespace"),
			Port:        gatewayInt(m, "port"),
			SectionName: gatewayStr(m, "section_name"),
		})
	}
	for _, item := range gatewayList(data, "rule") {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rule := k8sGatewayHTTPRouteRule{
			Name:        gatewayStr(m, "name"),
			Matches:     []k8sGatewayHTTPRouteMatch{},
			Filters:     expandGatewayHTTPFilters(gatewayList(m, "filter")),
			BackendRefs: []k8sGatewayHTTPBackendRef{},
		}
		for _, rawMatch := range gatewayList(m, "match") {
			// An empty match {} block comes back as nil: it matches
			// everything, which an empty match object says too.
			mm, ok := rawMatch.(map[string]interface{})
			if !ok {
				rule.Matches = append(rule.Matches, k8sGatewayHTTPRouteMatch{})
				continue
			}
			match := k8sGatewayHTTPRouteMatch{
				Method:      gatewayStr(mm, "method"),
				Headers:     expandGatewayTypedMatches(gatewayList(mm, "header")),
				QueryParams: expandGatewayTypedMatches(gatewayList(mm, "query_param")),
			}
			if pt, pv := gatewayStr(mm, "path_type"), gatewayStr(mm, "path_value"); pt != "" || pv != "" {
				match.Path = &k8sGatewayTypedMatch{Type: pt, Value: pv}
			}
			rule.Matches = append(rule.Matches, match)
		}
		for _, rawBackend := range gatewayList(m, "backend_ref") {
			bm, ok := rawBackend.(map[string]interface{})
			if !ok {
				continue
			}
			backend := k8sGatewayHTTPBackendRef{
				BackendRef: k8sGatewayBackendRef{
					BackendObjectReference: k8sGatewayBackendObjectRef{
						Group:     gatewayStr(bm, "group"),
						Kind:      gatewayStr(bm, "kind"),
						Name:      gatewayStr(bm, "name"),
						Namespace: gatewayStr(bm, "namespace"),
						Port:      gatewayInt(bm, "port"),
					},
					// Always sent: zero is a valid weight (the backend
					// receives nothing), so it cannot stand for unset.
					Weight: gatewayInt(bm, "weight"),
				},
				Filters: expandGatewayHTTPFilters(gatewayList(bm, "filter")),
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

// gatewayHTTPRouteDefaultMatch reports whether matches is the single match
// Kubernetes gives a rule configured without one: PathPrefix on /.
func gatewayHTTPRouteDefaultMatch(matches []k8sGatewayHTTPRouteMatch) bool {
	if len(matches) != 1 {
		return false
	}
	m := matches[0]
	return m.Path != nil && m.Path.Type == "PathPrefix" && m.Path.Value == "/" && m.Path.Name == "" &&
		m.Method == "" && len(m.Headers) == 0 && len(m.QueryParams) == 0
}

// flattenGatewayHTTPRouteRules flattens the rules. omitDefaultMatch(i) says
// whether rule i was configured without a match block, whose single default
// match is then left out: the SDKv2 version kept it in state as a computed
// block, which the framework has no way to declare.
func flattenGatewayHTTPRouteRules(rules []k8sGatewayHTTPRouteRule, omitDefaultMatch func(int) bool) []interface{} {
	out := make([]interface{}, 0, len(rules))
	for i, r := range rules {
		matches := make([]interface{}, 0, len(r.Matches))
		if !gatewayHTTPRouteDefaultMatch(r.Matches) || !omitDefaultMatch(i) {
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

// read refreshes data from the route, reporting false when it is gone.
// frameworkState says whether the prior state was written by this version
// (k8sfwStateKey): there a rule without match blocks in state was configured
// without one. In a state written by SDKv2 (v3.0.0 and before) every rule
// carries the matches Kubernetes reported, defaults included, and a rule
// with only the default match is taken as configured without one, the usual
// way to write it.
func (r *kubernetesGatewayHTTPRouteResource) read(ctx context.Context, data map[string]interface{}, frameworkState bool) (bool, error) {
	id, _ := data["id"].(string)
	envID, namespace, name, err := parseGatewayAPIID(id)
	if err != nil {
		return false, err
	}

	var routes []k8sGatewayHTTPRoute
	if err := doJSON(ctx, r.client, http.MethodGet, gatewayHTTPRoutesURL(r.client, envID, namespace), nil, &routes); err != nil {
		// The environment itself is gone.
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read HTTPRoute %s/%s: %w", namespace, name, err)
	}

	var route *k8sGatewayHTTPRoute
	for i := range routes {
		if routes[i].Name == name {
			route = &routes[i]
			break
		}
	}
	if route == nil {
		return false, nil
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

	priorRules := gatewayList(data, "rule")
	omitDefaultMatch := func(i int) bool {
		if !frameworkState {
			return true
		}
		if i >= len(priorRules) {
			return true
		}
		return len(gatewayList(gatewayMap(priorRules[i]), "match")) == 0
	}

	data["environment_id"] = envID
	data["namespace"] = namespace
	data["name"] = name
	data["hostnames"] = route.Hostnames
	data["parent_ref"] = parents
	data["rule"] = flattenGatewayHTTPRouteRules(route.Rules, omitDefaultMatch)
	data["resource_version"] = route.ResourceVersion
	return true, nil
}

func (r *kubernetesGatewayHTTPRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	envID, _ := data["environment_id"].(int)
	route := buildKubernetesGatewayHTTPRoute(data)
	route.ResourceVersion = ""

	if err := doJSON(ctx, r.client, http.MethodPost, gatewayHTTPRoutesURL(r.client, envID, route.Namespace), route, nil); err != nil {
		err = explainUnsupported(ctx, r.client, err, "portainer_kubernetes_gateway_http_route", requiresBE30)
		resp.Diagnostics.AddError("Failed to create the HTTPRoute", fmt.Errorf("failed to create HTTPRoute %s/%s: %w", route.Namespace, route.Name, err).Error())
		return
	}

	data["id"] = fmt.Sprintf("%d:%s:%s", envID, route.Namespace, route.Name)
	found, err := r.read(ctx, data, true)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the HTTPRoute", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read the HTTPRoute", fmt.Sprintf("HTTPRoute %s/%s not found after it was created", route.Namespace, route.Name))
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesGatewayHTTPRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.read(ctx, data, !fromSDK)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the HTTPRoute", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesGatewayHTTPRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	before, _ := k8sfwData(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := before["id"].(string)
	envID, namespace, name, err := parseGatewayAPIID(id)
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}
	// The resource version is the one last read, so a concurrent change is
	// rejected instead of overwritten.
	data["id"] = id
	data["resource_version"] = before["resource_version"]

	if err := doJSON(ctx, r.client, http.MethodPut, gatewayHTTPRoutesURL(r.client, envID, namespace), buildKubernetesGatewayHTTPRoute(data), nil); err != nil {
		resp.Diagnostics.AddError("Failed to update the HTTPRoute", fmt.Errorf("failed to update HTTPRoute %s/%s: %w", namespace, name, err).Error())
		return
	}
	found, err := r.read(ctx, data, true)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the HTTPRoute", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read the HTTPRoute", fmt.Sprintf("HTTPRoute %s/%s not found after it was updated", namespace, name))
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete removes the route through the bulk delete, which takes the names to
// delete per namespace.
func (r *kubernetesGatewayHTTPRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID, namespace, name, err := parseGatewayAPIID(id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	body := map[string][]string{namespace: {name}}
	url := fmt.Sprintf("%s/kubernetes/%d/gatewayhttproutes/delete", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, body, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete the HTTPRoute", fmt.Errorf("failed to delete HTTPRoute %s/%s: %w", namespace, name, err).Error())
	}
}

// ImportState takes "<environment_id>:<namespace>:<name>".
func (r *kubernetesGatewayHTTPRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
