package tui

import "testing"

func TestMatchPaletteEmptyInput(t *testing.T) {
	if matches := matchPalette(""); matches != nil {
		t.Errorf("expected nil for empty input, got %v", matches)
	}
}

func TestMatchPalettePrefixMatch(t *testing.T) {
	matches := matchPalette("dep")
	if len(matches) != 1 || matches[0].resource != ResourceDeployments {
		t.Errorf("expected single Deployments match, got %v", matches)
	}
}

func TestMatchPaletteDedupesAliasesForSameResource(t *testing.T) {
	matches := matchPalette("pod")
	if len(matches) != 1 || matches[0].resource != ResourcePods {
		t.Errorf("expected single deduped Pods match, got %v", matches)
	}
}

func TestMatchPaletteNoMatch(t *testing.T) {
	if matches := matchPalette("xyz"); matches != nil {
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
