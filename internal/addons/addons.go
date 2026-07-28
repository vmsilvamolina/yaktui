// Package addons defines CRD-backed resource addons (e.g. Kyverno policies)
// that can be auto-detected on a cluster or enabled via config file, without
// requiring code changes to internal/tui for each new addon.
package addons

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Column describes one table column rendered from an addon item.
type Column struct {
	Header string `json:"header"`
	Path   string `json:"path"`
}

// Resource identifies one CRD kind belonging to a Definition. A Definition
// with multiple Resources (e.g. Kyverno's ClusterPolicy + Policy) renders as
// a single sidebar entry showing rows from every listed kind.
type Resource struct {
	Kind          string `json:"kind"`
	Resource      string `json:"resource"`
	ClusterScoped bool   `json:"clusterScoped"`
}

// Definition describes a CRD-backed addon, possibly spanning more than one
// CRD kind sharing the same API group/version (e.g. cluster- and
// namespace-scoped variants of the same tool).
type Definition struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"displayName"`
	Enabled     string     `json:"enabled"`
	Group       string     `json:"group"`
	Version     string     `json:"version"`
	Resources   []Resource `json:"resources"`
	Aliases     []string   `json:"aliases"`
	Columns     []Column   `json:"columns"`
}

// GVR returns the GroupVersionResource identifying r within d.
func (d Definition) GVR(r Resource) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: d.Group, Version: d.Version, Resource: r.Resource}
}

// HasNamespacedResource reports whether at least one of d's Resources is
// namespace-scoped (relevant for the all-namespaces toggle and the
// NAMESPACE column).
func (d Definition) HasNamespacedResource() bool {
	for _, r := range d.Resources {
		if !r.ClusterScoped {
			return true
		}
	}
	return false
}

// Builtins is the set of addon definitions shipped with yaktui by default.
var Builtins = []Definition{
	{
		Name:        "kyverno",
		DisplayName: "Kyverno",
		Enabled:     "auto",
		Group:       "kyverno.io",
		Version:     "v1",
		Resources: []Resource{
			{Kind: "ClusterPolicy", Resource: "clusterpolicies", ClusterScoped: true},
			{Kind: "Policy", Resource: "policies", ClusterScoped: false},
		},
		Aliases: []string{"kyverno", "clusterpolicy", "clusterpolicies", "cpol", "policy", "policies", "pol"},
		Columns: []Column{
			{Header: "NAME", Path: "metadata.name"},
			{Header: "BACKGROUND", Path: "spec.background"},
			{Header: "VALIDATE ACTION", Path: "spec.validationFailureAction"},
			{Header: "AGE", Path: "metadata.creationTimestamp"},
		},
	},
	{
		Name:        "cert-manager",
		DisplayName: "Cert-Manager",
		Enabled:     "auto",
		Group:       "cert-manager.io",
		Version:     "v1",
		Resources: []Resource{
			{Kind: "Certificate", Resource: "certificates", ClusterScoped: false},
			{Kind: "Issuer", Resource: "issuers", ClusterScoped: false},
			{Kind: "ClusterIssuer", Resource: "clusterissuers", ClusterScoped: true},
		},
		Aliases: []string{"cert-manager", "certmanager", "cert", "certificate", "certificates", "issuer", "issuers", "clusterissuer", "clusterissuers"},
		Columns: []Column{
			{Header: "NAME", Path: "metadata.name"},
			{Header: "SECRET", Path: "spec.secretName"},
			{Header: "AGE", Path: "metadata.creationTimestamp"},
		},
	},
	{
		Name:        "argocd",
		DisplayName: "ArgoCD",
		Enabled:     "auto",
		Group:       "argoproj.io",
		Version:     "v1alpha1",
		Resources: []Resource{
			{Kind: "Application", Resource: "applications", ClusterScoped: false},
			{Kind: "AppProject", Resource: "appprojects", ClusterScoped: false},
		},
		Aliases: []string{"argocd", "argo", "application", "applications", "app", "appproject", "appprojects"},
		Columns: []Column{
			{Header: "NAME", Path: "metadata.name"},
			{Header: "SYNC", Path: "status.sync.status"},
			{Header: "HEALTH", Path: "status.health.status"},
			{Header: "AGE", Path: "metadata.creationTimestamp"},
		},
	},
}

// Merge overlays overrides onto base by Name: an override replaces a base
// entry with the same Name, otherwise it is appended as a new definition.
func Merge(base, overrides []Definition) []Definition {
	result := make([]Definition, len(base))
	copy(result, base)

	index := make(map[string]int, len(result))
	for i, d := range result {
		index[d.Name] = i
	}

	for _, o := range overrides {
		if i, ok := index[o.Name]; ok {
			result[i] = o
		} else {
			index[o.Name] = len(result)
			result = append(result, o)
		}
	}

	return result
}

// ActivateDefinitions filters defs down to the ones that should be active,
// based on each definition's Enabled field and the set of API groups
// detected on the cluster.
func ActivateDefinitions(defs []Definition, detectedGroups map[string]bool) []Definition {
	var active []Definition
	for _, d := range defs {
		switch d.Enabled {
		case "false":
			continue
		case "true":
			active = append(active, d)
		default: // "auto" or ""
			if detectedGroups[d.Group] {
				active = append(active, d)
			}
		}
	}
	return active
}

// ExtractColumn resolves a dot-separated path (e.g. "spec.background") into
// obj, returning its string representation, or "" if the path is missing.
func ExtractColumn(obj map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	var current interface{} = obj

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}

	if current == nil {
		return ""
	}
	if s, ok := current.(string); ok {
		return s
	}
	return fmt.Sprint(current)
}
