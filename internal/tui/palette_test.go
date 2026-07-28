package tui

import "testing"

func TestMatchPaletteEmptyInput(t *testing.T) {
	if matches := matchPalette("", nil); matches != nil {
		t.Errorf("expected nil for empty input, got %v", matches)
	}
}

func TestMatchPalettePrefixMatch(t *testing.T) {
	matches := matchPalette("dep", nil)
	if len(matches) != 1 || matches[0].resource != ResourceDeployments {
		t.Errorf("expected single Deployments match, got %v", matches)
	}
}

func TestMatchPaletteDedupesAliasesForSameResource(t *testing.T) {
	matches := matchPalette("pod", nil)
	if len(matches) != 1 || matches[0].resource != ResourcePods {
		t.Errorf("expected single deduped Pods match, got %v", matches)
	}
}

func TestMatchPaletteNoMatch(t *testing.T) {
	if matches := matchPalette("xyz", nil); matches != nil {
		t.Errorf("expected nil for unmatched input, got %v", matches)
	}
}

func TestMatchPaletteWithAddonAlias(t *testing.T) {
	extra := []addonAlias{
		{alias: "cpol", match: commandMatch{label: "Cl.Policies", resource: resourceAddonBase}},
		{alias: "clusterpolicies", match: commandMatch{label: "Cl.Policies", resource: resourceAddonBase}},
	}

	matches := matchPalette("cpol", extra)
	if len(matches) != 1 || matches[0].label != "Cl.Policies" || matches[0].resource != resourceAddonBase {
		t.Errorf("expected single Cl.Policies match, got %v", matches)
	}
}

func TestMatchPaletteAddonAliasNoMatch(t *testing.T) {
	extra := []addonAlias{
		{alias: "cpol", match: commandMatch{label: "Cl.Policies", resource: resourceAddonBase}},
	}
	if matches := matchPalette("xyz", extra); matches != nil {
		t.Errorf("expected nil for unmatched input, got %v", matches)
	}
}

func TestResourceTypeString(t *testing.T) {
	cases := map[ResourceType]string{
		ResourcePods:         "Pods",
		ResourceDeployments:  "Deployments",
		ResourceServices:     "Services",
		ResourceConfigMaps:   "ConfigMaps",
		ResourceSecrets:      "Secrets",
		ResourceNamespaces:   "Namespaces",
		ResourceNodes:        "Nodes",
		ResourceStatefulSets: "StatefulSets",
		ResourceDaemonSets:   "DaemonSets",
		ResourceEvents:       "Events",
	}
	for rt, want := range cases {
		if got := rt.String(); got != want {
			t.Errorf("ResourceType(%d).String() = %q, want %q", rt, got, want)
		}
	}
}
